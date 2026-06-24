package auth

import (
	"fmt"
	"sync"
)

// UserStore defines the persistence interface for user accounts.
type UserStore interface {
	Create(user *User) error
	GetByID(id string) (*User, error)
	GetByEmail(email string) (*User, error)
	Update(user *User) error
	Delete(id string) error
	List() ([]*User, error)
}

// InMemoryUserStore is a thread-safe in-memory implementation of UserStore for MVP use.
type InMemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*User // keyed by ID
	byEmail map[string]string // email → ID
}

// NewInMemoryUserStore creates a ready-to-use InMemoryUserStore.
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{
		users:   make(map[string]*User),
		byEmail: make(map[string]string),
	}
}

// Create inserts a new user. Returns an error if the email already exists.
// The user is stored by value (copied) to prevent external mutation.
func (s *InMemoryUserStore) Create(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byEmail[user.Email]; exists {
		return fmt.Errorf("user with email %q already exists", user.Email)
	}

	// Store a copy to prevent external mutations from corrupting the index.
	u := *user
	s.users[user.ID] = &u
	s.byEmail[user.Email] = user.ID
	return nil
}

// GetByID returns the user with the given ID, or an error if not found.
func (s *InMemoryUserStore) GetByID(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user %q not found", id)
	}
	return user, nil
}

// GetByEmail returns the user with the given email, or an error if not found.
func (s *InMemoryUserStore) GetByEmail(email string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, exists := s.byEmail[email]
	if !exists {
		return nil, fmt.Errorf("user with email %q not found", email)
	}

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user %q not found (inconsistent state)", id)
	}
	return user, nil
}

// Update persists changes to an existing user.
// The user is stored by value (copied) to prevent external mutation.
func (s *InMemoryUserStore) Update(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.users[user.ID]
	if !exists {
		return fmt.Errorf("user %q not found", user.ID)
	}

	// If email changed, update the email index
	if existing.Email != user.Email {
		delete(s.byEmail, existing.Email)
		s.byEmail[user.Email] = user.ID
	}

	// Store a copy to prevent external mutations from corrupting the index.
	u := *user
	s.users[user.ID] = &u
	return nil
}

// Delete removes a user by ID.
func (s *InMemoryUserStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return fmt.Errorf("user %q not found", id)
	}

	delete(s.byEmail, user.Email)
	delete(s.users, id)
	return nil
}

// List returns a list of all users in the store.
func (s *InMemoryUserStore) List() ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*User
	for _, u := range s.users {
		copied := *u
		list = append(list, &copied)
	}
	return list, nil
}
