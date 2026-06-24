package main

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
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/dashboard"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
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
	)

	// Mount realtime WebSocket endpoint.
	server.SetRealtimeHub(realtimeHub, cfg.JWTSecret)
	logger.Info("Realtime WebSocket endpoint registered at /realtime/v1/ws")

	// Register admin API (with config reference for GET/POST /admin/config)
	adminMux := http.NewServeMux()
	adminServer := api.NewAdminServer(projectMgr, cfg, cfg.JWTSecret, cfg.AdminEmail, cfg.AdminPassword)
	adminServer.RegisterRoutes(adminMux)
	server.RegisterAdmin(adminMux)
	logger.Info("Admin API registered")

	// Serve embedded dashboard
	server.SetDashboard(dashboard.Handler())
	logger.Info("Dashboard registered")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start replication node if configured
	if cfg.IsReplicationEnabled() {
		if err := startReplication(ctx, cfg, engine); err != nil {
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
	}

	// Start backup scheduler (every 1 hour)
	backupTicker := time.NewTicker(1 * time.Hour)
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
				if err := os.MkdirAll(backupPath, 0755); err != nil {
					logger.Error("Failed to create backup", "error", err)
					continue
				}
				// Copy projects data
				projectsDir := filepath.Join(cfg.DataDir, "projects")
				entries, _ := os.ReadDir(projectsDir)
				for _, e := range entries {
					if e.IsDir() {
						src := filepath.Join(projectsDir, e.Name())
						dst := filepath.Join(backupPath, e.Name())
						copyDir(src, dst)
					}
				}
				logger.Info("Scheduled backup completed", "name", backupName, "path", backupPath)
			case <-ctx.Done():
				return
			}
		}
	}()
	logger.Info("Backup scheduler started (interval: 1h)")

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

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
