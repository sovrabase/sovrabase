package email

import (
	"encoding/json"
	"fmt"
)

type BrevoMailer struct {
	cfg Config
}

func (m *BrevoMailer) IsConfigured() bool {
	return m.cfg.APIKey != "" && m.cfg.Sender != ""
}

func (m *BrevoMailer) SendMail(to []string, subject, body string) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: Brevo not configured")
	}
	recipients := make([]map[string]string, len(to))
	for i, addr := range to {
		recipients[i] = map[string]string{"email": addr}
	}
	payload := map[string]interface{}{
		"sender":     map[string]string{"email": m.cfg.Sender},
		"to":         recipients,
		"subject":    subject,
		"textContent": body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal: %w", err)
	}
	return sendHTTPJSON("https://api.brevo.com/v3/smtp/email", "POST", m.cfg.APIKey, "", "api-key", data)
}
