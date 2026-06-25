package auth

import (
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

func newTestAuditStore(t *testing.T) (*AuditStore, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "audit_store_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open pebble DB: %v", err)
	}

	store := NewAuditStore(db)

	cleanup := func() {
		db.Close()
		os.RemoveAll(dir)
	}

	return store, cleanup
}

func newTestAuditEntry(adminID, adminEmail, action, targetType, targetID string, success bool) *AuditEntry {
	return &AuditEntry{
		AdminID:    adminID,
		AdminEmail: adminEmail,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Details: map[string]interface{}{
			"reason": "test",
		},
		IP:      "127.0.0.1",
		Success: success,
	}
}

func TestAuditStore_LogAndCount(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	entry := newTestAuditEntry("admin-1", "admin@test.com", "project.create", "project", "proj-1", true)

	if err := store.Log(entry); err != nil {
		t.Fatalf("Log() error: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected ID to be auto-generated")
	}
	if entry.Timestamp.IsZero() {
		t.Fatal("expected Timestamp to be auto-set")
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}
}

func TestAuditStore_LogMultiple(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		entry := newTestAuditEntry(
			"admin-1",
			"admin@test.com",
			"project.create",
			"project",
			"proj-1",
			true,
		)
		// Stagger timestamps so ordering is deterministic
		entry.Timestamp = time.Now().UTC().Add(-time.Duration(i) * time.Second)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 10 {
		t.Fatalf("expected count 10, got %d", count)
	}
}

func TestAuditStore_List(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	// Create entries with known timestamps
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		entry := newTestAuditEntry(
			"admin-1",
			"admin@test.com",
			"project.create",
			"project",
			"proj-1",
			true,
		)
		entry.Timestamp = now.Add(-time.Duration(i) * time.Hour)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// List without filters — should return newest first
	entries, total, err := store.List(10, 0, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Verify descending order (newest first)
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Timestamp.Before(entries[i].Timestamp) {
			t.Fatalf("entries not in descending order at index %d", i)
		}
	}
}

func TestAuditStore_List_Pagination(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		entry := newTestAuditEntry(
			"admin-1",
			"admin@test.com",
			"project.create",
			"project",
			"proj-1",
			true,
		)
		entry.Timestamp = now.Add(-time.Duration(i) * time.Minute)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// Page 1: limit=3, offset=0
	entries, total, err := store.List(3, 0, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 10 {
		t.Fatalf("expected total 10, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries on page 1, got %d", len(entries))
	}

	// Page 2: limit=3, offset=3
	entries, total, err = store.List(3, 3, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries on page 2, got %d", len(entries))
	}

	// Page 4 (last): limit=3, offset=9
	entries, total, err = store.List(3, 9, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry on last page, got %d", len(entries))
	}

	// Offset beyond total
	entries, total, err = store.List(3, 100, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries beyond total, got %d", len(entries))
	}
	if total != 10 {
		t.Fatalf("expected total 10, got %d", total)
	}

	// Negative offset (treated as 0)
	entries, total, err = store.List(3, -1, nil)
	if err != nil {
		t.Fatalf("List() error with negative offset: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries with negative offset")
	}
}

func TestAuditStore_ListByAdmin(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	// Create entries for two admins
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		entry := newTestAuditEntry("admin-1", "admin1@test.com", "project.create", "project", "proj-1", true)
		entry.Timestamp = now.Add(-time.Duration(i) * time.Minute)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		entry := newTestAuditEntry("admin-2", "admin2@test.com", "project.delete", "project", "proj-2", true)
		entry.Timestamp = now.Add(-time.Duration(i+10) * time.Minute)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// Filter by admin-1
	entries, total, err := store.ListByAdmin("admin-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByAdmin() error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3 for admin-1, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries for admin-1, got %d", len(entries))
	}
	for _, e := range entries {
		if e.AdminID != "admin-1" {
			t.Fatalf("expected admin-1, got %s", e.AdminID)
		}
	}

	// Filter by admin-2
	entries, total, err = store.ListByAdmin("admin-2", 10, 0)
	if err != nil {
		t.Fatalf("ListByAdmin() error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5 for admin-2, got %d", total)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries for admin-2, got %d", len(entries))
	}
}

