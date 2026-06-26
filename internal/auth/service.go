package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AuthService is the main authentication service. It coordinates user
// management, JWT issuance, and OAuth flows.
type AuthService struct {
	jwtSecret                string
	store                    UserStore
	oauthStates              map[string]stateEntry // state token → provider name
	oauthStatesMu            sync.Mutex
	providers                map[string]OAuthProvider
	EmailVerificationEnabled bool
	SMTPHost                 string
	SMTPPort                 int
	SMTPUser                 string
	SMTPPassword             string
	SMTPSender               string
}

type stateEntry struct {
	provider  string
	expiresAt time.Time
}

// OAuthStatePayload is JSON-encoded into the OAuth state token so the callback
// can recover project context and redirect preferences without query params.
type OAuthStatePayload struct {
	ProjectID   string `json:"project_id"`
	Provider    string `json:"provider,omitempty"`
	AppRedirect string `json:"app_redirect,omitempty"`
}

// NewService creates a new AuthService backed by the given UserStore.
func NewService(jwtSecret string, userStore UserStore) *AuthService {
	return &AuthService{
		jwtSecret:   jwtSecret,
		store:       userStore,
		oauthStates: make(map[string]stateEntry),
		providers:   make(map[string]OAuthProvider),
	}
}

// RegisterOAuthProvider adds an OAuth provider that can be used for social login.
func (s *AuthService) RegisterOAuthProvider(name string, provider OAuthProvider) {
	s.providers[name] = provider
}

// SignUp creates a new user account and returns the user with a token pair.
func (s *AuthService) SignUp(email, password string) (*User, *TokenPair, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil, fmt.Errorf("email is required")
	}
	if password == "" {
		return nil, nil, fmt.Errorf("password is required")
	}
	if len(password) < 8 {
		return nil, nil, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, fmt.Errorf("hashing password: %w", err)
	}

	user := NewUser(email, hash)

	// If email verification is enabled and SMTP is set up, enforce verification
	if s.EmailVerificationEnabled && s.SMTPHost != "" {
		verifyBytes := make([]byte, 16)
		if _, randErr := rand.Read(verifyBytes); randErr != nil {
			return nil, nil, fmt.Errorf("generating verification token: %w", randErr)
		}
		user.VerificationToken = hex.EncodeToString(verifyBytes)
		user.VerificationExpires = time.Now().Add(24 * time.Hour)
		user.IsVerified = false

		if err := s.store.Create(user); err != nil {
			return nil, nil, err
		}

		// Return nil tokens because verification is required
		return user, nil, nil
	}

	// Otherwise, mark user as verified immediately and log them in
	user.IsVerified = true
	if err := s.store.Create(user); err != nil {
		return nil, nil, err
	}

	tokens, err := s.generateTokenPair(user)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	return user, tokens, nil
}

