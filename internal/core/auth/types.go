package auth

import "time"

type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         UserRole
	IsRoot       bool
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
