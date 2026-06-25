package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/imgtransform"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
)

// ─── Auth Handlers ───────────────────────────────────────────────────────────

type signUpRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token"`
}

// @Summary Sign up a new user
// @Description Create a new user account with email and password. Returns the created user and authentication tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body signUpRequest true "Sign up request"
// @Success 201 {object} map[string]interface{} "User created"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 409 {object} map[string]string "User already exists"
// @Failure 403 {object} map[string]string "Captcha verification failed"
// @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/signup [post]
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

	// Verify captcha if enabled.
	if s.captchaVerifier != nil && s.captchaVerifier.IsEnabled() {
		ok, err := s.captchaVerifier.Verify(r.Context(), req.CaptchaToken)
		if err != nil || !ok {
			writeError(w, http.StatusForbidden, "captcha verification failed: "+err.Error())
			return
		}
	}

	user, tokens, err := s.getAuth(r).SignUp(req.Email, req.Password)
	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			_ = s.meterStore.Inc(projectID, metering.MetricDBWrites, 1)
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
		}
	}
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
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captcha_token"`
}

// @Summary Sign in an existing user
// @Description Authenticate a user with email and password. Returns access and refresh tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body signInRequest true "Sign in request"
// @Success 200 {object} TokenPair "Authentication tokens"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Invalid credentials"
// @Failure 403 {object} map[string]string "Captcha verification failed"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/signin [post]
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

	// Verify captcha if enabled.
	if s.captchaVerifier != nil && s.captchaVerifier.IsEnabled() {
		ok, err := s.captchaVerifier.Verify(r.Context(), req.CaptchaToken)
		if err != nil || !ok {
			writeError(w, http.StatusForbidden, "captcha verification failed: "+err.Error())
			return
		}
	}

	tokens, err := s.getAuth(r).SignIn(req.Email, req.Password)
	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
		}
	}
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// @Summary Refresh authentication tokens
// @Description Exchange a refresh token for a new access and refresh token pair.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body refreshRequest true "Refresh token request"
// @Success 200 {object} TokenPair "New token pair"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Invalid refresh token"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/refresh [post]
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

// @Summary Initiate OAuth redirect
// @Description Get the OAuth authorization URL for the specified provider. Optionally redirects the browser if redirect=true.
// @Tags auth
// @Accept json
// @Produce json
// @Param provider path string true "OAuth provider name"
// @Param final_redirect query string false "URL to redirect after authentication"
// @Param redirect query string false "Set to 'true' for HTTP redirect instead of JSON response"
// @Param project_key query string false "Project key for frontend embedding"
// @Success 200 {object} map[string]string "OAuth redirect info"
// @Success 302 "Browser redirect to OAuth provider"
// @Failure 400 {object} map[string]string "Invalid provider"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/oauth/{provider} [get]
func (s *Server) handleOAuthRedirect(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	projectID := getProjectID(r)
	appRedirect := r.URL.Query().Get("final_redirect")
	authURL, state, err := s.getAuth(r).CreateOAuthStateURL(provider, projectID, appRedirect)
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
// It reads ?code and ?state from query params (standard browser redirect).
// The project ID and redirect preferences are decoded from the state token itself.
// @Summary OAuth callback handler
// @Description Called by the OAuth provider after user authentication. Reads code and state from query params, exchanges for tokens, and redirects the browser with tokens in the URL fragment.
// @Tags auth
// @Accept json
// @Produce json
// @Param provider path string true "OAuth provider name"
// @Param code query string true "Authorization code from OAuth provider"
// @Param state query string true "State token from OAuth redirect"
// @Success 302 "Redirect with tokens in fragment"
// @Failure 400 {object} map[string]string "Missing code or state"
// @Failure 401 {object} map[string]string "Authentication failed"
// @Router /auth/v1/oauth/{provider}/callback [get]
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	// Decode the full payload from the state token (project ID, app redirect, etc.).
	payload, err := s.auth.DecodeStatePayload(state)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid state: "+err.Error())
		return
	}

	// Load the project environment and inject it into the request context.
	proj, err := s.projects.GetProject(payload.ProjectID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "project not found")
		return
	}
	env, err := s.projects.GetProjectEnv(proj.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load project environment")
		return
	}
	ctx := context.WithValue(r.Context(), projectEnvKey, env)
	ctx = context.WithValue(ctx, projectIDKey, proj.ID)
	r = r.WithContext(ctx)

	user, tokens, err := s.getAuth(r).HandleOAuthCallback(provider, code, state)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Redirect to the app's post-login URL encoded in the state (defaults to /).
	appRedirect := payload.AppRedirect
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

