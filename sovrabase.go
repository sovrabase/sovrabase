// Package sovrabase is the entry point for embedding Sovrabase as a library.
//
// Usage:
//
//	package main
//
//	import "github.com/ketsuna-org/sovrabase"
//
//	func main() {
//	    app := sovrabase.New(sovrabase.Config{
//	        DataDir:   "./data",
//	        JWTSecret: "my-secret-key",
//	    })
//	    app.Serve()
//	}
package sovrabase

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/api"
	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/dashboard"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"github.com/ketsuna-org/sovrabase/plugin"
)

// Config is the configuration for a Sovrabase instance.
// Only DataDir is required; sensible defaults are used for everything else.
type Config struct {
	DataDir    string // required — where PebbleDB and files are stored
	ListenAddr string // default ":6070"
	JWTSecret  string // default "change-me-in-production" (warns in production)
	Env        string // "development" or "production"

	// Admin credentials (created on first run if they don't exist).
	AdminEmail    string
	AdminPassword string

	// SMTP for email verification and password reset.
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPSender   string

	// S3-compatible storage (MinIO, R2, etc.).
	S3Enabled      bool
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3BucketPrefix string

	// Rate limiting.
	RateLimitPerMinute int // default 100
	RateLimitBurst     int // default 20
}

// App is a Sovrabase application instance.
type App struct {
	cfg         *config.Config
	hookManager *plugin.HookManager
	pluginApp   *plugin.App
	engine      *db.Engine
	server      *api.Server
	realtimeHub *realtime.Hub
	projectMgr  *tenant.ProjectManager
}

// New creates a new Sovrabase application.
func New(cfg Config) *App {
	c := &config.Config{
		DataDir:            cfg.DataDir,
		ListenAddr:         cfg.ListenAddr,
		JWTSecret:          cfg.JWTSecret,
		Env:                cfg.Env,
		AdminEmail:         cfg.AdminEmail,
		AdminPassword:      cfg.AdminPassword,
		SMTPHost:           cfg.SMTPHost,
		SMTPPort:           cfg.SMTPPort,
		SMTPUser:           cfg.SMTPUser,
		SMTPPassword:       cfg.SMTPPassword,
		SMTPSender:         cfg.SMTPSender,
		S3Enabled:          cfg.S3Enabled,
		S3Endpoint:          cfg.S3Endpoint,
		S3AccessKey:         cfg.S3AccessKey,
		S3SecretKey:         cfg.S3SecretKey,
		S3BucketPrefix:      cfg.S3BucketPrefix,
		RateLimitPerMinute:  cfg.RateLimitPerMinute,
		RateLimitBurst:      cfg.RateLimitBurst,
	}

	// Apply defaults.
	if c.ListenAddr == "" {
		c.ListenAddr = ":6070"
	}
	if c.RateLimitPerMinute == 0 {
		c.RateLimitPerMinute = 100
	}
	if c.RateLimitBurst == 0 {
		c.RateLimitBurst = 20
	}

	// Expand data dir to absolute path so config file resolution works.
	if abs, err := filepath.Abs(c.DataDir); err == nil {
		c.DataDir = abs
	}

	// Initialize plugin system.
	hookManager := plugin.NewHookManager()
	pluginApp := plugin.NewApp(hookManager)

	app := &App{
		cfg:         c,
		hookManager: hookManager,
		pluginApp:   pluginApp,
	}

	return app
}

// Plugins returns the plugin App so the caller can register hooks and plugins.
//
//	app.Plugins().OnRecordCreate("posts").Do(func(e *plugin.RecordEvent) error {
//	    e.Record["status"] = "draft"
//	    return nil
//	})
func (a *App) Plugins() *plugin.App {
	return a.pluginApp
}

