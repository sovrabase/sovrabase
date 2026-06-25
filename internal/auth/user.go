package auth

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role represents a user's authorization level.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// OAuthProviderMetadata stores per-provider OAuth data for a user.
// A user can link multiple providers (Discord + GitHub, etc.) without overwriting.
type OAuthProviderMetadata struct {
	Provider     string    `json:"provider"`
	ProviderID   string    `json:"provider_id"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	TokenExpiry  time.Time `json:"-"`
}

// User represents a Sovrabase user account.
type User struct {
	ID                  string                   `json:"id"`
	Email               string                   `json:"email"`
	Username            string                   `json:"username,omitempty"`
	PasswordHash        string                   `json:"-"`
	Role                Role                     `json:"role"`
	Name                string                   `json:"name,omitempty"`
	AvatarURL           string                   `json:"avatar_url,omitempty"`

	OAuthProviders      []OAuthProviderMetadata  `json:"_metadata,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
	IsVerified          bool                     `json:"is_verified"`
	VerificationToken   string                   `json:"verification_token,omitempty"`
	VerificationExpires time.Time                `json:"verification_expires,omitempty"`
	ResetToken          string                   `json:"reset_token,omitempty"`
	ResetExpires        time.Time                `json:"reset_expires,omitempty"`
	MagicLinkToken      string                   `json:"magic_link_token,omitempty"`
	MagicLinkExpires    time.Time                `json:"magic_link_expires,omitempty"`
	MFAEnabled          bool                     `json:"mfa_enabled"`
	MFASecret           string                   `json:"-"`
	MFABackupCodes      []string                 `json:"-"`
}

// TokenPair contains the access and refresh tokens returned on login/signup.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // seconds until access token expires
}

// NewUser creates a User with a generated UUID and timestamps.
func NewUser(email, passwordHash string) *User {
	now := time.Now().UTC()
	return &User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: passwordHash,
		Role:         RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword compares a bcrypt hash against a plaintext password.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
