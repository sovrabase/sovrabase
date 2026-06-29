package email

import (
	"encoding/json"
	"fmt"
)

type ResendMailer struct {
	cfg Config
}

func (m *ResendMailer) IsConfigured() bool {
	return m.cfg.APIKey != "" && m.cfg.Sender != ""
}

func (m *ResendMailer) SendMail(to []string, subject, body string) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: Resend not configured")
	}
	payload := map[string]interface{}{
		"from":    m.cfg.Sender,
		"to":      to,
		"subject": subject,
		"text":    body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("email: marshal: %w", err)
	}
	return sendHTTPJSON("https://api.resend.com/emails", "POST", m.cfg.APIKey, "", "", data)
}
