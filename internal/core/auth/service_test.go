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
	mu         sync.Mutex
	rootUser   *User
	usersByID  map[string]User
	users      map[string]User
	roles      map[string]RoleRecord
	scopes     map[string]ScopeRecord
	userRoles  map[string]map[string]bool
	roleScopes map[string]map[string]bool
}

func newMemUserStore() *memUserStore {
	return &memUserStore{
		users:      make(map[string]User),
		usersByID:  make(map[string]User),
		roles:      make(map[string]RoleRecord),
		scopes:     make(map[string]ScopeRecord),
		userRoles:  make(map[string]map[string]bool),
		roleScopes: make(map[string]map[string]bool),
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
		AccountType:  AccountTypeAdmin,
		IsRoot:       true,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.rootUser = &user
	m.users[email] = user
	m.usersByID[user.ID] = user
	return user, nil
}

func (m *memUserStore) CreateUser(_ context.Context, email, passwordHash string, role UserRole, accountType AccountType, isRoot bool) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[email]; exists {
		return User{}, ErrConflict
	}
	now := time.Now().UTC()
	user := User{ID: "u-" + email, Email: email, PasswordHash: passwordHash, Role: role, AccountType: accountType, IsRoot: isRoot, IsActive: true, CreatedAt: now, UpdatedAt: now}
	m.users[email] = user
	m.usersByID[user.ID] = user
	return user, nil
}

func (m *memUserStore) ListUsers(context.Context) ([]User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]User, 0, len(m.usersByID))
	for _, user := range m.usersByID {
		out = append(out, user)
	}
	return out, nil
}

func (m *memUserStore) GetByID(_ context.Context, userID string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (m *memUserStore) UpdateUser(_ context.Context, userID string, updates UpdateUserStoreInput) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.usersByID[userID]
	if !ok {
		return User{}, ErrUserNotFound
	}
	if updates.Email != nil {
		delete(m.users, user.Email)
		user.Email = *updates.Email
	}
	if updates.PasswordHash != nil {
		user.PasswordHash = *updates.PasswordHash
	}
	if updates.Role != nil {
		user.Role = *updates.Role
	}
	if updates.AccountType != nil {
		user.AccountType = *updates.AccountType
	}
	if updates.IsActive != nil {
		user.IsActive = *updates.IsActive
	}
	user.UpdatedAt = time.Now().UTC()
	m.users[user.Email] = user
	m.usersByID[user.ID] = user
	return user, nil
}

func (m *memUserStore) DeleteUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	delete(m.usersByID, userID)
	delete(m.users, user.Email)
	return nil
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

func (m *memUserStore) CreateRole(_ context.Context, name, description string, parentRoleID *string) (RoleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role := RoleRecord{ID: "r-" + name, Name: name, Description: description, ParentRoleID: parentRoleID, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.roles[role.ID] = role
	return role, nil
}

func (m *memUserStore) ListRoles(context.Context) ([]RoleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]RoleRecord, 0, len(m.roles))
	for _, role := range m.roles {
		out = append(out, role)
	}
	return out, nil
}

func (m *memUserStore) GetRoleByID(_ context.Context, roleID string) (RoleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role, ok := m.roles[roleID]
	if !ok {
		return RoleRecord{}, ErrRoleNotFound
	}
	return role, nil
}

func (m *memUserStore) UpdateRole(_ context.Context, roleID string, updates UpdateRoleStoreInput) (RoleRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	role, ok := m.roles[roleID]
	if !ok {
		return RoleRecord{}, ErrRoleNotFound
	}
	if updates.Name != nil {
		role.Name = *updates.Name
	}
	if updates.Description != nil {
		role.Description = *updates.Description
	}
	if updates.ParentRoleID != nil {
		role.ParentRoleID = updates.ParentRoleID
	}
	role.UpdatedAt = time.Now().UTC()
	m.roles[roleID] = role
	return role, nil
}

func (m *memUserStore) DeleteRole(_ context.Context, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roles[roleID]; !ok {
		return ErrRoleNotFound
	}
	delete(m.roles, roleID)
	return nil
}

func (m *memUserStore) CreateScope(_ context.Context, key, description string) (ScopeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	scope := ScopeRecord{ID: "s-" + key, Key: key, Description: description, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	m.scopes[scope.ID] = scope
	return scope, nil
}

func (m *memUserStore) ListScopes(context.Context) ([]ScopeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ScopeRecord, 0, len(m.scopes))
	for _, scope := range m.scopes {
		out = append(out, scope)
	}
	return out, nil
}

func (m *memUserStore) GetScopeByID(_ context.Context, scopeID string) (ScopeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	scope, ok := m.scopes[scopeID]
	if !ok {
		return ScopeRecord{}, ErrScopeNotFound
	}
	return scope, nil
}

func (m *memUserStore) UpdateScope(_ context.Context, scopeID string, updates UpdateScopeStoreInput) (ScopeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	scope, ok := m.scopes[scopeID]
	if !ok {
		return ScopeRecord{}, ErrScopeNotFound
	}
	if updates.Key != nil {
		scope.Key = *updates.Key
	}
	if updates.Description != nil {
		scope.Description = *updates.Description
	}
	scope.UpdatedAt = time.Now().UTC()
	m.scopes[scopeID] = scope
	return scope, nil
}

func (m *memUserStore) DeleteScope(_ context.Context, scopeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.scopes[scopeID]; !ok {
		return ErrScopeNotFound
	}
	delete(m.scopes, scopeID)
	return nil
}

func (m *memUserStore) AssignRoleToUser(_ context.Context, userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.usersByID[userID]; !ok {
		return ErrUserNotFound
	}
	if _, ok := m.roles[roleID]; !ok {
		return ErrRoleNotFound
	}
	if m.userRoles[userID] == nil {
		m.userRoles[userID] = map[string]bool{}
	}
	m.userRoles[userID][roleID] = true
	return nil
}

func (m *memUserStore) RemoveRoleFromUser(_ context.Context, userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.userRoles[userID] != nil {
		delete(m.userRoles[userID], roleID)
	}
	return nil
}

func (m *memUserStore) AssignScopeToRole(_ context.Context, roleID, scopeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.roles[roleID]; !ok {
		return ErrRoleNotFound
	}
	if _, ok := m.scopes[scopeID]; !ok {
		return ErrScopeNotFound
	}
	if m.roleScopes[roleID] == nil {
		m.roleScopes[roleID] = map[string]bool{}
	}
	m.roleScopes[roleID][scopeID] = true
	return nil
}

func (m *memUserStore) RemoveScopeFromRole(_ context.Context, roleID, scopeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.roleScopes[roleID] != nil {
		delete(m.roleScopes[roleID], scopeID)
	}
	return nil
}

func (m *memUserStore) ResolveUserScopes(_ context.Context, userID string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	roleSet := m.userRoles[userID]
	out := make([]string, 0)
	seen := map[string]bool{}
	for roleID := range roleSet {
		for scopeID := range m.roleScopes[roleID] {
			scope, ok := m.scopes[scopeID]
			if ok && !seen[scope.Key] {
				seen[scope.Key] = true
				out = append(out, scope.Key)
			}
		}
	}
	return out, nil
}

func (m *memUserStore) RoleParentContains(_ context.Context, roleID, parentRoleID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := parentRoleID
	for current != "" {
		if current == roleID {
			return true, nil
		}
		role, ok := m.roles[current]
		if !ok || role.ParentRoleID == nil {
			return false, nil
		}
		current = *role.ParentRoleID
	}
	return false, nil
}
