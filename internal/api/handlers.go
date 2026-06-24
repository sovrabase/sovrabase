package api

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

	user, tokens, err := s.auth.SignUp(req.Email, req.Password)
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

	tokens, err := s.auth.SignIn(req.Email, req.Password)
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

	tokens, err := s.auth.RefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	state, err := s.auth.CreateOAuthState(provider)
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

	user, tokens, err := s.auth.HandleOAuthCallback(provider, req.Code, req.State)
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

	user, err := s.auth.GetUser(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ─── Database Handlers ───────────────────────────────────────────────────────

func (s *Server) handleInsert(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var doc map[string]interface{}
	if err := decodeJSON(r, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid document")
		return
	}

	id := uuid.New().String()
	if err := s.db.Insert(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return the created document
	created, _ := s.db.Get(collection, id)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	doc, err := s.db.Get(collection, id)
	if err != nil {
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

	if err := s.db.Update(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := s.db.Get(collection, id)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	if err := s.db.Delete(collection, id); err != nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	docs, err := s.db.List(collection)
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
	Filter map[string]interface{} `json:"filter"`
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")

	var req queryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid query")
		return
	}

	docs, err := s.db.Query(collection, req.Filter)
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

	info, err := s.store.Upload(bucket, path, file, contentType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, info)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	path := chi.URLParam(r, "path")

	reader, info, err := s.store.Download(bucket, path)
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

	if err := s.store.Delete(bucket, path); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleStorageList(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	prefix := r.URL.Query().Get("prefix")

	files, err := s.store.List(bucket, prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if files == nil {
		files = []FileInfo{}
	}

	writeJSON(w, http.StatusOK, files)
}
