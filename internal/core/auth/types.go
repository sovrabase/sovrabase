package auth

import "time"

type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
	UserRoleSvc   UserRole = "service"
)

type AccountType string

const (
	AccountTypeAdmin   AccountType = "admin"
	AccountTypeEndUser AccountType = "end_user"
	AccountTypeService AccountType = "service"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         UserRole
	AccountType  AccountType
	IsRoot       bool
	IsActive     bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PublicUser struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Role  UserRole `json:"role"`
}

type AuthResult struct {
	TokenType   string     `json:"token_type"`
	AccessToken string     `json:"access_token"`
	ExpiresIn   int        `json:"expires_in"`
	User        PublicUser `json:"user"`
}

type CreateUserInput struct {
	ActorUserID string
	Email       string
	Password    string
	Role        UserRole
	AccountType AccountType
}

type UpdateUserInput struct {
	ActorUserID string
	UserID      string
	Email       *string
	Password    *string
	Role        *UserRole
	AccountType *AccountType
	IsActive    *bool
}

type RoleRecord struct {
	ID           string
	Name         string
	Description  string
	ParentRoleID *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ScopeRecord struct {
	ID          string
	Key         string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateRoleInput struct {
	ActorUserID  string
	Name         string
	Description  string
	ParentRoleID *string
}

type UpdateRoleInput struct {
	ActorUserID  string
	RoleID       string
	Name         *string
	Description  *string
	ParentRoleID *string
}

type CreateScopeInput struct {
	ActorUserID string
	Key         string
	Description string
}

type UpdateScopeInput struct {
	ActorUserID string
	ScopeID     string
	Key         *string
	Description *string
}
