package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ketsuna-org/sovrabase/internal/integrations"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
)

// ─── Public Integration API ──────────────────────────────────────────────────

// handleGetIntegrations returns the public (non-secret) configuration for all
// enabled integrations on the current project. Apps use this to initialize
// client-side SDKs (e.g. PayPal SDK, Stripe.js, Google Analytics).
//
// GET /api/v1/integrations
// Auth: Bearer token + X-Project-Key
func (s *Server) handleGetIntegrations(w http.ResponseWriter, r *http.Request) {
	projectID, _ := r.Context().Value(projectIDKey).(string)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "missing project context")
		return
	}

	proj, err := s.projects.GetProject(projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	type publicIntegration struct {
		ID     string                 `json:"id"`
		Name   string                 `json:"name"`
		Config map[string]interface{} `json:"config"`
	}

	result := make([]publicIntegration, 0, len(proj.Integrations))
	for _, integ := range proj.Integrations {
		def := integrations.GetByID(integ.ID)
		if def == nil {
			continue
		}

		// Build public config: include everything EXCEPT password-type fields
		publicCfg := make(map[string]interface{})
		secretKeys := make(map[string]bool)
		for _, cf := range def.ConfigFields {
			if cf.Type == "password" {
				secretKeys[cf.Key] = true
			}
		}
		for k, v := range integ.Config {
			if !secretKeys[k] {
				publicCfg[k] = v
			}
		}

		result = append(result, publicIntegration{
			ID:     integ.ID,
			Name:   def.Name,
			Config: publicCfg,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"integrations": result,
	})
}

// ─── Webhook Receivers ───────────────────────────────────────────────────────

// handleIntegrationWebhook receives webhook events from third-party providers
// (PayPal, Stripe, etc.) and stores them as documents in the project's database.
// The webhook URL format is:
//
//	POST /api/v1/integrations/{provider}/webhook?project_key=XXX
//
// No auth header required (providers can't send our JWT), only project_key.
// Events are stored in the `_webhooks` collection with provider metadata.
func (s *Server) handleIntegrationWebhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	projectID, _ := r.Context().Value(projectIDKey).(string)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "missing project context")
		return
	}

	proj, err := s.projects.GetProject(projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Find the integration config
	var integConfig map[string]interface{}
	for _, integ := range proj.Integrations {
		if integ.ID == provider {
			integConfig = integ.Config
			break
		}
	}
	if integConfig == nil {
		writeError(w, http.StatusNotFound, "integration not enabled")
		return
	}

	// Read raw body for signature verification
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	// Verify webhook signature based on provider
	switch provider {
	case "stripe":
		if !s.verifyStripeWebhook(integConfig, r.Header.Get("Stripe-Signature"), rawBody) {
			s.logger.Warn("stripe webhook signature verification failed", "project", projectID)
			writeError(w, http.StatusUnauthorized, "stripe signature verification failed")
			return
		}
	case "paypal":
		if !s.verifyPayPalWebhook(integConfig, rawBody, r.Header) {
			s.logger.Warn("paypal webhook verification failed", "project", projectID)
			writeError(w, http.StatusUnauthorized, "paypal verification failed")
			return
		}
	}

	// Parse the verified body
	body := make(map[string]interface{})
	if err := json.NewDecoder(bytes.NewReader(rawBody)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Store the webhook event in the project's database
	env, err := s.projects.GetProjectEnv(projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project env")
		return
	}

	doc := map[string]interface{}{
		"provider":    provider,
		"event_type":  body["type"],
		"event_id":    body["id"],
		"payload":     body,
		"created_at":  nil, // engine sets _createdAt
		"raw_headers": map[string]interface{}{
			"user_agent":   r.Header.Get("User-Agent"),
			"content_type": r.Header.Get("Content-Type"),
		},
	}

	// Use _webhooks collection (auto-created on first insert)
	if err := env.Engine.Insert("_webhooks", "", doc); err != nil {
		s.logger.Error("failed to store webhook event", "provider", provider, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to store event")
		return
	}

	// Publish realtime event if hub is available
	if s.realtimeHub != nil {
		s.realtimeHub.Publish(&realtime.Event{
			Type:       realtime.EventInsert,
			Collection: "_webhooks",
			ProjectID:  projectID,
			Data:       doc,
			Timestamp:  time.Now().UTC(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "received",
		"provider": provider,
	})
}

// verifyStripeWebhook verifies the Stripe-Signature header using HMAC-SHA256.
// Format: "t=TIMESTAMP,v1=HEX_SIGNATURE"
func (s *Server) verifyStripeWebhook(cfg map[string]interface{}, sigHeader string, body []byte) bool {
	secret := getString(cfg, "webhook_secret")
	if secret == "" {
		// If no secret configured, skip verification (backward compat)
		s.logger.Warn("stripe webhook_secret not configured — skipping verification")
		return true
	}
	if sigHeader == "" {
		return false
	}

	var timestamp, sigFromHeader string
	for _, part := range strings.Split(sigHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "t=") {
			timestamp = part[2:]
		} else if strings.HasPrefix(part, "v1=") {
			sigFromHeader = part[3:]
		}
	}
	if timestamp == "" || sigFromHeader == "" {
		return false
	}

	// Compute expected signature: HMAC-SHA256(timestamp + "." + body)
	signedPayload := timestamp + "." + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(sigFromHeader))
}

// verifyPayPalWebhook verifies a PayPal webhook by calling PayPal's verify-webhook-signature API.
func (s *Server) verifyPayPalWebhook(cfg map[string]interface{}, body []byte, headers http.Header) bool {
	webhookID := getString(cfg, "webhook_id")
	if webhookID == "" {
		// If no webhook_id configured, skip verification (backward compat)
		s.logger.Warn("paypal webhook_id not configured — skipping verification")
		return true
	}

	clientID := getString(cfg, "client_id")
	clientSecret := getString(cfg, "client_secret")
	if clientID == "" || clientSecret == "" {
		return false
	}

	sandbox := cfg["sandbox"] == true
	baseURL := "https://api-m.paypal.com"
	if sandbox {
		baseURL = "https://api-m.sandbox.paypal.com"
	}

	// Get access token
	token, err := getPayPalAccessToken(baseURL, clientID, clientSecret)
	if err != nil {
		return false
	}

	// Build verification request
	verifyReq := map[string]interface{}{
		"auth_algo":         headers.Get("PAYPAL-AUTH-ALGO"),
		"cert_url":          headers.Get("PAYPAL-CERT-URL"),
		"transmission_id":   headers.Get("PAYPAL-TRANSMISSION-ID"),
		"transmission_sig":  headers.Get("PAYPAL-TRANSMISSION-SIG"),
		"transmission_time": headers.Get("PAYPAL-TRANSMISSION-TIME"),
		"webhook_id":        webhookID,
		"webhook_event":     json.RawMessage(body),
	}
	reqBody, _ := json.Marshal(verifyReq)

	req, _ := http.NewRequest("POST", baseURL+"/v1/notifications/verify-webhook-signature", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		VerificationStatus string `json:"verification_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return strings.EqualFold(result.VerificationStatus, "SUCCESS")
}

// ─── Server-side Integration Actions ─────────────────────────────────────────

// handleIntegrationAction executes a server-side action for an integration.
// This allows apps to perform operations that require the secret API key
// without exposing it to the client.
//
// POST /api/v1/integrations/{provider}/action
// Body: { "action": "create_order", "data": { ... } }
//
// Supported actions vary by provider. See the switch statement below.
func (s *Server) handleIntegrationAction(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")

	projectID, _ := r.Context().Value(projectIDKey).(string)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "missing project context")
		return
	}

	proj, err := s.projects.GetProject(projectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Find the integration config
	var config map[string]interface{}
	for _, integ := range proj.Integrations {
		if integ.ID == provider {
			config = integ.Config
			break
		}
	}
	if config == nil {
		writeError(w, http.StatusNotFound, "integration not enabled")
		return
	}

	var req struct {
		Action string                 `json:"action"`
		Data   map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Route to provider-specific handler
	switch provider {
	case "paypal":
		s.handlePayPalAction(w, r, config, req.Action, req.Data)
	case "stripe":
		s.handleStripeAction(w, r, config, req.Action, req.Data)
	case "sendgrid":
		s.handleSendGridAction(w, r, config, req.Action, req.Data)
	case "discord_webhook":
		s.handleDiscordWebhookAction(w, r, config, req.Action, req.Data)
	case "slack_webhook":
		s.handleSlackWebhookAction(w, r, config, req.Action, req.Data)
	case "twilio":
		s.handleTwilioAction(w, r, config, req.Action, req.Data)
	case "onesignal":
		s.handleOneSignalAction(w, r, config, req.Action, req.Data)
	case "algolia":
		s.handleAlgoliaAction(w, r, config, req.Action, req.Data)
	default:
		writeError(w, http.StatusBadRequest, "no server-side actions available for integration: "+provider)
	}
}

// getString is a helper to extract a string from a map[string]interface{}
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