// @Summary Verify email address
// @Description Verify a user's email address using a verification token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body verifyEmailRequest true "Email verification request"
// @Success 200 {object} map[string]string "Verification successful"
// @Failure 400 {object} map[string]string "Invalid or expired token"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/verify-email [post]
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

// @Summary Request password reset
// @Description Send a password reset email to the specified address. Returns a reset token.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body forgotPasswordRequest true "Forgot password request"
// @Success 200 {object} map[string]string "Password reset email sent"
// @Failure 400 {object} map[string]string "Invalid request"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/forgot-password [post]
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

// @Summary Reset password
// @Description Reset a user's password using a reset token and new password.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body resetPasswordRequest true "Reset password request"
// @Success 200 {object} map[string]string "Password reset successfully"
// @Failure 400 {object} map[string]string "Invalid token or password"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/reset-password [post]
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

// ─── Magic Link Handlers ─────────────────────────────────────────────────────

type magicLinkRequest struct {
	Email string `json:"email"`
}

// @Summary Create magic link
// @Description Generate a magic link for passwordless authentication. Returns a token for the magic link.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body magicLinkRequest true "Magic link request"
// @Success 200 {object} map[string]string "Magic link generated"
// @Failure 400 {object} map[string]string "Invalid request"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/magic-link [post]
func (s *Server) handleCreateMagicLink(w http.ResponseWriter, r *http.Request) {
	var req magicLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	token, err := s.getAuth(r).CreateMagicLink(req.Email)
	if err != nil {
		// Don't leak whether email exists for security.
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "if the email exists, a magic link has been sent",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "magic link generated",
		"token":   token,
	})
}

type verifyMagicLinkRequest struct {
	Email string `json:"email"`
	Token string `json:"token"`
}

// @Summary Verify magic link
// @Description Verify a magic link token and return authentication tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body verifyMagicLinkRequest true "Verify magic link request"
// @Success 200 {object} TokenPair "Authentication tokens"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 401 {object} map[string]string "Invalid or expired token"
// // @Security ProjectKey
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/verify-magic-link [post]
func (s *Server) handleVerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	var req verifyMagicLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "email and token are required")
		return
	}

	tokens, err := s.getAuth(r).VerifyMagicLink(req.Email, req.Token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

// ─── MFA Handlers ────────────────────────────────────────────────────────────

// @Summary Setup MFA
// @Description Generate an MFA secret and URI for TOTP setup. Requires authentication.
// @Tags auth,mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Success 200 {object} map[string]string "MFA setup data"
// @Failure 400 {object} map[string]string "Setup failed"
// @Failure 401 {object} map[string]string "Authentication required"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/mfa/setup [post]
func (s *Server) handleMFASetup(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	secret, uri, err := s.getAuth(r).SetupMFA(claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"secret": secret,
		"uri":    uri,
	})
}

// @Summary Confirm MFA setup
// @Description Confirm MFA setup by providing a valid TOTP code. Returns backup codes. Requires authentication.
// @Tags auth,mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param request body map[string]string true "MFA confirmation with code"
// @Success 200 {object} map[string]interface{} "MFA enabled with backup codes"
// @Failure 400 {object} map[string]string "Invalid code"
// @Failure 401 {object} map[string]string "Authentication required"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/mfa/confirm [post]
func (s *Server) handleMFAConfirm(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	backupCodes, err := s.getAuth(r).ConfirmMFA(claims.UserID, req.Code)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "MFA enabled successfully",
		"backup_codes": backupCodes,
	})
}

// @Summary Disable MFA
// @Description Disable MFA for the authenticated user. Requires a valid TOTP code. Requires authentication.
// @Tags auth,mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param request body map[string]string true "MFA disable with code"
// @Success 200 {object} map[string]string "MFA disabled"
// @Failure 400 {object} map[string]string "Invalid code"
// @Failure 401 {object} map[string]string "Authentication required"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/mfa/disable [post]
func (s *Server) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.getAuth(r).DisableMFA(claims.UserID, req.Code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "MFA disabled"})
}

