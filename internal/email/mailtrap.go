package email

import (
	"encoding/json"
	"fmt"
)

type MailtrapMailer struct {
	cfg Config
}

func (m *MailtrapMailer) IsConfigured() bool {
	return m.cfg.APIKey != "" && m.cfg.Sender != ""
}

func (m *MailtrapMailer) SendMail(to []string, subject, body string) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: Mailtrap not configured")
	}
	recipients := make([]map[string]string, len(to))
	for i, addr := range to {
		recipients[i] = map[string]string{"email": addr}
	}
	payload := map[string]interface{}{
		"from":    map[string]string{"email": m.cfg.Sender},
		"to":      recipients,
		"subject": subject,
		"text":    body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal: %w", err)
	}
	return sendHTTPJSON("https://send.api.mailtrap.io/api/send", "POST", m.cfg.APIKey, "", "Api-Token", data)
}
