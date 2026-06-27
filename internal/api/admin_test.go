package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
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
