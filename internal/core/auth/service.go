package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	sharedcrypto "github.com/ketsuna-org/sovrabase/internal/shared/crypto"
)

const (
	DefaultTokenTTL   = 24 * time.Hour
	minPasswordLength = 12
	defaultTokenType  = "Bearer"
)

type Service interface {
	GetConfigState(ctx context.Context) (bootstrapRequired bool, err error)
	BootstrapFirstAdmin(ctx context.Context, email, password string) (AuthResult, error)
	Login(ctx context.Context, email, password string) (AuthResult, error)
	CreateAdmin(ctx context.Context, actorUserID, email, password string) (User, error)
	CreateUser(ctx context.Context, input CreateUserInput) (User, error)
	ListUsers(ctx context.Context, actorUserID string) ([]User, error)
	GetUser(ctx context.Context, actorUserID, userID string) (User, error)
	UpdateUser(ctx context.Context, input UpdateUserInput) (User, error)
	DeleteUser(ctx context.Context, actorUserID, userID string) error

	CreateRole(ctx context.Context, input CreateRoleInput) (RoleRecord, error)
	ListRoles(ctx context.Context, actorUserID string) ([]RoleRecord, error)
	GetRole(ctx context.Context, actorUserID, roleID string) (RoleRecord, error)
	UpdateRole(ctx context.Context, input UpdateRoleInput) (RoleRecord, error)
	DeleteRole(ctx context.Context, actorUserID, roleID string) error

	CreateScope(ctx context.Context, input CreateScopeInput) (ScopeRecord, error)
	ListScopes(ctx context.Context, actorUserID string) ([]ScopeRecord, error)
	GetScope(ctx context.Context, actorUserID, scopeID string) (ScopeRecord, error)
	UpdateScope(ctx context.Context, input UpdateScopeInput) (ScopeRecord, error)
	DeleteScope(ctx context.Context, actorUserID, scopeID string) error

	AssignRoleToUser(ctx context.Context, actorUserID, userID, roleID string) error
	RemoveRoleFromUser(ctx context.Context, actorUserID, userID, roleID string) error
	AssignScopeToRole(ctx context.Context, actorUserID, roleID, scopeID string) error
	RemoveScopeFromRole(ctx context.Context, actorUserID, roleID, scopeID string) error

	Authorize(ctx context.Context, actorUserID, scope string) error
}

type ServiceDeps struct {
	Store     UserStore
	JWTSecret string
	TokenTTL  time.Duration
	NowFn     func() time.Time
}

type service struct {
	store     UserStore
	jwtSecret string
	tokenTTL  time.Duration
	nowFn     func() time.Time
}

func NewService(deps ServiceDeps) (Service, error) {
	if deps.Store == nil {
		return nil, errors.New("auth store is required")
	}
	if strings.TrimSpace(deps.JWTSecret) == "" {
		return nil, errors.New("jwt secret is required")
	}
	if deps.TokenTTL <= 0 {
		deps.TokenTTL = DefaultTokenTTL
	}
	if deps.NowFn == nil {
		deps.NowFn = func() time.Time { return time.Now().UTC() }
	}

	return &service{
		store:     deps.Store,
		jwtSecret: deps.JWTSecret,
		tokenTTL:  deps.TokenTTL,
		nowFn:     deps.NowFn,
	}, nil
}

func (s *service) GetConfigState(ctx context.Context) (bool, error) {
	return s.store.BootstrapRequired(ctx)
}

func (s *service) BootstrapFirstAdmin(ctx context.Context, email, password string) (AuthResult, error) {
	normalizedEmail, err := normalizeAndValidateEmail(email)
	if err != nil {
		return AuthResult{}, err
	}
	if err := validatePassword(password); err != nil {
		return AuthResult{}, err
	}

	required, err := s.store.BootstrapRequired(ctx)
	if err != nil {
		return AuthResult{}, fmt.Errorf("check bootstrap state: %w", err)
	}
	if !required {
		return AuthResult{}, ErrBootstrapAlreadyDone
	}

	passwordHash, err := sharedcrypto.HashPassword(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash admin password: %w", err)
	}

	user, err := s.store.CreateFirstAdmin(ctx, normalizedEmail, passwordHash)
	if err != nil {
		if errors.Is(err, ErrBootstrapAlreadyDone) {
			return AuthResult{}, ErrBootstrapAlreadyDone
		}
		return AuthResult{}, fmt.Errorf("create first admin: %w", err)
	}

	return s.buildAuthResult(user)
}

func (s *service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	normalizedEmail, err := normalizeAndValidateEmail(email)
	if err != nil {
		return AuthResult{}, err
	}
	if err := validatePassword(password); err != nil {
		return AuthResult{}, err
	}

	required, err := s.store.BootstrapRequired(ctx)
	if err != nil {
		return AuthResult{}, fmt.Errorf("check bootstrap state: %w", err)
	}
	if required {
		return AuthResult{}, ErrBootstrapRequired
	}

	user, err := s.store.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, fmt.Errorf("find user by email: %w", err)
	}

	if !sharedcrypto.CheckPassword(password, user.PasswordHash) {
		return AuthResult{}, ErrInvalidCredentials
	}

	now := s.nowFn()
	if err := s.store.TouchLastLogin(ctx, user.ID, now); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, fmt.Errorf("touch last login: %w", err)
	}

	user.LastLoginAt = &now
	return s.buildAuthResult(user)
}

func (s *service) buildAuthResult(user User) (AuthResult, error) {
	token, err := sharedcrypto.GenerateTokenWithRole(user.ID, string(user.Role), sharedcrypto.TokenTypeAdmin, s.jwtSecret, s.tokenTTL)
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate jwt: %w", err)
	}

	return AuthResult{
		TokenType:   defaultTokenType,
		AccessToken: token,
		ExpiresIn:   int(s.tokenTTL.Seconds()),
		User: PublicUser{
			ID:    user.ID,
			Email: user.Email,
			Role:  user.Role,
		},
	}, nil
}

func normalizeAndValidateEmail(email string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(email))
	if value == "" {
		return "", fmt.Errorf("%w: email is required", ErrInvalidInput)
	}
	if _, err := mail.ParseAddress(value); err != nil {
		return "", fmt.Errorf("%w: email is invalid", ErrInvalidInput)
	}
	return value, nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("%w: password is required", ErrInvalidInput)
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}
	return nil
}

func validateRole(role UserRole) error {
	switch role {
	case UserRoleAdmin, UserRoleUser, UserRoleSvc:
		return nil
	default:
		return fmt.Errorf("%w: unsupported role", ErrInvalidInput)
	}
}

func validateAccountType(accountType AccountType) error {
	switch accountType {
	case AccountTypeAdmin, AccountTypeEndUser, AccountTypeService:
		return nil
	default:
		return fmt.Errorf("%w: unsupported account_type", ErrInvalidInput)
	}
}
