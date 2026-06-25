package main

// @title           Sovrabase API
// @version         1.0
// @description     Sovereign European BaaS — lightweight open-source Supabase alternative. Provides JSON document database (PebbleDB), JWT/OAuth authentication, file storage (local + S3), realtime WebSocket, queues, and config maps — all in a single ~40 MB binary.
// @contact.name    Sovrabase
// @contact.url     https://sovrabase.eu
// @contact.email   hello@sovrabase.eu
// @license.name    AGPL-3.0
// @license.url     https://github.com/ketsuna-org/sovrabase/blob/main/LICENSE
// @host            localhost:6070
// @BasePath        /
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT access token: "Bearer <token>"

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/api"
	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/captcha"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/dashboard"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/plugin"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/replication"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"github.com/ketsuna-org/sovrabase/plugins"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("Starting Sovrabase",
		"data_dir", cfg.DataDir,
		"listen_addr", cfg.ListenAddr,
		"role", cfg.Role,
		"node_id", cfg.NodeID,
		"config_file", cfg.ConfigFile,
	)

	// Warn about default JWT secret.
	if cfg.JWTSecret == "change-me-in-production" {
		logger.Warn("Using default JWT secret — CHANGE IT in config.yaml or SOVRABASE_JWT_SECRET environment variable")
	}

	// Initialize database engine
	engine, err := db.NewEngine(filepath.Join(cfg.DataDir, "db"))
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer engine.Close()

	// Initialize auth service
	userStore := auth.NewInMemoryUserStore()
	authService := auth.NewService(cfg.JWTSecret, userStore)
	authService.EmailVerificationEnabled = cfg.EmailVerification
	authService.SMTPHost = cfg.SMTPHost
	authService.SMTPPort = cfg.SMTPPort
	authService.SMTPUser = cfg.SMTPUser
	authService.SMTPPassword = cfg.SMTPPassword
	authService.SMTPSender = cfg.SMTPSender

	// Initialize storage driver (S3 if enabled, local otherwise)
	var storageDriver storage.Driver
	if cfg.S3Enabled && cfg.S3AccessKey != "" {
		storageDriver, err = storage.NewS3DriverFromEnv()
		if err != nil {
			logger.Warn("Failed to init S3 driver, falling back to local", "error", err)
			storageDriver, _ = storage.NewLocalDriver(cfg.StorageDir, "")
		} else {
			logger.Info("S3 storage driver active", "endpoint", cfg.S3Endpoint)
		}
	} else {
		storageDriver, err = storage.NewLocalDriver(cfg.StorageDir, "")
		if err != nil {
			logger.Error("Failed to initialize storage driver", "error", err)
			os.Exit(1)
		}
	}

	// Initialize multi-tenant project manager
	projectMgr, err := tenant.NewProjectManager(cfg.DataDir, cfg)
	if err != nil {
		logger.Error("Failed to initialize project manager", "error", err)
		os.Exit(1)
	}
	defer projectMgr.Close()

	// Initialize realtime hub
	realtimeHub := realtime.NewHub()
	realtimeHub.Start()
	defer realtimeHub.Stop()

	logger.Info("Realtime hub started")

	// Initialize plugin system.
	hookManager := plugin.NewHookManager()
	app := plugin.NewApp(hookManager)

	// Register plugins.
	statusPlugin := &statusplugin.StatusPlugin{}
	if err := statusPlugin.Register(app); err != nil {
		logger.Error("Failed to register status-plugin", "error", err)
	}

	// Create API server
	server := api.NewServer(
		&api.Config{
			ListenAddr:         cfg.ListenAddr,
			AllowOrigins:       cfg.AllowOrigins,
			JWTSecret:          cfg.JWTSecret,
			RateLimitPerMinute: cfg.RateLimitPerMinute,
			RateLimitBurst:     cfg.RateLimitBurst,
			CertFile:           cfg.CertFile,
			KeyFile:            cfg.KeyFile,
			DataDir:            cfg.DataDir,
		},
		engine,
		api.WrapAuthService(authService),
		api.WrapStorageDriver(storageDriver),
		projectMgr,
		hookManager,
	)

	// Mount realtime WebSocket endpoint.
	server.SetRealtimeHub(realtimeHub, cfg.JWTSecret)
	logger.Info("Realtime WebSocket endpoint registered at /realtime/v1/ws")

	// Register admin API (with config reference for GET/POST /admin/config)
	adminMux := http.NewServeMux()
	adminServer := api.NewAdminServer(projectMgr, cfg, cfg.JWTSecret, cfg.AdminEmail, cfg.AdminPassword)

	// Initialize admin store from master DB and seed from config
	adminStore := auth.NewAdminStore(engine.DB(), cfg.AdminEmail, cfg.AdminPassword)
	adminServer.SetAdminStore(adminStore)

	// Initialize audit store from master DB
	auditStore := auth.NewAuditStore(engine.DB())
	adminServer.SetAuditStore(auditStore)

	// Initialize metering store for usage tracking
	meterStore, err := metering.OpenMeterStore(filepath.Join(cfg.DataDir, "metering"))
	if err != nil {
		logger.Error("Failed to initialize metering store", "error", err)
		os.Exit(1)
	}
	defer meterStore.Close()
	logger.Info("Metering store initialized")

	// Wire metering into API server and admin server
	server.SetMeterStore(meterStore)
	adminServer.SetMeterStore(meterStore)

	// Wire captcha verifier if enabled.
	if cfg.CaptchaEnabled && cfg.CaptchaSecret != "" {
		verifier := captcha.NewVerifier(captcha.Config{
			Provider:  captcha.Provider(cfg.CaptchaProvider),
			SiteKey:   cfg.CaptchaSiteKey,
			SecretKey: cfg.CaptchaSecret,
			Enabled:   true,
		})
		server.SetCaptchaVerifier(verifier)
		logger.Info("Captcha protection enabled", "provider", cfg.CaptchaProvider)
	}

	// Wire team store into admin server
	adminServer.SetTeamStore(projectMgr.GetTeamStore())

	adminServer.RegisterRoutes(adminMux)
	server.RegisterAdmin(adminMux)
	logger.Info("Admin API registered")

	// Serve embedded dashboard
	server.SetDashboard(dashboard.Handler())
	logger.Info("Dashboard registered")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start cron schedulers for all existing projects.
	api.StartProjectSchedulers(ctx, projectMgr)
	logger.Info("Cron schedulers started", "projects", projectMgr.ProjectCount())

	// Start replication node if configured
	if cfg.IsReplicationEnabled() {
		replNode, err := startReplication(ctx, cfg, engine)
		if err != nil {
			logger.Error("Failed to start replication", "error", err)
			os.Exit(1)
		}
		adminServer.SetReplicationStatus(&api.ReplicationStatus{
			Enabled: true,
			Role:    cfg.Role,
			NodeID:  cfg.NodeID,
			Peers:   len(cfg.Peers),
		})
		logger.Info("Replication node started", "role", cfg.Role)

		// Wrap system engine with ReplicatedDB so API routes go through WAL
		replicatedDB := replication.NewReplicatedDB(engine, replNode)
		server.SetReplicatedDB(replicatedDB)
		logger.Info("ReplicatedDB set on API server")
	}

	// Start backup scheduler if enabled
	if cfg.BackupInterval > 0 {
		backupTicker := time.NewTicker(cfg.BackupInterval)
		defer backupTicker.Stop()
		go func() {
			for {
				select {
				case <-backupTicker.C:
					logger.Info("Running scheduled backup...")
					backupDir := filepath.Join(cfg.DataDir, "backups")
					if err := os.MkdirAll(backupDir, 0755); err != nil {
						logger.Error("Failed to create backup directory", "error", err)
						continue
					}
					timestamp := time.Now().UTC().Format("20060102T150405Z")
					backupName := "backup-" + timestamp
					backupPath := filepath.Join(backupDir, backupName)
					// Use ProjectManager.Backup which uses Pebble checkpoints
					// (consistent snapshots) instead of naively copying live SST files.
					if err := projectMgr.Backup(backupPath); err != nil {
						logger.Error("Failed to create backup", "error", err)
						continue
					}
					logger.Info("Scheduled backup completed", "name", backupName, "path", backupPath)
				case <-ctx.Done():
					return
				}
			}
		}()
		logger.Info("Backup scheduler started", "interval", cfg.BackupInterval)
	} else {
		logger.Info("Backup scheduler disabled (interval <= 0)")
	}

	// Graceful shutdown / restart handler
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	// Restart channel: when a restart is requested via the dashboard
	adminServer.OnRestart = func() {
		logger.Info("Restart requested via dashboard — respawning...")
		go func() {
			time.Sleep(300 * time.Millisecond)
			exe, err := os.Executable()
			if err != nil {
				exe = os.Args[0]
			}
			cmd := exec.Command(exe, os.Args[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			if startErr := cmd.Start(); startErr != nil {
				logger.Error("Failed to restart process", "error", startErr)
				return
			}
			cancel()
		}()
	}

	// Start server in a goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	// Wait for graceful shutdown signal or server error
	select {
	case err := <-serverErrCh:
		logger.Error("Server error", "error", err)
	case <-ctx.Done():
		logger.Info("Graceful shutdown initiated")
	}

	// Graceful shutdown of the HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}

	logger.Info("Sovrabase stopped")
}
