// Package tenant implements multi-tenant project isolation for Sovrabase.
//
// Each project gets its own Pebble database instance, JWT secret, storage
// namespace, and replication group. The ProjectManager stores project metadata
// in a master Pebble database and creates/destroys isolated environments.
package tenant

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/storage"
)

// teamStoreCache holds the lazily-created TeamStore for the master DB.
var teamStoreOnce sync.Once
var masterTeamStore *TeamStore

// Project represents an isolated Sovrabase project (tenant).
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OwnerID      string    `json:"owner_id"`
	JWTSecret    string    `json:"jwt_secret"`
	DataDir      string    `json:"data_dir"`
	StorageDir   string    `json:"storage_dir"`
	CreatedAt    time.Time `json:"created_at"`
	Status       string    `json:"status"` // "active", "suspended", "deleted"
	ReplGroup    string    `json:"repl_group"` // replication group identifier
	StorageQuota int64  `json:"storage_quota"`   // in bytes
	AllowOrigins string `json:"allow_origins"` // comma-separated allowed origins for CORS
	OAuthProviders []auth.OAuthProviderConfig `json:"oauth_providers"`
	Integrations []ProjectIntegration `json:"integrations"`
}

// ProjectIntegration holds the configuration for a single integration enabled on a project.
type ProjectIntegration struct {
	ID     string                 `json:"id"`
	Config map[string]interface{} `json:"config"`
}

// ProjectEnv holds project-specific database, auth service, and storage driver.
type ProjectEnv struct {
	Engine  *db.Engine
	Auth    *auth.AuthService
	Storage storage.Driver
}

// ProjectManager manages all projects in the system.
type ProjectManager struct {
	mu          sync.RWMutex
	db          *pebble.DB
	baseDir     string
	cfg         *config.Config
	projects    map[string]*Project     // in-memory cache
	envs        map[string]*ProjectEnv // project ID -> active environment cache
	teamStore   *TeamStore             // lazily initialized
	secretIndex map[string]*Project    // JWT secret -> project (O(1) lookup)
}

// NewProjectManager creates a new project manager backed by the master database
// stored at {baseDir}/_master.
func NewProjectManager(baseDir string, cfg *config.Config) (*ProjectManager, error) {
	masterDir := filepath.Join(baseDir, "_master")
	if err := os.MkdirAll(masterDir, 0755); err != nil {
		return nil, fmt.Errorf("tenant: create master dir: %w", err)
	}

	db, err := pebble.Open(masterDir, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("tenant: open master db: %w", err)
	}

	pm := &ProjectManager{
		db:          db,
		baseDir:     baseDir,
		cfg:         cfg,
		projects:    make(map[string]*Project),
		envs:        make(map[string]*ProjectEnv),
		secretIndex: make(map[string]*Project),
	}

	// Load existing projects into cache
	if err := pm.loadAll(); err != nil {
		db.Close()
		return nil, fmt.Errorf("tenant: load projects: %w", err)
	}

	return pm, nil
}

// Close shuts down the project manager's database and closes all project databases.
func (pm *ProjectManager) Close() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, env := range pm.envs {
		_ = env.Engine.Close()
	}
	return pm.db.Close()
}

// GetTeamStore returns (and lazily initializes) the TeamStore backed by the
// master Pebble database.
func (pm *ProjectManager) GetTeamStore() *TeamStore {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.teamStore == nil {
		pm.teamStore = NewTeamStore(pm.db)
	}
	return pm.teamStore
}

// CreateProject creates a new isolated project.
func (pm *ProjectManager) CreateProject(name, ownerID string) (*Project, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check for duplicate names
	for _, p := range pm.projects {
		if p.Name == name && p.Status != "deleted" {
			return nil, fmt.Errorf("tenant: project %q already exists", name)
		}
	}

	id := uuid.New().String()
	jwtSecret := pm.cfg.JWTSecret
	if jwtSecret == "" {
		jwtSecret = "change-me-in-production"
	}

	projectDir := filepath.Join(pm.baseDir, "projects", id)
	proj := &Project{
		ID:           id,
		Name:         name,
		OwnerID:      ownerID,
		JWTSecret:    jwtSecret,
		DataDir:      filepath.Join(projectDir, "db"),
		StorageDir:   filepath.Join(projectDir, "storage"),
		CreatedAt:    time.Now().UTC(),
		Status:       "active",
		ReplGroup:    "default",
		StorageQuota: 100 * 1024 * 1024, // 100MB default quota
		AllowOrigins: "*",               // allow all origins by default
	}

	// Create project directories
	os.MkdirAll(proj.DataDir, 0755)
	os.MkdirAll(proj.StorageDir, 0755)

	// Persist to master DB
	if err := pm.saveProject(proj); err != nil {
		return nil, err
	}

	// Auto-add the project owner to global member-project index.
	// The actual __members insertion happens in handleCreateProject (admin.go)
	// since the engine is lazily created.
	ts := NewTeamStore(pm.db)
	if err := ts.AddMemberProjectIndex(ownerID, id); err != nil {
		return nil, fmt.Errorf("tenant: add owner to index: %w", err)
	}

	pm.projects[id] = proj
	pm.secretIndex[jwtSecret] = proj
	return proj, nil
}

