package sqlstore

import (
	"context"
	"fmt"
	"time"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func (s *Store) AssignRoleToUser(ctx context.Context, userID, roleID string) error {
	now := time.Now().UTC()
	query := s.insertIgnore("sb_user_roles", "user_id, role_id, created_at", 3)
	_, err := s.db.ExecContext(ctx, query, userID, roleID, now)
	if err != nil {
		if isForeignKeyViolation(err) {
			if _, userErr := s.GetByID(ctx, userID); userErr != nil {
				return userErr
			}
			return coreauth.ErrRoleNotFound
		}
		return fmt.Errorf("assign role to user: %w", err)
	}
	return nil
}

func (s *Store) RemoveRoleFromUser(ctx context.Context, userID, roleID string) error {
	query := fmt.Sprintf("DELETE FROM sb_user_roles WHERE user_id = %s AND role_id = %s", s.placeholder(1), s.placeholder(2))
	_, err := s.db.ExecContext(ctx, query, userID, roleID)
	if err != nil {
		return fmt.Errorf("remove role from user: %w", err)
	}
	return nil
}

func (s *Store) AssignScopeToRole(ctx context.Context, roleID, scopeID string) error {
	now := time.Now().UTC()
	query := s.insertIgnore("sb_role_scopes", "role_id, scope_id, created_at", 3)
	_, err := s.db.ExecContext(ctx, query, roleID, scopeID, now)
	if err != nil {
		if isForeignKeyViolation(err) {
			if _, roleErr := s.GetRoleByID(ctx, roleID); roleErr != nil {
				return roleErr
			}
			return coreauth.ErrScopeNotFound
		}
		return fmt.Errorf("assign scope to role: %w", err)
	}
	return nil
}

func (s *Store) RemoveScopeFromRole(ctx context.Context, roleID, scopeID string) error {
	query := fmt.Sprintf("DELETE FROM sb_role_scopes WHERE role_id = %s AND scope_id = %s", s.placeholder(1), s.placeholder(2))
	_, err := s.db.ExecContext(ctx, query, roleID, scopeID)
	if err != nil {
		return fmt.Errorf("remove scope from role: %w", err)
	}
	return nil
}

func (s *Store) ResolveUserScopes(ctx context.Context, userID string) ([]string, error) {
	query := `
WITH RECURSIVE inherited_roles(id) AS (
  SELECT ur.role_id
  FROM sb_user_roles ur
  WHERE ur.user_id = ` + s.placeholder(1) + `
  UNION
  SELECT r.parent_role_id
  FROM sb_roles r
  JOIN inherited_roles ir ON r.id = ir.id
  WHERE r.parent_role_id IS NOT NULL
)
SELECT DISTINCT s.scope_key
FROM inherited_roles ir
JOIN sb_role_scopes rs ON rs.role_id = ir.id
JOIN sb_scopes s ON s.id = rs.scope_id
ORDER BY s.scope_key ASC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("resolve user scopes: %w", err)
	}
	defer rows.Close()

	scopes := make([]string, 0)
	for rows.Next() {
		var scope string
		if scanErr := rows.Scan(&scope); scanErr != nil {
			return nil, fmt.Errorf("scan scope: %w", scanErr)
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve user scopes rows: %w", err)
	}
	return scopes, nil
}
