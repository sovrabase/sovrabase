package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

func newTestAdminStore(t *testing.T) (*AdminStore, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "admin_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open pebble DB: %v", err)
	}

	store := NewAdminStore(db, "admin@test.com", "testpass123")

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return store, cleanup
}

func TestAdminStore_SeedDefault(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	// The store should have been auto-seeded with the default admin
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 seeded admin, got %d", count)
	}

	// Verify the default admin can be retrieved by email
	admin, err := store.GetByEmail("admin@test.com")
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if admin == nil {
		t.Fatal("expected non-nil admin")
	}
	if admin.Role != AdminRoleSuper {
		t.Fatalf("expected role super_admin, got %s", admin.Role)
	}
	if admin.Name != "Default Admin" {
		t.Fatalf("expected name 'Default Admin', got %q", admin.Name)
	}
}

func TestAdminStore_NoDoubleSeed(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	// Seed only happens once
	store.seedDefault("admin@test.com", "testpass123")
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 admin after re-seed attempt, got %d", count)
	}
}

func TestAdminStore_CreateAndGet(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if admin == nil {
		t.Fatal("expected non-nil admin")
	}
	if admin.Email != "alice@test.com" {
		t.Fatalf("expected email alice@test.com, got %s", admin.Email)
	}
	if admin.Role != AdminRoleAdmin {
		t.Fatalf("expected role admin, got %s", admin.Role)
	}
	if admin.Name != "Alice" {
		t.Fatalf("expected name Alice, got %s", admin.Name)
	}
	if admin.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if admin.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	if admin.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero UpdatedAt")
	}

	// Retrieve by ID
	fetched, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if fetched.Email != admin.Email {
		t.Fatalf("email mismatch: %s vs %s", fetched.Email, admin.Email)
	}

	// Retrieve by email
	fetchedByEmail, err := store.GetByEmail(admin.Email)
	if err != nil {
		t.Fatalf("GetByEmail() error: %v", err)
	}
	if fetchedByEmail.ID != admin.ID {
		t.Fatalf("ID mismatch: %s vs %s", fetchedByEmail.ID, admin.ID)
	}
}

func TestAdminStore_CreateDuplicateEmail(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("first Create() error: %v", err)
	}

	_, err = store.Create("alice@test.com", "otherpass", "support", "Alice Dup")
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestAdminStore_GetByID_NotFound(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.GetByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ID")
	}
}

func TestAdminStore_GetByEmail_NotFound(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.GetByEmail("nobody@test.com")
	if err == nil {
		t.Fatal("expected error for nonexistent email")
	}
}

func TestAdminStore_List(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	// We have the seeded default admin
	_, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	_, err = store.Create("bob@test.com", "password456", "support", "Bob")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 admins, got %d", len(list))
	}

	// Verify all emails present
	emails := make(map[string]bool)
	for _, a := range list {
		emails[a.Email] = true
	}
	if !emails["admin@test.com"] {
		t.Fatal("missing default admin email")
	}
	if !emails["alice@test.com"] {
		t.Fatal("missing alice email")
	}
	if !emails["bob@test.com"] {
		t.Fatal("missing bob email")
	}
}

func TestAdminStore_Update(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	admin.Name = "Alice Updated"
	admin.Email = "alice.new@test.com"
	if err := store.Update(admin); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	// Fetch and verify
	updated, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if updated.Name != "Alice Updated" {
		t.Fatalf("expected name 'Alice Updated', got %q", updated.Name)
	}
	if updated.Email != "alice.new@test.com" {
		t.Fatalf("expected email alice.new@test.com, got %s", updated.Email)
	}
	if updated.UpdatedAt.Before(admin.CreatedAt) {
		t.Fatal("UpdatedAt should be after CreatedAt")
	}

	// Verify old email is gone and new email works
	_, err = store.GetByEmail("alice@test.com")
	if err == nil {
		t.Fatal("expected old email to be unavailable")
	}

	fetchedByNewEmail, err := store.GetByEmail("alice.new@test.com")
	if err != nil {
		t.Fatalf("GetByEmail(new email) error: %v", err)
	}
	if fetchedByNewEmail.ID != admin.ID {
		t.Fatal("new email should resolve to the updated admin")
	}
}

func TestAdminStore_Delete(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := store.Delete(admin.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	// Should not be found by ID
	_, err = store.GetByID(admin.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}

	// Should not be found by email
	_, err = store.GetByEmail(admin.Email)
	if err == nil {
		t.Fatal("expected error for deleted admin by email")
	}

	// Count should decrease
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	// Only the seeded default admin should remain
	if count != 1 {
		t.Fatalf("expected 1 admin after delete, got %d", count)
	}
}

func TestAdminStore_Delete_NotFound(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	err := store.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent admin")
	}
}

