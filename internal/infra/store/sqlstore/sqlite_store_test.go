package sqlstore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"github.com/ketsuna-org/sovrabase/internal/core/store"
)

func TestSQLiteStoreMigrateAndCRUD(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sovrabase.db")
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

	now := time.Now().UTC()
	record := connections.ConnectionRecord{
		ID:           uuid.NewString(),
		ProjectID:    "proj1",
		Slug:         "primary",
		DisplayName:  "Primary DB",
		Engine:       connections.ConnectionEnginePostgres,
		EncryptedDSN: "enc:v1:abc",
		Options:      map[string]string{"sslmode": "disable"},
		Managed:      false,
		Status:       connections.ConnectionStatusHealthy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.Create(ctx, record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := s.Get(ctx, "proj1", "primary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ProjectID != "proj1" || got.Slug != "primary" {
		t.Fatalf("Get() returned wrong record: %+v", got)
	}

	list, err := s.List(ctx, "proj1")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}

	msg := "temporary network issue"
	if err := s.UpdateHealth(ctx, "proj1", "primary", connections.ConnectionStatusUnreachable, &msg, now); err != nil {
		t.Fatalf("UpdateHealth() error = %v", err)
	}

	updated, err := s.Get(ctx, "proj1", "primary")
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if updated.Status != connections.ConnectionStatusUnreachable {
		t.Fatalf("Status = %q, want %q", updated.Status, connections.ConnectionStatusUnreachable)
	}
	if updated.LastError == nil || *updated.LastError != msg {
		t.Fatalf("LastError = %v, want %q", updated.LastError, msg)
	}

	if err := s.Delete(ctx, "proj1", "primary"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := s.Get(ctx, "proj1", "primary"); err == nil {
		t.Fatalf("Get() after delete expected error, got nil")
	}
}

func TestSQLiteStoreUniqueConstraint(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sovrabase.db")
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

	now := time.Now().UTC()
	first := connections.ConnectionRecord{
		ID:           uuid.NewString(),
		ProjectID:    "proj1",
		Slug:         "db",
		DisplayName:  "DB 1",
		Engine:       connections.ConnectionEngineMongo,
		EncryptedDSN: "enc:v1:abc",
		Status:       connections.ConnectionStatusHealthy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	second := first
	second.ID = uuid.NewString()

	if err := s.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	if err := s.Create(ctx, second); err == nil {
		t.Fatalf("Create(second) expected unique constraint error, got nil")
	}

	if err := s.Delete(ctx, "proj1", "missing"); err == nil {
		t.Fatalf("Delete(missing) expected error, got nil")
	}
	if err := s.Delete(ctx, "proj1", "missing"); err != nil && err != store.ErrConnectionNotFound {
		t.Fatalf("Delete(missing) error = %v", err)
	}
}
