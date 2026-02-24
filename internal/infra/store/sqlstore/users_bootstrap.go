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

func (s *Store) BootstrapRequired(ctx context.Context) (bool, error) {
	query := fmt.Sprintf(`SELECT 1 FROM sb_users WHERE is_root = %s LIMIT 1`, s.placeholder(1))
	var sentinel int
	err := s.db.QueryRowContext(ctx, query, s.boolTrue()).Scan(&sentinel)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check root admin existence: %w", err)
	}
	return false, nil
}

func (s *Store) CreateFirstAdmin(ctx context.Context, email, passwordHash string) (coreauth.User, error) {
	if strings.TrimSpace(email) == "" {
		return coreauth.User{}, fmt.Errorf("%w: email is required", coreauth.ErrInvalidInput)
	}
	if strings.TrimSpace(passwordHash) == "" {
		return coreauth.User{}, fmt.Errorf("%w: password_hash is required", coreauth.ErrInvalidInput)
	}

	now := time.Now().UTC()
	user := coreauth.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         coreauth.UserRoleAdmin,
		AccountType:  coreauth.AccountTypeAdmin,
		IsRoot:       true,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coreauth.User{}, fmt.Errorf("begin create first admin transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	required, err := s.bootstrapRequiredTx(ctx, tx)
	if err != nil {
		return coreauth.User{}, err
	}
	if !required {
		return coreauth.User{}, coreauth.ErrBootstrapAlreadyDone
	}

	query := s.insertFirstAdminQuery()
	res, execErr := tx.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		string(user.AccountType),
		s.boolTrue(),
		s.boolTrue(),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if execErr != nil {
		return coreauth.User{}, fmt.Errorf("insert first admin: %w", execErr)
	}

	affected, execErr := res.RowsAffected()
	if execErr != nil {
		return coreauth.User{}, fmt.Errorf("insert first admin rows affected: %w", execErr)
	}
	if affected == 0 {
		return coreauth.User{}, coreauth.ErrBootstrapAlreadyDone
	}

	if err := s.assignDefaultRoleByNameTx(ctx, tx, user.ID, string(user.Role), now); err != nil {
		return coreauth.User{}, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return coreauth.User{}, fmt.Errorf("commit create first admin transaction: %w", commitErr)
	}
	committed = true

	return user, nil
}

func (s *Store) bootstrapRequiredTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	query := fmt.Sprintf(`SELECT 1 FROM sb_users WHERE is_root = %s LIMIT 1`, s.placeholder(1))
	var sentinel int
	err := tx.QueryRowContext(ctx, query, s.boolTrue()).Scan(&sentinel)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("check root admin existence (tx): %w", err)
	}
	return false, nil
}

func (s *Store) insertFirstAdminQuery() string {
	if s.dialect == DialectPostgres {
		return `INSERT INTO sb_users (
  id, email, password_hash, role, account_type, is_root, is_active, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT DO NOTHING`
	}
	return `INSERT OR IGNORE INTO sb_users (
  id, email, password_hash, role, account_type, is_root, is_active, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
}

func (s *Store) boolTrue() any {
	if s.dialect == DialectSQLite {
		return 1
	}
	return true
}
