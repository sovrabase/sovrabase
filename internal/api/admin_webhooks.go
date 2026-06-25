package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// handleAdminListWebhooks returns all webhooks for a project.
func (a *AdminServer) handleAdminListWebhooks(w http.ResponseWriter, r *http.Request) {
	env, err := a.getProjectEnv(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	docs, err := env.Engine.List("_webhooks")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}, "count": 0})
		return
	}
	webhooks := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		webhooks = append(webhooks, map[string]interface{}{
			"id":         d["id"],
			"url":        d["url"],
			"events":     d["events"],
			"enabled":    d["enabled"],
			"created_at": d["_createdAt"],
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  webhooks,
		"count": len(webhooks),
	})
}

// handleAdminCreateWebhook creates a new webhook.
func (a *AdminServer) handleAdminCreateWebhook(w http.ResponseWriter, r *http.Request) {
	env, err := a.getProjectEnv(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var req struct {
		URL     string `json:"url"`
		Events  string `json:"events"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}

	env.Engine.CreateCollection("_webhooks")

	id := uuid.New().String()
	doc := map[string]interface{}{
		"id":      id,
		"url":     req.URL,
		"events":  req.Events,
		"enabled": req.Enabled,
	}
	if err := env.Engine.Insert("_webhooks", id, doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	projectID := r.PathValue("id")
	invalidateWebhookCacheProject(projectID)

	writeJSON(w, http.StatusCreated, doc)
}

// handleAdminDeleteWebhook removes a webhook.
func (a *AdminServer) handleAdminDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	env, err := a.getProjectEnv(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook ID is required")
		return
	}
	if err := env.Engine.Delete("_webhooks", webhookID); err != nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	projectID := r.PathValue("id")
	invalidateWebhookCacheProject(projectID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleAdminUpdateWebhook updates a webhook.
func (a *AdminServer) handleAdminUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	env, err := a.getProjectEnv(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	webhookID := r.PathValue("webhookId")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "webhook ID is required")
		return
	}
	existing, err := env.Engine.Get("_webhooks", webhookID)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	var req struct {
		URL     *string `json:"url"`
		Events  *string `json:"events"`
		Enabled *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL != nil {
		existing["url"] = *req.URL
	}
	if req.Events != nil {
		existing["events"] = *req.Events
	}
	if req.Enabled != nil {
		existing["enabled"] = *req.Enabled
	}
	if err := env.Engine.Update("_webhooks", webhookID, existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	projectID := r.PathValue("id")
	invalidateWebhookCacheProject(projectID)
	writeJSON(w, http.StatusOK, existing)
}

// getProjectEnv helper for admin server.
func (a *AdminServer) getProjectEnv(r *http.Request) (*tenant.ProjectEnv, error) {
	projectID := r.PathValue("id")
	if projectID == "" {
		return nil, fmt.Errorf("project not found")
	}
	return a.projects.GetProjectEnv(projectID)
}
