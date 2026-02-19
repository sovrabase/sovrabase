package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAuthServiceFlow(t *testing.T) {
	store := newMemUserStore()
	svc := mustAuthService(t, store)

	required, err := svc.GetConfigState(context.Background())
	if err != nil {
		t.Fatalf("GetConfigState() error = %v", err)
	}
	if !required {
		t.Fatalf("GetConfigState() = false, want true before bootstrap")
	}

	result, err := svc.BootstrapFirstAdmin(context.Background(), " Admin@Example.com ", "very-strong-password")
	if err != nil {
		t.Fatalf("BootstrapFirstAdmin() error = %v", err)
	}
	if result.AccessToken == "" {
		t.Fatalf("BootstrapFirstAdmin() token is empty")
	}
	if result.User.Email != "admin@example.com" {
		t.Fatalf("BootstrapFirstAdmin() email = %q, want %q", result.User.Email, "admin@example.com")
	}

	required, err = svc.GetConfigState(context.Background())
	if err != nil {
		t.Fatalf("GetConfigState() error = %v", err)
	}
	if required {
		t.Fatalf("GetConfigState() = true, want false after bootstrap")
	}

	_, err = svc.BootstrapFirstAdmin(context.Background(), "admin@example.com", "very-strong-password")
	if !errors.Is(err, ErrBootstrapAlreadyDone) {
		t.Fatalf("BootstrapFirstAdmin() second call error = %v, want ErrBootstrapAlreadyDone", err)
	}

	loginResult, err := svc.Login(context.Background(), "admin@example.com", "very-strong-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if loginResult.AccessToken == "" {
		t.Fatalf("Login() token is empty")
	}
}

func TestAuthServiceInvalidCredentials(t *testing.T) {
	store := newMemUserStore()
	svc := mustAuthService(t, store)

	if _, err := svc.Login(context.Background(), "admin@example.com", "very-strong-password"); !errors.Is(err, ErrBootstrapRequired) {
		t.Fatalf("Login() before bootstrap error = %v, want ErrBootstrapRequired", err)
	}

	_, err := svc.BootstrapFirstAdmin(context.Background(), "admin@example.com", "very-strong-password")
	if err != nil {
		t.Fatalf("BootstrapFirstAdmin() error = %v", err)
	}

	if _, err := svc.Login(context.Background(), "admin@example.com", "wrong-password-here"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() wrong password error = %v, want ErrInvalidCredentials", err)
	}

	if _, err := svc.Login(context.Background(), "missing@example.com", "very-strong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() missing user error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthServiceValidation(t *testing.T) {
	store := newMemUserStore()
	svc := mustAuthService(t, store)

	if _, err := svc.BootstrapFirstAdmin(context.Background(), "not-an-email", "very-strong-password"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("BootstrapFirstAdmin() invalid email error = %v, want ErrInvalidInput", err)
	}

	if _, err := svc.BootstrapFirstAdmin(context.Background(), "admin@example.com", "short"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("BootstrapFirstAdmin() short password error = %v, want ErrInvalidInput", err)
	}
}

func mustAuthService(t *testing.T, store UserStore) Service {
	t.Helper()
	svc, err := NewService(ServiceDeps{
		Store:     store,
		JWTSecret: "super-secret-jwt-key",
		TokenTTL:  24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

type memUserStore struct {
	mu       sync.Mutex
	rootUser *User
	users    map[string]User
}

func newMemUserStore() *memUserStore {
	return &memUserStore{
		users: make(map[string]User),
	}
}

func (m *memUserStore) BootstrapRequired(context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rootUser == nil, nil
}

func (m *memUserStore) CreateFirstAdmin(_ context.Context, email, passwordHash string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rootUser != nil {
		return User{}, ErrBootstrapAlreadyDone
	}
	now := time.Now().UTC()
	user := User{
		ID:           "user-1",
		Email:        email,
		PasswordHash: passwordHash,
		Role:         UserRoleAdmin,
		IsRoot:       true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.rootUser = &user
	m.users[email] = user
	return user, nil
}

func (m *memUserStore) GetByEmail(_ context.Context, email string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[email]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (m *memUserStore) TouchLastLogin(_ context.Context, userID string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for email, user := range m.users {
		if user.ID == userID {
			user.LastLoginAt = &at
			user.UpdatedAt = at
			m.users[email] = user
			if m.rootUser != nil && m.rootUser.ID == userID {
				updated := user
				m.rootUser = &updated
			}
			return nil
		}
	}
	return ErrUserNotFound
}
