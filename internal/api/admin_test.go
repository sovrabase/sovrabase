package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

func TestAdminFileDownload(t *testing.T) {
	// 1. Setup temporary directory
	dir, err := os.MkdirTemp("", "sovrabase-admin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// 2. Initialize project manager
	cfg := &config.Config{JWTSecret: "test-secret"}
	pm, err := tenant.NewProjectManager(dir, cfg)
	if err != nil {
		t.Fatalf("failed to create project manager: %v", err)
	}
	defer pm.Close()

	// 3. Create a project
	proj, err := pm.CreateProject("isolated-project", "owner-1")
	if err != nil {
		t.Fatalf("failed to create project: %v", err)
	}

	// 4. Create bucket and upload file in project storage
	env, err := pm.GetProjectEnv(proj.ID)
	if err != nil {
		t.Fatalf("failed to get project env: %v", err)
	}
	err = env.Storage.CreateBucket(context.Background(), "test-bucket")
	if err != nil {
		t.Fatalf("failed to create bucket: %v", err)
	}

	fileContent := "hello admin file download"
	_, err = env.Storage.Upload(context.Background(), "test-bucket", "docs/hello.txt", strings.NewReader(fileContent), "text/plain")
	if err != nil {
		t.Fatalf("failed to upload file: %v", err)
	}

	// 5. Initialize AdminServer
	cfg = &config.Config{
		DataDir:         dir,
		SessionDuration: 1 * time.Hour,
	}
	jwtSecret := "admin_test_secret"
	adminEmail := "admin@example.com"
	adminPassword := "adminpass"
	adminServer := NewAdminServer(pm, cfg, jwtSecret, adminEmail, adminPassword)

	mux := http.NewServeMux()
	adminServer.RegisterRoutes(mux)

	// 6. Generate valid admin token
	adminUser := &auth.User{
		ID:    "admin",
		Email: adminEmail,
		Role:  auth.RoleAdmin,
	}
	token, err := auth.GenerateAccessToken(adminUser, jwtSecret, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	// Test case 1: Successful download via token query param
	t.Run("Query Param Auth - Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/projects/"+proj.ID+"/storage/buckets/test-bucket/files/docs/hello.txt?token="+token, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
		if w.Body.String() != fileContent {
			t.Errorf("expected body %q, got %q", fileContent, w.Body.String())
		}
		if w.Header().Get("Content-Type") != "text/plain" {
			t.Errorf("expected Content-Type %q, got %q", "text/plain", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Content-Disposition") != `inline; filename="hello.txt"` {
			t.Errorf("expected Content-Disposition %q, got %q", `inline; filename="hello.txt"`, w.Header().Get("Content-Disposition"))
		}
	})

	// Test case 2: Successful download via Authorization header
	t.Run("Auth Header - Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/projects/"+proj.ID+"/storage/buckets/test-bucket/files/docs/hello.txt", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", w.Code)
		}
		if w.Body.String() != fileContent {
			t.Errorf("expected body %q, got %q", fileContent, w.Body.String())
		}
	})

	// Test case 3: Unauthenticated request fails (401)
	t.Run("No Auth - Unauthorized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/projects/"+proj.ID+"/storage/buckets/test-bucket/files/docs/hello.txt", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 Unauthorized, got %d", w.Code)
		}
	})

	// Test case 4: Non-existent file fails (404)
	t.Run("File Not Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin/projects/"+proj.ID+"/storage/buckets/test-bucket/files/docs/nonexistent.txt?token="+token, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404 Not Found, got %d", w.Code)
		}
	})
}

