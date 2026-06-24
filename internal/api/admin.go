package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// AdminServer handles administrative (control plane) API routes.
type AdminServer struct {
	projects      *tenant.ProjectManager
	dataDir       string
	cfg           *config.Config
	replStatus    *ReplicationStatus
	jwtSecret     string
	adminEmail    string
	adminPassword string
	// OnRestart is called when the dashboard requests a server restart.
	OnRestart func()
}

// ReplicationStatus holds replication information exposed via admin API.
type ReplicationStatus struct {
	Enabled bool   `json:"enabled"`
	Role    string `json:"role"`
	NodeID  string `json:"node_id"`
	Peers   int    `json:"peers"`
}

// NewAdminServer creates a new admin API handler.
func NewAdminServer(pm *tenant.ProjectManager, cfg *config.Config, jwtSecret, adminEmail, adminPassword string) *AdminServer {
	return &AdminServer{
		projects:      pm,
		dataDir:       cfg.DataDir,
		cfg:           cfg,
		jwtSecret:     jwtSecret,
		adminEmail:    adminEmail,
		adminPassword: adminPassword,
	}
}

// SetReplicationStatus sets the replication info for the stats endpoint.
func (a *AdminServer) SetReplicationStatus(status *ReplicationStatus) {
	a.replStatus = status
}

// authMiddleware protects routes with admin JWT checks.
func (a *AdminServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := ""
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				tokenString = parts[1]
			}
		} else {
			tokenString = r.URL.Query().Get("token")
		}

		if tokenString == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header or token query parameter")
			return
		}

		claims, err := auth.ValidateToken(tokenString, a.jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		if claims.Role != string(auth.RoleAdmin) {
			writeError(w, http.StatusForbidden, "forbidden: admin role required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleLogin handles admin login and issues a JWT token.
func (a *AdminServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email != a.adminEmail || req.Password != a.adminPassword {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	adminUser := &auth.User{
		ID:    "admin",
		Email: a.adminEmail,
		Role:  auth.RoleAdmin,
	}

	tokens, err := auth.GenerateAccessToken(adminUser, a.jwtSecret, a.cfg.SessionDuration)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": tokens,
	})
}

func (a *AdminServer) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/health", a.handleHealth)
	mux.HandleFunc("POST /admin/login", a.handleLogin)

	mux.Handle("GET /admin/projects", a.authMiddleware(http.HandlerFunc(a.handleListProjects)))
	mux.Handle("POST /admin/projects", a.authMiddleware(http.HandlerFunc(a.handleCreateProject)))
	mux.Handle("DELETE /admin/projects/{id}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDeleteProject))))
	mux.Handle("GET /admin/projects/{id}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleGetProject))))
	mux.Handle("GET /admin/stats", a.authMiddleware(http.HandlerFunc(a.handleStats)))

	// Server config & restart
	mux.Handle("GET /admin/config", a.authMiddleware(http.HandlerFunc(a.handleGetConfig)))
	mux.Handle("POST /admin/config", a.authMiddleware(http.HandlerFunc(a.handleSaveConfig)))
	mux.Handle("POST /admin/restart", a.authMiddleware(http.HandlerFunc(a.handleRestart)))

	// Database management endpoints
	mux.Handle("GET /admin/projects/{id}/collections", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleListCollections))))
	mux.Handle("POST /admin/projects/{id}/collections", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleCreateCollection))))
	mux.Handle("DELETE /admin/projects/{id}/collections/{name}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDropCollection))))
	mux.Handle("GET /admin/projects/{id}/collections/{name}/documents", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleListDocuments))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/documents", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleInsertDocument))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/import", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleImportCollection))))
	mux.Handle("PUT /admin/projects/{id}/collections/{name}/documents/{docId}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleUpdateDocument))))
	mux.Handle("DELETE /admin/projects/{id}/collections/{name}/documents/{docId}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDeleteDocument))))
	mux.Handle("GET /admin/projects/{id}/collections/{name}/rules", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleGetRules))))
	mux.Handle("POST /admin/projects/{id}/collections/{name}/rules", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleSetRules))))

	// Auth management endpoints
	mux.Handle("GET /admin/projects/{id}/users", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleListUsers))))
	mux.Handle("POST /admin/projects/{id}/users", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleCreateUser))))
	mux.Handle("DELETE /admin/projects/{id}/users/{userId}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDeleteUser))))

	// Storage management endpoints
	mux.Handle("GET /admin/projects/{id}/storage/buckets", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleListBuckets))))
	mux.Handle("POST /admin/projects/{id}/storage/buckets", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleCreateBucket))))
	mux.Handle("DELETE /admin/projects/{id}/storage/buckets/{bucket}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDeleteBucket))))
	mux.Handle("GET /admin/projects/{id}/storage/buckets/{bucket}/files", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleListFiles))))
	mux.Handle("POST /admin/projects/{id}/storage/buckets/{bucket}/files", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleUploadFile))))
	mux.Handle("GET /admin/projects/{id}/storage/buckets/{bucket}/files/{path...}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDownloadFile))))
	mux.Handle("DELETE /admin/projects/{id}/storage/buckets/{bucket}/files/{path...}", a.authMiddleware(a.projectLogger(http.HandlerFunc(a.handleDeleteFile))))

	// Request logs endpoint
	mux.Handle("GET /admin/projects/{id}/logs", a.authMiddleware(http.HandlerFunc(a.handleListLogs)))
	mux.Handle("DELETE /admin/projects/{id}/logs", a.authMiddleware(http.HandlerFunc(a.handleFlushLogs)))
}

