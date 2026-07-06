package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

type SmtpMailer struct {
	cfg Config
}

func (m *SmtpMailer) IsConfigured() bool {
	return m.cfg.SMTPHost != "" && m.cfg.SMTPPort > 0 && m.cfg.Sender != ""
}

func (m *SmtpMailer) SendMail(to []string, subject string, msg Message) error {
	if !m.IsConfigured() {
		return fmt.Errorf("email: SMTP not configured")
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)

	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}

	raw := m.buildRaw(to, subject, msg)
	return smtp.SendMail(addr, auth, m.cfg.Sender, to, []byte(raw))
}

func (m *SmtpMailer) buildRaw(to []string, subject string, msg Message) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", m.cfg.Sender))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTML != "" {
		b.WriteString("Content-Type: multipart/alternative; boundary=\"sovrabase-boundary\"\r\n")
		b.WriteString("\r\n")
		b.WriteString("--sovrabase-boundary\r\n")
		b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.Text)
		b.WriteString("\r\n")
		b.WriteString("--sovrabase-boundary\r\n")
		b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.HTML)
		b.WriteString("\r\n")
		b.WriteString("--sovrabase-boundary--\r\n")
	} else {
		b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.Text)
	}
	b.WriteString("\r\n")
	return b.String()
}
