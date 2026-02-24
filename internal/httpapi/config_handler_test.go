package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func TestGetConfigBootstrapRequired(t *testing.T) {
	authSvc := &fakeAuthService{bootstrapRequired: true}
	logger := &fakeLogger{}
	cfg := config.Default()
	cfg.Metadata.Driver = "postgres"
	cfg.Metadata.Postgres.DSN = "postgres://user:pass@db:5432/sovrabase?sslmode=disable"
	cfg.Provisioning.Docker.Endpoint = "unix:///var/run/docker.sock"
	cfg.Provisioning.Docker.HostAddress = "10.0.0.1"
	cfg.Provisioning.Docker.NetworkName = "my-private-net"

	mux := http.NewServeMux()
	err := RegisterRoutes(mux, Dependencies{
		Config:                  cfg,
		AuthService:             authSvc,
		MetadataPinger:          fakePinger{err: nil},
		Logger:                  logger,
		JWTSecret:               "test-secret",
		EncryptionKeyConfigured: true,
		JWTSigningKeyConfigured: true,
	})
	if err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /config status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response error = %v", err)
	}

	if payload["bootstrap_required"] != true {
		t.Fatalf("bootstrap_required = %v, want true", payload["bootstrap_required"])
	}
	authSection := payload["auth"].(map[string]any)
	if authSection["mode"] != "bootstrap" {
		t.Fatalf("auth.mode = %v, want bootstrap", authSection["mode"])
	}

	configSection := payload["config"].(map[string]any)
	provisioning := configSection["provisioning"].(map[string]any)
	docker := provisioning["docker"].(map[string]any)
	if docker["endpoint"] != "[redacted]" || docker["host_address"] != "[redacted]" || docker["network_name"] != "[redacted]" {
		t.Fatalf("docker redaction failed: %+v", docker)
	}

	text := rec.Body.String()
	if strings.Contains(text, "postgres://user:pass@db:5432/sovrabase?sslmode=disable") {
		t.Fatalf("response leaked postgres dsn: %s", text)
	}
	if strings.Contains(text, "unix:///var/run/docker.sock") || strings.Contains(text, "10.0.0.1") || strings.Contains(text, "my-private-net") {
		t.Fatalf("response leaked docker runtime settings: %s", text)
	}
	if strings.Contains(text, "SOVRABASE_MASTER_KEY") || strings.Contains(text, "SOVRABASE_JWT_SECRET") {
		t.Fatalf("response leaked secret env names: %s", text)
	}
	if len(logger.logs) == 0 {
		t.Fatalf("expected reminder log for bootstrap-required GET /config")
	}
}

func TestGetConfigConfigured(t *testing.T) {
	authSvc := &fakeAuthService{bootstrapRequired: false}

	mux := http.NewServeMux()
	err := RegisterRoutes(mux, Dependencies{
		Config:                  config.Default(),
		AuthService:             authSvc,
		MetadataPinger:          fakePinger{err: nil},
		Logger:                  &fakeLogger{},
		JWTSecret:               "test-secret",
		EncryptionKeyConfigured: true,
		JWTSigningKeyConfigured: true,
	})
	if err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /config status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response error = %v", err)
	}
	if payload["bootstrap_required"] != false {
		t.Fatalf("bootstrap_required = %v, want false", payload["bootstrap_required"])
	}
	authSection := payload["auth"].(map[string]any)
	if authSection["mode"] != "login" {
		t.Fatalf("auth.mode = %v, want login", authSection["mode"])
	}
}

type fakeAuthService struct {
	bootstrapRequired bool

	bootstrapResult coreauth.AuthResult
	bootstrapErr    error
	loginResult     coreauth.AuthResult
	loginErr        error
}

func (f *fakeAuthService) GetConfigState(context.Context) (bool, error) {
	return f.bootstrapRequired, nil
}

func (f *fakeAuthService) BootstrapFirstAdmin(context.Context, string, string) (coreauth.AuthResult, error) {
	return f.bootstrapResult, f.bootstrapErr
}

func (f *fakeAuthService) Login(context.Context, string, string) (coreauth.AuthResult, error) {
	return f.loginResult, f.loginErr
}

func (f *fakeAuthService) CreateAdmin(context.Context, string, string, string) (coreauth.User, error) {
	return coreauth.User{}, nil
}

func (f *fakeAuthService) CreateUser(context.Context, coreauth.CreateUserInput) (coreauth.User, error) {
	return coreauth.User{}, nil
}

func (f *fakeAuthService) ListUsers(context.Context, string) ([]coreauth.User, error) {
	return []coreauth.User{}, nil
}

func (f *fakeAuthService) GetUser(context.Context, string, string) (coreauth.User, error) {
	return coreauth.User{}, nil
}

func (f *fakeAuthService) UpdateUser(context.Context, coreauth.UpdateUserInput) (coreauth.User, error) {
	return coreauth.User{}, nil
}

func (f *fakeAuthService) DeleteUser(context.Context, string, string) error {
	return nil
}

func (f *fakeAuthService) CreateRole(context.Context, coreauth.CreateRoleInput) (coreauth.RoleRecord, error) {
	return coreauth.RoleRecord{}, nil
}

func (f *fakeAuthService) ListRoles(context.Context, string) ([]coreauth.RoleRecord, error) {
	return []coreauth.RoleRecord{}, nil
}

func (f *fakeAuthService) GetRole(context.Context, string, string) (coreauth.RoleRecord, error) {
	return coreauth.RoleRecord{}, nil
}

func (f *fakeAuthService) UpdateRole(context.Context, coreauth.UpdateRoleInput) (coreauth.RoleRecord, error) {
	return coreauth.RoleRecord{}, nil
}

func (f *fakeAuthService) DeleteRole(context.Context, string, string) error {
	return nil
}

func (f *fakeAuthService) CreateScope(context.Context, coreauth.CreateScopeInput) (coreauth.ScopeRecord, error) {
	return coreauth.ScopeRecord{}, nil
}

func (f *fakeAuthService) ListScopes(context.Context, string) ([]coreauth.ScopeRecord, error) {
	return []coreauth.ScopeRecord{}, nil
}

func (f *fakeAuthService) GetScope(context.Context, string, string) (coreauth.ScopeRecord, error) {
	return coreauth.ScopeRecord{}, nil
}

func (f *fakeAuthService) UpdateScope(context.Context, coreauth.UpdateScopeInput) (coreauth.ScopeRecord, error) {
	return coreauth.ScopeRecord{}, nil
}

func (f *fakeAuthService) DeleteScope(context.Context, string, string) error {
	return nil
}

func (f *fakeAuthService) AssignRoleToUser(context.Context, string, string, string) error {
	return nil
}

func (f *fakeAuthService) RemoveRoleFromUser(context.Context, string, string, string) error {
	return nil
}

func (f *fakeAuthService) AssignScopeToRole(context.Context, string, string, string) error {
	return nil
}

func (f *fakeAuthService) RemoveScopeFromRole(context.Context, string, string, string) error {
	return nil
}

func (f *fakeAuthService) Authorize(context.Context, string, string) error {
	return nil
}

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error {
	return f.err
}

type fakeLogger struct {
	logs []string
}

func (f *fakeLogger) Printf(format string, args ...any) {
	f.logs = append(f.logs, format)
}