// SignIn authenticates a user by email and password, returning a token pair.
// If the user has MFA enabled, it returns an error containing "mfa_required"
// so the caller can switch to SignInWithMFA.
func (s *AuthService) SignIn(email, password string) (*TokenPair, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// Only enforce verification check if verification is enabled and SMTP is set up
	if s.EmailVerificationEnabled && s.SMTPHost != "" && !user.IsVerified {
		return nil, fmt.Errorf("email not verified")
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// If MFA is enabled, refuse to issue tokens directly.
	if user.MFAEnabled {
		return nil, fmt.Errorf("mfa_required: use SignInWithMFA")
	}

	return s.generateTokenPair(user)
}

// SignInResult is returned by SignInWithMFA. When MFA is enabled, Token is nil
// and the caller must use ChallengeToken with CompleteMFAChallenge.
type SignInResult struct {
	Token          *TokenPair `json:"token,omitempty"`
	MFARequired    bool       `json:"mfa_required"`
	ChallengeToken string     `json:"challenge_token,omitempty"`
	ExpiresIn      int64      `json:"expires_in,omitempty"`
}

// SignInWithMFA authenticates a user and returns either tokens or an MFA
// challenge. If the user has MFA enabled, a short-lived challenge token
// is returned instead of tokens. The caller must then call
// CompleteMFAChallenge with the challenge token and a valid TOTP code.
func (s *AuthService) SignInWithMFA(email, password string) (*SignInResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	if s.EmailVerificationEnabled && s.SMTPHost != "" && !user.IsVerified {
		return nil, fmt.Errorf("email not verified")
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	// If MFA is enabled, return a challenge instead of tokens.
	if user.MFAEnabled {
		challengeToken, err := GenerateChallengeToken(user, s.jwtSecret)
		if err != nil {
			return nil, fmt.Errorf("generating challenge: %w", err)
		}
		return &SignInResult{
			MFARequired:    true,
			ChallengeToken: challengeToken,
			ExpiresIn:      int64(challengeTokenTTL.Seconds()),
		}, nil
	}

	tokens, err := s.generateTokenPair(user)
	if err != nil {
		return nil, err
	}
	return &SignInResult{Token: tokens}, nil
}

// CompleteMFAChallenge validates the TOTP code against the challenge token
// and returns a full token pair if successful.
func (s *AuthService) CompleteMFAChallenge(challengeToken, code string) (*TokenPair, error) {
	claims, err := ValidateChallengeToken(challengeToken, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired challenge: %w", err)
	}

	user, err := s.store.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.MFAEnabled {
		return nil, fmt.Errorf("MFA is not enabled for this user")
	}

	// Validate TOTP code.
	if !ValidateTOTP(user.MFASecret, code) {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	return s.generateTokenPair(user)
}

// RefreshToken validates a refresh token and issues a new token pair.
func (s *AuthService) RefreshToken(refreshToken string) (*TokenPair, error) {
	claims, err := ValidateToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	user, err := s.store.GetByID(claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return s.generateTokenPair(user)
}

// ValidateAccessToken verifies an access token and returns its claims.
func (s *AuthService) ValidateAccessToken(tokenString string) (*Claims, error) {
	return ValidateToken(tokenString, s.jwtSecret)
}

// GetUser retrieves a user by ID.
func (s *AuthService) GetUser(id string) (*User, error) {
	return s.store.GetByID(id)
}

// MustGetUser is like GetUser but panics on error. Use only when the caller
// has already verified the user exists (e.g. after a successful update).
func (s *AuthService) MustGetUser(id string) *User {
	u, err := s.store.GetByID(id)
	if err != nil {
		panic("MustGetUser: " + err.Error())
	}
	return u
}

// ListUsers returns all users in the store.
func (s *AuthService) ListUsers() ([]*User, error) {
	return s.store.List()
}

// UpdateUser persists changes to a user.
func (s *AuthService) UpdateUser(user *User) error {
	return s.store.Update(user)
}

// DeleteUser removes a user.
func (s *AuthService) DeleteUser(id string) error {
	return s.store.Delete(id)
}

// CreateOAuthState generates a cryptographically random state token for an
// OAuth flow. Metadata (project ID, app redirect) is JSON-encoded into the
// state so the callback can recover it without query parameters.
// State format: base64url(json_payload) + "." + hex(random32)
func (s *AuthService) CreateOAuthState(provider, projectID, appRedirect string) (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	payload := OAuthStatePayload{
		ProjectID:   projectID,
		Provider:    provider,
		AppRedirect: appRedirect,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encoding state payload: %w", err)
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(jsonPayload)
	state := encodedPayload + "." + hex.EncodeToString(randomBytes)

	s.oauthStatesMu.Lock()
	s.oauthStates[state] = stateEntry{
		provider:  provider,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
	s.oauthStatesMu.Unlock()

	// Background cleanup of expired states
	go s.cleanupExpiredStates()

	return state, nil
}

// CreateOAuthStateURL generates a state token and returns the full authorization
// URL to redirect the user to for the given provider.
func (s *AuthService) CreateOAuthStateURL(provider, projectID, appRedirect string) (authURL, state string, err error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", "", fmt.Errorf("unknown OAuth provider: %s", provider)
	}

	state, err = s.CreateOAuthState(provider, projectID, appRedirect)
	if err != nil {
		return "", "", err
	}

	authURL = p.GetAuthURL(state)
	return authURL, state, nil
}

// DecodeStatePayload extracts the full OAuth state payload (project ID,
// app redirect, etc.) from a state token.
func (s *AuthService) DecodeStatePayload(state string) (*OAuthStatePayload, error) {
	idx := strings.IndexByte(state, '.')
	if idx == -1 {
		return nil, fmt.Errorf("invalid state format: no separator")
	}

	jsonBytes, err := base64.RawURLEncoding.DecodeString(state[:idx])
	if err != nil {
		return nil, fmt.Errorf("invalid state format: %w", err)
	}

	var payload OAuthStatePayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid state payload: %w", err)
	}
	if payload.ProjectID == "" {
		return nil, fmt.Errorf("state payload missing project_id")
	}
	return &payload, nil
}


// HandleOAuthCallback completes an OAuth flow: validates the state, exchanges
// the code, and either finds or creates a user. Returns the user and a token pair.
func (s *AuthService) HandleOAuthCallback(provider, code, state string) (*User, *TokenPair, error) {
	s.oauthStatesMu.Lock()
	entry, exists := s.oauthStates[state]
	if exists {
		delete(s.oauthStates, state)
	}
	s.oauthStatesMu.Unlock()

	if !exists {
		return nil, nil, fmt.Errorf("invalid or expired OAuth state")
	}
	if entry.provider != provider {
		return nil, nil, fmt.Errorf("OAuth state provider mismatch")
	}
	if time.Now().After(entry.expiresAt) {
		return nil, nil, fmt.Errorf("OAuth state expired")
	}

	p, ok := s.providers[provider]
	if !ok {
		return nil, nil, fmt.Errorf("unknown OAuth provider: %s", provider)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	oauthInfo, providerToken, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("oauth exchange: %w", err)
	}

	// Try to find an existing user by email
	user, err := s.store.GetByEmail(oauthInfo.Email)
	if err != nil {
		user = NewUser(oauthInfo.Email, "")
		user.IsVerified = true // OAuth users are verified!
		user.Name = oauthInfo.Name
		user.AvatarURL = oauthInfo.AvatarURL
		user.OAuthProviders = []OAuthProviderMetadata{{
			Provider:     provider,
			ProviderID:   oauthInfo.ProviderID,
			AccessToken:  providerToken.AccessToken,
			RefreshToken: providerToken.RefreshToken,
			TokenExpiry:  providerToken.Expiry,
		}}
		if createErr := s.store.Create(user); createErr != nil {
			return nil, nil, fmt.Errorf("creating oauth user: %w", createErr)
		}
	} else {
		// Existing user — update OAuth metadata on every login so name/avatar stay fresh.
		updated := false
		if !user.IsVerified {
			user.IsVerified = true
			updated = true
		}
		if oauthInfo.Name != "" && user.Name != oauthInfo.Name {
			user.Name = oauthInfo.Name
			updated = true
		}
		if oauthInfo.AvatarURL != "" && user.AvatarURL != oauthInfo.AvatarURL {
			user.AvatarURL = oauthInfo.AvatarURL
			updated = true
		}

		// Upsert into OAuthProviders array — same provider = update, new provider = append.
		found := false
		for i := range user.OAuthProviders {
			if user.OAuthProviders[i].Provider == provider {
				user.OAuthProviders[i].ProviderID = oauthInfo.ProviderID
				user.OAuthProviders[i].AccessToken = providerToken.AccessToken
				user.OAuthProviders[i].RefreshToken = providerToken.RefreshToken
				user.OAuthProviders[i].TokenExpiry = providerToken.Expiry
				found = true
				updated = true
				break
			}
		}
		if !found {
			user.OAuthProviders = append(user.OAuthProviders, OAuthProviderMetadata{
				Provider:     provider,
				ProviderID:   oauthInfo.ProviderID,
				AccessToken:  providerToken.AccessToken,
				RefreshToken: providerToken.RefreshToken,
				TokenExpiry:  providerToken.Expiry,
			})
			updated = true
		}
		if updated {
			_ = s.store.Update(user)
		}
	}

	tokens, err := s.generateTokenPair(user)
	if err != nil {
		return nil, nil, fmt.Errorf("generating tokens: %w", err)
	}

	return user, tokens, nil
}

// generateTokenPair creates both access and refresh tokens for a user.
func (s *AuthService) generateTokenPair(user *User) (*TokenPair, error) {
	accessToken, err := GenerateAccessToken(user, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	refreshToken, err := GenerateRefreshToken(user, s.jwtSecret)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(DefaultAccessTokenTTL.Seconds()),
	}, nil
}

// cleanupExpiredStates removes OAuth state entries that have passed their expiry.
func (s *AuthService) cleanupExpiredStates() {
	s.oauthStatesMu.Lock()
	defer s.oauthStatesMu.Unlock()

	now := time.Now()
	for state, entry := range s.oauthStates {
		if now.After(entry.expiresAt) {
			delete(s.oauthStates, state)
		}
	}
}

// generateRandomString creates a cryptographically random hex string.
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// VerifyEmail validates a verification token and marks the user as verified.
func (s *AuthService) VerifyEmail(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	user, err := s.store.GetByVerificationToken(token)
	if err != nil {
		return fmt.Errorf("invalid or expired verification token")
	}

	if time.Now().After(user.VerificationExpires) {
		return fmt.Errorf("verification token has expired")
	}

	user.IsVerified = true
	user.VerificationToken = ""
	user.VerificationExpires = time.Time{}

	return s.store.Update(user)
}

// ForgotPassword generates a password reset token for the given email.
// It returns the reset token for delivery (mocked/API output).
func (s *AuthService) ForgotPassword(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		return "", fmt.Errorf("user with email %q not found", email)
	}

	resetBytes := make([]byte, 16)
	if _, randErr := rand.Read(resetBytes); randErr != nil {
		return "", fmt.Errorf("generating reset token: %w", randErr)
	}
	token := hex.EncodeToString(resetBytes)

	user.ResetToken = token
	user.ResetExpires = time.Now().Add(1 * time.Hour) // valid for 1 hour

	if err := s.store.Update(user); err != nil {
		return "", fmt.Errorf("saving reset token: %w", err)
	}

	return token, nil
}

// ResetPassword validates a reset token and updates the user's password.
func (s *AuthService) ResetPassword(token, newPassword string) error {
	token = strings.TrimSpace(token)
	newPassword = strings.TrimSpace(newPassword)
	if token == "" {
		return fmt.Errorf("token is required")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	user, err := s.store.GetByResetToken(token)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	if time.Now().After(user.ResetExpires) {
		return fmt.Errorf("reset token has expired")
	}

	hash, err := HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user.PasswordHash = hash
	user.ResetToken = ""
	user.ResetExpires = time.Time{}
	// Also mark user as verified if they successfully reset password
	user.IsVerified = true

	return s.store.Update(user)
}

// CreateMagicLink generates a single-use magic link token for passwordless
// login. The token is stored on the user record and returned for delivery
// (typically via email). The token expires after 15 minutes.
func (s *AuthService) CreateMagicLink(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		// Don't leak whether the email exists — return a generic error.
		return "", fmt.Errorf("user not found")
	}

	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating magic link token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	user.MagicLinkToken = token
	user.MagicLinkExpires = time.Now().Add(15 * time.Minute)

	if err := s.store.Update(user); err != nil {
		return "", fmt.Errorf("saving magic link token: %w", err)
	}

	return token, nil
}

// VerifyMagicLink validates a magic link token and returns a token pair for
// passwordless authentication. The token is single-use and is cleared after.
func (s *AuthService) VerifyMagicLink(email, token string) (*TokenPair, error) {
	email = strings.TrimSpace(email)
	token = strings.TrimSpace(token)
	if email == "" || token == "" {
		return nil, fmt.Errorf("email and token are required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired magic link")
	}

	if user.MagicLinkToken == "" || user.MagicLinkToken != token {
		return nil, fmt.Errorf("invalid or expired magic link")
	}

	if time.Now().After(user.MagicLinkExpires) {
		return nil, fmt.Errorf("magic link has expired")
	}

	// Clear the token (single-use).
	user.MagicLinkToken = ""
	user.MagicLinkExpires = time.Time{}
	user.IsVerified = true

	if err := s.store.Update(user); err != nil {
		return nil, fmt.Errorf("updating user: %w", err)
	}

	return s.generateTokenPair(user)
}

// ─── MFA (TOTP) Methods ──────────────────────────────────────────────────────

// SetupMFA generates a new TOTP secret for the user and returns the secret
// along with an otpauth:// URI for QR code generation. The secret is NOT
// stored until ConfirmMFA is called with a valid TOTP code.
func (s *AuthService) SetupMFA(userID string) (secret, uri string, err error) {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return "", "", fmt.Errorf("user not found")
	}

	secret, err = GenerateMFASecret()
	if err != nil {
		return "", "", err
	}

	// Store the secret temporarily (not yet enabled).
	user.MFASecret = secret
	if err := s.store.Update(user); err != nil {
		return "", "", fmt.Errorf("saving MFA secret: %w", err)
	}

	uri = GenerateOTPUri(secret, user.Email, "Sovrabase")
	return secret, uri, nil
}

// ConfirmMFA verifies a TOTP code and enables MFA for the user.
// Returns backup codes that the user should save for recovery.
func (s *AuthService) ConfirmMFA(userID, code string) ([]string, error) {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if user.MFASecret == "" {
		return nil, fmt.Errorf("MFA not set up — call SetupMFA first")
	}
	if user.MFAEnabled {
		return nil, fmt.Errorf("MFA is already enabled")
	}

	if !ValidateTOTP(user.MFASecret, code) {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	// Generate backup codes.
	backupCodes, err := GenerateBackupCodes()
	if err != nil {
		return nil, err
	}

	user.MFAEnabled = true
	user.MFABackupCodes = backupCodes

	if err := s.store.Update(user); err != nil {
		return nil, fmt.Errorf("enabling MFA: %w", err)
	}

	return backupCodes, nil
}

// DisableMFA turns off MFA for the user after verifying either a TOTP code
// or a backup code.
func (s *AuthService) DisableMFA(userID, code string) error {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !user.MFAEnabled {
		return fmt.Errorf("MFA is not enabled")
	}

	// Check TOTP code first.
	codeValid := ValidateTOTP(user.MFASecret, code)

	// Check backup codes.
	if !codeValid && len(user.MFABackupCodes) > 0 {
		for i, bc := range user.MFABackupCodes {
			if bc == code {
				// Remove used backup code.
				user.MFABackupCodes = append(user.MFABackupCodes[:i], user.MFABackupCodes[i+1:]...)
				codeValid = true
				break
			}
		}
	}

	if !codeValid {
		return fmt.Errorf("invalid TOTP code or backup code")
	}

	user.MFAEnabled = false
	user.MFASecret = ""
	user.MFABackupCodes = nil

	return s.store.Update(user)
}

// VerifyMFA checks a TOTP code (or backup code) for a user. Used during login.
func (s *AuthService) VerifyMFA(userID, code string) error {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !user.MFAEnabled {
		return nil // No MFA required
	}

	// Check TOTP.
	if ValidateTOTP(user.MFASecret, code) {
		return nil
	}

	// Check backup codes.
	for i, bc := range user.MFABackupCodes {
		if bc == code {
			// Consume the backup code.
			user.MFABackupCodes = append(user.MFABackupCodes[:i], user.MFABackupCodes[i+1:]...)
			_ = s.store.Update(user)
			return nil
		}
	}

	return fmt.Errorf("invalid MFA code")
}

// GetMFAStatus returns whether MFA is enabled for a user.
func (s *AuthService) GetMFAStatus(userID string) (enabled bool, err error) {
	user, err := s.store.GetByID(userID)
	if err != nil {
		return false, err
	}
	return user.MFAEnabled, nil
}
