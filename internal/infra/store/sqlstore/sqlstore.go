package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"github.com/ketsuna-org/sovrabase/internal/core/store"
	_ "modernc.org/sqlite"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

type Store struct {
	db      *sql.DB
	dialect Dialect
}

func OpenSQLite(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("sqlite path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db, dialect: DialectSQLite}, nil
}

func OpenPostgres(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("postgres dsn is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres database: %w", err)
	}
	return &Store{db: db, dialect: DialectPostgres}, nil
}

func (s *Store) Dialect() Dialect {
	return s.dialect
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) Migrate(ctx context.Context) error {
	return s.runMigrations(ctx)
}

func (s *Store) Create(ctx context.Context, rec connections.ConnectionRecord) error {
	if rec.ID == "" {
		return errors.New("record id is required")
	}
	if err := connections.ValidateProjectID(rec.ProjectID); err != nil {
		return err
	}
	if err := connections.ValidateSlug(rec.Slug); err != nil {
		return err
	}
	if err := rec.Engine.Validate(); err != nil {
		return err
	}
	if err := rec.Status.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(rec.EncryptedDSN) == "" {
		return errors.New("encrypted_dsn is required")
	}

	createdAt := rec.CreatedAt
	updatedAt := rec.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	optionsJSON, err := json.Marshal(defaultOptions(rec.Options))
	if err != nil {
		return fmt.Errorf("marshal options: %w", err)
	}

	query := fmt.Sprintf(`
INSERT INTO sb_connections (
  id, project_id, slug, display_name, engine, encrypted_dsn, options_json,
  managed, managed_provider, managed_resource_id, status, last_error,
  last_checked_at, created_at, updated_at
) VALUES (%s)
`, s.placeholders(15))

	_, err = s.db.ExecContext(
		ctx,
		query,
		rec.ID,
		rec.ProjectID,
		rec.Slug,
		rec.DisplayName,
		string(rec.Engine),
		rec.EncryptedDSN,
		string(optionsJSON),
		rec.Managed,
		derefProvider(rec.ManagedProvider),
		derefString(rec.ManagedResourceID),
		string(rec.Status),
		derefString(rec.LastError),
		rec.LastCheckedAt,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert connection: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, projectID, slug string) (connections.ConnectionRecord, error) {
	if err := connections.ValidateProjectID(projectID); err != nil {
		return connections.ConnectionRecord{}, err
	}
	if err := connections.ValidateSlug(slug); err != nil {
		return connections.ConnectionRecord{}, err
	}

	query := fmt.Sprintf(
		`SELECT %s FROM sb_connections WHERE project_id = %s AND slug = %s`,
		connectionColumns(),
		s.placeholder(1),
		s.placeholder(2),
	)

	row := s.db.QueryRowContext(ctx, query, projectID, slug)
	rec, err := scanConnectionRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connections.ConnectionRecord{}, store.ErrConnectionNotFound
		}
		return connections.ConnectionRecord{}, err
	}
	return rec, nil
}

