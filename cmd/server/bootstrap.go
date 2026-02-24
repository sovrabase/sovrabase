package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/core/adapters"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
	"github.com/ketsuna-org/sovrabase/internal/core/security"
	"github.com/ketsuna-org/sovrabase/internal/httpapi"
	mongoadapter "github.com/ketsuna-org/sovrabase/internal/infra/adapters/mongo"
	postgresadapter "github.com/ketsuna-org/sovrabase/internal/infra/adapters/postgres"
	dockerprovider "github.com/ketsuna-org/sovrabase/internal/infra/provisioning/docker"
	k8sprovider "github.com/ketsuna-org/sovrabase/internal/infra/provisioning/kubernetes"
	"github.com/ketsuna-org/sovrabase/internal/infra/store/sqlstore"
)

const (
	startupTimeout    = 30 * time.Second
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	adapterProbeTTL   = 5 * time.Second
	authTokenTTL      = 24 * time.Hour
)

func runServerLifecycle(ctx context.Context, cfg config.Config, logger *log.Logger) error {
	startupCtx, startupCancel := context.WithTimeout(ctx, startupTimeout)
	defer startupCancel()

	cipher, jwtSecret, err := buildSecurity(cfg)
	if err != nil {
		return err
	}

	metadataStore, err := openAndPrepareMetadataStore(startupCtx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		_ = metadataStore.Close()
	}()

	authService, err := buildAuthService(metadataStore, jwtSecret)
	if err != nil {
		return err
	}

	runtimeRegistry, stopRuntimeRegistry, err := buildConnectionRuntime(cfg)
	if err != nil {
		return err
	}
	defer stopRuntimeRegistry()

	provisioners, closeProvisioners, err := buildProvisioners(cfg)
	if err != nil {
		return err
	}
	defer closeProvisioners()

	if err := initializeConnectionService(metadataStore, runtimeRegistry, provisioners, cipher); err != nil {
		return err
	}

	if err := logBootstrapState(startupCtx, authService, logger); err != nil {
		return err
	}

	handler, err := buildHTTPHandler(cfg, authService, metadataStore, logger, jwtSecret)
	if err != nil {
		return err
	}

	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- runHTTPServer(serverCtx, cfg, handler, logger)
	}()

	waitForShutdownSignal()
	logger.Printf("shutdown signal received")
	serverCancel()

	if err := <-serverErrCh; err != nil {
		return err
	}

	return nil
}

func loadRuntimeConfig(_ *log.Logger) (config.Config, error) {
	return config.Load()
}

func buildSecurity(cfg config.Config) (*security.DSNCipher, string, error) {
	masterKey, err := security.ResolveMasterKeyFromEnv(cfg.Core.MasterKeyEnv)
	if err != nil {
		return nil, "", fmt.Errorf("master key error: %w", err)
	}
	cipher, err := security.NewDSNCipher(masterKey)
	if err != nil {
		return nil, "", fmt.Errorf("cipher init error: %w", err)
	}
	jwtSecret, err := resolveRequiredSecret(cfg.Auth.JWTSecretEnv)
	if err != nil {
		return nil, "", fmt.Errorf("jwt secret error: %w", err)
	}

	return cipher, jwtSecret, nil
}

func openAndPrepareMetadataStore(ctx context.Context, cfg config.Config) (*sqlstore.Store, error) {
	metadataStore, err := openMetadataStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("metadata store init error: %w", err)
	}
	if err := metadataStore.Ping(ctx); err != nil {
		_ = metadataStore.Close()
		return nil, fmt.Errorf("metadata store ping failed: %w", err)
	}
	if err := metadataStore.Migrate(ctx); err != nil {
		_ = metadataStore.Close()
		return nil, fmt.Errorf("metadata migration failed: %w", err)
	}

	return metadataStore, nil
}

func buildAuthService(store *sqlstore.Store, jwtSecret string) (coreauth.Service, error) {
	authService, err := coreauth.NewService(coreauth.ServiceDeps{
		Store:     store,
		JWTSecret: jwtSecret,
		TokenTTL:  authTokenTTL,
	})
	if err != nil {
		return nil, fmt.Errorf("auth service init error: %w", err)
	}
	return authService, nil
}

