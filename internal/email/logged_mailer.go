package email

import (
	"strings"
	"time"
)

type LoggedMailer struct {
	inner    Mailer
	logStore *LogStore
	provider string
	from     string
}

func NewLoggedMailer(inner Mailer, logStore *LogStore, provider, from string) *LoggedMailer {
	return &LoggedMailer{inner: inner, logStore: logStore, provider: provider, from: from}
}

func (m *LoggedMailer) IsConfigured() bool {
	return m.inner.IsConfigured()
}

func (m *LoggedMailer) SendMail(to []string, subject, body string) error {
	err := m.inner.SendMail(to, subject, body)

	entry := EmailLogEntry{
		Timestamp: time.Now().UTC(),
		Provider:  m.provider,
		From:      m.from,
		To:        strings.Join(to, ", "),
		Subject:   subject,
		Success:   err == nil,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	if appendErr := m.logStore.Append(&entry); appendErr != nil {
		_ = appendErr // Don't mask the original send error
	}
	return err
}
