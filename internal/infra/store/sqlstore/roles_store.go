package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func (s *Store) CreateRole(ctx context.Context, name, description string, parentRoleID *string) (coreauth.RoleRecord, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	query := fmt.Sprintf(`INSERT INTO sb_roles (id, name, description, parent_role_id, created_at, updated_at) VALUES (%s)`, s.placeholders(6))
	_, err := s.db.ExecContext(ctx, query, id, strings.TrimSpace(name), strings.TrimSpace(description), normalizeOptionalID(parentRoleID), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.RoleRecord{}, coreauth.ErrConflict
		}
		if isForeignKeyViolation(err) {
			return coreauth.RoleRecord{}, coreauth.ErrRoleNotFound
		}
		return coreauth.RoleRecord{}, fmt.Errorf("create role: %w", err)
	}
	return s.GetRoleByID(ctx, id)
}

func (s *Store) ListRoles(ctx context.Context) ([]coreauth.RoleRecord, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, description, parent_role_id, created_at, updated_at FROM sb_roles ORDER BY name ASC")
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	roles := make([]coreauth.RoleRecord, 0)
	for rows.Next() {
		role, scanErr := scanRoleRow(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list roles rows: %w", err)
	}
	return roles, nil
}

func (s *Store) GetRoleByID(ctx context.Context, roleID string) (coreauth.RoleRecord, error) {
	query := fmt.Sprintf("SELECT id, name, description, parent_role_id, created_at, updated_at FROM sb_roles WHERE id = %s LIMIT 1", s.placeholder(1))
	row := s.db.QueryRowContext(ctx, query, roleID)
	role, err := scanRoleRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coreauth.RoleRecord{}, coreauth.ErrRoleNotFound
		}
		return coreauth.RoleRecord{}, err
	}
	return role, nil
}

func (s *Store) UpdateRole(ctx context.Context, roleID string, updates coreauth.UpdateRoleStoreInput) (coreauth.RoleRecord, error) {
	if strings.TrimSpace(roleID) == "" {
		return coreauth.RoleRecord{}, fmt.Errorf("%w: role id is required", coreauth.ErrInvalidInput)
	}

	parts := make([]string, 0, 4)
	args := make([]any, 0, 5)
	idx := 1
	if updates.Name != nil {
		parts = append(parts, fmt.Sprintf("name = %s", s.placeholder(idx)))
		args = append(args, strings.TrimSpace(*updates.Name))
		idx++
	}
	if updates.Description != nil {
		parts = append(parts, fmt.Sprintf("description = %s", s.placeholder(idx)))
		args = append(args, strings.TrimSpace(*updates.Description))
		idx++
	}
	if updates.ParentRoleID != nil {
		parts = append(parts, fmt.Sprintf("parent_role_id = %s", s.placeholder(idx)))
		args = append(args, normalizeOptionalID(updates.ParentRoleID))
		idx++
	}
	parts = append(parts, fmt.Sprintf("updated_at = %s", s.placeholder(idx)))
	args = append(args, time.Now().UTC())
	idx++

	query := fmt.Sprintf("UPDATE sb_roles SET %s WHERE id = %s", strings.Join(parts, ", "), s.placeholder(idx))
	args = append(args, roleID)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.RoleRecord{}, coreauth.ErrConflict
		}
		if isForeignKeyViolation(err) {
			return coreauth.RoleRecord{}, coreauth.ErrRoleNotFound
		}
		return coreauth.RoleRecord{}, fmt.Errorf("update role: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return coreauth.RoleRecord{}, fmt.Errorf("update role rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.RoleRecord{}, coreauth.ErrRoleNotFound
	}
	return s.GetRoleByID(ctx, roleID)
}

func (s *Store) DeleteRole(ctx context.Context, roleID string) error {
	query := fmt.Sprintf("DELETE FROM sb_roles WHERE id = %s", s.placeholder(1))
	res, err := s.db.ExecContext(ctx, query, roleID)
	if err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete role rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.ErrRoleNotFound
	}
	return nil
}

func scanRoleRow(scanFn func(dest ...any) error) (coreauth.RoleRecord, error) {
	var (
		id          string
		name        string
		description string
		parentRaw   any
		createdRaw  any
		updatedRaw  any
	)
	if err := scanFn(&id, &name, &description, &parentRaw, &createdRaw, &updatedRaw); err != nil {
		return coreauth.RoleRecord{}, err
	}
	parentRoleID, err := decodeOptionalString(parentRaw)
	if err != nil {
		return coreauth.RoleRecord{}, err
	}
	createdAt, err := decodeRequiredTime(createdRaw)
	if err != nil {
		return coreauth.RoleRecord{}, err
	}
	updatedAt, err := decodeRequiredTime(updatedRaw)
	if err != nil {
		return coreauth.RoleRecord{}, err
	}
	return coreauth.RoleRecord{
		ID:           id,
		Name:         name,
		Description:  description,
		ParentRoleID: parentRoleID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}
