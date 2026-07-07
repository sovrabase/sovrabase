package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/api"
	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/captcha"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/dashboard"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/email"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/replication"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"github.com/ketsuna-org/sovrabase/plugin"
)

func runSovrabase(logger *slog.Logger, cfg *config.Config) {
	// Warn about default JWT secret.
	if cfg.JWTSecret == "change-me-in-production" {
		logger.Warn("Using default JWT secret — CHANGE IT in config.yaml or SOVRABASE_JWT_SECRET environment variable")
	}

	killStaleInstances(logger)

	validateHAConfig(cfg, logger)

	engine, err := db.NewEngine(filepath.Join(cfg.DataDir, "db"))
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	engineClosed := false
	defer func() {
		if !engineClosed {
			engine.Close()
		}
	}()

	userStore := auth.NewDBUserStore(engine)
	authService := auth.NewService(cfg.JWTSecret, userStore)
	authService.EmailVerificationEnabled = cfg.EmailVerification
	authService.SMTPHost = cfg.SMTPHost
	authService.SMTPPort = cfg.SMTPPort
	authService.SMTPUser = cfg.SMTPUser
	authService.SMTPPassword = cfg.SMTPPassword
	authService.SMTPSender = cfg.SMTPSender

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

	projectMgr, err := tenant.NewProjectManager(cfg.DataDir, cfg)
	if err != nil {
		logger.Error("Failed to initialize project manager", "error", err)
		os.Exit(1)
	}
	defer projectMgr.Close()

	realtimeHub := realtime.NewHub()
	realtimeHub.Start()
	defer realtimeHub.Stop()

	logger.Info("Realtime hub started")

	hookManager := plugin.NewHookManager()
	app := plugin.NewApp(hookManager)

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

	server.SetRealtimeHub(realtimeHub, cfg.JWTSecret)
	logger.Info("Realtime WebSocket endpoint registered at /realtime/v1/ws")

	adminMux := http.NewServeMux()
	adminServer := api.NewAdminServer(projectMgr, cfg, cfg.JWTSecret, cfg.AdminEmail, cfg.AdminPassword)

	adminStore := auth.NewAdminStore(engine.DB(), cfg.AdminEmail, cfg.AdminPassword)
	if cfg.AdminEmailFromEnv || cfg.AdminPasswordFromEnv {
		if err := adminStore.SyncFromEnv(cfg.AdminEmail, cfg.AdminPassword, cfg.AdminEmailFromEnv, cfg.AdminPasswordFromEnv); err != nil {
			logger.Warn("Failed to sync admin credentials from environment", "error", err)
		} else if cfg.AdminEmailFromEnv || cfg.AdminPasswordFromEnv {
			logger.Info("Admin credentials managed by environment variables")
		}
	}
	adminServer.SetAdminStore(adminStore)

	auditStore := auth.NewAuditStore(engine.DB())
	adminServer.SetAuditStore(auditStore)

	meterStore, err := metering.OpenMeterStore(filepath.Join(cfg.DataDir, "metering"))
	if err != nil {
		logger.Error("Failed to initialize metering store", "error", err)
		os.Exit(1)
	}
	defer meterStore.Close()
	logger.Info("Metering store initialized")

	server.SetMeterStore(meterStore)
	adminServer.SetMeterStore(meterStore)
	adminServer.SetRealtimeHub(realtimeHub)

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

	adminServer.SetTeamStore(projectMgr.GetTeamStore())

	emailLogStore := email.NewLogStore(engine.DB())
	adminServer.SetEmailLogStore(emailLogStore)

	mailer := email.NewMailer(email.Config{
		Provider:  email.Provider(cfg.EmailProvider),
		Sender:    cfg.SMTPSender,
		SMTPHost:  cfg.SMTPHost,
		SMTPPort:  cfg.SMTPPort,
		SMTPUser:  cfg.SMTPUser,
		SMTPPass:  cfg.SMTPPassword,
		APIKey:    cfg.EmailAPIKey,
		APISecret: cfg.EmailAPISecret,
	})
	loggedMailer := email.NewLoggedMailer(mailer, emailLogStore, cfg.EmailProvider, cfg.SMTPSender)
	adminServer.SetMailer(loggedMailer)

	adminServer.RegisterRoutes(adminMux)

	adminMux.Handle("/admin/plugins", adminServer.AdminAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := plugin.PluginInfo{
			Plugins: app.Plugins(),
			Hooks:   hookManager.Info(),
			Routes:  []plugin.RouteInfo{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var masterJobsHandle *masterJobs

	if cfg.IsReplicationEnabled() {
		replCluster, err := startReplication(ctx, cfg, engine)
		if err != nil {
			logger.Error("Failed to start replication", "error", err)
			os.Exit(1)
		}

		projectMgr.SetCluster(replCluster)
		setupReplicationRealtime(realtimeHub, replCluster)

		server.SetCluster(replCluster)
		adminServer.SetCluster(replCluster)
		server.SetAdminReplicationMiddleware(adminServer.ReplicationMiddleware)

		replicatedDB := replication.NewReplicatedDB(engine, replCluster)
		server.SetReplicatedDB(replicatedDB)

		adminServer.SetReplicationStatus(&api.ReplicationStatus{
			Enabled: true,
			Role:    string(replCluster.Role()),
			NodeID:  cfg.NodeID,
			Peers:   len(cfg.Peers),
		})
		logger.Info("Replication cluster started", "role", replCluster.Role())

		if replCluster.Role() == replication.RoleMaster {
			masterJobsHandle = startMasterBackgroundJobs(ctx, logger, cfg, projectMgr, meterStore)
		}

		replCluster.SetOnPromote(func() {
			projectMgr.InvalidateEnvCache()
			if masterJobsHandle != nil {
				masterJobsHandle.Stop()
			}
			masterJobsHandle = startMasterBackgroundJobs(ctx, logger, cfg, projectMgr, meterStore)
			logger.Info("Master background jobs restarted after promotion")
		})
	} else {
		masterJobsHandle = startMasterBackgroundJobs(ctx, logger, cfg, projectMgr, meterStore)
		logger.Info("Cron schedulers started", "projects", projectMgr.ProjectCount())
	}

	server.RegisterAdmin(adminMux)
	logger.Info("Admin API registered")

	server.SetDashboard(dashboard.Handler())
	logger.Info("Dashboard registered")

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	restartCh := make(chan struct{}, 1)
	adminServer.OnRestart = func() {
		logger.Info("Restart requested via dashboard — respawning...")
		restartCh <- struct{}{}
		cancel()
	}

	serverErrCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	select {
	case err := <-serverErrCh:
		logger.Error("Server error", "error", err)
	case <-ctx.Done():
		logger.Info("Graceful shutdown initiated")
	}

	if masterJobsHandle != nil {
		masterJobsHandle.Stop()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server shutdown error", "error", err)
	}

	if err := meterStore.Close(); err != nil {
		logger.Error("MeterStore close error", "error", err)
	}

	select {
	case <-restartCh:
		projectMgr.Close()
		engine.Close()
		engineClosed = true
		exe, err := os.Executable()
		if err != nil {
			exe = os.Args[0]
		}
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(exe), ".exe") {
			exe += ".exe"
		}
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if startErr := cmd.Start(); startErr != nil {
			logger.Error("Failed to restart process", "error", startErr)
		}
		os.Exit(0)
	default:
	}

	logger.Info("Sovrabase stopped")
}