func TestAuditStore_ListByAction(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	actions := []string{"project.create", "project.delete", "config.update", "project.create", "admin.create"}
	for i, action := range actions {
		entry := newTestAuditEntry("admin-1", "admin@test.com", action, "project", "proj-1", true)
		entry.Timestamp = now.Add(-time.Duration(i) * time.Minute)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// Filter by action
	entries, total, err := store.ListByAction("project.create", 10, 0)
	if err != nil {
		t.Fatalf("ListByAction() error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2 for project.create, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for project.create, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Action != "project.create" {
			t.Fatalf("expected action project.create, got %s", e.Action)
		}
	}

	// Filter by action with no matches
	entries, total, err = store.ListByAction("backup.restore", 10, 0)
	if err != nil {
		t.Fatalf("ListByAction() error: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestAuditStore_ListByTarget(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	// Log entries with different targets
	entry1 := newTestAuditEntry("admin-1", "admin@test.com", "project.create", "project", "proj-1", true)
	entry1.Timestamp = now.Add(-1 * time.Minute)
	if err := store.Log(entry1); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	entry2 := newTestAuditEntry("admin-1", "admin@test.com", "project.delete", "project", "proj-2", true)
	entry2.Timestamp = now.Add(-2 * time.Minute)
	if err := store.Log(entry2); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	entry3 := newTestAuditEntry("admin-1", "admin@test.com", "project.update", "project", "proj-1", true)
	entry3.Timestamp = now.Add(-3 * time.Minute)
	if err := store.Log(entry3); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	// Filter by target
	entries, total, err := store.ListByTarget("project", "proj-1", 10, 0)
	if err != nil {
		t.Fatalf("ListByTarget() error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2 for project/proj-1, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.TargetID != "proj-1" {
			t.Fatalf("expected targetID proj-1, got %s", e.TargetID)
		}
	}
}

func TestAuditStore_ListWithFilters(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	// Mix of entries from different admins
	for i := 0; i < 3; i++ {
		e := newTestAuditEntry("admin-1", "a1@t.com", "project.create", "project", "p-1", true)
		e.Timestamp = now.Add(-time.Duration(i) * time.Minute)
		store.Log(e)
	}
	for i := 0; i < 2; i++ {
		e := newTestAuditEntry("admin-2", "a2@t.com", "project.delete", "project", "p-2", true)
		e.Timestamp = now.Add(-time.Duration(i+10) * time.Minute)
		store.Log(e)
	}

	// List with admin_id filter
	entries, total, err := store.List(10, 0, map[string]string{"admin_id": "admin-1"})
	if err != nil {
		t.Fatalf("List() with filter error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3 for admin-1 filter, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// List with action filter
	entries, total, err = store.List(10, 0, map[string]string{"action": "project.delete"})
	if err != nil {
		t.Fatalf("List() with action filter error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2 for project.delete, got %d", total)
	}

	// List with target_type and target_id
	entries, total, err = store.List(10, 0, map[string]string{"target_type": "project", "target_id": "p-1"})
	if err != nil {
		t.Fatalf("List() with target filter error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3 for target p-1, got %d", total)
	}
}

func TestAuditStore_PurgeBefore(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	// Create entries with different ages
	for i := 0; i < 5; i++ {
		entry := newTestAuditEntry("admin-1", "admin@test.com", "project.create", "project", "proj-1", true)
		// 2 old (100h ago), 3 new (1h ago)
		if i < 2 {
			entry.Timestamp = now.Add(-100 * time.Hour)
		} else {
			entry.Timestamp = now.Add(-1 * time.Hour)
		}
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected count 5 before purge, got %d", count)
	}

	// Purge entries older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	if err := store.PurgeBefore(cutoff); err != nil {
		t.Fatalf("PurgeBefore() error: %v", err)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Count() error after purge: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3 after purge (oldest 2 removed), got %d", count)
	}
}

func TestAuditStore_PurgeBefore_All(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		entry := newTestAuditEntry("admin-1", "admin@test.com", "project.create", "project", "proj-1", true)
		entry.Timestamp = now.Add(-100 * time.Hour)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	if err := store.PurgeBefore(now); err != nil {
		t.Fatalf("PurgeBefore() error: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0 after purge all, got %d", count)
	}
}

func TestAuditStore_PurgeBefore_Noop(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		entry := newTestAuditEntry("admin-1", "admin@test.com", "project.create", "project", "proj-1", true)
		entry.Timestamp = now.Add(-1 * time.Hour)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// Purge with a cutoff before all entries — should be no-op
	if err := store.PurgeBefore(now.Add(-100 * time.Hour)); err != nil {
		t.Fatalf("PurgeBefore() error: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected count 3 after noop purge, got %d", count)
	}
}

func TestAuditStore_EmptyStore(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Count() error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}

	entries, total, err := store.List(10, 0, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}

	entries, total, err = store.ListByAdmin("nonexistent", 10, 0)
	if err != nil {
		t.Fatalf("ListByAdmin() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
}

func TestAuditStore_ListOrdering(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	// Create entries with explicit timestamps to test ordering
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	entries := []*AuditEntry{
		newTestAuditEntry("admin-1", "a@t.com", "first", "project", "p-1", true),
		newTestAuditEntry("admin-1", "a@t.com", "second", "project", "p-1", true),
		newTestAuditEntry("admin-1", "a@t.com", "third", "project", "p-1", true),
	}
	entries[0].Timestamp = base.Add(-2 * time.Hour) // oldest
	entries[1].Timestamp = base                      // middle
	entries[2].Timestamp = base.Add(-1 * time.Hour)  // middle-ish

	for _, e := range entries {
		if err := store.Log(e); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	listed, total, err := store.List(10, 0, nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}

	// Should be newest first: middle-ish, middle, oldest
	if len(listed) < 3 {
		t.Fatalf("expected at least 3 entries, got %d", len(listed))
	}
	expectedOrder := []string{"second", "third", "first"}
	for i, e := range listed {
		if i < len(expectedOrder) && e.Action != expectedOrder[i] {
			t.Fatalf("position %d: expected action %s, got %s", i, expectedOrder[i], e.Action)
		}
	}
}

func TestAuditStore_FilteredListPagination(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	now := time.Now().UTC()
	for i := 0; i < 8; i++ {
		entry := newTestAuditEntry("admin-1", "a1@t.com", "project.create", "project", "p-1", true)
		entry.Timestamp = now.Add(-time.Duration(i) * time.Minute)
		if err := store.Log(entry); err != nil {
			t.Fatalf("Log() error: %v", err)
		}
	}

	// Paginate filtered results
	entries, total, err := store.ListByAdmin("admin-1", 3, 0)
	if err != nil {
		t.Fatalf("ListByAdmin() error: %v", err)
	}
	if total != 8 {
		t.Fatalf("expected total 8, got %d", total)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	entries, total, err = store.ListByAdmin("admin-1", 3, 3)
	if err != nil {
		t.Fatalf("ListByAdmin(page 2) error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries on page 2, got %d", len(entries))
	}

	entries, total, err = store.ListByAdmin("admin-1", 3, 6)
	if err != nil {
		t.Fatalf("ListByAdmin(page 3) error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries on last page, got %d", len(entries))
	}
}

func TestAuditStore_LogWithAllFields(t *testing.T) {
	store, cleanup := newTestAuditStore(t)
	defer cleanup()

	entry := &AuditEntry{
		AdminID:    "admin-1",
		AdminEmail: "admin@test.com",
		Action:     "config.update",
		TargetType: "config",
		TargetID:   "smtp_settings",
		Details: map[string]interface{}{
			"old_value": "smtp.gmail.com",
			"new_value": "smtp.sendgrid.net",
			"changed_by": "admin-1",
		},
		IP:      "192.168.1.1",
		Success: true,
	}

	if err := store.Log(entry); err != nil {
		t.Fatalf("Log() error: %v", err)
	}

	// Retrieve via List and verify fields
	entries, _, err := store.List(10, 0, map[string]string{"action": "config.update"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.AdminID != "admin-1" {
		t.Fatalf("AdminID mismatch: %s", e.AdminID)
	}
	if e.AdminEmail != "admin@test.com" {
		t.Fatalf("AdminEmail mismatch: %s", e.AdminEmail)
	}
	if e.Action != "config.update" {
		t.Fatalf("Action mismatch: %s", e.Action)
	}
	if e.TargetType != "config" {
		t.Fatalf("TargetType mismatch: %s", e.TargetType)
	}
	if e.TargetID != "smtp_settings" {
		t.Fatalf("TargetID mismatch: %s", e.TargetID)
	}
	if e.IP != "192.168.1.1" {
		t.Fatalf("IP mismatch: %s", e.IP)
	}
	if !e.Success {
		t.Fatal("Success should be true")
	}
	if e.Details == nil {
		t.Fatal("Details should not be nil")
	}
	if e.Details["old_value"] != "smtp.gmail.com" {
		t.Fatalf("Details old_value mismatch: %v", e.Details["old_value"])
	}
}