func TestAdminStore_Authenticate_Success(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	admin, err := store.Authenticate("alice@test.com", "password123")
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if admin.Email != "alice@test.com" {
		t.Fatalf("expected email alice@test.com, got %s", admin.Email)
	}
}

func TestAdminStore_Authenticate_WrongPassword(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	_, err = store.Authenticate("alice@test.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAdminStore_Authenticate_WrongEmail(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	_, err := store.Authenticate("nobody@test.com", "password123")
	if err == nil {
		t.Fatal("expected error for unknown email")
	}
}

func TestAdminStore_UpdateRole(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := store.UpdateRole(admin.ID, AdminRoleSuper); err != nil {
		t.Fatalf("UpdateRole() error: %v", err)
	}

	updated, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if updated.Role != AdminRoleSuper {
		t.Fatalf("expected role super_admin, got %s", updated.Role)
	}
}

func TestAdminStore_UpdateLastLogin(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin, err := store.Create("alice@test.com", "password123", "admin", "Alice")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if admin.LastLogin != nil {
		t.Fatal("expected nil LastLogin initially")
	}

	if err := store.UpdateLastLogin(admin.ID); err != nil {
		t.Fatalf("UpdateLastLogin() error: %v", err)
	}

	updated, err := store.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if updated.LastLogin == nil {
		t.Fatal("expected non-nil LastLogin after update")
	}
	// Should be recent
	if time.Since(*updated.LastLogin) > 5*time.Second {
		t.Fatal("LastLogin should be within the last 5 seconds")
	}
}

func TestAdminStore_Count(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 (seeded), got %d", count)
	}

	store.Create("a@t.com", "p1", "admin", "A")
	store.Create("b@t.com", "p2", "support", "B")

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3, got %d", count)
	}
}

func TestAdminStore_HasPermission(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	super, _ := store.Create("super@t.com", "p", "super_admin", "Super")
	admin, _ := store.Create("admin@t.com", "p", "admin", "Admin")
	support, _ := store.Create("support@t.com", "p", "support", "Support")

	// Super admin should have all permissions
	if !store.HasPermission(super.ID, AdminRoleSupport) {
		t.Error("super should have support permission")
	}
	if !store.HasPermission(super.ID, AdminRoleAdmin) {
		t.Error("super should have admin permission")
	}
	if !store.HasPermission(super.ID, AdminRoleSuper) {
		t.Error("super should have super permission")
	}

	// Admin should have support and admin, but not super
	if !store.HasPermission(admin.ID, AdminRoleSupport) {
		t.Error("admin should have support permission")
	}
	if !store.HasPermission(admin.ID, AdminRoleAdmin) {
		t.Error("admin should have admin permission")
	}
	if store.HasPermission(admin.ID, AdminRoleSuper) {
		t.Error("admin should NOT have super permission")
	}

	// Support should have only support
	if !store.HasPermission(support.ID, AdminRoleSupport) {
		t.Error("support should have support permission")
	}
	if store.HasPermission(support.ID, AdminRoleAdmin) {
		t.Error("support should NOT have admin permission")
	}
	if store.HasPermission(support.ID, AdminRoleSuper) {
		t.Error("support should NOT have super permission")
	}

	// Nonexistent admin should have no permissions
	if store.HasPermission("nonexistent", AdminRoleSupport) {
		t.Error("nonexistent should have no permissions")
	}
}

func TestAdminStore_NewStorePreservesExistingData(t *testing.T) {
	// Create a temporary database directory
	dir, err := os.MkdirTemp("", "admin_store_persist_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	dbPath := filepath.Join(dir, "pebble")
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		t.Fatalf("failed to open pebble DB: %v", err)
	}

	store1 := NewAdminStore(db, "admin1@test.com", "pass1")
	admin, err := store1.Create("custom@test.com", "mypass", "admin", "Custom")
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	db.Close()

	// Re-open with a different default email (should not re-seed)
	db2, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		t.Fatalf("failed to re-open pebble DB: %v", err)
	}
	store2 := NewAdminStore(db2, "different@test.com", "different")

	// Should still have the first default + custom = 2
	count, err := store2.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 admins (seeded + custom), got %d", count)
	}

	// Custom admin should still exist
	fetched, err := store2.GetByID(admin.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if fetched.Email != "custom@test.com" {
		t.Fatalf("expected custom@test.com, got %s", fetched.Email)
	}

	// The new default email should NOT have been seeded
	_, err = store2.GetByEmail("different@test.com")
	if err == nil {
		t.Fatal("expected 'different@test.com' NOT to be seeded (data already existed)")
	}

	db2.Close()
}

func TestAdminStore_Update_NonExistent(t *testing.T) {
	store, cleanup := newTestAdminStore(t)
	defer cleanup()

	admin := &AdminUser{
		ID:    "nobody",
		Email: "nobody@test.com",
		Role:  AdminRoleSupport,
	}
	err := store.Update(admin)
	if err == nil {
		t.Fatal("expected error updating nonexistent admin")
	}
}