// GetProject retrieves a project by ID.
func (pm *ProjectManager) GetProject(id string) (*Project, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proj, ok := pm.projects[id]
	if !ok {
		return nil, fmt.Errorf("tenant: project %q not found", id)
	}
	return proj, nil
}

// GetProjectBySecret retrieves a project by its JWT secret (API key).
// Uses an in-memory secret index for O(1) lookup.
func (pm *ProjectManager) GetProjectBySecret(secret string) (*Project, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	proj, ok := pm.secretIndex[secret]
	if !ok {
		return nil, fmt.Errorf("tenant: invalid project secret")
	}
	return proj, nil
}

// ListProjects returns all active projects.
func (pm *ProjectManager) ListProjects() []*Project {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Project
	for _, p := range pm.projects {
		if p.Status == "active" {
			result = append(result, p)
		}
	}
	return result
}

// UpdateProject saves changes to an existing project.
func (pm *ProjectManager) UpdateProject(proj *Project) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	existing, ok := pm.projects[proj.ID]
	if !ok {
		return fmt.Errorf("tenant: project %q not found", proj.ID)
	}
	if existing.Status == "deleted" {
		return fmt.Errorf("tenant: project %q is deleted", proj.ID)
	}

	pm.projects[proj.ID] = proj
	return pm.saveProject(proj)
}

// DeleteProject marks a project as deleted and removes its data.

// ReloadProjectEnv closes and removes the cached env for a project, forcing
// recreation on the next GetProjectEnv call (e.g. after OAuth config changes).
func (pm *ProjectManager) ReloadProjectEnv(id string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if env, ok := pm.envs[id]; ok {
		_ = env.Engine.Close()
		delete(pm.envs, id)
	}
}

// DeleteProject marks a project as deleted and removes its data.
func (pm *ProjectManager) DeleteProject(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	proj, ok := pm.projects[id]
	if !ok {
		return fmt.Errorf("tenant: project %q not found", id)
	}

	proj.Status = "deleted"
	if err := pm.saveProject(proj); err != nil {
		return err
	}

	// Close database engine and remove from cache if open
	if env, ok := pm.envs[id]; ok {
		_ = env.Engine.Close()
		delete(pm.envs, id)
	}

	// Remove project data directory
	projectDir := filepath.Join(pm.baseDir, "projects", id)
	os.RemoveAll(projectDir)

	delete(pm.projects, id)
	delete(pm.secretIndex, proj.JWTSecret)
	// Remove from master DB
	key := projectDBKey(id)
	return pm.db.Delete(key, pebble.Sync)
}

// ProjectCount returns the number of active projects.
func (pm *ProjectManager) ProjectCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	count := 0
	for _, p := range pm.projects {
		if p.Status == "active" {
			count++
		}
	}
	return count
}

// Backup creates Pebble checkpoints of the master database and all active
// project databases into the given destination directory.
func (pm *ProjectManager) Backup(destDir string) error {
	// Snapshot the list of active projects under read lock.
	pm.mu.RLock()
	var activeIDs []string
	for _, p := range pm.projects {
		if p.Status == "active" {
			activeIDs = append(activeIDs, p.ID)
		}
	}
	pm.mu.RUnlock()

	// Create the destination directory.
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("tenant: create backup dir %q: %w", destDir, err)
	}

	// Create a checkpoint of the master database.
	masterCp := filepath.Join(destDir, "_master")
	if err := pm.db.Checkpoint(masterCp); err != nil {
		return fmt.Errorf("tenant: master checkpoint: %w", err)
	}

	// Create checkpoints for each active project.
	for _, id := range activeIDs {
		env, err := pm.GetProjectEnv(id)
		if err != nil {
			continue // skip projects that fail to load
		}
		projCp := filepath.Join(destDir, id)
		if err := env.Engine.DB().Checkpoint(projCp); err != nil {
			return fmt.Errorf("tenant: project %q checkpoint: %w", id, err)
		}
	}

	return nil
}

// saveProject persists a project to the master database.
func (pm *ProjectManager) saveProject(proj *Project) error {
	data, err := json.Marshal(proj)
	if err != nil {
		return fmt.Errorf("tenant: marshal project: %w", err)
	}
	return pm.db.Set(projectDBKey(proj.ID), data, pebble.Sync)
}

