package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// Server is the HTTP API server for Sovrabase.
type Server struct {
	router    chi.Router
	config    *Config
	db        DatabaseService
	auth      AuthService
	store     StorageService
	projects  *tenant.ProjectManager
	adminMux  *http.ServeMux // optional admin routes
	dashboard http.Handler   // optional dashboard UI
	logger    *slog.Logger
}

// Config holds API-specific configuration.
type Config struct {
	ListenAddr   string
	AllowOrigins string
	JWTSecret    string
}

// DatabaseService is the interface expected from the db package.
type DatabaseService interface {
	Insert(collection, id string, doc map[string]interface{}) error
	Get(collection, id string) (map[string]interface{}, error)
	Update(collection, id string, doc map[string]interface{}) error
	Delete(collection, id string) error
	List(collection string) ([]map[string]interface{}, error)
	Query(collection string, filter map[string]interface{}) ([]map[string]interface{}, error)
}

// AuthService is the interface expected from the auth package.
type AuthService interface {
	SignUp(email, password string) (*UserInfo, *TokenPair, error)
	SignIn(email, password string) (*TokenPair, error)
	RefreshToken(refreshToken string) (*TokenPair, error)
	ValidateAccessToken(tokenString string) (*UserClaims, error)
	GetUser(id string) (*UserInfo, error)
	CreateOAuthState(provider string) (string, error)
	HandleOAuthCallback(provider, code, state string) (*UserInfo, *TokenPair, error)
}

// StorageService is the interface expected from the storage package.
type StorageService interface {
	Upload(bucket, path string, reader io.Reader, contentType string) (*FileInfo, error)
	Download(bucket, path string) (io.ReadCloser, *FileInfo, error)
	Delete(bucket, path string) error
	List(bucket, prefix string) ([]FileInfo, error)
}

// UserInfo is a simplified user view for the API layer.
type UserInfo struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
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
)

func getProjectEnv(r *http.Request) *tenant.ProjectEnv {
	env, ok := r.Context().Value(projectEnvKey).(*tenant.ProjectEnv)
	if !ok {
		return nil
	}
	return env
}

func (s *Server) getDB(r *http.Request) DatabaseService {
	if env := getProjectEnv(r); env != nil {
		return env.Engine
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
	s := &Server{
		config:   cfg,
		db:       db,
		auth:     authSvc,
		store:    store,
		projects: pm,
		logger:   slog.Default(),
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AllowOrigins},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Project-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health
	r.Get("/health", s.handleHealth)

	// Auth routes (public)
	r.Route("/auth/v1", func(r chi.Router) {
		r.Use(s.projectMiddleware)
		r.Post("/signup", s.handleSignUp)
		r.Post("/signin", s.handleSignIn)
		r.Post("/refresh", s.handleRefresh)
		r.Get("/oauth/{provider}", s.handleOAuthRedirect)
		r.Get("/oauth/{provider}/callback", s.handleOAuthCallback)
	})

	// API routes (protected)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.projectMiddleware)
		r.Use(s.authMiddleware)
		r.Get("/me", s.handleGetMe)

		// Database
		r.Route("/collections/{collection}", func(r chi.Router) {
			r.Post("/", s.handleInsert)
			r.Get("/", s.handleList)
			r.Post("/query", s.handleQuery)
			r.Get("/{id}", s.handleGet)
			r.Put("/{id}", s.handleUpdate)
			r.Patch("/{id}", s.handleUpdate)
			r.Delete("/{id}", s.handleDelete)
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
	s.logger.Info("Sovrabase server starting", "addr", s.config.ListenAddr)
	return http.ListenAndServe(s.config.ListenAddr, s.router)
}

// Handler returns the HTTP handler (for testing).
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