// Serve starts the Sovrabase server and blocks until a shutdown signal is received.
func (a *App) Serve() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := a.cfg
	logger.Info("Starting Sovrabase",
		"data_dir", cfg.DataDir,
		"listen_addr", cfg.ListenAddr,
	)

	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		logger.Warn("Using default JWT secret — CHANGE IT in production")
		cfg.JWTSecret = "change-me-in-production"
	}

	// Database.
	engine, err := db.NewEngine(filepath.Join(cfg.DataDir, "db"))
	if err != nil {
		return err
	}
	defer engine.Close()
	a.engine = engine

	// Auth.
	userStore := auth.NewInMemoryUserStore()
	authService := auth.NewService(cfg.JWTSecret, userStore)
	if cfg.SMTPHost != "" {
		authService.SMTPHost = cfg.SMTPHost
		authService.SMTPPort = cfg.SMTPPort
		authService.SMTPUser = cfg.SMTPUser
		authService.SMTPPassword = cfg.SMTPPassword
		authService.SMTPSender = cfg.SMTPSender
	}

	// Storage.
	var storageDriver storage.Driver
	if cfg.S3Enabled && cfg.S3AccessKey != "" {
		storageDriver, err = storage.NewS3DriverFromEnv()
		if err != nil {
			logger.Warn("S3 driver init failed, falling back to local", "error", err)
			storageDriver, _ = storage.NewLocalDriver(filepath.Join(cfg.DataDir, "storage"), "")
		}
	} else {
		storageDriver, _ = storage.NewLocalDriver(filepath.Join(cfg.DataDir, "storage"), "")
	}

	// Multi-tenant project manager.
	projectMgr, err := tenant.NewProjectManager(cfg.DataDir, cfg)
	if err != nil {
		return err
	}
	defer projectMgr.Close()
	a.projectMgr = projectMgr

	// Realtime hub.
	realtimeHub := realtime.NewHub()
	realtimeHub.Start()
	defer realtimeHub.Stop()
	a.realtimeHub = realtimeHub

	// API server.
	server := api.NewServer(
		&api.Config{
			ListenAddr:         cfg.ListenAddr,
			AllowOrigins:       cfg.AllowOrigins,
			JWTSecret:          cfg.JWTSecret,
			RateLimitPerMinute: cfg.RateLimitPerMinute,
			RateLimitBurst:     cfg.RateLimitBurst,
			DataDir:            cfg.DataDir,
		},
		engine,
		api.WrapAuthService(authService),
		api.WrapStorageDriver(storageDriver),
		projectMgr,
		a.hookManager,
	)

	// Give plugins access to DB and storage.
	a.pluginApp.SetDB(&dbAdapter{engine})
	a.pluginApp.SetStorage(&storageAdapter{storageDriver, projectMgr})

	server.SetRealtimeHub(realtimeHub, cfg.JWTSecret)
	server.SetDashboard(dashboard.Handler())
	a.server = server

	// Graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	// Start server.
	go func() {
		if err := server.ListenAndServe(); err != nil {
			logger.Error("Server error", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}

	logger.Info("Sovrabase stopped")
	return nil
}

// ─── Adapters ────────────────────────────────────────────────────────

// dbAdapter wraps db.Engine to implement plugin.DB.
type dbAdapter struct {
	*db.Engine
}

// storageAdapter wraps a storage.Driver to implement plugin.Storage.
type storageAdapter struct {
	driver  storage.Driver
	project *tenant.ProjectManager
}

func (a *storageAdapter) Upload(bucket, path, contentType string, size int64) (*plugin.FileInfo, error) {
	// Upload needs a reader — library users should use form uploads
	// or we expose a simpler API. For now return not implemented.
	return nil, nil
}

func (a *storageAdapter) List(bucket, prefix string) ([]plugin.FileInfo, error) {
	files, err := a.driver.List(bucket, prefix)
	if err != nil {
		return nil, err
	}
	result := make([]plugin.FileInfo, len(files))
	for i, f := range files {
		result[i] = plugin.FileInfo{
			Bucket:      f.Bucket,
			Path:        f.Path,
			Size:        f.Size,
			ContentType: f.ContentType,
		}
	}
	return result, nil
}

func (a *storageAdapter) Delete(bucket, path string) error {
	return a.driver.Delete(bucket, path)
}