// loadAll loads all projects from the master database into memory.
func (pm *ProjectManager) loadAll() error {
	prefix := []byte("project:")
	iter, err := pm.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var proj Project
		if err := json.Unmarshal(iter.Value(), &proj); err != nil {
			continue
		}
		if proj.JWTSecret == "" {
			proj.JWTSecret = pm.cfg.JWTSecret
			if proj.JWTSecret == "" {
				proj.JWTSecret = "change-me-in-production"
			}
			// Save it back to master DB
			data, err := json.Marshal(&proj)
			if err == nil {
				_ = pm.db.Set(projectDBKey(proj.ID), data, pebble.Sync)
			}
		}

		pm.projects[proj.ID] = &proj
		// Build secret index for O(1) lookups.
		if proj.JWTSecret != "" {
			pm.secretIndex[proj.JWTSecret] = &proj
		}
	}
	return iter.Error()
}

// GetProjectEnv returns (and opens if not cached) the project's isolated environment.
func (pm *ProjectManager) GetProjectEnv(id string) (*ProjectEnv, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 1. Check cache
	if env, ok := pm.envs[id]; ok {
		return env, nil
	}

	// 2. Fetch project
	proj, ok := pm.projects[id]
	if !ok || proj.Status == "deleted" {
		return nil, fmt.Errorf("tenant: project %q not found", id)
	}

	// 3. Initialize db engine
	engine, err := db.NewEngine(proj.DataDir)
	if err != nil {
		return nil, fmt.Errorf("tenant: failed to initialize project db: %w", err)
	}

	// 4. Initialize auth service
	userStore := auth.NewDBUserStore(engine)
	authSvc := auth.NewService(proj.JWTSecret, userStore)
	// Use central config if available, otherwise fall back to env vars.
	if pm.cfg != nil {
		authSvc.EmailVerificationEnabled = pm.cfg.EmailVerification
		authSvc.SMTPHost = pm.cfg.SMTPHost
		authSvc.SMTPPort = pm.cfg.SMTPPort
		authSvc.SMTPUser = pm.cfg.SMTPUser
		authSvc.SMTPPassword = pm.cfg.SMTPPassword
		authSvc.SMTPSender = pm.cfg.SMTPSender
	} else {
		// Legacy fallback for tests passing nil config.
		authSvc.EmailVerificationEnabled = os.Getenv("SOVRABASE_EMAIL_VERIFICATION") == "true"
		authSvc.SMTPHost = os.Getenv("SOVRABASE_SMTP_HOST")
		if portVal, err := strconv.Atoi(os.Getenv("SOVRABASE_SMTP_PORT")); err == nil {
			authSvc.SMTPPort = portVal
		}
		authSvc.SMTPUser = os.Getenv("SOVRABASE_SMTP_USER")
		authSvc.SMTPPassword = os.Getenv("SOVRABASE_SMTP_PASSWORD")
		authSvc.SMTPSender = os.Getenv("SOVRABASE_SMTP_SENDER")
	}

	// Register per-project OAuth providers
	for _, pCfg := range proj.OAuthProviders {
		provider, err := auth.NewGenericOAuthProvider(pCfg)
		if err != nil {
			slog.Warn("Failed to create OAuth provider for project", "project_id", id, "provider", pCfg.Name, "error", err)
			continue
		}
		authSvc.RegisterOAuthProvider(pCfg.Name, provider)
	}

	// 5. Initialize storage driver
	var storageDriver storage.Driver
	s3Enabled := false
	if pm.cfg != nil {
		s3Enabled = pm.cfg.S3Enabled && pm.cfg.S3AccessKey != ""
	} else {
		s3Enabled = os.Getenv("S3_ACCESS_KEY") != ""
	}
	if s3Enabled {
		s3Svc, err := storage.NewS3DriverFromEnv()
		if err != nil {
			engine.Close()
			return nil, fmt.Errorf("tenant: failed to initialize project S3 driver: %w", err)
		}
		storageDriver = s3Svc
	} else {
		localSvc, err := storage.NewLocalDriver(proj.StorageDir, "")
		if err != nil {
			engine.Close()
			return nil, fmt.Errorf("tenant: failed to initialize project local storage driver: %w", err)
		}
		storageDriver = localSvc
	}

	env := &ProjectEnv{
		Engine:  engine,
		Auth:    authSvc,
		Storage: storageDriver,
	}

	pm.envs[id] = env
	return env, nil
}

func projectDBKey(id string) []byte {
	return []byte("project:" + id)
}

func keyUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return append(prefix, 0x00)
}

func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
