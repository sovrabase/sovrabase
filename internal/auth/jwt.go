package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// DefaultAccessTokenTTL is used when no session duration is configured.
	DefaultAccessTokenTTL = 24 * time.Hour
	refreshTokenTTL       = 7 * 24 * time.Hour
)

// Claims extends the standard JWT claims with Sovrabase-specific fields.
type Claims struct {
	jwt.RegisteredClaims
	UserID      string `json:"user_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	AdminRole   string `json:"admin_role,omitempty"`   // admin RBAC role: super_admin, admin, support
	ProjectRole string `json:"project_role,omitempty"` // team role within a specific project
}

// GenerateAccessToken creates a JWT for API authentication with the given TTL.
// If ttl is zero, DefaultAccessTokenTTL (24h) is used.
func GenerateAccessToken(user *User, secret string, ttl ...time.Duration) (string, error) {
	expiry := DefaultAccessTokenTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		expiry = ttl[0]
	}
	now := time.Now().UTC()
	jti, err := generateJTI()
	if err != nil {
		return "", fmt.Errorf("generating token ID: %w", err)
	}
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sovrabase",
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			ID:        jti,
		},
		UserID: user.ID,
		Email:  user.Email,
		Role:   string(user.Role),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateRefreshToken creates a long-lived JWT for obtaining new access tokens.
func GenerateRefreshToken(user *User, secret string) (string, error) {
	now := time.Now().UTC()
	jti, err := generateJTI()
	if err != nil {
		return "", fmt.Errorf("generating token ID: %w", err)
	}
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sovrabase",
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTokenTTL)),
			ID:        jti,
		},
		UserID: user.ID,
		Email:  user.Email,
		Role:   string(user.Role),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// generateJTI creates a cryptographically random 16-byte hex JWT ID.
func generateJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateToken parses and validates a JWT token, returning its claims.
func ValidateToken(tokenString string, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
