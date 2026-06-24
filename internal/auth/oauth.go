package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthUserInfo holds the user profile returned by an OAuth provider.
type OAuthUserInfo struct {
	Email      string `json:"email"`
	Name       string `json:"name"`
	AvatarURL  string `json:"avatar_url"`
	ProviderID string `json:"provider_id"`
}

// OAuthProvider defines the interface for OAuth login connectors.
type OAuthProvider interface {
	GetAuthURL(state string) string
	Exchange(ctx context.Context, code string) (*OAuthUserInfo, *oauth2.Token, error)
}

// GoogleProvider implements OAuthProvider for Google Sign-In.
type GoogleProvider struct {
	config *oauth2.Config
}

// NewGoogleProvider reads GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET from the
// environment and returns a configured GoogleProvider. The redirect URL
// defaults to http://localhost:6070/auth/google/callback but can be overridden
// via the GOOGLE_REDIRECT_URL environment variable.
func NewGoogleProvider() *GoogleProvider {
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:6070/auth/google/callback"
	}

	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  redirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

// GetAuthURL returns the URL to redirect the user to for Google consent.
func (p *GoogleProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// Exchange swaps the OAuth authorization code for a token and fetches the
// user's profile from Google.
func (p *GoogleProvider) Exchange(ctx context.Context, code string) (*OAuthUserInfo, *oauth2.Token, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("google exchange: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, nil, fmt.Errorf("google userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var info struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, nil, fmt.Errorf("google userinfo decode: %w", err)
	}

	return &OAuthUserInfo{
		Email:      info.Email,
		Name:       info.Name,
		AvatarURL:  info.Picture,
		ProviderID: info.Sub,
	}, token, nil
}

// GitHubProvider implements OAuthProvider for GitHub OAuth.
type GitHubProvider struct {
	config *oauth2.Config
}

// NewGitHubProvider reads GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET from the
// environment and returns a configured GitHubProvider. The redirect URL
// defaults to http://localhost:6070/auth/github/callback but can be overridden
// via the GITHUB_REDIRECT_URL environment variable.
func NewGitHubProvider() *GitHubProvider {
	redirectURL := os.Getenv("GITHUB_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:6070/auth/github/callback"
	}

	return &GitHubProvider{
		config: &oauth2.Config{
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
	}
}

// GetAuthURL returns the URL to redirect the user to for GitHub consent.
func (p *GitHubProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

// Exchange swaps the OAuth authorization code for a token and fetches the
// user's profile from GitHub.
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*OAuthUserInfo, *oauth2.Token, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("github exchange: %w", err)
	}

	client := p.config.Client(ctx, token)

	// Fetch user profile
	userResp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, nil, fmt.Errorf("github user: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("github user returned status %d", userResp.StatusCode)
	}

	var ghUser struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.NewDecoder(userResp.Body).Decode(&ghUser); err != nil {
		return nil, nil, fmt.Errorf("github user decode: %w", err)
	}

	email := ghUser.Email
	if email == "" {
		// Fetch primary email if not set on profile
		email, err = p.getPrimaryEmail(client)
		if err != nil {
			return nil, nil, err
		}
	}

	name := ghUser.Name
	if name == "" {
		name = ghUser.Login
	}

	return &OAuthUserInfo{
		Email:      email,
		Name:       name,
		AvatarURL:  ghUser.AvatarURL,
		ProviderID: fmt.Sprintf("%d", ghUser.ID),
	}, token, nil
}

// OAuthProviderConfig holds the configuration for any OAuth2-compliant provider.
type OAuthProviderConfig struct {
	Name         string   `json:"name"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	RedirectURL  string   `json:"redirect_url"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	UserInfoURL  string   `json:"userinfo_url"`
	Scopes       []string `json:"scopes"`
	EmailField   string   `json:"email_field"`
	NameField    string   `json:"name_field"`
	AvatarField  string   `json:"avatar_field"`
	IDField      string   `json:"id_field"`
}

// GenericOAuthProvider implements OAuthProvider for any OAuth2-compliant provider.
type GenericOAuthProvider struct {
	config      *oauth2.Config
	userInfoURL string
	emailField  string
	nameField   string
	avatarField string
	idField     string
}

// NewGenericOAuthProvider creates a provider from configuration.
func NewGenericOAuthProvider(cfg OAuthProviderConfig) (*GenericOAuthProvider, error) {
	if cfg.Name == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("generic oauth: name, client_id, and client_secret are required")
	}
	// Use defaults for well-known providers if fields are empty
	emailField := cfg.EmailField
	nameField := cfg.NameField
	avatarField := cfg.AvatarField
	idField := cfg.IDField
	if emailField == "" {
		emailField = "email"
	}
	if nameField == "" {
		nameField = "name"
	}
	if avatarField == "" {
		avatarField = "avatar_url"
	}
	if idField == "" {
		idField = "id"
	}

	return &GenericOAuthProvider{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
		},
		userInfoURL: cfg.UserInfoURL,
		emailField:  emailField,
		nameField:   nameField,
		avatarField: avatarField,
		idField:     idField,
	}, nil
}

// GetAuthURL returns the authorization URL.
func (p *GenericOAuthProvider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

// Exchange swaps the code for a token and fetches the user profile.
func (p *GenericOAuthProvider) Exchange(ctx context.Context, code string) (*OAuthUserInfo, *oauth2.Token, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("oauth exchange: %w", err)
	}

	client := p.config.Client(ctx, token)
	resp, err := client.Get(p.userInfoURL)
	if err != nil {
		return nil, nil, fmt.Errorf("userinfo fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("userinfo returned status %d", resp.StatusCode)
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("userinfo decode: %w", err)
	}

	// Extract fields using configured paths (support nested like "data.email")
	getField := func(field string) string {
		val, _ := getNestedField(raw, field)
		if s, ok := val.(string); ok {
			return s
		}
		// Handle numeric IDs
		if f, ok := val.(float64); ok {
			return fmt.Sprintf("%.0f", f)
		}
		return ""
	}

	email := getField(p.emailField)
	if email == "" {
		return nil, nil, fmt.Errorf("oauth: email not found in userinfo (field: %s)", p.emailField)
	}

	providerID := getField(p.idField)
	if providerID == "" {
		// Fall back to email hash as ID
		providerID = email
	}

	return &OAuthUserInfo{
		Email:      email,
		Name:       getField(p.nameField),
		AvatarURL:  getField(p.avatarField),
		ProviderID: providerID,
	}, token, nil
}

// getNestedField retrieves a nested field from a map using dot notation
// e.g. "data.email" → raw["data"]["email"]
func getNestedField(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, ok := m[part]
		if !ok {
			return nil, false
		}
		current = val
	}
	return current, true
}

// getPrimaryEmail fetches the primary verified email from GitHub when the
// public profile does not include it.
func (p *GitHubProvider) getPrimaryEmail(client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", fmt.Errorf("github emails: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails returned status %d", resp.StatusCode)
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("github emails decode: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no primary verified email found")
}
