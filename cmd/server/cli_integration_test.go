package main

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAdminBootstrapCreateAndListUsers(t *testing.T) {
	t.Setenv("SOVRABASE_METADATA_SQLITE_PATH", filepath.Join(t.TempDir(), "cli.db"))
	t.Setenv("SOVRABASE_JWT_SECRET", "cli-test-secret")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	logger := log.New(bytes.NewBuffer(nil), "", 0)
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	err := runCLI(ctx, []string{"admin", "bootstrap", "--name", "admin@example.com", "--password", "very-strong-password"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI bootstrap error = %v", err)
	}
	if !strings.Contains(stdout.String(), "bootstrapped") {
		t.Fatalf("bootstrap output missing status, got %q", stdout.String())
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"admin", "create-user", "--name", "user@example.com", "--password", "another-strong-password", "--role", "admin"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI create-user error = %v", err)
	}
	if !strings.Contains(stdout.String(), "user@example.com") {
		t.Fatalf("create-user output missing user email, got %q", stdout.String())
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"admin", "list-users"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI list-users error = %v", err)
	}
	if !strings.Contains(stdout.String(), "admin@example.com") || !strings.Contains(stdout.String(), "user@example.com") {
		t.Fatalf("list-users output missing expected users, got %q", stdout.String())
	}
}

func TestCLIAdminCommandsRequireBootstrap(t *testing.T) {
	t.Setenv("SOVRABASE_METADATA_SQLITE_PATH", filepath.Join(t.TempDir(), "cli-require-bootstrap.db"))
	t.Setenv("SOVRABASE_JWT_SECRET", "cli-test-secret")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	logger := log.New(bytes.NewBuffer(nil), "", 0)
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	err := runCLI(ctx, []string{"admin", "list-users"}, stdout, stderr, logger)
	if err == nil {
		t.Fatalf("runCLI list-users error = nil, want bootstrap error")
	}
	if !strings.Contains(err.Error(), bootstrapRequiredMessage) {
		t.Fatalf("unexpected error = %v", err)
	}
}
