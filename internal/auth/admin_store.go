package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// AdminRole represents an administrative permission level.
type AdminRole string

const (
	AdminRoleSuper   AdminRole = "super_admin"
	AdminRoleAdmin   AdminRole = "admin"
	AdminRoleSupport AdminRole = "support"
)

// AdminUser represents an administrative account.
type AdminUser struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"password_hash"`
	Role         AdminRole  `json:"role"`
	Name         string     `json:"name,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastLogin    *time.Time `json:"last_login,omitempty"`
}

// AdminStore manages administrative user accounts, backed by a Pebble database
// (typically the master database from ProjectManager).
type AdminStore struct {
	db *pebble.DB
}

// NewAdminStore creates an AdminStore backed by the given Pebble database.
// On first open, if no admin accounts exist, it auto-seeds a default admin
// from the provided config values.
func NewAdminStore(db *pebble.DB, adminEmail, adminPassword string) *AdminStore {
	s := &AdminStore{db: db}
	s.seedDefault(adminEmail, adminPassword)
	return s
}

// seedDefault creates the initial admin user if no admins exist yet.
func (as *AdminStore) seedDefault(email, password string) {
	count, err := as.Count()
	if err != nil || count > 0 {
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	admin := &AdminUser{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         AdminRoleSuper,
		Name:         "Default Admin",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	data, _ := json.Marshal(admin)
	_ = as.db.Set(adminUserKey(admin.ID), data, pebble.Sync)
	_ = as.db.Set(adminEmailKey(email), []byte(admin.ID), pebble.Sync)
}

// key helpers

func adminUserKey(id string) []byte {
	return []byte("admin_user:" + id)
}

func adminEmailKey(email string) []byte {
	return []byte("admin_email:" + email)
}

func adminUserPrefix() []byte {
	return []byte("admin_user:")
}

// Create registers a new admin user. Returns the created AdminUser.
func (as *AdminStore) Create(email, password, role, name string) (*AdminUser, error) {
	// Check for duplicate email
	_, err := as.GetByEmail(email)
	if err == nil {
		return nil, fmt.Errorf("admin: email %q already exists", email)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("admin: hash password: %w", err)
	}

	now := time.Now().UTC()
	admin := &AdminUser{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         AdminRole(role),
		Name:         name,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(admin)
	if err != nil {
		return nil, fmt.Errorf("admin: marshal: %w", err)
	}

	if err := as.db.Set(adminUserKey(admin.ID), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("admin: save: %w", err)
	}
	if err := as.db.Set(adminEmailKey(admin.Email), []byte(admin.ID), pebble.Sync); err != nil {
		return nil, fmt.Errorf("admin: save email index: %w", err)
	}

	return admin, nil
}

// GetByID retrieves an admin user by ID.
func (as *AdminStore) GetByID(id string) (*AdminUser, error) {
	val, closer, err := as.db.Get(adminUserKey(id))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("admin: user %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("admin: get by ID: %w", err)
	}
	defer closer.Close()

	var admin AdminUser
	if err := json.Unmarshal(val, &admin); err != nil {
		return nil, fmt.Errorf("admin: unmarshal: %w", err)
	}
	return &admin, nil
}

// GetByEmail retrieves an admin user by email.
func (as *AdminStore) GetByEmail(email string) (*AdminUser, error) {
	val, closer, err := as.db.Get(adminEmailKey(email))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("admin: email %q not found", email)
	}
	if err != nil {
		return nil, fmt.Errorf("admin: get by email: %w", err)
	}
	defer closer.Close()

	return as.GetByID(string(val))
}

// List returns all admin users.
func (as *AdminStore) List() ([]*AdminUser, error) {
	prefix := adminUserPrefix()
	iter, err := as.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("admin: list iter: %w", err)
	}
	defer iter.Close()

	var admins []*AdminUser
	for iter.First(); iter.Valid(); iter.Next() {
		var a AdminUser
		if err := json.Unmarshal(iter.Value(), &a); err != nil {
			continue
		}
		admins = append(admins, &a)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("admin: iterate: %w", err)
	}
	return admins, nil
}

// Update persists changes to an existing admin user.
func (as *AdminStore) Update(admin *AdminUser) error {
	existing, err := as.GetByID(admin.ID)
	if err != nil {
		return err
	}

	oldEmail := existing.Email

	admin.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(admin)
	if err != nil {
		return fmt.Errorf("admin: marshal update: %w", err)
	}

	if err := as.db.Set(adminUserKey(admin.ID), data, pebble.Sync); err != nil {
		return fmt.Errorf("admin: save update: %w", err)
	}

	// Update email index for the new email
	if err := as.db.Set(adminEmailKey(admin.Email), []byte(admin.ID), pebble.Sync); err != nil {
		return fmt.Errorf("admin: update email index: %w", err)
	}

	// If email changed, delete old email index
	if oldEmail != admin.Email {
		if err := as.db.Delete(adminEmailKey(oldEmail), pebble.Sync); err != nil {
			return fmt.Errorf("admin: delete old email index: %w", err)
		}
	}

	return nil
}

// Delete removes an admin user by ID.
func (as *AdminStore) Delete(id string) error {
	admin, err := as.GetByID(id)
	if err != nil {
		return err
	}

	if err := as.db.Delete(adminUserKey(id), pebble.Sync); err != nil {
		return fmt.Errorf("admin: delete: %w", err)
	}
	if err := as.db.Delete(adminEmailKey(admin.Email), pebble.Sync); err != nil {
		return fmt.Errorf("admin: delete email index: %w", err)
	}
	return nil
}

// Authenticate validates admin credentials. Returns the AdminUser on success.
func (as *AdminStore) Authenticate(email, password string) (*AdminUser, error) {
	admin, err := as.GetByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("admin: invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("admin: invalid email or password")
	}

	return admin, nil
}

// UpdateRole changes an admin's role.
func (as *AdminStore) UpdateRole(id string, newRole AdminRole) error {
	admin, err := as.GetByID(id)
	if err != nil {
		return err
	}
	admin.Role = newRole
	return as.Update(admin)
}

// UpdateLastLogin sets the LastLogin timestamp to now for the given admin.
func (as *AdminStore) UpdateLastLogin(id string) error {
	admin, err := as.GetByID(id)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	admin.LastLogin = &now
	return as.Update(admin)
}

// Count returns the number of admin users.
func (as *AdminStore) Count() (int, error) {
	prefix := adminUserPrefix()
	iter, err := as.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("admin: count iter: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	return count, iter.Error()
}

// HasPermission checks whether an admin has at least the required role level.
// Permission hierarchy: super_admin > admin > support.
func (as *AdminStore) HasPermission(adminID string, requiredRole AdminRole) bool {
	admin, err := as.GetByID(adminID)
	if err != nil {
		return false
	}
	return roleLevel(admin.Role) >= roleLevel(requiredRole)
}

// roleLevel assigns a numeric level to each role for comparison.
func roleLevel(r AdminRole) int {
	switch r {
	case AdminRoleSuper:
		return 3
	case AdminRoleAdmin:
		return 2
	case AdminRoleSupport:
		return 1
	default:
		return 0
	}
}

// keyUpperBound returns an exclusive upper bound key for prefix iteration.
func keyUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return append(prefix, 0x00)
}
