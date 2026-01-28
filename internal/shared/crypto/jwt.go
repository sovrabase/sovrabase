package crypto

import (
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
	claims := Claims{
		UserID: userID,
		Type:   tType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
