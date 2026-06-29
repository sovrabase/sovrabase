package email

import (
	"fmt"
	"net/smtp"
)

type SmtpMailer struct {
	cfg Config
}

func (m *SmtpMailer) IsConfigured() bool {
	return m.cfg.SMTPHost != "" && m.cfg.SMTPPort > 0 && m.cfg.Sender != ""
}

func (m *SmtpMailer) SendMail(to []string, subject, body string) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: SMTP not configured")
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)

	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}

	msg := buildMessage(m.cfg.Sender, to, subject, body)

	return smtp.SendMail(addr, auth, m.cfg.Sender, to, []byte(msg))
}
