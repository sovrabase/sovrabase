package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ─── Auth ──────────────────────────────────────────────────────────────────────

// SignUp creates a new user account.
func (c *Client) SignUp(email, password string) (*AuthResponse, error) {
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	// SignUp returns {user, token}.
	var raw struct {
		User  *User        `json:"user"`
		Token *AuthResponse `json:"token"`
	}
	if err := c.doJSON("POST", "/auth/v1/signup", body, &raw); err != nil {
		return nil, err
	}

	if raw.Token == nil {
		// Email verification required — return user without tokens.
		// The caller should prompt the user to check their email.
		return &AuthResponse{User: raw.User}, nil
	}
	resp := raw.Token
	resp.User = raw.User

	// Store tokens.
	c.SetAuth(resp.AccessToken, resp.RefreshToken)
	return resp, nil
}

// SignIn authenticates a user by email and password.
// When MFA is enabled, the returned SignInResult will have MFARequired=true
// and a ChallengeToken that must be completed via CompleteMFAChallenge.
func (c *Client) SignIn(email, password string) (*SignInResult, error) {
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	fullURL := c.baseURL + "/auth/v1/signin"
	req, err := http.NewRequest("POST", fullURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	var raw struct {
		AccessToken    string `json:"access_token"`
		RefreshToken   string `json:"refresh_token"`
		ExpiresIn      int64  `json:"expires_in"`
		User           *User  `json:"user"`
		Error          string `json:"error"`
		ChallengeToken string `json:"challenge_token"`
	}

	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// MFA required — return challenge without calling SignInWithMFA automatically.
	if raw.Error == "mfa_required" {
		return &SignInResult{
			MFARequired:    true,
			ChallengeToken: raw.ChallengeToken,
			ExpiresIn:      raw.ExpiresIn,
		}, nil
	}

	if resp.StatusCode >= 400 {
		if raw.Error != "" {
			return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, raw.Error)
		}
		return nil, fmt.Errorf("server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	token := &TokenPair{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresIn:    int(raw.ExpiresIn),
	}
	c.SetAuth(token.AccessToken, token.RefreshToken)

	return &SignInResult{Token: token}, nil
}

// SignInWithMFA initiates the MFA sign-in flow, returning a challenge token.
// The caller must complete the challenge with CompleteMFAChallenge.
func (c *Client) SignInWithMFA(email, password string) (*SignInResult, error) {
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	var result SignInResult
	if err := c.doJSON("POST", "/auth/v1/signin-mfa", body, &result); err != nil {
		// If server returned mfa_required, try to extract the SignInResult from the error.
		if strings.Contains(err.Error(), "mfa_required") {
			return nil, err
		}
		return nil, err
	}
	if result.Token != nil {
		c.SetAuth(result.Token.AccessToken, result.Token.RefreshToken)
	}
	return &result, nil
}

// CompleteMFAChallenge completes an MFA challenge with a TOTP or backup code.
// Returns the full token pair on success.
func (c *Client) CompleteMFAChallenge(challengeToken, code string) (*TokenPair, error) {
	body := map[string]interface{}{
		"challenge_token": challengeToken,
		"code":            code,
	}
	var token TokenPair
	if err := c.doJSON("POST", "/auth/v1/mfa/complete", body, &token); err != nil {
		return nil, err
	}
	c.SetAuth(token.AccessToken, token.RefreshToken)
	return &token, nil
}

// Refresh uses the stored refresh token to obtain new access and refresh tokens.
func (c *Client) Refresh() error {
	c.mu.Lock()
	refreshToken := c.refreshToken
	c.mu.Unlock()

	if refreshToken == "" {
		return fmt.Errorf("refresh: no refresh token available")
	}

	body := map[string]interface{}{
		"refresh_token": refreshToken,
	}

	var resp AuthResponse
	if err := c.doJSON("POST", "/auth/v1/refresh", body, &resp); err != nil {
		return err
	}

	c.SetAuth(resp.AccessToken, resp.RefreshToken)

	c.mu.Lock()
	if c.OnTokenRefresh != nil {
		c.OnTokenRefresh(resp.AccessToken, resp.RefreshToken)
	}
	c.mu.Unlock()

	return nil
}

// ForgotPassword sends a password reset email.
// Returns the server response which includes the reset token in dev mode.
func (c *Client) ForgotPassword(email string) (Document, error) {
	body := map[string]interface{}{
		"email": email,
	}

	var resp Document
	if err := c.doJSON("POST", "/auth/v1/forgot-password", body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ResetPassword sets a new password using a reset token.
func (c *Client) ResetPassword(token, newPassword string) error {
	body := map[string]interface{}{
		"token":    token,
		"password": newPassword,
	}
	return c.doJSON("POST", "/auth/v1/reset-password", body, nil)
}

// VerifyEmail verifies a user's email address using a verification token.
func (c *Client) VerifyEmail(token string) error {
	body := map[string]interface{}{
		"token": token,
	}
	return c.doJSON("POST", "/auth/v1/verify-email", body, nil)
}

// GetMe returns the currently authenticated user's profile.
func (c *Client) GetMe() (*User, error) {
	var user User
	if err := c.doJSON("GET", "/api/v1/me", nil, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateMe updates the current user's name and/or avatar URL.
// Pass nil for fields that should not be changed.
func (c *Client) UpdateMe(name, avatarURL *string) (*UserInfo, error) {
	body := map[string]interface{}{}
	if name != nil {
		body["name"] = *name
	}
	if avatarURL != nil {
		body["avatar_url"] = *avatarURL
	}
	var info UserInfo
	if err := c.doJSON("PATCH", "/api/v1/me", body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// OAuthURL returns the authorization URL for an OAuth provider.
// Set redirect=true to have the server redirect the browser directly.
// Set finalRedirect to specify where to redirect after OAuth completes.
func (c *Client) OAuthURL(provider string) (string, error) {
	path := fmt.Sprintf("/auth/v1/oauth/%s", provider)
	var resp struct {
		URL string `json:"url"`
	}
	if err := c.doJSON("GET", path, nil, &resp); err != nil {
		return "", err
	}
	return resp.URL, nil
}

// ─── Magic Links ──────────────────────────────────────────────────────────────

// CreateMagicLink sends a magic link to the given email for passwordless login.
// Returns the token (in dev mode) or an empty string.
func (c *Client) CreateMagicLink(email string) (string, error) {
	body := map[string]interface{}{"email": email}
	var resp struct {
		Message string `json:"message"`
		Token   string `json:"token"`
	}
	if err := c.doJSON("POST", "/auth/v1/magic-link", body, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// VerifyMagicLink completes a magic link login flow.
// Returns the authentication tokens on success.
func (c *Client) VerifyMagicLink(email, token string) error {
	body := map[string]interface{}{
		"email": email,
		"token": token,
	}
	var resp AuthResponse
	if err := c.doJSON("POST", "/auth/v1/verify-magic-link", body, &resp); err != nil {
		return err
	}
	c.SetAuth(resp.AccessToken, resp.RefreshToken)
	return nil
}

// ─── MFA (TOTP) ───────────────────────────────────────────────────────────────

// MFASetupResponse contains the TOTP secret and otpauth URI from SetupMFA.
type MFASetupResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

// MFAConfirmResponse contains the backup codes returned after enabling MFA.
type MFAConfirmResponse struct {
	Message      string   `json:"message"`
	BackupCodes  []string `json:"backup_codes"`
}

// SetupMFA generates a new TOTP secret. The secret must be confirmed with
// ConfirmMFA before MFA is fully enabled.
func (c *Client) SetupMFA() (*MFASetupResponse, error) {
	var resp MFASetupResponse
	if err := c.doJSON("POST", "/auth/v1/mfa/setup", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConfirmMFA verifies a TOTP code and enables MFA. Returns backup codes.
func (c *Client) ConfirmMFA(code string) ([]string, error) {
	body := map[string]interface{}{"code": code}
	var resp MFAConfirmResponse
	if err := c.doJSON("POST", "/auth/v1/mfa/confirm", body, &resp); err != nil {
		return nil, err
	}
	return resp.BackupCodes, nil
}

// DisableMFA turns off MFA after verifying a TOTP or backup code.
func (c *Client) DisableMFA(code string) error {
	body := map[string]interface{}{"code": code}
	return c.doJSON("POST", "/auth/v1/mfa/disable", body, nil)
}

// GetMFAStatus returns whether MFA is enabled for the current user.
func (c *Client) GetMFAStatus() (bool, error) {
	var resp struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.doJSON("GET", "/auth/v1/mfa/status", nil, &resp); err != nil {
		return false, err
	}
	return resp.Enabled, nil
}
