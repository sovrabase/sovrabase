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

// projectMiddleware extracts the project key from the X-Project-Key header
// or the ?project_key= query parameter (needed for browser OAuth redirects
// which cannot send custom headers), resolves it to a ProjectEnv, and injects
// it into the context.
func (s *Server) projectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Header takes priority; fall back to query param for browser redirects.
		projectKey := r.Header.Get("X-Project-Key")
		if projectKey == "" {
			projectKey = r.URL.Query().Get("project_key")
		}
		if projectKey == "" {
			writeError(w, http.StatusBadRequest, "missing X-Project-Key header or project_key query param")
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

// getUserID extracts the authenticated user ID from the request context.
func getUserID(r *http.Request) string {
	if claims := getClaims(r); claims != nil {
		return claims.UserID
	}
	return ""
}

// clientRequestLoggerMiddleware logs all requests (API and Auth) to the project's requests.log file.
func (s *Server) clientRequestLoggerMiddleware(next http.Handler) http.Handler {
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
			"timestamp": time.Now().Format(time.RFC3339Nano),
			"method":    r.Method,
			"path":      r.URL.Path,
			"email":     email,
			"status":    sw.status,
			"duration":  time.Since(start).String(),
			"ip":        r.RemoteAddr,
		}

		if strings.HasPrefix(r.URL.Path, "/api/v1/collections/") {
			rest := strings.TrimPrefix(r.URL.Path, "/api/v1/collections/")
			parts := strings.Split(rest, "/")
			if len(parts) > 0 && parts[0] != "" {
				logEntry["collection"] = parts[0]
			}
			if len(parts) >= 2 && parts[1] != "" {
				logEntry["doc_id"] = parts[1]
			}
		}

		bytes, err := json.Marshal(logEntry)
		if err == nil {
			_, _ = f.Write(append(bytes, '\n'))
		}
	})
}
