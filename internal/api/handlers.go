package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/db"
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
	state, err := s.getAuth(r).CreateOAuthState(provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"provider": provider,
		"state":    state,
	})
}

type oauthCallbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var req oauthCallbackRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, tokens, err := s.getAuth(r).HandleOAuthCallback(provider, req.Code, req.State)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": tokens,
	})
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

func (s *Server) handleInsert(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document")
		return
	}

	id := uuid.New().String()
	doc["_id"] = id // Set the ID beforehand for rule evaluation

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
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

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

	filter := make(map[string]interface{})
	for key, values := range q {
		if key == "select" {
			continue
		}
		if len(values) > 0 {
			filter[key] = values[0]
		}
	}

	var docs []map[string]interface{}
	var err error
	if len(filter) > 0 || len(projection) > 0 {
		docs, err = s.getDB(r).Query(collection, filter, projection)
	} else {
		docs, err = s.getDB(r).List(collection)
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

	writeJSON(w, http.StatusOK, docs)
}

type queryRequest struct {
	Filter     map[string]interface{} `json:"filter"`
	Select     []string               `json:"select"`
	Projection []string               `json:"projection"`
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

	docs, err := s.getDB(r).Query(collection, req.Filter, proj)
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

	writeJSON(w, http.StatusOK, docs)
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
