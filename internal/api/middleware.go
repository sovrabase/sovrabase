package api

import (
	"context"
	"net/http"
	"strings"
)

// authMiddleware validates the Bearer token and injects user claims into the context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
			return
		}

		claims, err := s.auth.ValidateAccessToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey string

const claimsKey contextKey = "claims"

// getClaims extracts user claims from the request context.
func getClaims(r *http.Request) *UserClaims {
	claims, ok := r.Context().Value(claimsKey).(*UserClaims)
	if !ok {
		return nil
	}
	return claims
}
