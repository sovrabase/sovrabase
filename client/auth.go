package client

import (
	"fmt"
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
func (c *Client) SignIn(email, password string) (*AuthResponse, error) {
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}

	// SignIn returns the token pair directly.
	var resp AuthResponse
	if err := c.doJSON("POST", "/auth/v1/signin", body, &resp); err != nil {
		return nil, err
	}

	c.SetAuth(resp.AccessToken, resp.RefreshToken)
	return &resp, nil
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
