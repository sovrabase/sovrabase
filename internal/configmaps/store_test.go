package configmaps

import (
	"testing"
)

func TestSetAndGet(t *testing.T) {
	store := newTestStore(t)

	entry, err := store.Set("api_timeout", float64(30000), ValueNumber, "API timeout in ms", false)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if entry.Type != ValueNumber {
		t.Errorf("expected type %q, got %q", ValueNumber, entry.Type)
	}

	got, err := store.Get("api_timeout")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Key != "api_timeout" {
		t.Errorf("expected key %q, got %q", "api_timeout", got.Key)
	}
}

func TestSetInferType(t *testing.T) {
	store := newTestStore(t)

	tests := []struct {
		name     string
		value    interface{}
		expected ValueType
	}{
		{"string", "hello", ValueString},
		{"bool", true, ValueBoolean},
		{"int", float64(42), ValueNumber},
		{"json", map[string]interface{}{"a": "b"}, ValueJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := store.Set("test_"+tt.name, tt.value, "", "", false)
			if err != nil {
				t.Fatalf("Set failed: %v", err)
			}
			if entry.Type != tt.expected {
				t.Errorf("expected type %q, got %q", tt.expected, entry.Type)
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)

	_, _ = store.Set("temp_key", "temp_val", ValueString, "", false)
	if err := store.Delete("temp_key"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := store.Get("temp_key")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)

	_, _ = store.Set("zebra", "z", ValueString, "", false)
	_, _ = store.Set("alpha", "a", ValueString, "", false)
	_, _ = store.Set("mango", "m", ValueString, "", false)

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Key != "alpha" || entries[1].Key != "mango" || entries[2].Key != "zebra" {
		t.Errorf("entries not sorted: %s, %s, %s", entries[0].Key, entries[1].Key, entries[2].Key)
	}
}

func TestListPublic(t *testing.T) {
	store := newTestStore(t)

	_, _ = store.Set("public_key", "val", ValueString, "", true)
	_, _ = store.Set("private_key", "val", ValueString, "", false)
	_, _ = store.Set("another_public", float64(42), ValueNumber, "", true)

	public, err := store.ListPublic()
	if err != nil {
		t.Fatalf("ListPublic failed: %v", err)
	}
	if len(public) != 2 {
		t.Fatalf("expected 2 public entries, got %d", len(public))
	}
}

func TestSetPreservesCreatedAt(t *testing.T) {
	store := newTestStore(t)

	entry1, err := store.Set("key1", "v1", ValueString, "", false)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	entry2, err := store.Set("key1", "v2", ValueString, "", false)
	if err != nil {
		t.Fatalf("Set update failed: %v", err)
	}
	if !entry1.CreatedAt.Equal(entry2.CreatedAt) {
		t.Errorf("CreatedAt changed: %v vs %v", entry1.CreatedAt, entry2.CreatedAt)
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		key    string
		expect bool
	}{
		{"valid_key", true},
		{"", false},
		{"   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := validateKey(tt.key)
			if (err == nil) != tt.expect {
				t.Errorf("validateKey(%q) = %v, expect success=%v", tt.key, err, tt.expect)
			}
		})
	}
}

func TestUpdateDescription(t *testing.T) {
	store := newTestStore(t)

	_, _ = store.Set("key_desc", "val", ValueString, "original", false)
	entry, err := store.Get("key_desc")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if entry.Description != "original" {
		t.Errorf("expected description %q, got %q", "original", entry.Description)
	}

	updated, err := store.Set("key_desc", "val", ValueString, "updated description", false)
	if err != nil {
		t.Fatalf("Set update failed: %v", err)
	}
	if updated.Description != "updated description" {
		t.Errorf("expected description %q, got %q", "updated description", updated.Description)
	}
}
