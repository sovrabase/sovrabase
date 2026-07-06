package email

import "fmt"

type MaildevMailer struct {
	cfg Config
}

func (m *MaildevMailer) IsConfigured() bool {
	return m.cfg.Sender != ""
}

func (m *MaildevMailer) SendMail(to []string, subject string, msg Message) error {
	if m.cfg.Sender == "" {
		return fmt.Errorf("email: maildev requires a sender email")
	}
	return nil
}