func (s *Store) List(ctx context.Context, projectID string) ([]connections.ConnectionRecord, error) {
	if err := connections.ValidateProjectID(projectID); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT %s FROM sb_connections WHERE project_id = %s ORDER BY slug ASC`,
		connectionColumns(),
		s.placeholder(1),
	)

	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list connections: %w", err)
	}
	defer rows.Close()

	var records []connections.ConnectionRecord
	for rows.Next() {
		rec, scanErr := scanConnectionRow(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list connections rows: %w", err)
	}
	return records, nil
}

func (s *Store) Delete(ctx context.Context, projectID, slug string) error {
	if err := connections.ValidateProjectID(projectID); err != nil {
		return err
	}
	if err := connections.ValidateSlug(slug); err != nil {
		return err
	}

	query := fmt.Sprintf(
		`DELETE FROM sb_connections WHERE project_id = %s AND slug = %s`,
		s.placeholder(1),
		s.placeholder(2),
	)

	res, err := s.db.ExecContext(ctx, query, projectID, slug)
	if err != nil {
		return fmt.Errorf("delete connection: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete connection rows affected: %w", err)
	}
	if affected == 0 {
		return store.ErrConnectionNotFound
	}
	return nil
}

func (s *Store) UpdateHealth(ctx context.Context, projectID, slug string, status connections.ConnectionStatus, lastErr *string, checkedAt time.Time) error {
	if err := status.Validate(); err != nil {
		return err
	}

	query := fmt.Sprintf(
		`UPDATE sb_connections SET status = %s, last_error = %s, last_checked_at = %s, updated_at = %s WHERE project_id = %s AND slug = %s`,
		s.placeholder(1),
		s.placeholder(2),
		s.placeholder(3),
		s.placeholder(4),
		s.placeholder(5),
		s.placeholder(6),
	)

	res, err := s.db.ExecContext(ctx, query, string(status), derefString(lastErr), checkedAt, time.Now().UTC(), projectID, slug)
	if err != nil {
		return fmt.Errorf("update connection health: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update health rows affected: %w", err)
	}
	if affected == 0 {
		return store.ErrConnectionNotFound
	}
	return nil
}

func (s *Store) placeholder(i int) string {
	if s.dialect == DialectPostgres {
		return fmt.Sprintf("$%d", i)
	}
	return "?"
}

func (s *Store) placeholders(n int) string {
	var parts []string
	for i := 1; i <= n; i++ {
		parts = append(parts, s.placeholder(i))
	}
	return strings.Join(parts, ", ")
}

func connectionColumns() string {
	return strings.Join([]string{
		"id",
		"project_id",
		"slug",
		"display_name",
		"engine",
		"encrypted_dsn",
		"options_json",
		"managed",
		"managed_provider",
		"managed_resource_id",
		"status",
		"last_error",
		"last_checked_at",
		"created_at",
		"updated_at",
	}, ", ")
}

func scanConnectionRow(scanFn func(dest ...any) error) (connections.ConnectionRecord, error) {
	var (
		id             string
		projectID      string
		slug           string
		displayName    string
		engineRaw      string
		encryptedDSN   string
		optionsRaw     any
		managedRaw     any
		managedProvRaw any
		resourceIDRaw  any
		statusRaw      string
		lastErrorRaw   any
		lastCheckedRaw any
		createdAtRaw   any
		updatedAtRaw   any
	)

	if err := scanFn(
		&id,
		&projectID,
		&slug,
		&displayName,
		&engineRaw,
		&encryptedDSN,
		&optionsRaw,
		&managedRaw,
		&managedProvRaw,
		&resourceIDRaw,
		&statusRaw,
		&lastErrorRaw,
		&lastCheckedRaw,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return connections.ConnectionRecord{}, err
	}

	engine := connections.ConnectionEngine(engineRaw)
	if err := engine.Validate(); err != nil {
		return connections.ConnectionRecord{}, err
	}
	status := connections.ConnectionStatus(statusRaw)
	if err := status.Validate(); err != nil {
		return connections.ConnectionRecord{}, err
	}

	options, err := decodeOptions(optionsRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	managed, err := decodeBool(managedRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	managedProvider, err := decodeManagedProvider(managedProvRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	managedResourceID, err := decodeOptionalString(resourceIDRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	lastError, err := decodeOptionalString(lastErrorRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	lastCheckedAt, err := decodeOptionalTime(lastCheckedRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	createdAt, err := decodeRequiredTime(createdAtRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}
	updatedAt, err := decodeRequiredTime(updatedAtRaw)
	if err != nil {
		return connections.ConnectionRecord{}, err
	}

	return connections.ConnectionRecord{
		ID:                id,
		ProjectID:         projectID,
		Slug:              slug,
		DisplayName:       displayName,
		Engine:            engine,
		EncryptedDSN:      encryptedDSN,
		Options:           options,
		Managed:           managed,
		ManagedProvider:   managedProvider,
		ManagedResourceID: managedResourceID,
		Status:            status,
		LastError:         lastError,
		LastCheckedAt:     lastCheckedAt,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func decodeOptions(raw any) (map[string]string, error) {
	data, ok := rawToBytes(raw)
	if !ok {
		return map[string]string{}, nil
	}
	if len(data) == 0 {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode options_json: %w", err)
	}
	return out, nil
}

func decodeManagedProvider(raw any) (*connections.ManagedProvider, error) {
	value, err := decodeOptionalString(raw)
	if err != nil || value == nil {
		return nil, err
	}
	v := connections.ManagedProvider(*value)
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return &v, nil
}

func decodeBool(raw any) (bool, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case []byte:
		value := strings.TrimSpace(string(v))
		switch value {
		case "1", "t", "true", "TRUE":
			return true, nil
		case "0", "f", "false", "FALSE":
			return false, nil
		}
	case string:
		value := strings.TrimSpace(v)
		switch value {
		case "1", "t", "true", "TRUE":
			return true, nil
		case "0", "f", "false", "FALSE":
			return false, nil
		}
	}
	return false, fmt.Errorf("unsupported bool value %T", raw)
}

func decodeOptionalString(raw any) (*string, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		value := v
		return &value, nil
	case []byte:
		text := strings.TrimSpace(string(v))
		if text == "" {
			return nil, nil
		}
		return &text, nil
	default:
		return nil, fmt.Errorf("unsupported string value %T", raw)
	}
}

func decodeOptionalTime(raw any) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case time.Time:
		t := v.UTC()
		return &t, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		t, err := parseTimeFlexible(v)
		if err != nil {
			return nil, fmt.Errorf("parse time %q: %w", v, err)
		}
		ut := t.UTC()
		return &ut, nil
	case []byte:
		if len(v) == 0 {
			return nil, nil
		}
		t, err := parseTimeFlexible(string(v))
		if err != nil {
			return nil, fmt.Errorf("parse time %q: %w", string(v), err)
		}
		ut := t.UTC()
		return &ut, nil
	default:
		return nil, fmt.Errorf("unsupported time value %T", raw)
	}
}

func decodeRequiredTime(raw any) (time.Time, error) {
	t, err := decodeOptionalTime(raw)
	if err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, errors.New("missing required time value")
	}
	return *t, nil
}

func parseTimeFlexible(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func rawToBytes(raw any) ([]byte, bool) {
	switch v := raw.(type) {
	case nil:
		return nil, false
	case string:
		return []byte(v), true
	case []byte:
		return v, true
	default:
		return nil, false
	}
}

func defaultOptions(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func derefString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func derefProvider(value *connections.ManagedProvider) any {
	if value == nil {
		return nil
	}
	return string(*value)
}
