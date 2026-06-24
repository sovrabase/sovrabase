package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
)

// ─── Auth Handlers ───────────────────────────────────────────────────────────

type signUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleSignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, tokens, err := s.getAuth(r).SignUp(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user":  user,
		"token": tokens,
	})
}

type signInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req signInRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	tokens, err := s.getAuth(r).SignIn(req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tokens, err := s.getAuth(r).RefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	authURL, state, err := s.getAuth(r).CreateOAuthStateURL(provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// If the client requests a redirect (browser navigation), do it directly.
	if r.URL.Query().Get("redirect") == "true" {
		http.Redirect(w, r, authURL, http.StatusFound)
		return
	}

	// Otherwise return the URL so the frontend can redirect programmatically.
	// Also expose the project_key so the frontend can embed it in the redirect URL.
	projectKey := r.URL.Query().Get("project_key")
	if projectKey == "" {
		projectKey = r.Header.Get("X-Project-Key")
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"provider":    provider,
		"state":       state,
		"url":         authURL,
		"project_key": projectKey,
	})
}


// handleOAuthCallback is called by the OAuth provider after the user authenticates.
// It reads ?code and ?state from query params (standard browser redirect),
// exchanges the code for tokens, and redirects the user back to the app.
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	user, tokens, err := s.getAuth(r).HandleOAuthCallback(provider, code, state)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// If there's an app redirect URL configured, send the user back to the app
	// with the tokens in the URL fragment (never in query string — avoids server logs).
	appRedirect := r.URL.Query().Get("app_redirect")
	if appRedirect == "" {
		appRedirect = "/"
	}

	_ = user // user is embedded inside tokens
	fragment := fmt.Sprintf("access_token=%s&refresh_token=%s&provider=%s",
		tokens.AccessToken, tokens.RefreshToken, provider)
	http.Redirect(w, r, appRedirect+"#"+fragment, http.StatusFound)
}


type verifyEmailRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	err := s.getAuth(r).VerifyEmail(req.Token)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "email verified successfully"})
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	token, err := s.getAuth(r).ForgotPassword(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password reset email sent",
		"token":   token,
	})
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "token and password are required")
		return
	}

	err := s.getAuth(r).ResetPassword(req.Token, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password reset successfully"})
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := s.getAuth(r).GetUser(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ─── Database Handlers ───────────────────────────────────────────────────────

func (s *Server) checkRLS(r *http.Request, collection string, action string, docID string, newDoc map[string]interface{}) (bool, error) {
	engine := s.getDB(r)

	rulesCfg, err := engine.GetRules(collection)
	if err != nil {
		return true, nil // Allow access if rules cannot be read
	}

	if !rulesCfg.Enabled {
		return true, nil
	}

	ruleExpr, ok := rulesCfg.Rules[action]
	if !ok || ruleExpr == "" {
		return false, nil // Default deny if rule is not specified but RLS is enabled
	}

	var authEnv map[string]interface{}
	claims := getClaims(r)
	if claims != nil {
		authEnv = map[string]interface{}{
			"uid":   claims.UserID,
			"email": claims.Email,
			"role":  claims.Role,
		}
	}

	env := map[string]interface{}{
		"auth": authEnv,
		"id":   docID,
	}

	if action == "create" || action == "update" {
		env["data"] = newDoc
	} else if action == "get" && docID != "" {
		existing, err := engine.Get(collection, docID)
		if err == nil && existing != nil {
			env["data"] = existing
		}
	} else if action == "delete" {
		existing, err := engine.Get(collection, docID)
		if err == nil && existing != nil {
			env["data"] = existing
		}
	}

	return db.EvaluateRule(ruleExpr, env)
}

func (s *Server) filterDocs(r *http.Request, collection string, docs []map[string]interface{}) ([]map[string]interface{}, error) {
	engine := s.getDB(r)

	rulesCfg, err := engine.GetRules(collection)
	if err != nil || !rulesCfg.Enabled {
		return docs, nil
	}

	ruleExpr, ok := rulesCfg.Rules["list"]
	if !ok || ruleExpr == "" {
		return []map[string]interface{}{}, nil
	}

	var authEnv map[string]interface{}
	claims := getClaims(r)
	if claims != nil {
		authEnv = map[string]interface{}{
			"uid":   claims.UserID,
			"email": claims.Email,
			"role":  claims.Role,
		}
	}

	var filtered []map[string]interface{}
	for _, doc := range docs {
		docID, _ := doc["_id"].(string)
		env := map[string]interface{}{
			"auth": authEnv,
			"id":   docID,
			"data": doc,
		}
		allowed, err := db.EvaluateRule(ruleExpr, env)
		if err == nil && allowed {
			filtered = append(filtered, doc)
		}
	}
	return filtered, nil
}

// publishRealtime sends a realtime event for a data mutation.
func (s *Server) publishRealtime(eventType realtime.EventType, projectID, collection, docID string, data map[string]interface{}) {
	if s.realtimeHub == nil {
		return
	}
	s.realtimeHub.Publish(&realtime.Event{
		Type:       eventType,
		Collection: collection,
		DocID:      docID,
		Data:       data,
		ProjectID:  projectID,
		Timestamp:  time.Now().UTC(),
	})
}

// getProjectIDFromRequest extracts the project ID from the request context.
func (s *Server) getProjectIDFromRequest(r *http.Request) string {
	if env := getProjectEnv(r); env != nil {
		// Walk through the projects to find the matching env.
		// For now, use a simple empty string — the realtime filter will use
		// the project key from the environment.
		return ""
	}
	return ""
}

func (s *Server) handleInsert(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document")
		return
	}

	id := uuid.New().String()
	doc["_id"] = id

	allowed, err := s.checkRLS(r, collection, "create", id, doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "RLS policy restricts insertion")
		return
	}

	if err := s.getDB(r).Insert(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	created, _ := s.getDB(r).Get(collection, id)

	// Publish realtime event.
	if s.realtimeHub != nil {
		projectID := getProjectID(r)
		s.publishRealtime(realtime.EventInsert, projectID, collection, id, created)
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	allowed, err := s.checkRLS(r, collection, "get", id, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "RLS policy restricts access")
		return
	}

	doc, err := s.getDB(r).Get(collection, id)
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document")
		return
	}

	allowed, err := s.checkRLS(r, collection, "update", id, doc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "RLS policy restricts update")
		return
	}

	if err := s.getDB(r).Update(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := s.getDB(r).Get(collection, id)

	// Publish realtime event.
	if s.realtimeHub != nil {
		projectID := getProjectID(r)
		s.publishRealtime(realtime.EventUpdate, projectID, collection, id, updated)
	}

	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	// Fetch existing doc before delete (for RLS + realtime event).
	existing, _ := s.getDB(r).Get(collection, id)

	allowed, err := s.checkRLS(r, collection, "delete", id, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "RLS policy restricts deletion")
		return
	}

	if err := s.getDB(r).Delete(collection, id); err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	// Publish realtime event.
	if s.realtimeHub != nil && existing != nil {
		projectID := getProjectID(r)
		s.publishRealtime(realtime.EventDelete, projectID, collection, id, existing)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	q := r.URL.Query()
	var projection []string
	if selectStr := q.Get("select"); selectStr != "" {
		for _, part := range strings.Split(selectStr, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				projection = append(projection, part)
			}
		}
	}

	// Parse pagination params.
	limit := parseIntParam(q.Get("limit"), 50)
	offset := parseIntParam(q.Get("offset"), 0)
	hasPagination := q.Has("limit") || q.Has("offset")

	filter := make(map[string]interface{})
	for key, values := range q {
		if key == "select" || key == "limit" || key == "offset" {
			continue
		}
		if len(values) > 0 {
			filter[key] = values[0]
		}
	}

	var docs []map[string]interface{}
	var err error

	if hasPagination || limit > 0 || offset > 0 {
		if len(filter) > 0 || len(projection) > 0 {
			docs, err = s.getDB(r).QueryPaged(collection, filter, projection, limit, offset)
		} else {
			docs, err = s.getDB(r).ListPaged(collection, limit, offset)
		}
	} else {
		if len(filter) > 0 || len(projection) > 0 {
			docs, err = s.getDB(r).Query(collection, filter, projection)
		} else {
			docs, err = s.getDB(r).List(collection)
		}
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	docs, err = s.filterDocs(r, collection, docs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if docs == nil {
		docs = []map[string]interface{}{}
	}

	// Return paginated response when pagination params are present.
	if hasPagination {
		total, _ := s.getDB(r).Count(collection)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data":   docs,
			"limit":  limit,
			"offset": offset,
			"total":  total,
		})
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

type queryRequest struct {
	Filter     map[string]interface{} `json:"filter"`
	Select     []string               `json:"select"`
	Projection []string               `json:"projection"`
	Limit      int                    `json:"limit"`
	Offset     int                    `json:"offset"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var req queryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid query")
		return
	}

	proj := req.Select
	if len(proj) == 0 {
		proj = req.Projection
	}

	hasPagination := req.Limit > 0 || req.Offset > 0

	var docs []map[string]interface{}
	var err error

	if hasPagination {
		docs, err = s.getDB(r).QueryPaged(collection, req.Filter, proj, req.Limit, req.Offset)
	} else {
		docs, err = s.getDB(r).Query(collection, req.Filter, proj)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	docs, err = s.filterDocs(r, collection, docs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if docs == nil {
		docs = []map[string]interface{}{}
	}

	if hasPagination {
		total, _ := s.getDB(r).Count(collection)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data":   docs,
			"limit":  req.Limit,
			"offset": req.Offset,
			"total":  total,
		})
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

// parseIntParam parses an integer from a string, returning defaultVal on failure.
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	// Cap at 1000 for safety.
	if v > 1000 {
		v = 1000
	}
	return v
}

// ─── Storage Handlers ────────────────────────────────────────────────────────

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")

	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return
	}
	defer file.Close()

	// Enforce project storage quota
	projectID := getProjectID(r)
	if projectID != "" {
		proj, err := s.projects.GetProject(projectID)
		if err == nil && proj != nil && proj.StorageQuota > 0 {
			var currentUsage int64
			if _, statErr := os.Stat(proj.StorageDir); statErr == nil {
				_ = filepath.Walk(proj.StorageDir, func(path string, info os.FileInfo, walkErr error) error {
					if walkErr != nil {
						return nil
					}
					if !info.IsDir() {
						currentUsage += info.Size()
					}
					return nil
				})
			}
			if currentUsage+header.Size > proj.StorageQuota {
				writeError(w, http.StatusForbidden, fmt.Sprintf("storage quota exceeded (used %d/%d bytes, attempting to upload %d bytes)", currentUsage, proj.StorageQuota, header.Size))
				return
			}
		}
	}

	path := r.FormValue("path")
	if path == "" {
		path = header.Filename
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	info, err := s.getStorage(r).Upload(bucket, path, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	path := chi.URLParam(r, "path")

	reader, info, err := s.getStorage(r).Download(bucket, path)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+info.Path+"\"")
	io.Copy(w, reader)
}

func (s *Server) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	path := chi.URLParam(r, "path")

	if err := s.getStorage(r).Delete(bucket, path); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleStorageList(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	files, err := s.getStorage(r).List(bucket, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []FileInfo{}
	}

	writeJSON(w, http.StatusOK, files)
}

// ─── Batch Handler ─────────────────────────────────────────────────────────────

type batchOperation struct {
	Op   string                 `json:"op"`   // "insert", "update", "delete"
	ID   string                 `json:"id"`   // document ID (optional for insert)
	Data map[string]interface{} `json:"data"` // document data
}

type batchRequest struct {
	Operations []batchOperation `json:"operations"`
}

type batchResult struct {
	Index   int         `json:"index"`
	Op      string      `json:"op"`
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var req batchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	engine := s.getDB(r)
	projectID := getProjectID(r)
	results := make([]batchResult, 0, len(req.Operations))

	for i, op := range req.Operations {
		result := batchResult{
			Index:   i,
			Op:      op.Op,
			ID:      op.ID,
			Success: false,
		}

		switch op.Op {
		case "insert":
			id := op.ID
			if id == "" {
				id = uuid.New().String()
			}
			op.Data["_id"] = id
			result.ID = id

			allowed, err := s.checkRLS(r, collection, "create", id, op.Data)
			if err != nil || !allowed {
				result.Error = "RLS policy restricts insertion"
				results = append(results, result)
				continue
			}

			if err := engine.Insert(collection, id, op.Data); err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}

			created, _ := engine.Get(collection, id)
			result.Success = true
			result.Data = created

			// Realtime event
			if s.realtimeHub != nil {
				s.publishRealtime(realtime.EventInsert, projectID, collection, id, created)
			}

			// Webhook trigger
			s.fireWebhooks(r, collection, "insert", id, created)

		case "update":
			if op.ID == "" {
				result.Error = "id is required for update"
				results = append(results, result)
				continue
			}

			allowed, err := s.checkRLS(r, collection, "update", op.ID, op.Data)
			if err != nil || !allowed {
				result.Error = "RLS policy restricts update"
				results = append(results, result)
				continue
			}

			if err := engine.Update(collection, op.ID, op.Data); err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}

			updated, _ := engine.Get(collection, op.ID)
			result.Success = true
			result.Data = updated

			// Realtime event
			if s.realtimeHub != nil {
				s.publishRealtime(realtime.EventUpdate, projectID, collection, op.ID, updated)
			}

			// Webhook trigger
			s.fireWebhooks(r, collection, "update", op.ID, updated)

		case "delete":
			if op.ID == "" {
				result.Error = "id is required for delete"
				results = append(results, result)
				continue
			}

			existing, _ := engine.Get(collection, op.ID)

			allowed, err := s.checkRLS(r, collection, "delete", op.ID, nil)
			if err != nil || !allowed {
				result.Error = "RLS policy restricts deletion"
				results = append(results, result)
				continue
			}

			if err := engine.Delete(collection, op.ID); err != nil {
				result.Error = err.Error()
				results = append(results, result)
				continue
			}

			result.Success = true
			result.Data = map[string]string{"status": "deleted"}

			// Realtime event
			if s.realtimeHub != nil && existing != nil {
				s.publishRealtime(realtime.EventDelete, projectID, collection, op.ID, existing)
			}

			// Webhook trigger
			s.fireWebhooks(r, collection, "delete", op.ID, existing)

		default:
			result.Error = "unknown operation: " + op.Op
			results = append(results, result)
			continue
		}

		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

// ─── Search Handler ────────────────────────────────────────────────────────────

type searchRequest struct {
	Query  string   `json:"query"`
	Fields []string `json:"fields"`
	Limit  int      `json:"limit"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var req searchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	docs, err := s.getDB(r).Search(collection, req.Query, req.Fields, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Apply list RLS filtering
	docs, err = s.filterDocs(r, collection, docs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if docs == nil {
		docs = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  docs,
		"count": len(docs),
	})
}

// ─── Webhook Triggers ─────────────────────────────────────────────────────────

func (s *Server) fireWebhooks(r *http.Request, collection, eventType, docID string, data map[string]interface{}) {
	go func() {
		engine := s.getDB(r)
		webhookDocs, err := engine.List("_webhooks")
		if err != nil || len(webhookDocs) == 0 {
			return
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"event":      eventType,
			"collection": collection,
			"doc_id":     docID,
			"data":       data,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		})

		for _, wh := range webhookDocs {
			url, ok := wh["url"].(string)
			if !ok || url == "" {
				continue
			}

			// Fire in a sub-goroutine per webhook
			go func(webhookURL string) {
				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(payload))
				if err != nil {
					// Silently ignore webhook errors
					return
				}
				resp.Body.Close()
			}(url)
		}
	}()
}
