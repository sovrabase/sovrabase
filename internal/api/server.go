package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/replication"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// Server is the HTTP API server for Sovrabase.
type Server struct {
	router       chi.Router
	config       *Config
	db           DatabaseService
	replicatedDB *replication.ReplicatedDB
	auth         AuthService
	store        StorageService
	projects     *tenant.ProjectManager
	adminMux     *http.ServeMux // optional admin routes
	dashboard    http.Handler   // optional dashboard UI
	realtimeHub  *realtime.Hub  // optional realtime hub
	rateLimiters *tenantRateLimiter
	logger       *slog.Logger
	httpServer   *http.Server // reference for graceful shutdown
}

// Config holds API-specific configuration.
type Config struct {
	ListenAddr         string
	AllowOrigins       string
	JWTSecret          string
	RateLimitPerMinute int
	RateLimitBurst     int
	CertFile           string // path to SSL/TLS certificate file
	KeyFile            string // path to SSL/TLS private key file
	DataDir            string // path to data directory (for audit logs)
}

// DatabaseService is the interface expected from the db package.
type DatabaseService interface {
	Insert(collection, id string, doc map[string]interface{}) error
	Get(collection, id string) (map[string]interface{}, error)
	Update(collection, id string, doc map[string]interface{}) error
	Delete(collection, id string) error
	List(collection string) ([]map[string]interface{}, error)
	ListPaged(collection string, limit, offset int) ([]map[string]interface{}, error)
	Query(collection string, filter map[string]interface{}, projection []string) ([]map[string]interface{}, error)
	QueryPaged(collection string, filter map[string]interface{}, projection []string, limit, offset int) ([]map[string]interface{}, error)
	Count(collection string) (int64, error)
	CreateIndex(collection, field string, idxType db.IndexType) error
	DropIndex(collection, field string) error
	ListIndexes(collection string) ([]db.IndexConfig, error)
	GetRules(collection string) (*db.RulesConfig, error)
	SetRules(collection string, cfg *db.RulesConfig) error
	Search(collection string, query string, fields []string, limit int) ([]map[string]interface{}, error)
}

// AuthService is the interface expected from the auth package.
type AuthService interface {
	SignUp(email, password string) (*UserInfo, *TokenPair, error)
	SignIn(email, password string) (*TokenPair, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	ValidateAccessToken(tokenString string) (*UserClaims, error)
	GetUser(id string) (*UserInfo, error)
	CreateOAuthState(provider, projectID, appRedirect string) (string, error)
	CreateOAuthStateURL(provider, projectID, appRedirect string) (authURL, state string, err error)
	DecodeStatePayload(state string) (*auth.OAuthStatePayload, error)
	HandleOAuthCallback(provider, code, state string) (*UserInfo, *TokenPair, error)
	VerifyEmail(token string) error
	ForgotPassword(email string) (string, error)
	ResetPassword(token, newPassword string) error
}

// StorageService is the interface expected from the storage package.
type StorageService interface {
	Upload(bucket, path string, reader io.Reader, contentType string) (*FileInfo, error)
	Download(bucket, path string) (io.ReadCloser, *FileInfo, error)
	Delete(bucket, path string) error
	List(bucket, prefix string) ([]FileInfo, error)
}

// ProviderMetaInfo is the public view of a linked OAuth provider (tokens excluded).
type ProviderMetaInfo struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
}

// UserInfo is a simplified user view for the API layer.
type UserInfo struct {
	ID                string             `json:"id"`
	Email             string             `json:"email"`
	Role              string             `json:"role"`
	Name              string             `json:"name,omitempty"`
	AvatarURL         string             `json:"avatar_url,omitempty"`
	OAuthProviders    []ProviderMetaInfo `json:"_metadata,omitempty"`
	CreatedAt         time.Time          `json:"created_at"`
	IsVerified        bool               `json:"is_verified"`
	VerificationToken string             `json:"verification_token,omitempty"`
}

