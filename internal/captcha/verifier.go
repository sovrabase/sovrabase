// Package captcha provides hCaptcha and Cloudflare Turnstile verification
// for auth endpoints.
package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider identifies the captcha service.
type Provider string

const (
	ProviderHCaptcha   Provider = "hcaptcha"
	ProviderTurnstile  Provider = "turnstile"
)

// Config holds captcha settings.
type Config struct {
	Provider  Provider
	SiteKey   string
	SecretKey string
	Enabled   bool
}

// Verifier validates captcha tokens.
type Verifier struct {
	cfg     Config
	client  *http.Client
}

// NewVerifier creates a captcha verifier.
func NewVerifier(cfg Config) *Verifier {
	return &Verifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Verify validates a captcha token from the client.
// Returns true if captcha is disabled or verification succeeds.
func (v *Verifier) Verify(ctx context.Context, token string) (bool, error) {
	if !v.cfg.Enabled || v.cfg.SecretKey == "" {
		return true, nil // Captcha not configured — allow.
	}
	if token == "" {
		return false, fmt.Errorf("captcha: token is required")
	}

	var verifyURL string
	var postData url.Values

	switch v.cfg.Provider {
	case ProviderHCaptcha:
		verifyURL = "https://api.hcaptcha.com/siteverify"
		postData = url.Values{
			"secret":   {v.cfg.SecretKey},
			"response": {token},
		}
	case ProviderTurnstile:
		verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
		postData = url.Values{
			"secret":   {v.cfg.SecretKey},
			"response": {token},
		}
	default:
		return false, fmt.Errorf("captcha: unknown provider %q", v.cfg.Provider)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", verifyURL, strings.NewReader(postData.Encode()))
	if err != nil {
		return false, fmt.Errorf("captcha: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("captcha: verify request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return false, fmt.Errorf("captcha: read response: %w", err)
	}

	var result struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("captcha: parse response: %w", err)
	}

	if !result.Success {
		return false, fmt.Errorf("captcha: verification failed: %s", strings.Join(result.ErrorCodes, ", "))
	}
	return true, nil
}

// SiteKey returns the public site key for client-side rendering.
func (v *Verifier) SiteKey() string {
	return v.cfg.SiteKey
}

// IsEnabled returns whether captcha verification is active.
func (v *Verifier) IsEnabled() bool {
	return v.cfg.Enabled && v.cfg.SecretKey != ""
}
