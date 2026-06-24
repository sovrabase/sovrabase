package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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
		ctx = context.WithValue(ctx, projectIDKey, proj.ID)
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

// auditLoggerMiddleware logs mutation requests to the project's requests.log file.
// It wraps mutation methods (POST/PUT/PATCH/DELETE) on /api/v1/collections/.
func (s *Server) auditLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}

		projectID := getProjectID(r)
		if projectID == "" {
			return
		}

		claims := getClaims(r)
		email := ""
		if claims != nil {
			email = claims.Email
		}

		logFile := filepath.Join(s.config.DataDir, "projects", projectID, "requests.log")
		_ = os.MkdirAll(filepath.Dir(logFile), 0755)
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		defer f.Close()

		logEntry := map[string]interface{}{
			"timestamp":  time.Now().Format(time.RFC3339Nano),
			"method":     r.Method,
			"path":       r.URL.Path,
			"email":      email,
			"status":     sw.status,
			"duration":   time.Since(start).String(),
			"collection": strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/api/v1/collections/"), "/"),
		}

		// Extract doc_id from path if present (e.g., /api/v1/collections/{collection}/{id})
		pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/collections/"), "/")
		if len(pathParts) >= 2 {
			logEntry["doc_id"] = pathParts[1]
		}

		bytes, err := json.Marshal(logEntry)
		if err != nil {
			return
		}
		_, _ = f.Write(append(bytes, '\n'))
	})
}