func (a *AdminServer) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		OwnerID string `json:"owner_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	proj, err := a.projects.CreateProject(req.Name, req.OwnerID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"project":    proj,
		"api_key":    proj.JWTSecret,
		"api_url":    r.Host + "/api/v1",
		"project_id": proj.ID,
	})
}

func (a *AdminServer) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, err := a.projects.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"project":    proj,
		"api_key":    proj.JWTSecret,
		"api_url":    r.Host + "/api/v1",
		"project_id": proj.ID,
	})
}

func (a *AdminServer) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects := a.projects.ListProjects()
	if projects == nil {
		projects = []*tenant.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"count":    len(projects),
	})
}

func (a *AdminServer) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.projects.DeleteProject(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleStats(w http.ResponseWriter, r *http.Request) {
	storageUsed := dirSize(a.dataDir)
	count := a.projects.ProjectCount()

	repl := a.replStatus
	if repl == nil {
		repl = &ReplicationStatus{Enabled: false, Role: "standalone"}
	}

	storageDriver := "local"
	var s3Endpoint, s3AccessKey, s3Prefix string
	if os.Getenv("S3_ACCESS_KEY") != "" {
		storageDriver = "s3"
		s3Endpoint = os.Getenv("S3_ENDPOINT")
		if s3Endpoint == "" {
			s3Endpoint = "s3.fr-par.scw.cloud (default)"
		}
		rawAccessKey := os.Getenv("S3_ACCESS_KEY")
		if len(rawAccessKey) > 4 {
			s3AccessKey = rawAccessKey[:4] + "..."
		} else {
			s3AccessKey = "..."
		}
		s3Prefix = os.Getenv("S3_BUCKET_PREFIX")
		if s3Prefix == "" {
			s3Prefix = "sovrabase"
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects":       count,
		"version":        "0.3.0",
		"go_version":     runtime.Version(),
		"region":         "eu-west",
		"providers":      []string{"scaleway", "ovhcloud", "hetzner"},
		"storage_mb":     storageUsed / (1024 * 1024),
		"storage_bytes":  storageUsed,
		"storage_driver": storageDriver,
		"s3_endpoint":    s3Endpoint,
		"s3_access_key":  s3AccessKey,
		"s3_prefix":      s3Prefix,
		"os":             runtime.GOOS,
		"arch":           runtime.GOARCH,
		"replication":    repl,
		"uptime":         "since server start",
	})
}

func (a *AdminServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"version":  "0.3.0",
		"database": "connected",
		"region":   "eu-west",
	})
}

// dirSize calculates total size of a directory in bytes.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// === Database/Collections Handlers ===

func (a *AdminServer) handleListCollections(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	cols, err := env.Engine.ListCollections()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cols == nil {
		cols = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"collections": cols})
}

func (a *AdminServer) handleCreateCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "collection name is required")
		return
	}
	if err := env.Engine.CreateCollection(req.Name); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (a *AdminServer) handleDropCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.DropCollection(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var docs []map[string]interface{}
	q := r.URL.Query()
	filterKey := q.Get("filter_key")
	filterVal := q.Get("filter_val")

	if filterKey != "" {
		docs, err = env.Engine.Query(name, map[string]interface{}{
			filterKey: map[string]interface{}{
				"$contains": filterVal,
			},
		}, nil)
	} else {
		docs, err = env.Engine.List(name)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if docs == nil {
		docs = []map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, docs)
}

func (a *AdminServer) handleGetRules(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	cfg, err := env.Engine.GetRules(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (a *AdminServer) handleSetRules(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var cfg db.RulesConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules JSON")
		return
	}

	// Validate rule expressions
	for action, expr := range cfg.Rules {
		if expr != "" {
			tokens := db.Tokenize(expr)
			if _, err := db.ParseRulesExpr(tokens); err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid rule expression for %s: %v", action, err))
				return
			}
		}
	}

	if err := env.Engine.SetRules(name, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (a *AdminServer) handleInsertDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document JSON")
		return
	}
	docId, _ := doc["_id"].(string)
	if err := env.Engine.Insert(name, docId, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (a *AdminServer) handleImportCollection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var docs []map[string]interface{}
	if err := decodeJSON(r, &docs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid documents JSON array")
		return
	}
	for _, doc := range docs {
		docId, _ := doc["_id"].(string)
		if err := env.Engine.Insert(name, docId, doc); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to insert document: %v", err))
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "imported", "count": len(docs)})
}

func (a *AdminServer) handleUpdateDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	docId := r.PathValue("docId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document JSON")
		return
	}
	if err := env.Engine.Update(name, docId, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *AdminServer) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	name := r.PathValue("name")
	docId := r.PathValue("docId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Engine.Delete(name, docId); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Auth Handlers ===

func (a *AdminServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	users, err := env.Auth.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if users == nil {
		users = []*auth.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *AdminServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	user, _, err := env.Auth.SignUp(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if req.Role != "" && req.Role != string(auth.RoleUser) {
		user.Role = auth.Role(req.Role)
		if err := env.Auth.UpdateUser(user); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *AdminServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userId := r.PathValue("userId")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Auth.DeleteUser(userId); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Storage Handlers ===

func (a *AdminServer) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	buckets, err := env.Storage.ListBuckets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if buckets == nil {
		buckets = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"buckets": buckets})
}

func (a *AdminServer) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "bucket name is required")
		return
	}
	if err := env.Storage.CreateBucket(req.Name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (a *AdminServer) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Storage.DeleteBucket(bucket); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleListFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	prefix := r.URL.Query().Get("prefix")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	files, err := env.Storage.List(bucket, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []storage.FileInfo{}
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *AdminServer) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	path := r.FormValue("path")
	if path == "" {
		path = header.Filename
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := env.Storage.Upload(bucket, path, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, info)
}

func (a *AdminServer) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	path := r.PathValue("path")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := env.Storage.Delete(bucket, path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminServer) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	bucket := r.PathValue("bucket")
	pathVal := r.PathValue("path")
	env, err := a.projects.GetProjectEnv(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	reader, info, err := env.Storage.Download(bucket, pathVal)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+path.Base(info.Path)+"\"")
	io.Copy(w, reader)
}


func (a *AdminServer) handleListLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	logFile := filepath.Join(a.dataDir, "projects", id, "requests.log")
	
	// If file doesn't exist, return empty array
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	f, err := os.Open(logFile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer f.Close()

	var logs []map[string]interface{}
	
	// Read line by line
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err == nil {
			logs = append(logs, entry)
		}
	}
	
	if logs == nil {
		logs = []map[string]interface{}{}
	}
	writeJSON(w, http.StatusOK, logs)
}

func (a *AdminServer) handleFlushLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	logFile := filepath.Join(a.dataDir, "projects", id, "requests.log")
	
	// Delete the file
	if err := os.Remove(logFile); err != nil && !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "flushed"})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func (a *AdminServer) projectLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		
		projID := r.PathValue("id")
		if projID != "" {
			logFile := filepath.Join(a.dataDir, "projects", projID, "requests.log")
			_ = os.MkdirAll(filepath.Dir(logFile), 0755)
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				defer f.Close()
				logEntry := map[string]interface{}{
					"timestamp": time.Now().Format(time.RFC3339Nano),
					"method":    r.Method,
					"path":      r.URL.Path,
					"status":    sw.status,
					"duration":  time.Since(start).String(),
					"ip":        r.RemoteAddr,
				}
				bytes, err := json.Marshal(logEntry)
				if err == nil {
					_, _ = f.Write(append(bytes, '\n'))
				}
			}
		}
	})
}

// === Config Handlers ===

const secretMask = "••••••••"

// handleGetConfig returns the current server config with secrets masked.
func (a *AdminServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if a.cfg == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}
	// Build a safe response — mask any non-empty secrets
	masked := func(v string) string {
		if v != "" {
			return secretMask
		}
		return ""
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		// Core (read-only display)
		"listen_addr":     a.cfg.ListenAddr,
		"data_dir":        a.cfg.DataDir,
		"config_file":     a.cfg.ConfigFile,
		"admin_email":     a.cfg.AdminEmail,
		"admin_password":  masked(a.cfg.AdminPassword),
		"jwt_secret":      masked(a.cfg.JWTSecret),
		"session_duration": a.cfg.SessionDuration.String(),
		// S3
		"s3_enabled":       a.cfg.S3Enabled,
		"s3_endpoint":      a.cfg.S3Endpoint,
		"s3_access_key":    a.cfg.S3AccessKey,
		"s3_secret_key":    masked(a.cfg.S3SecretKey),
		"s3_bucket_prefix": a.cfg.S3BucketPrefix,
		"s3_use_ssl":       a.cfg.S3UseSSL,
		// Replication
		"role":      a.cfg.Role,
		"node_id":   a.cfg.NodeID,
		"repl_addr": a.cfg.ReplAddr,
		"peers":     a.cfg.Peers,
	})
}

// handleSaveConfig updates mutable fields in the config and persists to config.yaml.
func (a *AdminServer) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	if a.cfg == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}
	var req struct {
		// Admin account
		AdminEmail      *string `json:"admin_email"`
		AdminPassword   *string `json:"admin_password"`   // ignored if == secretMask or empty
		SessionDuration *string `json:"session_duration"` // e.g. "24h", "168h"
		// S3
		S3Enabled      *bool   `json:"s3_enabled"`
		S3Endpoint     *string `json:"s3_endpoint"`
		S3AccessKey    *string `json:"s3_access_key"`
		S3SecretKey    *string `json:"s3_secret_key"` // ignored if == secretMask
		S3BucketPrefix *string `json:"s3_bucket_prefix"`
		S3UseSSL       *bool   `json:"s3_use_ssl"`
		// Replication
		Role     *string  `json:"role"`
		NodeID   *string  `json:"node_id"`
		ReplAddr *string  `json:"repl_addr"`
		Peers    []string `json:"peers"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply mutable fields — secrets only updated if not the mask placeholder
	if req.AdminEmail != nil && *req.AdminEmail != "" {
		a.cfg.AdminEmail = *req.AdminEmail
		a.adminEmail = *req.AdminEmail
	}
	if req.AdminPassword != nil && *req.AdminPassword != "" && *req.AdminPassword != secretMask {
		a.cfg.AdminPassword = *req.AdminPassword
		a.adminPassword = *req.AdminPassword
	}
	if req.SessionDuration != nil && *req.SessionDuration != "" {
		if d, err := time.ParseDuration(*req.SessionDuration); err == nil && d > 0 {
			a.cfg.SessionDuration = d
		}
	}
	if req.S3Enabled != nil {
		a.cfg.S3Enabled = *req.S3Enabled
	}
	if req.S3Endpoint != nil {
		a.cfg.S3Endpoint = *req.S3Endpoint
	}
	if req.S3AccessKey != nil {
		a.cfg.S3AccessKey = *req.S3AccessKey
	}
	if req.S3SecretKey != nil && *req.S3SecretKey != secretMask {
		a.cfg.S3SecretKey = *req.S3SecretKey
	}
	if req.S3BucketPrefix != nil {
		a.cfg.S3BucketPrefix = *req.S3BucketPrefix
	}
	if req.S3UseSSL != nil {
		a.cfg.S3UseSSL = *req.S3UseSSL
	}
	if req.Role != nil {
		a.cfg.Role = *req.Role
	}
	if req.NodeID != nil {
		a.cfg.NodeID = *req.NodeID
	}
	if req.ReplAddr != nil {
		a.cfg.ReplAddr = *req.ReplAddr
	}
	if req.Peers != nil {
		a.cfg.Peers = req.Peers
	}

	// Persist to config.yaml
	cfgPath := a.cfg.ConfigFile
	if cfgPath == "" {
		cfgPath = filepath.Join(a.dataDir, "config.yaml")
	}
	if err := a.cfg.SaveToFile(cfgPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":      "saved",
		"config_file": cfgPath,
	})
}

// handleRestart triggers a graceful server restart.
func (a *AdminServer) handleRestart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "restarting",
	})
	// Flush the response before triggering the restart
	if fl, ok := w.(http.Flusher); ok {
		fl.Flush()
	}
	if a.OnRestart != nil {
		a.OnRestart()
	}
}
