package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
	"github.com/ketsuna-org/sovrabase/internal/infra/store/sqlstore"
)

func TestAdminAPIRequiresToken(t *testing.T) {
	mux, _ := buildAdminTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/admin/v1/users status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRootCanCreateAdmin(t *testing.T) {
	mux, _ := buildAdminTestMux(t)

	rootToken, _ := bootstrapAndGetAuth(t, mux)

	body := bytes.NewBufferString(`{"email":"admin2@example.com","password":"very-strong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/admins", body)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/admin/v1/admins status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
}

func TestRootImmutableCannotBeDeleted(t *testing.T) {
	mux, _ := buildAdminTestMux(t)

	rootToken, rootID := bootstrapAndGetAuth(t, mux)

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/users/"+rootID, nil)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("DELETE /api/admin/v1/users/{root} status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestRoleHierarchyCycleRejected(t *testing.T) {
	mux, _ := buildAdminTestMux(t)

	rootToken, _ := bootstrapAndGetAuth(t, mux)

	roleAID := createRoleForTest(t, mux, rootToken, `{"name":"roleA","description":"A"}`)
	roleBID := createRoleForTest(t, mux, rootToken, `{"name":"roleB","description":"B","parent_role_id":"`+roleAID+`"}`)

	body := bytes.NewBufferString(`{"parent_role_id":"` + roleBID + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/v1/roles/"+roleAID, body)
	req.Header.Set("Authorization", "Bearer "+rootToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("PUT /api/admin/v1/roles/{roleA} cycle status = %d, want %d body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func buildAdminTestMux(t *testing.T) (*http.ServeMux, *sqlstore.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "sovrabase-admin-api.db")
	store, err := sqlstore.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	authService, err := coreauth.NewService(coreauth.ServiceDeps{
		Store:     store,
		JWTSecret: "test-jwt-secret",
		TokenTTL:  24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	mux := http.NewServeMux()
	if err := RegisterRoutes(mux, Dependencies{
		Config:                  config.Default(),
		AuthService:             authService,
		MetadataPinger:          store,
		Logger:                  &fakeLogger{},
		JWTSecret:               "test-jwt-secret",
		EncryptionKeyConfigured: true,
		JWTSigningKeyConfigured: true,
	}); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	return mux, store
}

func bootstrapAndGetAuth(t *testing.T, mux *http.ServeMux) (string, string) {
	t.Helper()

	body := bytes.NewBufferString(`{"email":"admin@example.com","password":"very-strong-password"}`)
	req := httptest.NewRequest(http.MethodPost, "/config", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /config status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode bootstrap response error = %v", err)
	}
	if payload.AccessToken == "" {
		t.Fatalf("bootstrap token is empty")
	}
	if payload.User.ID == "" {
		t.Fatalf("bootstrap user id is empty")
	}
	return payload.AccessToken, payload.User.ID
}

func createRoleForTest(t *testing.T, mux *http.ServeMux, token, payload string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/roles", bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/admin/v1/roles status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var role struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &role); err != nil {
		t.Fatalf("decode role response error = %v", err)
	}
	if role.ID == "" {
		t.Fatalf("created role id is empty")
	}
	return role.ID
}
