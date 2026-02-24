package crypto

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TokenType permet de différencier un token Admin d'un token User
type TokenType string

const (
	TokenTypeAdmin TokenType = "admin"
	TokenTypeUser  TokenType = "user"
)

type Claims struct {
	UserID string    `json:"uid"`
	Type   TokenType `json:"type"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// Outils de Hashage (utilisés par Admin ET Auth)
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Générateur de Token unique
func GenerateToken(userID string, tType TokenType, secret string) (string, error) {
	return GenerateTokenWithRole(userID, "", tType, secret, 24*time.Hour)
}

func GenerateTokenWithRole(userID, role string, tType TokenType, secret string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	claims := Claims{
		UserID: userID,
		Type:   tType,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseToken(tokenRaw, secret string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenRaw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected jwt signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return Claims{}, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid jwt claims")
	}
	return *claims, nil
}
