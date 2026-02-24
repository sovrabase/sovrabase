package main

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

func TestCLIBootstrapUsesLocalMetadataPathWhenDefaultIsUnusable(t *testing.T) {
	t.Setenv("SOVRABASE_JWT_SECRET", "cli-test-secret")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SOVRABASE_METADATA_SQLITE_PATH", "")

	logger := log.New(bytes.NewBuffer(nil), "", 0)
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	err := runCLI(ctx, []string{"admin", "bootstrap", "--name", "admin@example.com", "--password", "very-strong-password"}, stdout, stderr, logger)
	if err != nil {
		t.Fatalf("runCLI bootstrap error = %v", err)
	}
	if !strings.Contains(stdout.String(), "bootstrapped") {
		t.Fatalf("unexpected bootstrap output: %q", stdout.String())
	}
}
