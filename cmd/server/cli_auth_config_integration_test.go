package main

import (
	"bytes"
	"context"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIConfigStatusAndAuthLoginFlow(t *testing.T) {
	t.Setenv("SOVRABASE_METADATA_SQLITE_PATH", filepath.Join(t.TempDir(), "cli-auth-config.db"))
	t.Setenv("SOVRABASE_JWT_SECRET", "cli-test-secret")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	logger := log.New(bytes.NewBuffer(nil), "", 0)
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	err := runCLI(ctx, []string{"config", "status"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI config status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "\"bootstrap_required\": true") {
		t.Fatalf("unexpected config status output: %q", stdout.String())
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"auth", "login", "--name", "admin@example.com", "--password", "very-strong-password"}, stdout, stderr, logger)
	if err == nil {
		t.Fatalf("runCLI auth login error = nil, want bootstrap required")
	}
	if !strings.Contains(err.Error(), "bootstrap required") {
		t.Fatalf("unexpected login error = %v", err)
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"admin", "bootstrap", "--name", "admin@example.com", "--password", "very-strong-password"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI admin bootstrap error = %v", err)
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"auth", "login", "--name", "admin@example.com", "--password", "very-strong-password"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI auth login error = %v", err)
	}
	if !strings.Contains(stdout.String(), "logged_in") {
		t.Fatalf("unexpected login output: %q", stdout.String())
	}

	stdout.Reset()
	err = runCLI(ctx, []string{"config", "status"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI config status error = %v", err)
	}
	if !strings.Contains(stdout.String(), "\"bootstrap_required\": false") {
		t.Fatalf("unexpected config status output: %q", stdout.String())
	}
}
