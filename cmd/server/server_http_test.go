package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
	"github.com/ketsuna-org/sovrabase/internal/httpapi"
	"github.com/ketsuna-org/sovrabase/internal/infra/store/sqlstore"
)

func TestServerBootstrapAndLoginFlow(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sovrabase-server-flow.db")
	store, err := sqlstore.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	authService, err := coreauth.NewService(coreauth.ServiceDeps{
		Store:     store,
		JWTSecret: "test-jwt-secret",
		TokenTTL:  24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	mux := http.NewServeMux()
	if err := httpapi.RegisterRoutes(mux, httpapi.Dependencies{
		Config:                  config.Default(),
		AuthService:             authService,
		MetadataPinger:          store,
		Logger:                  testLogger{},
		JWTSecret:               "test-jwt-secret",
		EncryptionKeyConfigured: true,
		JWTSigningKeyConfigured: true,
	}); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	getConfigResp := executeRequest(mux, http.MethodGet, "/config", nil)
	if getConfigResp.Code != http.StatusOK || !bytes.Contains(getConfigResp.Body.Bytes(), []byte(`"bootstrap_required":true`)) {
		t.Fatalf("initial GET /config unexpected response: code=%d body=%s", getConfigResp.Code, getConfigResp.Body.String())
	}

	bootstrapBody := bytes.NewBufferString(`{"email":"admin@example.com","password":"very-strong-password"}`)
	bootstrapResp := executeRequest(mux, http.MethodPost, "/config", bootstrapBody)
	if bootstrapResp.Code != http.StatusCreated {
		t.Fatalf("POST /config expected %d, got %d body=%s", http.StatusCreated, bootstrapResp.Code, bootstrapResp.Body.String())
	}

	getConfigAfterResp := executeRequest(mux, http.MethodGet, "/config", nil)
	if getConfigAfterResp.Code != http.StatusOK || !bytes.Contains(getConfigAfterResp.Body.Bytes(), []byte(`"bootstrap_required":false`)) {
		t.Fatalf("post-bootstrap GET /config unexpected response: code=%d body=%s", getConfigAfterResp.Code, getConfigAfterResp.Body.String())
	}

	loginBody := bytes.NewBufferString(`{"email":"admin@example.com","password":"very-strong-password"}`)
	loginResp := executeRequest(mux, http.MethodPost, "/auth/login", loginBody)
	if loginResp.Code != http.StatusOK || !bytes.Contains(loginResp.Body.Bytes(), []byte(`"access_token"`)) {
		t.Fatalf("POST /auth/login unexpected response: code=%d body=%s", loginResp.Code, loginResp.Body.String())
	}
}

func executeRequest(handler http.Handler, method, path string, body *bytes.Buffer) *httptest.ResponseRecorder {
	var payload *bytes.Reader
	if body == nil {
		payload = bytes.NewReader(nil)
	} else {
		payload = bytes.NewReader(body.Bytes())
	}
	req := httptest.NewRequest(method, path, payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type testLogger struct{}

func (testLogger) Printf(string, ...any) {}
