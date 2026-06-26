package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"github.com/ketsuna-org/sovrabase/plugin"
)

func TestServerTenantRouting(t *testing.T) {
	// 1. Setup temporary directory for the control plane and projects
	dir, err := os.MkdirTemp("", "sovrabase-server-tenant-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// 2. Initialize project manager
	pm, err := tenant.NewProjectManager(dir, &config.Config{JWTSecret: "test-secret"})
	if err != nil {
		t.Fatalf("failed to create project manager: %v", err)
	}
	defer pm.Close()

	// 3. Create a project
	proj, err := pm.CreateProject("isolated-project", "owner-1")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// 4. Setup mock global services (fallbacks, not used by active project)
	globalDB, _ := db.NewEngine(filepath.Join(dir, "global_db"))
	defer globalDB.Close()
	globalAuth := auth.NewService("global_secret", auth.NewInMemoryUserStore())
	globalStorage, _ := storage.NewLocalDriver(filepath.Join(dir, "global_storage"), "")

	// 5. Create the API server
	cfg := &Config{
		ListenAddr:   ":8080",
		AllowOrigins: "*",
		JWTSecret:    "global_secret",
	}
	server := NewServer(cfg, globalDB, WrapAuthService(globalAuth), WrapStorageDriver(globalStorage), pm, plugin.NewHookManager())

	// 6. Test Request to /auth/v1/signup WITH X-Project-Key
	signUpData := map[string]string{
		"email":    "user@tenant.com",
		"password": "securepassword123",
	}
	signUpBody, _ := json.Marshal(signUpData)
	req := httptest.NewRequest("POST", "/auth/v1/signup", bytes.NewReader(signUpBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-Key", proj.JWTSecret)

	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected SignUp status 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	var signUpResp struct {
		User  map[string]interface{} `json:"user"`
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if err := json.NewDecoder(w.Body).Decode(&signUpResp); err != nil {
		t.Fatalf("failed to decode signup response: %v", err)
	}

	userToken := signUpResp.Token.AccessToken
	if userToken == "" {
		t.Fatal("expected non-empty user access token")
	}

	// Verify that the user was NOT created in the global auth service
	_, err = globalAuth.SignIn("user@tenant.com", "securepassword123")
	if err == nil {
		t.Fatal("expected error when trying to authenticate project user against global auth service")
	}

	// Create collection in the project database
	env, err := pm.GetProjectEnv(proj.ID)
	if err != nil {
		t.Fatalf("failed to get project env: %v", err)
	}
	if err := env.Engine.CreateCollection("items"); err != nil {
		t.Fatalf("failed to create collection: %v", err)
	}

	// 7. Test Request to /api/v1/collections/items (Insert Document)
	docData := map[string]interface{}{
		"title": "isolated document",
	}
	docBody, _ := json.Marshal(docData)
	req = httptest.NewRequest("POST", "/api/v1/collections/items", bytes.NewReader(docBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-Key", proj.JWTSecret)
	req.Header.Set("Authorization", "Bearer "+userToken)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected Insert status 201 Created, got %d. Body: %s", w.Code, w.Body.String())
	}

	var createdDoc map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&createdDoc); err != nil {
		t.Fatalf("failed to decode created document: %v", err)
	}

	docID, ok := createdDoc["_id"].(string)
	if !ok || docID == "" {
		t.Fatal("expected non-empty document ID in response")
	}

	// 8. Verify the document is isolated in the tenant's Pebble DB
	env, err = pm.GetProjectEnv(proj.ID)
	if err != nil {
		t.Fatalf("failed to get project env: %v", err)
	}

	docFromTenant, err := env.Engine.Get("items", docID)
	if err != nil {
		t.Fatalf("failed to find document in tenant's engine: %v", err)
	}
	if docFromTenant["title"] != "isolated document" {
		t.Fatalf("wrong document title retrieved from tenant DB: %v", docFromTenant["title"])
	}

	// Verify that the document does NOT exist in the global database
	docFromGlobal, err := globalDB.Get("items", docID)
	if err != nil {
		t.Fatalf("unexpected error checking global DB: %v", err)
	}
	if docFromGlobal != nil {
		t.Fatal("expected document to be absent in the global database")
	}

	// 9. Verify authentication fails if X-Project-Key is missing or wrong
	// Missing project key
	req = httptest.NewRequest("POST", "/api/v1/collections/items", bytes.NewReader(docBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing project key, got %d", w.Code)
	}

	// Wrong project key
	req = httptest.NewRequest("POST", "/api/v1/collections/items", bytes.NewReader(docBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Project-Key", "invalid-key")
	req.Header.Set("Authorization", "Bearer "+userToken)

	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for invalid project key, got %d", w.Code)
	}
}
