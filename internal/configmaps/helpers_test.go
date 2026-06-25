package configmaps

import (
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
)

// newTestStore creates a configmaps Store backed by a temporary Pebble DB.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "configmaps-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open pebble: %v", err)
	}
	// Store the dir for cleanup.
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return &Store{db: db}
}
