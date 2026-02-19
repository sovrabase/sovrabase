package sqlstore

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/sqlite/*.sql migrations/postgres/*.sql
var migrationsFS embed.FS

func (s *Store) runMigrations(ctx context.Context) error {
	folder := path.Join("migrations", string(s.dialect))
	entries, err := fs.ReadDir(migrationsFS, folder)
	if err != nil {
		return fmt.Errorf("read migrations folder %q: %w", folder, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)

	if err := s.ensureMigrationsTable(ctx); err != nil {
		return err
	}

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		if err := s.applyMigration(ctx, folder, name, version); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ensureMigrationsTable(ctx context.Context) error {
	query := `
CREATE TABLE IF NOT EXISTS sb_schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL
)`
	if s.dialect == DialectPostgres {
		query = `
CREATE TABLE IF NOT EXISTS sb_schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL
)`
	}
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, folder, filename, version string) error {
	file := path.Join(folder, filename)
	script, err := migrationsFS.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", file, err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx for %s: %w", version, err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	existsQuery := fmt.Sprintf(`SELECT 1 FROM sb_schema_migrations WHERE version = %s LIMIT 1`, s.placeholder(1))
	var sentinel int
	existsErr := tx.QueryRowContext(ctx, existsQuery, version).Scan(&sentinel)
	if existsErr == nil {
		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("commit already-applied migration %s: %w", version, commitErr)
		}
		return nil
	}
	if !errors.Is(existsErr, sql.ErrNoRows) {
		err = fmt.Errorf("check migration %s state: %w", version, existsErr)
		return err
	}

	if _, execErr := tx.ExecContext(ctx, string(script)); execErr != nil {
		err = fmt.Errorf("execute migration %s: %w", version, execErr)
		return err
	}

	insertQuery := fmt.Sprintf(`INSERT INTO sb_schema_migrations(version, applied_at) VALUES (%s, %s)`, s.placeholder(1), s.placeholder(2))
	if _, execErr := tx.ExecContext(ctx, insertQuery, version, time.Now().UTC()); execErr != nil {
		err = fmt.Errorf("record migration %s: %w", version, execErr)
		return err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit migration %s: %w", version, commitErr)
		return err
	}

	return nil
}