// @Summary Get MFA status
// @Description Check whether MFA is enabled for the authenticated user.
// @Tags auth,mfa
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Success 200 {object} map[string]bool "MFA status"
// @Failure 401 {object} map[string]string "Authentication required"
// @Failure 404 {object} map[string]string "User not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /auth/v1/mfa/status [get]
func (s *Server) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	enabled, err := s.getAuth(r).GetMFAStatus(claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}

// @Summary Get current user
// @Description Get the profile of the currently authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Success 200 {object} UserInfo "User profile"
// @Failure 401 {object} map[string]string "Not authenticated"
// @Failure 404 {object} map[string]string "User not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/me [get]
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

// @Summary Insert a document
// @Description Insert a new document into the specified collection. An _id is auto-generated if not provided.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param document body map[string]interface{} true "Document to insert"
// @Success 201 {object} map[string]interface{} "Created document"
// @Failure 400 {object} map[string]string "Invalid document"
// @Failure 403 {object} map[string]string "RLS policy restricts insertion"
// @Failure 500 {object} map[string]string "Internal error"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection} [post]
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

	projectID := getProjectID(r)
	if err := s.getDB(r).Insert(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBWrites, 1)
	}

	created, _ := s.getDB(r).Get(collection, id)
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
	}

	// Publish realtime event.
	if s.realtimeHub != nil {
		s.publishRealtime(realtime.EventInsert, projectID, collection, id, created)
	}

	// Webhook trigger
	s.fireWebhooks(r, collection, "insert", id, created)

	writeJSON(w, http.StatusCreated, created)
}

// @Summary Get a document by ID
// @Description Retrieve a single document by its ID from the specified collection.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]interface{} "Retrieved document"
// @Failure 403 {object} map[string]string "RLS policy restricts access"
// @Failure 404 {object} map[string]string "Document not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/{id} [get]
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
	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
		}
	}
	if err != nil || doc == nil {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// @Summary Update a document
// @Description Partially update an existing document by ID. Supports both PUT and PATCH methods.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param id path string true "Document ID"
// @Param document body map[string]interface{} true "Partial document update"
// @Success 200 {object} map[string]interface{} "Updated document"
// @Failure 400 {object} map[string]string "Invalid document"
// @Failure 403 {object} map[string]string "RLS policy restricts update"
// @Failure 404 {object} map[string]string "Document not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/{id} [put]
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

	projectID := getProjectID(r)
	if err := s.getDB(r).Update(collection, id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBWrites, 1)
	}

	updated, _ := s.getDB(r).Get(collection, id)
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
	}

	// Publish realtime event.
	if s.realtimeHub != nil {
		s.publishRealtime(realtime.EventUpdate, projectID, collection, id, updated)
	}

	// Webhook trigger
	s.fireWebhooks(r, collection, "update", id, updated)

	writeJSON(w, http.StatusOK, updated)
}

// @Summary Delete a document
// @Description Delete a document by ID from the specified collection.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]string "Document deleted"
// @Failure 403 {object} map[string]string "RLS policy restricts deletion"
// @Failure 404 {object} map[string]string "Document not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/{id} [delete]
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	collection := chi.URLParam(r, "collection")
	id := chi.URLParam(r, "id")

	projectID := getProjectID(r)
	// Fetch existing doc before delete (for RLS + realtime event).
	existing, _ := s.getDB(r).Get(collection, id)
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
	}

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
	if s.meterStore != nil && projectID != "" {
		_ = s.meterStore.Inc(projectID, metering.MetricDBWrites, 1)
	}

	// Publish realtime event.
	if s.realtimeHub != nil && existing != nil {
		s.publishRealtime(realtime.EventDelete, projectID, collection, id, existing)
	}

	// Webhook trigger
	s.fireWebhooks(r, collection, "delete", id, existing)

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// @Summary List documents
// @Description List documents in a collection with optional filtering, field selection, and pagination. Query parameters are treated as filter fields.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param select query string false "Comma-separated list of fields to return"
// @Param limit query int false "Maximum number of documents to return (paginated response)"
// @Param offset query int false "Number of documents to skip (paginated response)"
// @Success 200 {object} map[string]interface{} "List of documents or paginated response"
// @Failure 500 {object} map[string]string "Internal error"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection} [get]
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

	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			count := int64(1)
			if hasPagination {
				count = 2
			}
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, count)
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

