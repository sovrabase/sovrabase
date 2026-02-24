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

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string, role coreauth.UserRole, accountType coreauth.AccountType, isRoot bool) (coreauth.User, error) {
	if strings.TrimSpace(email) == "" {
		return coreauth.User{}, fmt.Errorf("%w: email is required", coreauth.ErrInvalidInput)
	}
	if strings.TrimSpace(passwordHash) == "" {
		return coreauth.User{}, fmt.Errorf("%w: password_hash is required", coreauth.ErrInvalidInput)
	}

	now := time.Now().UTC()
	user := coreauth.User{
		ID:           uuid.NewString(),
		Email:        strings.TrimSpace(email),
		PasswordHash: passwordHash,
		Role:         role,
		AccountType:  accountType,
		IsRoot:       isRoot,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return coreauth.User{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	query := fmt.Sprintf(`INSERT INTO sb_users (
  id, email, password_hash, role, account_type, is_root, is_active, created_at, updated_at
) VALUES (%s)`, s.placeholders(9))

	_, err = tx.ExecContext(
		ctx,
		query,
		user.ID,
		user.Email,
		user.PasswordHash,
		string(user.Role),
		string(user.AccountType),
		isRoot,
		s.boolTrue(),
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.User{}, coreauth.ErrConflict
		}
		return coreauth.User{}, fmt.Errorf("insert user: %w", err)
	}

	if err := s.assignDefaultRoleByNameTx(ctx, tx, user.ID, string(user.Role), now); err != nil {
		return coreauth.User{}, err
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return coreauth.User{}, fmt.Errorf("commit create user transaction: %w", commitErr)
	}
	committed = true

	return user, nil
}

func (s *Store) ListUsers(ctx context.Context) ([]coreauth.User, error) {
	query := fmt.Sprintf(`SELECT %s FROM sb_users ORDER BY created_at DESC`, userColumns())
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]coreauth.User, 0)
	for rows.Next() {
		user, scanErr := scanUserRow(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list users rows: %w", err)
	}
	return users, nil
}

func (s *Store) GetByID(ctx context.Context, userID string) (coreauth.User, error) {
	if strings.TrimSpace(userID) == "" {
		return coreauth.User{}, fmt.Errorf("%w: user id is required", coreauth.ErrInvalidInput)
	}

	query := fmt.Sprintf(`SELECT %s FROM sb_users WHERE id = %s LIMIT 1`, userColumns(), s.placeholder(1))
	row := s.db.QueryRowContext(ctx, query, userID)
	user, err := scanUserRow(row.Scan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return coreauth.User{}, coreauth.ErrUserNotFound
		}
		return coreauth.User{}, err
	}
	return user, nil
}

func (s *Store) UpdateUser(ctx context.Context, userID string, updates coreauth.UpdateUserStoreInput) (coreauth.User, error) {
	if strings.TrimSpace(userID) == "" {
		return coreauth.User{}, fmt.Errorf("%w: user id is required", coreauth.ErrInvalidInput)
	}

	parts := make([]string, 0, 6)
	args := make([]any, 0, 8)
	idx := 1

	if updates.Email != nil {
		parts = append(parts, fmt.Sprintf("email = %s", s.placeholder(idx)))
		args = append(args, strings.TrimSpace(*updates.Email))
		idx++
	}
	if updates.PasswordHash != nil {
		parts = append(parts, fmt.Sprintf("password_hash = %s", s.placeholder(idx)))
		args = append(args, *updates.PasswordHash)
		idx++
	}
	if updates.Role != nil {
		parts = append(parts, fmt.Sprintf("role = %s", s.placeholder(idx)))
		args = append(args, string(*updates.Role))
		idx++
	}
	if updates.AccountType != nil {
		parts = append(parts, fmt.Sprintf("account_type = %s", s.placeholder(idx)))
		args = append(args, string(*updates.AccountType))
		idx++
	}
	if updates.IsActive != nil {
		parts = append(parts, fmt.Sprintf("is_active = %s", s.placeholder(idx)))
		args = append(args, *updates.IsActive)
		idx++
	}
	parts = append(parts, fmt.Sprintf("updated_at = %s", s.placeholder(idx)))
	args = append(args, time.Now().UTC())
	idx++

	if len(parts) == 1 {
		return s.GetByID(ctx, userID)
	}

	query := fmt.Sprintf("UPDATE sb_users SET %s WHERE id = %s", strings.Join(parts, ", "), s.placeholder(idx))
	args = append(args, userID)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		if isUniqueViolation(err) {
			return coreauth.User{}, coreauth.ErrConflict
		}
		return coreauth.User{}, fmt.Errorf("update user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return coreauth.User{}, fmt.Errorf("update user rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.User{}, coreauth.ErrUserNotFound
	}

	if updates.Role != nil {
		if err := s.assignDefaultRoleByName(ctx, userID, string(*updates.Role)); err != nil {
			return coreauth.User{}, err
		}
	}

	return s.GetByID(ctx, userID)
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: user id is required", coreauth.ErrInvalidInput)
	}
	query := fmt.Sprintf("DELETE FROM sb_users WHERE id = %s", s.placeholder(1))
	res, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if affected == 0 {
		return coreauth.ErrUserNotFound
	}
	return nil
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

func userColumns() string {
	return strings.Join([]string{
		"id",
		"email",
		"password_hash",
		"role",
		"account_type",
		"is_root",
		"is_active",
		"last_login_at",
		"created_at",
		"updated_at",
	}, ", ")
}

func scanUserRow(scanFn func(dest ...any) error) (coreauth.User, error) {
	var (
		id             string
		email          string
		passwordHash   string
		roleRaw        string
		accountTypeRaw string
		isRootRaw      any
		isActiveRaw    any
		lastLoginRaw   any
		createdAtRaw   any
		updatedAtRaw   any
	)

	if err := scanFn(
		&id,
		&email,
		&passwordHash,
		&roleRaw,
		&accountTypeRaw,
		&isRootRaw,
		&isActiveRaw,
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
	isActive, err := decodeBool(isActiveRaw)
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

	return coreauth.User{
		ID:           id,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         coreauth.UserRole(roleRaw),
		AccountType:  coreauth.AccountType(accountTypeRaw),
		IsRoot:       isRoot,
		IsActive:     isActive,
		LastLoginAt:  lastLoginAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func (s *Store) assignDefaultRoleByName(ctx context.Context, userID, roleName string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin role assignment transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := s.assignDefaultRoleByNameTx(ctx, tx, userID, roleName, time.Now().UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role assignment transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *Store) assignDefaultRoleByNameTx(ctx context.Context, tx *sql.Tx, userID, roleName string, now time.Time) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(roleName) == "" {
		return nil
	}

	queryRole := fmt.Sprintf("SELECT id FROM sb_roles WHERE name = %s LIMIT 1", s.placeholder(1))
	var roleID string
	if err := tx.QueryRowContext(ctx, queryRole, roleName).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("resolve role %q: %w", roleName, err)
	}

	queryAssign := s.insertIgnore("sb_user_roles", "user_id, role_id, created_at", 3)
	if _, err := tx.ExecContext(ctx, queryAssign, userID, roleID, now); err != nil {
		return fmt.Errorf("assign default role: %w", err)
	}
	return nil
}
