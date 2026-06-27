package api

import (
	"encoding/json"
	"net/http"
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

	// Verify this integration is enabled on the project
	enabled := false
	for _, integ := range proj.Integrations {
		if integ.ID == provider {
			enabled = true
			break
		}
	}
	if !enabled {
		writeError(w, http.StatusNotFound, "integration not enabled")
		return
	}

	// Read the raw body
	body := make(map[string]interface{})
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
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