// @Summary Query documents
// @Description Query documents in a collection with a structured filter, field projection, and pagination.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param request body queryRequest true "Query parameters"
// @Success 200 {object} map[string]interface{} "Query results or paginated response"
// @Failure 400 {object} map[string]string "Invalid query"
// @Failure 500 {object} map[string]string "Internal error"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/query [post]
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

	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			count := int64(1)
			if hasPagination {
				count = 2
			}
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, count)
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

// @Summary Upload a file
// @Description Upload a file to the specified storage bucket. Supports multipart form data with optional path.
// @Tags storage
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param bucket path string true "Bucket name"
// @Param file formData file true "File to upload"
// @Param path formData string false "Custom file path (defaults to filename)"
// @Success 201 {object} FileInfo "Uploaded file info"
// @Failure 400 {object} map[string]string "Invalid form data"
// @Failure 403 {object} map[string]string "Storage quota exceeded"
// @Failure 500 {object} map[string]string "Upload failed"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/storage/{bucket}/upload [post]
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

	// Enforce project storage quota using the meterStore counter (O(1) instead
	// of walking the filesystem on every upload).
	projectID := getProjectID(r)
	if projectID != "" {
		proj, err := s.projects.GetProject(projectID)
		if err == nil && proj != nil && proj.StorageQuota > 0 {
			var currentUsage int64
			if s.meterStore != nil {
				currentUsage, _ = s.meterStore.GetStorageUsage(projectID)
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

	// Track bandwidth and storage in metering
	if s.meterStore != nil && projectID != "" && info != nil {
		_ = s.meterStore.Inc(projectID, metering.MetricBandwidthUp, info.Size)
		_ = s.meterStore.Inc(projectID, metering.MetricStorageBytes, info.Size)
	}

	writeJSON(w, http.StatusCreated, info)
}

// @Summary Download a file
// @Description Download a file from the specified storage bucket. Supports optional image transformation parameters.
// @Tags storage
// @Accept json
// @Produce octet-stream
// @Security BearerAuth
// @Security ProjectKey
// @Param bucket path string true "Bucket name"
// @Param path path string true "File path"
// @Param w query int false "Image width for transformation"
// @Param h query int false "Image height for transformation"
// @Param format query string false "Image output format (e.g., webp, jpeg)"
// @Param fit query string false "Image fit mode (e.g., cover, contain)"
// @Param quality query int false "Image quality (1-100)"
// @Success 200 {file} binary "File content"
// @Failure 404 {object} map[string]string "File not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/storage/{bucket}/{path} [get]
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	path := chi.URLParam(r, "path")

	reader, info, err := s.getStorage(r).Download(bucket, path)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	defer reader.Close()

	// Parse image transformation options from query params.
	q := r.URL.Query()
	imgOpts := imgransform.Options{}
	if wStr := q.Get("w"); wStr != "" {
		imgOpts.Width, _ = strconv.Atoi(wStr)
	}
	if hStr := q.Get("h"); hStr != "" {
		imgOpts.Height, _ = strconv.Atoi(hStr)
	}
	imgOpts.Format = q.Get("format")
	imgOpts.Fit = q.Get("fit")
	if qStr := q.Get("quality"); qStr != "" {
		imgOpts.Quality, _ = strconv.Atoi(qStr)
	}

	if imgransform.IsTransformRequested(imgOpts) {
		// Determine output MIME type.
		outCT := info.ContentType
		if imgOpts.Format != "" {
			outCT = "image/" + imgOpts.Format
		}
		w.Header().Set("Content-Type", outCT)
		w.Header().Set("Content-Disposition", "inline; filename=\""+info.Path+"\"")
		w.Header().Set("X-Image-Transformed", "sovrabase")
		if err := imgransform.Transform(reader, w, info.ContentType, imgOpts); err != nil {
			writeError(w, http.StatusInternalServerError, "image transformation failed: "+err.Error())
		}
		return
	}

	// Pass-through without transformation.
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+info.Path+"\"")
	io.Copy(w, reader)
}

