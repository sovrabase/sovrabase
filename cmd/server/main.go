// Package main Sovrabase API
//
//	@title			Sovrabase API
//	@version		1.0
//	@description	This is the Sovrabase API server.
//	@host			localhost:9056
//	@BasePath		/
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
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
	"github.com/ketsuna-org/sovrabase/internal/core/security"
	mongoadapter "github.com/ketsuna-org/sovrabase/internal/infra/adapters/mongo"
	postgresadapter "github.com/ketsuna-org/sovrabase/internal/infra/adapters/postgres"
	dockerprovider "github.com/ketsuna-org/sovrabase/internal/infra/provisioning/docker"
	k8sprovider "github.com/ketsuna-org/sovrabase/internal/infra/provisioning/kubernetes"
	"github.com/ketsuna-org/sovrabase/internal/infra/store/sqlstore"
)

func main() {
	logger := log.New(os.Stdout, "[sovrabase] ", log.LstdFlags|log.Lmsgprefix)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("configuration error: %v", err)
	}

	masterKey, err := security.ResolveMasterKeyFromEnv(cfg.Core.MasterKeyEnv)
	if err != nil {
		logger.Fatalf("master key error: %v", err)
	}
	cipher, err := security.NewDSNCipher(masterKey)
	if err != nil {
		logger.Fatalf("cipher init error: %v", err)
	}

	metadataStore, err := openMetadataStore(cfg)
	if err != nil {
		logger.Fatalf("metadata store init error: %v", err)
	}
	defer func() {
		_ = metadataStore.Close()
	}()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer startupCancel()

	if err := metadataStore.Ping(startupCtx); err != nil {
		logger.Fatalf("metadata store ping failed: %v", err)
	}
	if err := metadataStore.Migrate(startupCtx); err != nil {
		logger.Fatalf("metadata migration failed: %v", err)
	}

	targetAdapters, err := adapters.NewRegistry(
		postgresadapter.NewAdapter(5*time.Second),
		mongoadapter.NewAdapter(5*time.Second),
	)
	if err != nil {
		logger.Fatalf("adapters init error: %v", err)
	}

	cacheTTL, _ := cfg.CacheTTLDuration()
	sweepInterval, _ := cfg.SweepDuration()
	runtimeRegistry := connections.NewRegistry(cacheTTL, sweepInterval)
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = runtimeRegistry.Stop(stopCtx)
	}()

	var provisioners []provisioning.Provisioner
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
			logger.Fatalf("docker provisioner init error: %v", err)
		}
		defer func() {
			_ = dockerProvisioner.Close()
		}()
		provisioners = append(provisioners, dockerProvisioner)
	}
	provisioners = append(provisioners, k8sprovider.NewProvider())

	service, err := connections.NewService(connections.ServiceDeps{
		Store:        metadataStore,
		Adapters:     targetAdapters,
		Provisioners: provisioners,
		Registry:     runtimeRegistry,
		Cipher:       cipher,
	})
	if err != nil {
		logger.Fatalf("connection service init error: %v", err)
	}
	_ = service

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := metadataStore.Ping(ctx); err != nil {
			http.Error(w, "metadata store unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	address := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("server listening on %s", address)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			logger.Fatalf("http server failed: %v", listenErr)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignal

	logger.Printf("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("http shutdown error: %v", err)
	}
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
