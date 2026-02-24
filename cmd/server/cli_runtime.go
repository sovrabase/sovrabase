package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

const defaultSQLiteMetadataPath = "/data/sovrabase.db"

type cliRuntime struct {
	config      config.Config
	authService coreauth.Service
	stateStore  cliStateStore
	close       func()
}

func newCLIRuntime(ctx context.Context, cfg config.Config) (cliRuntime, error) {
	cfg, err := normalizeCLIConfig(cfg)
	if err != nil {
		return cliRuntime{}, err
	}

	metadataStore, err := openAndPrepareMetadataStore(ctx, cfg)
	if err != nil {
		return cliRuntime{}, err
	}

	jwtSecret, err := resolveRequiredSecret(cfg.Auth.JWTSecretEnv)
	if err != nil {
		_ = metadataStore.Close()
		return cliRuntime{}, fmt.Errorf("jwt secret error: %w", err)
	}

	authService, err := buildAuthService(metadataStore, jwtSecret)
	if err != nil {
		_ = metadataStore.Close()
		return cliRuntime{}, err
	}

	stateStore, err := newCLIStateStore()
	if err != nil {
		_ = metadataStore.Close()
		return cliRuntime{}, err
	}

	return cliRuntime{
		config:      cfg,
		authService: authService,
		stateStore:  stateStore,
		close: func() {
			_ = metadataStore.Close()
		},
	}, nil
}

func normalizeCLIConfig(cfg config.Config) (config.Config, error) {
	if strings.TrimSpace(cfg.Metadata.Driver) != "sqlite" {
		return cfg, nil
	}

	metadataPath := strings.TrimSpace(cfg.Metadata.SQLite.Path)
	if metadataPath != "" && metadataPath != defaultSQLiteMetadataPath {
		return cfg, nil
	}

	root, err := os.UserConfigDir()
	if err != nil {
		return config.Config{}, fmt.Errorf("resolve user config dir: %w", err)
	}
	localPath := filepath.Join(root, cliStateDirName, "metadata.db")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
		return config.Config{}, fmt.Errorf("create metadata directory: %w", err)
	}
	cfg.Metadata.SQLite.Path = localPath
	return cfg, nil
}
