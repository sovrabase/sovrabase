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
		return nil, fmt.Errorf("signup: missing token in response")
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