// UserClaims represents JWT claims for middleware.
type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// FileInfo holds file metadata.
type FileInfo struct {
	Bucket      string    `json:"bucket"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	URL         string    `json:"url"`
}

type contextKey string

const (
	claimsKey     contextKey = "claims"
	projectEnvKey contextKey = "projectEnv"
	projectIDKey  contextKey = "projectID"
)

func getProjectEnv(r *http.Request) *tenant.ProjectEnv {
	env, ok := r.Context().Value(projectEnvKey).(*tenant.ProjectEnv)
	if !ok {
		return nil
	}
	return env
}

func getProjectID(r *http.Request) string {
	id, ok := r.Context().Value(projectIDKey).(string)
	if !ok {
		return ""
	}
	return id
}

// SetRealtimeHub attaches a realtime hub and mounts the WebSocket endpoint.
func (s *Server) SetRealtimeHub(hub *realtime.Hub, jwtSecret string) {
	s.realtimeHub = hub
	s.router.Get("/realtime/v1/ws", realtime.NewWSHandler(hub, jwtSecret).ServeHTTP)
}

// SetRealtimeHubForHandler sets the hub reference (for handlers to publish events).
func (s *Server) setRealtimeHub(hub *realtime.Hub) {
	s.realtimeHub = hub
}

// SetReplicatedDB sets the replication-aware database service.
// When set, getDB returns the ReplicatedDB for non-project requests,
// routing all mutations through the WAL/replication pipeline.
func (s *Server) SetReplicatedDB(db *replication.ReplicatedDB) {
	s.replicatedDB = db
}

func (s *Server) getDB(r *http.Request) DatabaseService {
	if env := getProjectEnv(r); env != nil {
		return env.Engine
	}
	if s.replicatedDB != nil {
		return s.replicatedDB
	}
	return s.db
}

func (s *Server) getAuth(r *http.Request) AuthService {
	if env := getProjectEnv(r); env != nil {
		return WrapAuthService(env.Auth)
	}
	return s.auth
}

func (s *Server) getStorage(r *http.Request) StorageService {
	if env := getProjectEnv(r); env != nil {
		return WrapStorageDriver(env.Storage)
	}
	return s.store
}

// NewServer creates a new API server.
func NewServer(cfg *Config, db DatabaseService, authSvc AuthService, store StorageService, pm *tenant.ProjectManager) *Server {
	// Rate limiter: default 100 req/min with burst of 20.
	ratePerMin := cfg.RateLimitPerMinute
	if ratePerMin <= 0 {
		ratePerMin = 100
	}
	burst := cfg.RateLimitBurst
	if burst <= 0 {
		burst = 20
	}
	rl := newTenantRateLimiter(float64(ratePerMin)/60.0, burst)

	s := &Server{
		config:       cfg,
		db:           db,
		auth:         authSvc,
		store:        store,
		projects:     pm,
		rateLimiters: rl,
		logger:       slog.Default(),
	}

	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// First check global allowed origins
			if cfg.AllowOrigins == "*" || strings.EqualFold(cfg.AllowOrigins, origin) {
				return true
			}
			// For preflight, allow if origin matches global
			if r.Method == http.MethodOptions {
				return cfg.AllowOrigins == "*"
			}
			// Check per-project CORS
			projectKey := r.Header.Get("X-Project-Key")
			if projectKey != "" {
				proj, err := pm.GetProjectBySecret(projectKey)
				if err == nil && proj.AllowOrigins != "" {
					for _, allowed := range strings.Split(proj.AllowOrigins, ",") {
						if strings.TrimSpace(allowed) == origin || strings.TrimSpace(allowed) == "*" {
							return true
						}
					}
				}
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Project-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health & Metrics (no auth, no project middleware needed)
	r.Get("/health", s.handleHealth)
	r.Get("/metrics", s.handleMetrics)

	// Auth routes (public, no rate limit)
	r.Route("/auth/v1", func(r chi.Router) {
		r.Use(s.projectMiddleware)
		r.Post("/signup", s.handleSignUp)
		r.Post("/signin", s.handleSignIn)
		r.Post("/refresh", s.handleRefresh)
		r.Get("/oauth/{provider}", s.handleOAuthRedirect)
		r.Post("/verify-email", s.handleVerifyEmail)
		r.Post("/forgot-password", s.handleForgotPassword)
		r.Post("/reset-password", s.handleResetPassword)
	})
	// OAuth callback is outside projectMiddleware because it receives
	// ?code and ?state from the OAuth provider (no X-Project-Key header).
	// The project ID is decoded from the state token itself.
	r.Get("/auth/v1/oauth/{provider}/callback", s.handleOAuthCallback)

	// API routes (rate limited + auth)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.rateLimitMiddleware)
		r.Use(s.projectMiddleware)
		r.Use(s.authMiddleware)
		r.Get("/me", s.handleGetMe)

		// Database
		r.Route("/collections/{collection}", func(r chi.Router) {
			// Non-mutation endpoints
			r.Get("/", s.handleList)
			r.Post("/query", s.handleQuery)
			r.Get("/{id}", s.handleGet)

			// Batch and search (non-mutation, but need audit logging)
			r.Post("/batch", s.handleBatch)
			r.Post("/search", s.handleSearch)

			// Mutation endpoints (with audit logging)
			r.Group(func(r chi.Router) {
				r.Use(s.auditLoggerMiddleware)
				r.Post("/", s.handleInsert)
				r.Put("/{id}", s.handleUpdate)
				r.Patch("/{id}", s.handleUpdate)
				r.Delete("/{id}", s.handleDelete)
			})
		})

		// Storage
		r.Route("/storage/{bucket}", func(r chi.Router) {
			r.Post("/upload", s.handleUpload)
			r.Get("/list", s.handleStorageList)
			r.Get("/{path:.*}", s.handleDownload)
			r.Delete("/{path:.*}", s.handleStorageDelete)
		})
	})

	s.router = r
	return s
}

// RegisterAdmin attaches admin API routes to the server.
func (s *Server) RegisterAdmin(mux *http.ServeMux) {
	s.adminMux = mux
	s.router.Mount("/admin", mux)
}

// SetDashboard attaches a dashboard UI handler at /.
func (s *Server) SetDashboard(handler http.Handler) {
	s.dashboard = handler
	s.router.Handle("/", handler)
	s.router.Handle("/style.css", handler)
	s.router.Handle("/js/*", handler)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	s.httpServer = &http.Server{
		Addr:    s.config.ListenAddr,
		Handler: s.router,
	}

	if s.config.CertFile != "" && s.config.KeyFile != "" {
		s.logger.Info("Sovrabase server starting with TLS (HTTPS)", "addr", s.config.ListenAddr, "cert", s.config.CertFile)
		return s.httpServer.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}

	s.logger.Info("Sovrabase server starting (HTTP)", "addr", s.config.ListenAddr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		s.logger.Info("Stopping API server gracefully...")
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Handler returns the HTTP handler (for testing).
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	SetActiveProjects(s.projects.ProjectCount())
	HandleMetrics(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}