// @Summary Delete a file
// @Description Delete a file from the specified storage bucket.
// @Tags storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param bucket path string true "Bucket name"
// @Param path path string true "File path"
// @Success 200 {object} map[string]string "File deleted"
// @Failure 404 {object} map[string]string "File not found"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/storage/{bucket}/{path} [delete]
func (s *Server) handleStorageDelete(w http.ResponseWriter, r *http.Request) {
	bucket := chi.URLParam(r, "bucket")
	path := chi.URLParam(r, "path")

	if err := s.getStorage(r).Delete(bucket, path); err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// @Summary List files in bucket
// @Description List files in a storage bucket, optionally filtered by prefix.
// @Tags storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param bucket path string true "Bucket name"
// @Param prefix query string false "Path prefix to filter by"
// @Success 200 {array} FileInfo "List of files"
// @Failure 500 {object} map[string]string "Internal error"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/storage/{bucket}/list [get]
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

// @Summary Batch operations
// @Description Execute multiple insert, update, and delete operations in a single atomic batch.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param request body batchRequest true "Batch operations"
// @Success 200 {object} map[string]interface{} "Batch results"
// @Failure 400 {object} map[string]string "Invalid request"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/batch [post]
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

	var dbReads, dbWrites int64

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
			dbWrites++

			created, _ := engine.Get(collection, id)
			dbReads++
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
			dbWrites++

			updated, _ := engine.Get(collection, op.ID)
			dbReads++
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
			dbReads++

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
			dbWrites++

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

	if s.meterStore != nil && projectID != "" {
		if dbReads > 0 {
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, dbReads)
		}
		if dbWrites > 0 {
			_ = s.meterStore.Inc(projectID, metering.MetricDBWrites, dbWrites)
		}
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

// @Summary Full-text search
// @Description Perform a full-text search on documents in a collection.
// @Tags database
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ProjectKey
// @Param collection path string true "Collection name"
// @Param request body searchRequest true "Search parameters"
// @Success 200 {object} map[string]interface{} "Search results"
// @Failure 400 {object} map[string]string "Invalid request or query required"
// @Failure 500 {object} map[string]string "Internal error"
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router /api/v1/collections/{collection}/search [post]
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
	if s.meterStore != nil {
		if projectID := getProjectID(r); projectID != "" {
			_ = s.meterStore.Inc(projectID, metering.MetricDBReads, 1)
		}
	}

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

// webhookCacheTTL is how long the webhook list is cached per project.
const webhookCacheTTL = 30 * time.Second
// webhookCacheEntry holds a cached webhook list with expiry.
type webhookCacheEntry struct {
	webhooks []map[string]interface{}
	expiry   time.Time
}

// Global webhook cache shared between Server and AdminServer.
var (
	webhookGlobalCacheMu sync.RWMutex
	webhookGlobalCache   = make(map[string]webhookCacheEntry)
)

// invalidateWebhookCacheProject clears all cached webhooks for a project.
func invalidateWebhookCacheProject(projectID string) {
	webhookGlobalCacheMu.Lock()
	for k := range webhookGlobalCache {
		if len(k) > len(projectID) && k[:len(projectID)] == projectID {
			delete(webhookGlobalCache, k)
		}
	}
	webhookGlobalCacheMu.Unlock()
}

func (s *Server) fireWebhooks(r *http.Request, collection, eventType, docID string, data map[string]interface{}) {
	go func() {
		engine := s.getDB(r)

		// Check the global cache first.
		projectID := getProjectID(r)
		cacheKey := projectID + ":" + collection

		webhookGlobalCacheMu.RLock()
		entry, ok := webhookGlobalCache[cacheKey]
		webhookGlobalCacheMu.RUnlock()

		if !ok || time.Now().After(entry.expiry) {
			// Cache miss or expired — reload from DB.
			webhookDocs, err := engine.List("_webhooks")
			if err != nil || len(webhookDocs) == 0 {
				// Cache the empty result too, to avoid repeated lookups.
				webhookGlobalCacheMu.Lock()
				webhookGlobalCache[cacheKey] = webhookCacheEntry{
					webhooks: nil,
					expiry:   time.Now().Add(webhookCacheTTL),
				}
				webhookGlobalCacheMu.Unlock()
				return
			}

			webhookGlobalCacheMu.Lock()
			webhookGlobalCache[cacheKey] = webhookCacheEntry{
				webhooks: webhookDocs,
				expiry:   time.Now().Add(webhookCacheTTL),
			}
			webhookGlobalCacheMu.Unlock()
			entry.webhooks = webhookDocs
		}

		if len(entry.webhooks) == 0 {
			return
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"event":      eventType,
			"collection": collection,
			"doc_id":     docID,
			"data":       data,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		})

		for _, wh := range entry.webhooks {
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
