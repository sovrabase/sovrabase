package email

import (
	"encoding/json"
	"fmt"
)

type MailjetMailer struct {
	cfg Config
}

func (m *MailjetMailer) IsConfigured() bool {
	return m.cfg.APIKey != "" && m.cfg.Sender != ""
}

func (m *MailjetMailer) SendMail(to []string, subject, body string) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: Mailjet not configured")
	}
	recipients := make([]map[string]string, len(to))
	for i, addr := range to {
		recipients[i] = map[string]string{"Email": addr}
	}
	payload := map[string]interface{}{
		"Messages": []map[string]interface{}{
			{
				"From":     map[string]string{"Email": m.cfg.Sender},
				"To":       recipients,
				"Subject":  subject,
				"TextPart": body,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal: %w", err)
	}
	key := m.cfg.APIKey
	secret := m.cfg.APISecret
	return sendHTTPJSON("https://api.mailjet.com/v3.1/send", "POST", key, secret, "", data)
}
