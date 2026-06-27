package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// TriggerEvent represents a system event that may fire integrations.
type TriggerEvent struct {
	ProjectID  string                 `json:"project_id"`
	Type       string                 `json:"type"`       // "record:create", "record:update", "record:delete", "auth:signup", "auth:signin"
	Collection string                 `json:"collection,omitempty"`
	DocID      string                 `json:"doc_id,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	UserEmail  string                 `json:"user_email,omitempty"`
}

// IntegrationTriggerService watches for system events and fires enabled
// notification integrations (Discord, Slack, OneSignal) when they occur.
type IntegrationTriggerService struct {
	projects *tenant.ProjectManager
	logger   *slog.Logger
	mu       sync.Mutex
}

// NewIntegrationTriggerService creates a trigger service.
func NewIntegrationTriggerService(pm *tenant.ProjectManager) *IntegrationTriggerService {
	return &IntegrationTriggerService{
		projects: pm,
		logger:   slog.Default().With("component", "integration-triggers"),
	}
}

// Fire checks if the project has any notification integrations enabled and
// sends the event to all matching ones. Called asynchronously from hooks.
func (s *IntegrationTriggerService) Fire(event TriggerEvent) {
	if event.ProjectID == "" {
		return
	}

	proj, err := s.projects.GetProject(event.ProjectID)
	if err != nil {
		return
	}

	for _, integ := range proj.Integrations {
		// Check if this integration should fire for this event
		if !s.shouldFire(integ, event) {
			continue
		}

		switch integ.ID {
		case "discord_webhook":
			s.fireDiscord(integ.Config, event)
		case "slack_webhook":
			s.fireSlack(integ.Config, event)
		case "onesignal":
			s.fireOneSignal(integ.Config, event)
		}
	}
}

// shouldFire returns true if the integration is configured to receive this event.
// If Events is empty, all events match. If Collections is non-empty and the
// event is a record:* event, the collection must be in the list.
func (s *IntegrationTriggerService) shouldFire(integ tenant.ProjectIntegration, event TriggerEvent) bool {
	// Event filter: empty = match all
	if len(integ.Events) > 0 {
		matched := false
		for _, e := range integ.Events {
			if e == event.Type {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Collection filter: only applies to record:* events
	if len(integ.Collections) > 0 && event.Type != "auth:signup" && event.Type != "auth:signin" {
		matched := false
		for _, c := range integ.Collections {
			if c == event.Collection {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func (s *IntegrationTriggerService) fireDiscord(cfg map[string]interface{}, event TriggerEvent) {
	url := getString(cfg, "webhook_url")
	if url == "" {
		return
	}

	username := getString(cfg, "username")
	if username == "" {
		username = "Sovrabase"
	}

	title, color := formatEventTitle(event)
	payload := map[string]interface{}{
		"username": username,
		"embeds": []map[string]interface{}{{
			"title":       title,
			"color":       color,
			"description": formatEventDescription(event),
			"fields":      buildDiscordFields(event),
			"timestamp":   nil,
		}},
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		s.logger.Warn("discord trigger failed", "url", url[:30], "error", err)
		return
	}
	resp.Body.Close()
}

func (s *IntegrationTriggerService) fireSlack(cfg map[string]interface{}, event TriggerEvent) {
	url := getString(cfg, "webhook_url")
	if url == "" {
		return
	}

	title, _ := formatEventTitle(event)
	text := fmt.Sprintf("*%s*\n%s", title, formatEventDescription(event))

	payload, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		s.logger.Warn("slack trigger failed", "url", url[:30], "error", err)
		return
	}
	resp.Body.Close()
}

func (s *IntegrationTriggerService) fireOneSignal(cfg map[string]interface{}, event TriggerEvent) {
	appID := getString(cfg, "app_id")
	apiKey := getString(cfg, "rest_api_key")
	if appID == "" || apiKey == "" {
		return
	}

	title, _ := formatEventTitle(event)

	payload := map[string]interface{}{
		"app_id": appID,
		"headings": map[string]string{
			"en": title,
		},
		"contents": map[string]string{
			"en": formatEventDescription(event),
		},
		"included_segments": []string{"All"},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "https://onesignal.com/api/v1/notifications", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Warn("onesignal trigger failed", "error", err)
		return
	}
	resp.Body.Close()
}

// Formatting helpers

func formatEventTitle(event TriggerEvent) (string, int) {
	switch event.Type {
	case "record:create":
		return fmt.Sprintf("📝 New document in `%s`", event.Collection), 0x22C55E
	case "record:update":
		return fmt.Sprintf("✏️ Document updated in `%s`", event.Collection), 0xF59E0B
	case "record:delete":
		return fmt.Sprintf("🗑️ Document deleted from `%s`", event.Collection), 0xEF4444
	case "auth:signup":
		return "👤 New user signed up", 0x5B5BFF
	case "auth:signin":
		return "🔑 User signed in", 0x8B5CF6
	default:
		return fmt.Sprintf("🔔 Event: %s", event.Type), 0x6B7280
	}
}

func formatEventDescription(event TriggerEvent) string {
	desc := fmt.Sprintf("Collection: `%s`\n", event.Collection)
	if event.DocID != "" {
		desc += fmt.Sprintf("Document ID: `%s`\n", event.DocID)
	}
	if event.UserEmail != "" {
		desc += fmt.Sprintf("User: %s\n", event.UserEmail)
	}
	if event.Data != nil {
		for k, v := range event.Data {
			if k == "_id" || k == "_createdAt" || k == "_updatedAt" {
				continue
			}
			sv := fmt.Sprintf("%v", v)
			if len(sv) > 100 {
				sv = sv[:100] + "..."
			}
			desc += fmt.Sprintf("%s: %s\n", k, sv)
		}
	}
	return desc
}

func buildDiscordFields(event TriggerEvent) []map[string]interface{} {
	fields := []map[string]interface{}{
		{"name": "Collection", "value": event.Collection, "inline": true},
	}
	if event.DocID != "" {
		fields = append(fields, map[string]interface{}{"name": "Doc ID", "value": fmt.Sprintf("`%s`", event.DocID), "inline": true})
	}
	if event.UserEmail != "" {
		fields = append(fields, map[string]interface{}{"name": "User", "value": event.UserEmail, "inline": true})
	}
	return fields
}
