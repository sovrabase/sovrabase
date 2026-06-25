package scheduler

import (
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to open pebble: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return NewStore(db)
}
