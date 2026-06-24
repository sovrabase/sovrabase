package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AuthService is the main authentication service. It coordinates user
// management, JWT issuance, and OAuth flows.
type AuthService struct {
	jwtSecret     string
	store         UserStore
	oauthStates   map[string]stateEntry // state token → provider name
	oauthStatesMu sync.Mutex
	providers     map[string]OAuthProvider
}

type stateEntry struct {
	provider  string
	expiresAt time.Time
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
func (s *AuthService) SignIn(email, password string) (*TokenPair, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	user, err := s.store.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid email or password")
	}

	if err := CheckPassword(user.PasswordHash, password); err != nil {
		return nil, fmt.Errorf("invalid email or password")
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
// OAuth flow and stores it linked to the provider name.
func (s *AuthService) CreateOAuthState(provider string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	state := hex.EncodeToString(bytes)

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

	oauthInfo, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("oauth exchange: %w", err)
	}

	// Try to find an existing user by email
	user, err := s.store.GetByEmail(oauthInfo.Email)
	if err != nil {
		// User doesn't exist; create one with a random password (OAuth users
		// authenticate via provider, not password).
		randomPass, _ := generateRandomString(32)
		hash, hashErr := HashPassword(randomPass)
		if hashErr != nil {
			return nil, nil, fmt.Errorf("creating oauth user: %w", hashErr)
		}

		user = NewUser(oauthInfo.Email, hash)
		if createErr := s.store.Create(user); createErr != nil {
			return nil, nil, fmt.Errorf("creating oauth user: %w", createErr)
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
