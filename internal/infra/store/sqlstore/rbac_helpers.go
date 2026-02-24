package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Store) insertIgnore(table, columns string, n int) string {
	if s.dialect == DialectPostgres {
		return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", table, columns, s.placeholders(n))
	}
	return fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", table, columns, s.placeholders(n))
}

func normalizeOptionalID(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate key")
}

func isForeignKeyViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "foreign key") || strings.Contains(msg, "violates foreign key")
}

func (s *Store) RoleParentContains(ctx context.Context, roleID, parentRoleID string) (bool, error) {
	query := `
WITH RECURSIVE ancestors(id, parent_role_id) AS (
  SELECT id, parent_role_id FROM sb_roles WHERE id = ` + s.placeholder(1) + `
  UNION ALL
  SELECT r.id, r.parent_role_id
  FROM sb_roles r
  JOIN ancestors a ON r.id = a.parent_role_id
)
SELECT 1
FROM ancestors
WHERE id = ` + s.placeholder(2) + `
LIMIT 1`

	var sentinel int
	err := s.db.QueryRowContext(ctx, query, parentRoleID, roleID).Scan(&sentinel)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check role parent chain: %w", err)
	}
	return true, nil
}
