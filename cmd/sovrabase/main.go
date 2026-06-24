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

	// Create API server
	server := api.NewServer(
		&api.Config{
			ListenAddr:   cfg.ListenAddr,
			AllowOrigins: cfg.AllowOrigins,
			JWTSecret:    cfg.JWTSecret,
		},
		engine,
		api.WrapAuthService(authService),
		api.WrapStorageDriver(storageDriver),
	)

	// Initialize multi-tenant project manager
	projectMgr, err := tenant.NewProjectManager(cfg.DataDir)
	if err != nil {
		logger.Error("Failed to initialize project manager", "error", err)
		os.Exit(1)
	}
	defer projectMgr.Close()

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

	// Graceful shutdown / restart handler
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
		_ = engine.Close()
		os.Exit(0)
	}()

	// Restart channel: when a restart is requested via the dashboard
	adminServer.OnRestart = func() {
		logger.Info("Restart requested via dashboard — respawning...")
		// Give the HTTP response time to be sent before we exit
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
			// Detach and exit the current process
			cancel()
			_ = engine.Close()
			os.Exit(0)
		}()
	}

	logger.Info("Sovrabase API server starting", "addr", cfg.ListenAddr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}