func TestAdminConfigSaveSyncsAccount(t *testing.T) {
	dir, err := os.MkdirTemp("", "sovrabase-admin-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	cfg := &config.Config{
		DataDir:         dir,
		JWTSecret:       "test-secret",
		AdminEmail:      "admin@example.com",
		AdminPassword:   "oldpass",
		SessionDuration: 1 * time.Hour,
		ConfigFile:      filepath.Join(dir, "config.yaml"),
	}
	pm, err := tenant.NewProjectManager(dir, cfg)
	if err != nil {
		t.Fatalf("failed to create project manager: %v", err)
	}
	defer pm.Close()

	engine, err := db.NewEngine(filepath.Join(dir, "master"))
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	adminStore := auth.NewAdminStore(engine.DB(), cfg.AdminEmail, cfg.AdminPassword)
	admins, err := adminStore.List()
	if err != nil || len(admins) == 0 {
		t.Fatalf("expected seeded admin, got %v admins (err=%v)", len(admins), err)
	}
	admin := admins[0]

	adminServer := NewAdminServer(pm, cfg, cfg.JWTSecret, cfg.AdminEmail, cfg.AdminPassword)
	adminServer.SetAdminStore(adminStore)

	token, err := adminServer.generateAdminToken(admin)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	mux := http.NewServeMux()
	adminServer.RegisterRoutes(mux)

	postConfig := func(body map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/config", bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}

	t.Run("password change requires current password", func(t *testing.T) {
		w := postConfig(map[string]string{
			"admin_password": "newpass123",
		})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("password change updates AdminStore", func(t *testing.T) {
		w := postConfig(map[string]string{
			"admin_password":         "newpass123",
			"admin_current_password": "oldpass",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		if _, err := adminStore.Authenticate("admin@example.com", "newpass123"); err != nil {
			t.Fatalf("expected login with new password: %v", err)
		}
		if cfg.AdminPassword != "newpass123" {
			t.Fatalf("expected config password synced, got %q", cfg.AdminPassword)
		}
	})

	t.Run("email change updates AdminStore", func(t *testing.T) {
		w := postConfig(map[string]string{
			"admin_email":            "ops@example.com",
			"admin_current_password": "newpass123",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		if _, err := adminStore.Authenticate("ops@example.com", "newpass123"); err != nil {
			t.Fatalf("expected login with new email: %v", err)
		}
		if cfg.AdminEmail != "ops@example.com" {
			t.Fatalf("expected config email synced, got %q", cfg.AdminEmail)
		}
	})

	t.Run("get config returns logged-in admin email", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/config", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp["admin_email"] != "ops@example.com" {
			t.Fatalf("expected admin_email ops@example.com, got %v", resp["admin_email"])
		}
	})
}

func TestAdminConfigAccountLockedByEnv(t *testing.T) {
	dir, err := os.MkdirTemp("", "sovrabase-admin-env-lock-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	cfg := &config.Config{
		DataDir:              dir,
		JWTSecret:            "test-secret",
		AdminEmail:             "admin@example.com",
		AdminPassword:          "oldpass",
		AdminPasswordFromEnv:   true,
		SessionDuration:        1 * time.Hour,
		ConfigFile:             filepath.Join(dir, "config.yaml"),
	}
	pm, err := tenant.NewProjectManager(dir, cfg)
	if err != nil {
		t.Fatalf("failed to create project manager: %v", err)
	}
	defer pm.Close()

	engine, err := db.NewEngine(filepath.Join(dir, "master"))
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	defer engine.Close()

	adminStore := auth.NewAdminStore(engine.DB(), cfg.AdminEmail, cfg.AdminPassword)
	adminServer := NewAdminServer(pm, cfg, cfg.JWTSecret, cfg.AdminEmail, cfg.AdminPassword)
	adminServer.SetAdminStore(adminStore)

	admins, _ := adminStore.List()
	token, err := adminServer.generateAdminToken(admins[0])
	if err != nil {
		t.Fatalf("token: %v", err)
	}

	mux := http.NewServeMux()
	adminServer.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{
		"admin_password":         "newpass123",
		"admin_current_password": "oldpass",
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestEmailConfigForTest(t *testing.T) {
	cfg := &config.Config{
		EmailProvider: "smtp",
		SMTPSender:    "noreply@example.com",
		SMTPHost:      "smtp.example.com",
		SMTPPort:      587,
		SMTPUser:      "user",
		SMTPPassword:  "saved-pass",
	}
	srv := &AdminServer{cfg: cfg}

	got := srv.emailConfigForTest(nil)
	if got.SMTPHost != "smtp.example.com" || got.SMTPPass != "saved-pass" {
		t.Fatalf("expected saved config, got %+v", got)
	}

	port := 2525
	override := &struct {
		To             *string `json:"to"`
		EmailProvider  *string `json:"email_provider"`
		EmailAPIKey    *string `json:"email_api_key"`
		EmailAPISecret *string `json:"email_api_secret"`
		SMTPSender     *string `json:"smtp_sender"`
		SMTPHost       *string `json:"smtp_host"`
		SMTPPort       *int    `json:"smtp_port"`
		SMTPUser       *string `json:"smtp_user"`
		SMTPPassword   *string `json:"smtp_password"`
	}{
		SMTPHost: strPtr("sandbox.smtp.test"),
		SMTPPort: &port,
	}
	got = srv.emailConfigForTest(override)
	if got.SMTPHost != "sandbox.smtp.test" || got.SMTPPort != 2525 {
		t.Fatalf("expected overridden host/port, got %+v", got)
	}
	if got.SMTPPass != "saved-pass" {
		t.Fatalf("expected saved password when override omitted, got %q", got.SMTPPass)
	}
}

func strPtr(s string) *string { return &s }
