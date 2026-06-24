package api

import (
	"context"
	"net/http"
	"strings"
)

// projectMiddleware extracts the X-Project-Key header, resolves it to a ProjectEnv, and injects it into the context.
func (s *Server) projectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		projectKey := r.Header.Get("X-Project-Key")
		if projectKey == "" {
			writeError(w, http.StatusBadRequest, "missing X-Project-Key header")
			return
		}

		proj, err := s.projects.GetProjectBySecret(projectKey)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid project key")
			return
		}

		env, err := s.projects.GetProjectEnv(proj.ID)
		if err != nil {
			s.logger.Error("failed to load project environment", "project_id", proj.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "failed to load project environment")
			return
		}

		ctx := context.WithValue(r.Context(), projectEnvKey, env)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authMiddleware validates the Bearer token and injects user claims into the context.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			writeError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
			return
		}

		claims, err := s.getAuth(r).ValidateAccessToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getClaims extracts user claims from the request context.
func getClaims(r *http.Request) *UserClaims {
	claims, ok := r.Context().Value(claimsKey).(*UserClaims)
	if !ok {
		return nil
	}
	return claims
}
