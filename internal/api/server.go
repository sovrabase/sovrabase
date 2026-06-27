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
	"github.com/ketsuna-org/sovrabase/internal/captcha"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/plugin"
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
	meterStore   *metering.MeterStore
	adminMux     *http.ServeMux // optional admin routes
	dashboard    http.Handler   // optional dashboard UI
	realtimeHub  *realtime.Hub  // optional realtime hub
	rateLimiters *tenantRateLimiter
	logger       *slog.Logger
	hooks        *plugin.HookManager
	httpServer   *http.Server // reference for graceful shutdown
	captchaVerifier *captcha.Verifier
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
	CreateMagicLink(email string) (string, error)
	VerifyMagicLink(email, token string) (*TokenPair, error)
	SetupMFA(userID string) (secret, uri string, err error)
	ConfirmMFA(userID, code string) ([]string, error)
	DisableMFA(userID, code string) error
	VerifyMFA(userID, code string) error
	GetMFAStatus(userID string) (bool, error)
	UpdateUser(id string, name, avatarURL *string) (*UserInfo, error)
	SignInWithMFA(email, password string) (*SignInResult, error)
	CompleteMFAChallenge(challengeToken, code string) (*TokenPair, error)
}

// StorageService is the interface expected from the storage package.
type StorageService interface {
	Upload(bucket, path string, reader io.Reader, contentType string) (*FileInfo, error)
	Download(bucket, path string) (io.ReadCloser, *FileInfo, error)
	Delete(bucket, path string) error
	List(bucket, prefix string) ([]FileInfo, error)
}

// ProviderMetaInfo public OAuth provider info (tokens excluded).
// @name ProviderMetaInfo
type ProviderMetaInfo struct {
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
}

// UserInfo public user view.
// @name UserInfo
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

// UserClaims JWT claims for middleware.
// @name UserClaims
type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

// TokenPair access and refresh tokens.
// @name TokenPair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// SignInResult is returned by SignInWithMFA. When MFA is required, Token is nil
// and the caller must use the ChallengeToken with CompleteMFAChallenge.
// @name SignInResult
type SignInResult struct {
	Token          *TokenPair `json:"token,omitempty"`
	MFARequired    bool       `json:"mfa_required"`
	ChallengeToken string     `json:"challenge_token,omitempty"`
	ExpiresIn      int64      `json:"expires_in,omitempty"`
}

// FileInfo file metadata.
// @name FileInfo
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

// SetMeterStore attaches a metering store for per-project usage tracking.
func (s *Server) SetMeterStore(ms *metering.MeterStore) {
	s.meterStore = ms
}

// SetCaptchaVerifier attaches a captcha verifier for auth endpoint protection.
func (s *Server) SetCaptchaVerifier(v *captcha.Verifier) {
	s.captchaVerifier = v
}

// MeterStore returns the current meter store, or nil if not set.
func (s *Server) MeterStore() *metering.MeterStore {
	return s.meterStore
}

// meteringMiddleware wraps the MeteringMiddleware for use in the chi router.
func (s *Server) meteringMiddleware(next http.Handler) http.Handler {
	if s.meterStore == nil || s.projects == nil {
		return next
	}
	return metering.MeteringMiddleware(s.meterStore, s.projects)(next)
}

