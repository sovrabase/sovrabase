package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"path/filepath"
	"testing"
)

func TestCLIAdminRoleScopeAndAssignments(t *testing.T) {
	t.Setenv("SOVRABASE_METADATA_SQLITE_PATH", filepath.Join(t.TempDir(), "cli-rbac.db"))
	t.Setenv("SOVRABASE_JWT_SECRET", "cli-test-secret")
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	logger := log.New(bytes.NewBuffer(nil), "", 0)
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)

	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "bootstrap", "--name", "admin@example.com", "--password", "very-strong-password")
	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "create-user", "--name", "user@example.com", "--password", "another-strong-password", "--role", "user")
	userID := decodeUserID(t, stdout.Bytes())

	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "create-role", "--name", "manager", "--description", "Manager role")
	roleID := decodeIDField(t, stdout.Bytes(), "ID")

	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "create-scope", "--key", "users.read", "--description", "Read users")
	scopeID := decodeIDField(t, stdout.Bytes(), "ID")

	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "assign-role", "--user-id", userID, "--role-id", roleID)
	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "assign-scope", "--role-id", roleID, "--scope-id", scopeID)
	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "remove-scope", "--role-id", roleID, "--scope-id", scopeID)
	mustRunCLI(t, ctx, logger, stdout, stderr, "admin", "remove-role", "--user-id", userID, "--role-id", roleID)
}

func mustRunCLI(t *testing.T, ctx context.Context, logger *log.Logger, stdout, stderr *bytes.Buffer, args ...string) {
	t.Helper()
	stdout.Reset()
	stderr.Reset()
	if err := runCLI(ctx, args, stdout, stderr, logger); err != nil {
		t.Fatalf("runCLI(%v) error = %v", args, err)
	}
}

func decodeUserID(t *testing.T, data []byte) string {
	t.Helper()
	var user struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		t.Fatalf("decode user json error = %v (body=%s)", err, string(data))
	}
	if user.ID == "" {
		t.Fatalf("decoded user id empty (body=%s)", string(data))
	}
	return user.ID
}

func decodeIDField(t *testing.T, data []byte, key string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode json error = %v (body=%s)", err, string(data))
	}
	value, _ := payload[key].(string)
	if value == "" {
		t.Fatalf("decoded id field %q empty (body=%s)", key, string(data))
	}
	return value
}