func buildConnectionRuntime(cfg config.Config) (*connections.Registry, func(), error) {
	cacheTTL, err := cfg.CacheTTLDuration()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cache ttl duration: %w", err)
	}
	sweepInterval, err := cfg.SweepDuration()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid sweep interval duration: %w", err)
	}

	runtimeRegistry := connections.NewRegistry(cacheTTL, sweepInterval)
	cleanup := func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = runtimeRegistry.Stop(stopCtx)
	}

	return runtimeRegistry, cleanup, nil
}

func buildProvisioners(cfg config.Config) ([]provisioning.Provisioner, func(), error) {
	var provisioners []provisioning.Provisioner
	cleanup := func() {}

	if cfg.Provisioning.Docker.Enabled {
		dockerProvisioner, err := dockerprovider.NewProvider(dockerprovider.Config{
			Endpoint:      cfg.Provisioning.Docker.Endpoint,
			Mode:          cfg.Provisioning.Docker.Mode,
			HostAddress:   cfg.Provisioning.Docker.HostAddress,
			NetworkName:   cfg.Provisioning.Docker.NetworkName,
			PostgresImage: cfg.Provisioning.Docker.PostgresImage,
			MongoImage:    cfg.Provisioning.Docker.MongoImage,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("docker provisioner init error: %w", err)
		}
		cleanup = func() {
			_ = dockerProvisioner.Close()
		}
		provisioners = append(provisioners, dockerProvisioner)
	}

	provisioners = append(provisioners, k8sprovider.NewProvider())
	return provisioners, cleanup, nil
}

func initializeConnectionService(
	store *sqlstore.Store,
	runtimeRegistry *connections.Registry,
	provisioners []provisioning.Provisioner,
	cipher *security.DSNCipher,
) error {
	targetAdapters, err := adapters.NewRegistry(
		postgresadapter.NewAdapter(adapterProbeTTL),
		mongoadapter.NewAdapter(adapterProbeTTL),
	)
	if err != nil {
		return fmt.Errorf("adapters init error: %w", err)
	}

	if _, err := connections.NewService(connections.ServiceDeps{
		Store:        store,
		Adapters:     targetAdapters,
		Provisioners: provisioners,
		Registry:     runtimeRegistry,
		Cipher:       cipher,
	}); err != nil {
		return fmt.Errorf("connection service init error: %w", err)
	}

	return nil
}

func logBootstrapState(ctx context.Context, authService coreauth.Service, logger *log.Logger) error {
	bootstrapRequired, err := authService.GetConfigState(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap state check failed: %w", err)
	}
	if bootstrapRequired {
		logger.Printf("no admin user configured. bootstrap required via POST /config")
	} else {
		logger.Printf("admin bootstrap already configured. login available via POST /auth/login")
	}
	return nil
}

func buildHTTPHandler(cfg config.Config, authService coreauth.Service, metadataStore *sqlstore.Store, logger *log.Logger, jwtSecret string) (http.Handler, error) {
	mux := http.NewServeMux()
	if err := httpapi.RegisterRoutes(mux, httpapi.Dependencies{
		Config:                  cfg,
		AuthService:             authService,
		MetadataPinger:          metadataStore,
		Logger:                  logger,
		JWTSecret:               jwtSecret,
		EncryptionKeyConfigured: true,
		JWTSigningKeyConfigured: true,
	}); err != nil {
		return nil, fmt.Errorf("register routes failed: %w", err)
	}

	return mux, nil
}

func runHTTPServer(ctx context.Context, cfg config.Config, handler http.Handler, logger *log.Logger) error {
	address := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("server listening on %s", address)
		listenErr := server.ListenAndServe()
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server failed: %w", listenErr)
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Printf("http shutdown error: %v", err)
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func waitForShutdownSignal() {
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownSignal)
	<-shutdownSignal
}

func resolveRequiredSecret(envName string) (string, error) {
	name := envName
	if name == "" {
		return "", errors.New("secret env variable name is required")
	}
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("missing required secret env %q", name)
	}
	return value, nil
}

func openMetadataStore(cfg config.Config) (*sqlstore.Store, error) {
	switch cfg.Metadata.Driver {
	case "sqlite":
		return sqlstore.OpenSQLite(cfg.Metadata.SQLite.Path)
	case "postgres":
		return sqlstore.OpenPostgres(cfg.Metadata.Postgres.DSN)
	default:
		return nil, fmt.Errorf("unsupported metadata store driver %q", cfg.Metadata.Driver)
	}
}
