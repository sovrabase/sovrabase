package auth

import (
	"context"
	"time"
)

type UserStore interface {
	BootstrapRequired(ctx context.Context) (bool, error)
	CreateFirstAdmin(ctx context.Context, email, passwordHash string) (User, error)
	CreateUser(ctx context.Context, email, passwordHash string, role UserRole, accountType AccountType, isRoot bool) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	GetByID(ctx context.Context, userID string) (User, error)
	UpdateUser(ctx context.Context, userID string, updates UpdateUserStoreInput) (User, error)
	DeleteUser(ctx context.Context, userID string) error
	GetByEmail(ctx context.Context, email string) (User, error)
	TouchLastLogin(ctx context.Context, userID string, at time.Time) error

	CreateRole(ctx context.Context, name, description string, parentRoleID *string) (RoleRecord, error)
	ListRoles(ctx context.Context) ([]RoleRecord, error)
	GetRoleByID(ctx context.Context, roleID string) (RoleRecord, error)
	UpdateRole(ctx context.Context, roleID string, updates UpdateRoleStoreInput) (RoleRecord, error)
	DeleteRole(ctx context.Context, roleID string) error

	CreateScope(ctx context.Context, key, description string) (ScopeRecord, error)
	ListScopes(ctx context.Context) ([]ScopeRecord, error)
	GetScopeByID(ctx context.Context, scopeID string) (ScopeRecord, error)
	UpdateScope(ctx context.Context, scopeID string, updates UpdateScopeStoreInput) (ScopeRecord, error)
	DeleteScope(ctx context.Context, scopeID string) error

	AssignRoleToUser(ctx context.Context, userID, roleID string) error
	RemoveRoleFromUser(ctx context.Context, userID, roleID string) error
	AssignScopeToRole(ctx context.Context, roleID, scopeID string) error
	RemoveScopeFromRole(ctx context.Context, roleID, scopeID string) error
	ResolveUserScopes(ctx context.Context, userID string) ([]string, error)
	RoleParentContains(ctx context.Context, roleID, parentRoleID string) (bool, error)
}

type UpdateUserStoreInput struct {
	Email        *string
	PasswordHash *string
	Role         *UserRole
	AccountType  *AccountType
	IsActive     *bool
}

type UpdateRoleStoreInput struct {
	Name         *string
	Description  *string
	ParentRoleID *string
}

type UpdateScopeStoreInput struct {
	Key         *string
	Description *string
}
