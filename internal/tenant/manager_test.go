package tenant

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/pebble"
)

func newTestManager(t *testing.T) *ProjectManager {
	t.Helper()
	dir, err := os.MkdirTemp("", "sovrabase-tenant-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	pm, err := NewProjectManager(dir, nil)
	if err != nil {
		t.Fatalf("NewProjectManager: %v", err)
	}
	t.Cleanup(func() { pm.Close() })
	return pm
}

func TestCreateProject(t *testing.T) {
	pm := newTestManager(t)

	proj, err := pm.CreateProject("my-app", "user-1")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.ID == "" {
		t.Fatal("expected non-empty project ID")
	}
	if proj.Name != "my-app" {
		t.Fatalf("expected name my-app, got %s", proj.Name)
	}
	if proj.JWTSecret == "" {
		t.Fatal("expected non-empty JWT secret")
	}
	if proj.Status != "active" {
		t.Fatalf("expected status active, got %s", proj.Status)
	}

	// Directory should exist
	if _, err := os.Stat(proj.DataDir); os.IsNotExist(err) {
		t.Fatal("expected data dir to exist")
	}
	if _, err := os.Stat(proj.StorageDir); os.IsNotExist(err) {
		t.Fatal("expected storage dir to exist")
	}

	// Duplicate name should fail
	_, err = pm.CreateProject("my-app", "user-2")
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestGetProject(t *testing.T) {
	pm := newTestManager(t)
	created, _ := pm.CreateProject("test-get", "owner")

	proj, err := pm.GetProject(created.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if proj.Name != "test-get" {
		t.Fatalf("expected test-get, got %s", proj.Name)
	}

	// Missing project
	_, err = pm.GetProject("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing project")
	}
}

func TestListProjects(t *testing.T) {
	pm := newTestManager(t)

	pm.CreateProject("app-a", "u1")
	pm.CreateProject("app-b", "u2")
	pm.CreateProject("app-c", "u3")

	list := pm.ListProjects()
	if len(list) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(list))
	}
}

func TestDeleteProject(t *testing.T) {
	pm := newTestManager(t)
	proj, _ := pm.CreateProject("to-delete", "owner")

	if err := pm.DeleteProject(proj.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// Should be gone from list
	list := pm.ListProjects()
	for _, p := range list {
		if p.ID == proj.ID {
			t.Fatal("deleted project still in list")
		}
	}

	// Directory should be removed
	if _, err := os.Stat(proj.DataDir); !os.IsNotExist(err) {
		t.Fatal("expected data dir to be removed")
	}
}

func TestProjectCount(t *testing.T) {
	pm := newTestManager(t)

	if pm.ProjectCount() != 0 {
		t.Fatalf("expected 0, got %d", pm.ProjectCount())
	}

	pm.CreateProject("a", "u1")
	pm.CreateProject("b", "u2")

	if pm.ProjectCount() != 2 {
		t.Fatalf("expected 2, got %d", pm.ProjectCount())
	}
}

func TestGetProjectBySecret(t *testing.T) {
	pm := newTestManager(t)
	proj, _ := pm.CreateProject("secret-test", "owner")

	found, err := pm.GetProjectBySecret(proj.JWTSecret)
	if err != nil {
		t.Fatalf("GetProjectBySecret: %v", err)
	}
	if found.ID != proj.ID {
		t.Fatal("wrong project returned")
	}

	// Wrong secret
	_, err = pm.GetProjectBySecret("wrong-secret")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestPersistenceAcrossRestart(t *testing.T) {
	dir := t.TempDir()

	// First instance
	pm1, err := NewProjectManager(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	proj, _ := pm1.CreateProject("persistent", "owner")
	pm1.Close()

	// Second instance (simulates restart)
	pm2, err := NewProjectManager(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pm2.Close()

	loaded, err := pm2.GetProject(proj.ID)
	if err != nil {
		t.Fatalf("project not found after restart: %v", err)
	}
	if loaded.Name != "persistent" {
		t.Fatalf("expected persistent, got %s", loaded.Name)
	}
}

func TestAutoRepairEmptyJWTSecret(t *testing.T) {
	dir := t.TempDir()

	// 1. Create a project using a ProjectManager
	pm1, err := NewProjectManager(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	proj, err := pm1.CreateProject("test-repair", "owner")
	if err != nil {
		t.Fatal(err)
	}
	projectID := proj.ID
	pm1.Close()

	// 2. Open the master DB directly, read the project, set JWTSecret to empty, and save it back
	masterDir := filepath.Join(dir, "_master")
	db, err := pebble.Open(masterDir, &pebble.Options{})
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("project:" + projectID)
	val, closer, err := db.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	var loadedProj Project
	if err := json.Unmarshal(val, &loadedProj); err != nil {
		t.Fatal(err)
	}
	closer.Close()

	// Clear the secret
	loadedProj.JWTSecret = ""
	val2, err := json.Marshal(loadedProj)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Set(key, val2, pebble.Sync); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// 3. Re-open with a new ProjectManager. It should auto-repair the empty JWTSecret.
	pm2, err := NewProjectManager(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pm2.Close()

	repaired, err := pm2.GetProject(projectID)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.JWTSecret == "" {
		t.Fatal("expected JWTSecret to be auto-repaired (not empty)")
	}
	if len(repaired.JWTSecret) < 32 {
		t.Fatalf("expected a long secure token, got %s", repaired.JWTSecret)
	}
}
