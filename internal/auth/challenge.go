package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const challengeTokenTTL = 5 * time.Minute

// ChallengeClaims extends standard JWT claims with MFA-challenge-specific fields.
type ChallengeClaims struct {
	jwt.RegisteredClaims
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Purpose   string `json:"purpose"` // always "mfa_challenge"
}

// GenerateChallengeToken creates a short-lived JWT that can only be used to
// complete an MFA challenge. It does NOT grant API access.
func GenerateChallengeToken(user *User, secret string) (string, error) {
	now := time.Now().UTC()
	claims := &ChallengeClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "sovrabase",
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(challengeTokenTTL)),
		},
		UserID:  user.ID,
		Email:   user.Email,
		Purpose: "mfa_challenge",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateChallengeToken parses and validates an MFA challenge token.
func ValidateChallengeToken(tokenString string, secret string) (*ChallengeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ChallengeClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid challenge token: %w", err)
	}

	claims, ok := token.Claims.(*ChallengeClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid challenge token claims")
	}
	if claims.Purpose != "mfa_challenge" {
		return nil, fmt.Errorf("token is not an MFA challenge token")
	}

	return claims, nil
}
