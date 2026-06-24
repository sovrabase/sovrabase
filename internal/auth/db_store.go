package auth

import (
	"fmt"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/db"
)

// DBUserStore implements UserStore using a db.Engine.
type DBUserStore struct {
	engine *db.Engine
}

// NewDBUserStore creates a new DBUserStore and ensures the _users collection exists.
func NewDBUserStore(engine *db.Engine) *DBUserStore {
	// Ensure collection exists; ignore already exists errors.
	_ = engine.CreateCollection("_users")
	return &DBUserStore{engine: engine}
}

// Create inserts a new user.
func (s *DBUserStore) Create(user *User) error {
	// Check for duplicate emails
	existing, err := s.GetByEmail(user.Email)
	if err == nil && existing != nil {
		return fmt.Errorf("user with email %q already exists", user.Email)
	}

	doc := userToMap(user)
	if err := s.engine.Insert("_users", user.ID, doc); err != nil {
		return fmt.Errorf("auth: create user: %w", err)
	}
	return nil
}

// GetByID retrieves a user by ID.
func (s *DBUserStore) GetByID(id string) (*User, error) {
	doc, err := s.engine.Get("_users", id)
	if err != nil {
		return nil, fmt.Errorf("auth: get user by ID: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("user %q not found", id)
	}
	return mapToUser(doc)
}

// GetByEmail retrieves a user by email.
func (s *DBUserStore) GetByEmail(email string) (*User, error) {
	docs, err := s.engine.Query("_users", map[string]interface{}{"email": email}, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: query user by email: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("user with email %q not found", email)
	}
	return mapToUser(docs[0])
}

// GetByVerificationToken retrieves a user by their email verification token.
func (s *DBUserStore) GetByVerificationToken(token string) (*User, error) {
	docs, err := s.engine.Query("_users", map[string]interface{}{"verification_token": token}, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: query user by verification token: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("user with verification token %q not found", token)
	}
	return mapToUser(docs[0])
}

// GetByResetToken retrieves a user by their password reset token.
func (s *DBUserStore) GetByResetToken(token string) (*User, error) {
	docs, err := s.engine.Query("_users", map[string]interface{}{"reset_token": token}, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: query user by reset token: %w", err)
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("user with reset token %q not found", token)
	}
	return mapToUser(docs[0])
}

// Update persists changes to an existing user.
func (s *DBUserStore) Update(user *User) error {
	// Check that the user exists first
	if _, err := s.GetByID(user.ID); err != nil {
		return err
	}

	doc := userToMap(user)
	if err := s.engine.Update("_users", user.ID, doc); err != nil {
		return fmt.Errorf("auth: update user: %w", err)
	}
	return nil
}

// Delete removes a user by ID.
func (s *DBUserStore) Delete(id string) error {
	// Check that the user exists first
	if _, err := s.GetByID(id); err != nil {
		return err
	}

	if err := s.engine.Delete("_users", id); err != nil {
		return fmt.Errorf("auth: delete user: %w", err)
	}
	return nil
}

// List returns all users.
func (s *DBUserStore) List() ([]*User, error) {
	docs, err := s.engine.List("_users")
	if err != nil {
		return nil, fmt.Errorf("auth: list users: %w", err)
	}

	var users []*User
	for _, doc := range docs {
		user, err := mapToUser(doc)
		if err != nil {
			continue
		}
		users = append(users, user)
	}
	return users, nil
}

// Helper functions to map User to/from map[string]interface{}
func userToMap(u *User) map[string]interface{} {
	m := map[string]interface{}{
		"_id":          u.ID,
		"email":        u.Email,
		"password_hash": u.PasswordHash,
		"role":         string(u.Role),
		"created_at":   u.CreatedAt.Format(time.RFC3339Nano),
		"updated_at":   u.UpdatedAt.Format(time.RFC3339Nano),
		"is_verified":  u.IsVerified,
	}
	if u.Name != "" {
		m["name"] = u.Name
	}
	if u.AvatarURL != "" {
		m["avatar_url"] = u.AvatarURL
	}
	if u.Provider != "" {
		m["provider"] = u.Provider
	}
	if u.ProviderID != "" {
		m["provider_id"] = u.ProviderID
	}
	if u.ProviderAccessToken != "" {
		m["provider_access_token"] = u.ProviderAccessToken
	}
	if u.ProviderRefreshToken != "" {
		m["provider_refresh_token"] = u.ProviderRefreshToken
	}
	if !u.ProviderTokenExpiry.IsZero() {
		m["provider_token_expiry"] = u.ProviderTokenExpiry.Format(time.RFC3339Nano)
	}
	if u.VerificationToken != "" {
		m["verification_token"] = u.VerificationToken
		m["verification_expires"] = u.VerificationExpires.Format(time.RFC3339Nano)
	}
	if u.ResetToken != "" {
		m["reset_token"] = u.ResetToken
		m["reset_expires"] = u.ResetExpires.Format(time.RFC3339Nano)
	}
	return m
}

func mapToUser(m map[string]interface{}) (*User, error) {
	id, _ := m["_id"].(string)
	email, _ := m["email"].(string)
	pwHash, _ := m["password_hash"].(string)
	roleStr, _ := m["role"].(string)
	isVerified, _ := m["is_verified"].(bool)
	verifiedToken, _ := m["verification_token"].(string)
	resetToken, _ := m["reset_token"].(string)
	name, _ := m["name"].(string)
	avatarURL, _ := m["avatar_url"].(string)
	provider, _ := m["provider"].(string)
	providerID, _ := m["provider_id"].(string)
	providerAccessToken, _ := m["provider_access_token"].(string)
	providerRefreshToken, _ := m["provider_refresh_token"].(string)

	var createdAt, updatedAt time.Time
	if caStr, ok := m["created_at"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339Nano, caStr)
	}
	if uaStr, ok := m["updated_at"].(string); ok {
		updatedAt, _ = time.Parse(time.RFC3339Nano, uaStr)
	}

	var verificationExpires time.Time
	if veStr, ok := m["verification_expires"].(string); ok && veStr != "" {
		verificationExpires, _ = time.Parse(time.RFC3339Nano, veStr)
	}

	var resetExpires time.Time
	if reStr, ok := m["reset_expires"].(string); ok && reStr != "" {
		resetExpires, _ = time.Parse(time.RFC3339Nano, reStr)
	}

	var providerTokenExpiry time.Time
	if pteStr, ok := m["provider_token_expiry"].(string); ok && pteStr != "" {
		providerTokenExpiry, _ = time.Parse(time.RFC3339Nano, pteStr)
	}

	return &User{
		ID:                   id,
		Email:                email,
		PasswordHash:         pwHash,
		Role:                 Role(roleStr),
		Name:                 name,
		AvatarURL:            avatarURL,
		Provider:             provider,
		ProviderID:           providerID,
		ProviderAccessToken:  providerAccessToken,
		ProviderRefreshToken: providerRefreshToken,
		ProviderTokenExpiry:  providerTokenExpiry,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		IsVerified:           isVerified,
		VerificationToken:    verifiedToken,
		VerificationExpires:  verificationExpires,
		ResetToken:           resetToken,
		ResetExpires:         resetExpires,
	}, nil
}
