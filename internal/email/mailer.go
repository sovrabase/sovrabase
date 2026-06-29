package email

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

type Provider string

const (
	ProviderSMTP     Provider = "smtp"
	ProviderResend   Provider = "resend"
	ProviderMailtrap Provider = "mailtrap"
	ProviderBrevo    Provider = "brevo"
	ProviderMailjet  Provider = "mailjet"
)

type Mailer interface {
	SendMail(to []string, subject, body string) error
	IsConfigured() bool
}

type Config struct {
	Provider  Provider
	Sender    string
	SMTPHost  string
	SMTPPort  int
	SMTPUser  string
	SMTPPass  string
	APIKey    string
	APISecret string
}

func NewMailer(cfg Config) Mailer {
	switch cfg.Provider {
	case ProviderResend:
		return &ResendMailer{cfg: cfg}
	case ProviderMailtrap:
		return &MailtrapMailer{cfg: cfg}
	case ProviderBrevo:
		return &BrevoMailer{cfg: cfg}
	case ProviderMailjet:
		return &MailjetMailer{cfg: cfg}
	default:
		return &SmtpMailer{cfg: cfg}
	}
}

func buildMessage(from string, to []string, subject, body string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	b.WriteString("\r\n")
	return b.String()
}

func sendHTTPJSON(url, method, apiKey, apiSecret string, headerKey string, body []byte) error {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("email: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if headerKey != "" {
		req.Header.Set(headerKey, apiKey)
	} else if apiSecret != "" {
		req.SetBasicAuth(apiKey, apiSecret)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("email: send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("email: API error %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}
	return nil
}
