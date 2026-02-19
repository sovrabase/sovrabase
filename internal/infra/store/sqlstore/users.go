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
		IsRoot:       true,
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

	if commitErr := tx.Commit(); commitErr != nil {
		return coreauth.User{}, fmt.Errorf("commit create first admin transaction: %w", commitErr)
	}
	committed = true

	return user, nil
}

func (s *Store) GetByEmail(ctx context.Context, email string) (coreauth.User, error) {
	if strings.TrimSpace(email) == "" {
		return coreauth.User{}, fmt.Errorf("%w: email is required", coreauth.ErrInvalidInput)
	}

	query := fmt.Sprintf(`SELECT %s FROM sb_users WHERE email = %s LIMIT 1`, userColumns(), s.placeholder(1))
	row := s.db.QueryRowContext(ctx, query, email)
	user, err := scanUserRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coreauth.User{}, coreauth.ErrUserNotFound
		}
		return coreauth.User{}, err
	}
	return user, nil
}

func (s *Store) TouchLastLogin(ctx context.Context, userID string, at time.Time) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: user id is required", coreauth.ErrInvalidInput)
	}

	query := fmt.Sprintf(
		`UPDATE sb_users SET last_login_at = %s, updated_at = %s WHERE id = %s`,
		s.placeholder(1),
		s.placeholder(2),
		s.placeholder(3),
	)

	res, err := s.db.ExecContext(ctx, query, at, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("update user last login: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user last login rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.ErrUserNotFound
	}
	return nil
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
  id, email, password_hash, role, is_root, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING`
	}
	return `INSERT OR IGNORE INTO sb_users (
  id, email, password_hash, role, is_root, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`
}

func userColumns() string {
	return strings.Join([]string{
		"id",
		"email",
		"password_hash",
		"role",
		"is_root",
		"last_login_at",
		"created_at",
		"updated_at",
	}, ", ")
}

func scanUserRow(scanFn func(dest ...any) error) (coreauth.User, error) {
	var (
		id           string
		email        string
		passwordHash string
		roleRaw      string
		isRootRaw    any
		lastLoginRaw any
		createdAtRaw any
		updatedAtRaw any
	)

	if err := scanFn(
		&id,
		&email,
		&passwordHash,
		&roleRaw,
		&isRootRaw,
		&lastLoginRaw,
		&createdAtRaw,
		&updatedAtRaw,
	); err != nil {
		return coreauth.User{}, err
	}

	isRoot, err := decodeBool(isRootRaw)
	if err != nil {
		return coreauth.User{}, err
	}
	lastLoginAt, err := decodeOptionalTime(lastLoginRaw)
	if err != nil {
		return coreauth.User{}, err
	}
	createdAt, err := decodeRequiredTime(createdAtRaw)
	if err != nil {
		return coreauth.User{}, err
	}
	updatedAt, err := decodeRequiredTime(updatedAtRaw)
	if err != nil {
		return coreauth.User{}, err
	}

	role := coreauth.UserRole(roleRaw)
	if role != coreauth.UserRoleAdmin {
		return coreauth.User{}, fmt.Errorf("unsupported user role %q", roleRaw)
	}

	return coreauth.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		IsRoot:       isRoot,
		LastLoginAt:  lastLoginAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func (s *Store) boolTrue() any {
	if s.dialect == DialectSQLite {
		return 1
	}
	return true
}
