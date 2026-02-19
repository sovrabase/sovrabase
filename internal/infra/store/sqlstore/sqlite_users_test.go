package sqlstore

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func TestSQLiteUsersBootstrapAndUniqueness(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sovrabase-users.db")
	s, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	required, err := s.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired() error = %v", err)
	}
	if !required {
		t.Fatalf("BootstrapRequired() = false, want true on empty database")
	}

	user, err := s.CreateFirstAdmin(ctx, "admin@example.com", "hash-1")
	if err != nil {
		t.Fatalf("CreateFirstAdmin() error = %v", err)
	}
	if user.Email != "admin@example.com" || !user.IsRoot {
		t.Fatalf("CreateFirstAdmin() returned unexpected user: %+v", user)
	}

	required, err = s.BootstrapRequired(ctx)
	if err != nil {
		t.Fatalf("BootstrapRequired() error = %v", err)
	}
	if required {
		t.Fatalf("BootstrapRequired() = true, want false after bootstrap")
	}

	if _, err := s.CreateFirstAdmin(ctx, "admin2@example.com", "hash-2"); !errors.Is(err, coreauth.ErrBootstrapAlreadyDone) {
		t.Fatalf("CreateFirstAdmin() second call error = %v, want ErrBootstrapAlreadyDone", err)
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO sb_users (id, email, password_hash, role, is_root, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		"secondary-user", "admin@example.com", "hash-x", "admin", 0,
	)
	if err == nil {
		t.Fatalf("expected duplicate email insert to fail, got nil")
	}
}

func TestSQLiteUsersConcurrentBootstrap(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sovrabase-users-race.db")
	s, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer func() {
		_ = s.Close()
	}()

	ctx := context.Background()
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	var successes atomic.Int32
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			_, createErr := s.CreateFirstAdmin(ctx, "admin-race@example.com", "hash-race")
			if createErr == nil {
				successes.Add(1)
				return
			}
			if !errors.Is(createErr, coreauth.ErrBootstrapAlreadyDone) {
				t.Errorf("CreateFirstAdmin() unexpected error: %v", createErr)
			}
		}(i)
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Fatalf("successful bootstraps = %d, want 1", successes.Load())
	}
}
