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

func (s *Store) CreateScope(ctx context.Context, key, description string) (coreauth.ScopeRecord, error) {
	now := time.Now().UTC()
	id := uuid.NewString()
	query := fmt.Sprintf(`INSERT INTO sb_scopes (id, scope_key, description, created_at, updated_at) VALUES (%s)`, s.placeholders(5))
	_, err := s.db.ExecContext(ctx, query, id, strings.TrimSpace(key), strings.TrimSpace(description), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.ScopeRecord{}, coreauth.ErrConflict
		}
		return coreauth.ScopeRecord{}, fmt.Errorf("create scope: %w", err)
	}
	return s.GetScopeByID(ctx, id)
}

func (s *Store) ListScopes(ctx context.Context) ([]coreauth.ScopeRecord, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, scope_key, description, created_at, updated_at FROM sb_scopes ORDER BY scope_key ASC")
	if err != nil {
		return nil, fmt.Errorf("list scopes: %w", err)
	}
	defer rows.Close()
	scopes := make([]coreauth.ScopeRecord, 0)
	for rows.Next() {
		scope, scanErr := scanScopeRow(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list scopes rows: %w", err)
	}
	return scopes, nil
}

func (s *Store) GetScopeByID(ctx context.Context, scopeID string) (coreauth.ScopeRecord, error) {
	query := fmt.Sprintf("SELECT id, scope_key, description, created_at, updated_at FROM sb_scopes WHERE id = %s LIMIT 1", s.placeholder(1))
	row := s.db.QueryRowContext(ctx, query, scopeID)
	scope, err := scanScopeRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coreauth.ScopeRecord{}, coreauth.ErrScopeNotFound
		}
		return coreauth.ScopeRecord{}, err
	}
	return scope, nil
}

func (s *Store) UpdateScope(ctx context.Context, scopeID string, updates coreauth.UpdateScopeStoreInput) (coreauth.ScopeRecord, error) {
	parts := make([]string, 0, 3)
	args := make([]any, 0, 4)
	idx := 1
	if updates.Key != nil {
		parts = append(parts, fmt.Sprintf("scope_key = %s", s.placeholder(idx)))
		args = append(args, strings.TrimSpace(*updates.Key))
		idx++
	}
	if updates.Description != nil {
		parts = append(parts, fmt.Sprintf("description = %s", s.placeholder(idx)))
		args = append(args, strings.TrimSpace(*updates.Description))
		idx++
	}
	parts = append(parts, fmt.Sprintf("updated_at = %s", s.placeholder(idx)))
	args = append(args, time.Now().UTC())
	idx++

	query := fmt.Sprintf("UPDATE sb_scopes SET %s WHERE id = %s", strings.Join(parts, ", "), s.placeholder(idx))
	args = append(args, scopeID)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.ScopeRecord{}, coreauth.ErrConflict
		}
		return coreauth.ScopeRecord{}, fmt.Errorf("update scope: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return coreauth.ScopeRecord{}, fmt.Errorf("update scope rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.ScopeRecord{}, coreauth.ErrScopeNotFound
	}
	return s.GetScopeByID(ctx, scopeID)
}

func (s *Store) DeleteScope(ctx context.Context, scopeID string) error {
	query := fmt.Sprintf("DELETE FROM sb_scopes WHERE id = %s", s.placeholder(1))
	res, err := s.db.ExecContext(ctx, query, scopeID)
	if err != nil {
		return fmt.Errorf("delete scope: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete scope rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.ErrScopeNotFound
	}
	return nil
}

func scanScopeRow(scanFn func(dest ...any) error) (coreauth.ScopeRecord, error) {
	var (
		id          string
		key         string
		description string
		createdRaw  any
		updatedRaw  any
	)
	if err := scanFn(&id, &key, &description, &createdRaw, &updatedRaw); err != nil {
		return coreauth.ScopeRecord{}, err
	}
	createdAt, err := decodeRequiredTime(createdRaw)
	if err != nil {
		return coreauth.ScopeRecord{}, err
	}
	updatedAt, err := decodeRequiredTime(updatedRaw)
	if err != nil {
		return coreauth.ScopeRecord{}, err
	}
	return coreauth.ScopeRecord{ID: id, Key: key, Description: description, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}
