package main

import (
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/config"
)

func TestBuildConnectionRuntimeInvalidDurations(t *testing.T) {
	cfg := config.Default()
	cfg.Core.CacheTTL = "not-a-duration"

	_, _, err := buildConnectionRuntime(cfg)
	if err == nil {
		t.Fatalf("buildConnectionRuntime() error = nil, want invalid cache ttl duration")
	}

	cfg = config.Default()
	cfg.Core.Sweep = "not-a-duration"

	_, _, err = buildConnectionRuntime(cfg)
	if err == nil {
		t.Fatalf("buildConnectionRuntime() error = nil, want invalid sweep interval duration")
	}
}

func TestBuildConnectionRuntimeCleanup(t *testing.T) {
	cfg := config.Default()
	runtimeRegistry, cleanup, err := buildConnectionRuntime(cfg)
	if err != nil {
		t.Fatalf("buildConnectionRuntime() error = %v", err)
	}
	if runtimeRegistry == nil {
		t.Fatalf("buildConnectionRuntime() registry = nil")
	}

	cleanup()
}

func TestBuildSecurityMissingJWTSecret(t *testing.T) {
	cfg := config.Default()
	cfg.Core.MasterKeyEnv = "TEST_MASTER_KEY_ENV"
	cfg.Auth.JWTSecretEnv = "TEST_JWT_SECRET_ENV"

	t.Setenv(cfg.Core.MasterKeyEnv, "0123456789abcdef0123456789abcdef")
	t.Setenv(cfg.Auth.JWTSecretEnv, "")

	_, _, err := buildSecurity(cfg)
	if err == nil {
		t.Fatalf("buildSecurity() error = nil, want jwt secret error")
	}
}