// SetRealtimeHub attaches a realtime hub and mounts the WebSocket endpoint.
func (s *Server) SetRealtimeHub(hub *realtime.Hub, jwtSecret string) {
	s.realtimeHub = hub
	s.router.Get("/realtime/v1/ws", realtime.NewWSHandler(hub, jwtSecret, s.meterStore, s.projects).ServeHTTP)
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
func NewServer(cfg *Config, db DatabaseService, authSvc AuthService, store StorageService, pm *tenant.ProjectManager, hookManager *plugin.HookManager) *Server {
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
		hooks:        hookManager,
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

	// API Documentation (Redoc)
	r.Get("/docs", s.handleDocs)
	r.Get("/docs/swagger.json", s.handleSwaggerJSON)

	// Auth routes (public, no rate limit)
	r.Route("/auth/v1", func(r chi.Router) {
		r.Use(s.projectMiddleware)
		r.Use(s.clientRequestLoggerMiddleware)
		r.Use(s.meteringMiddleware)
		r.Post("/signup", s.handleSignUp)
		r.Post("/signin", s.handleSignIn)
		r.Post("/refresh", s.handleRefresh)
		r.Get("/oauth/{provider}", s.handleOAuthRedirect)
		r.Post("/verify-email", s.handleVerifyEmail)
		r.Post("/forgot-password", s.handleForgotPassword)
		r.Post("/reset-password", s.handleResetPassword)
		r.Post("/magic-link", s.handleCreateMagicLink)
		r.Post("/verify-magic-link", s.handleVerifyMagicLink)
		r.Post("/mfa/setup", s.handleMFASetup)
		r.Post("/mfa/confirm", s.handleMFAConfirm)
		r.Post("/mfa/disable", s.handleMFADisable)
			r.Get("/mfa/status", s.handleMFAStatus)
			r.Post("/mfa/challenge", s.handleMFAChallenge)
		})
	// OAuth callback is outside projectMiddleware because it receives
	// ?code and ?state from the OAuth provider (no X-Project-Key header).
	// The project ID is decoded from the state token itself.
	r.Get("/auth/v1/oauth/{provider}/callback", s.handleOAuthCallback)

	// Storage download — no auth required. Images embedded via <img> tags
	// cannot send Authorization headers. Only project key is needed.
	// Must be outside the /api/v1 Route block because chi requires
	// all middlewares to be defined before routes on a given mux.
	r.With(s.rateLimitMiddleware, s.projectMiddleware).Get("/api/v1/storage/{bucket}/{path:.*}", s.handleDownload)

	// Signed storage download — no auth, no project key needed. The
	// signed token in the query string carries all authorisation.
	r.With(s.rateLimitMiddleware).Get("/api/v1/storage/signed/{bucket}/{path:.*}", s.handleSignedDownload)

	// API routes (rate limited + auth)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.rateLimitMiddleware)
		r.Use(s.projectMiddleware)
		r.Use(s.clientRequestLoggerMiddleware)
		r.Use(s.authMiddleware)
		r.Use(s.meteringMiddleware)
		r.Get("/me", s.handleGetMe)
		r.Patch("/me", s.handleUpdateMe)

		// Database
		r.Route("/collections/{collection}", func(r chi.Router) {
			// Non-mutation endpoints
			r.Get("/", s.handleList)
			r.Post("/query", s.handleQuery)
			r.Get("/{id}", s.handleGet)

			// Batch and search (non-mutation, but need audit logging)
			r.Post("/batch", s.handleBatch)
			r.Post("/search", s.handleSearch)

			// Mutation endpoints
			r.Post("/", s.handleInsert)
			r.Put("/{id}", s.handleUpdate)
			r.Patch("/{id}", s.handleUpdate)
			r.Delete("/{id}", s.handleDelete)
		})

		// Storage
			r.Route("/storage/{bucket}", func(r chi.Router) {
				r.Post("/upload", s.handleUpload)
				r.Post("/signed-url", s.handleSignedURL)
				r.Get("/list", s.handleStorageList)
				r.Delete("/{path:.*}", s.handleStorageDelete)
			})

			// Analytics events ingestion
			r.Post("/events", s.handleIngestEvents)

			// Queues
			r.Route("/queues", func(r chi.Router) {
				r.Post("/send", s.handleQueueSend)
				r.Post("/receive", s.handleQueueReceive)
				r.Post("/delete", s.handleQueueDelete)
			})
			})

	// Remote config routes — registered on the chi router.
	s.router = r
	s.RegisterConfigMapsRoutes()

	// Run OnServe hooks so plugins can register custom routes.
	if s.hooks != nil {
		s.hooks.RunServeHooks(r)
	}

	return s
}

// RegisterAdmin attaches admin API routes to the server, wrapped with
// metering middleware for bandwidth tracking.
func (s *Server) RegisterAdmin(mux *http.ServeMux) {
	s.adminMux = mux
	if s.meterStore != nil {
		adminMW := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mw := &metering.MeterResponseWriter{ResponseWriter: w}
				next.ServeHTTP(mw, r)
				if mw.Written() > 0 {
					_ = s.meterStore.Inc("__admin__", metering.MetricBandwidthDown, int64(mw.Written()))
				}
			})
		}
		s.router.Mount("/admin", adminMW(mux))
	} else {
		s.router.Mount("/admin", mux)
	}
}

// SetDashboard attaches a dashboard UI handler at root, catching all non-API paths.
func (s *Server) SetDashboard(handler http.Handler) {
	s.dashboard = handler
	// Register as a catch-all on the router — chi will fall through to this
	// for any path not matched by more specific routes (API, admin, etc.)
	s.router.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	})
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
	// Run terminate hooks before shutting down.
	if s.hooks != nil {
		s.hooks.RunTerminateHooks()
	}
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
