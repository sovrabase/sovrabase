// Package configmaps provides per-project remote configuration key-value
// storage backed by Pebble. Each project engine gets its own namespace.
//
// Keys are strings (UTF-8, max 256 bytes). Values are typed JSON blobs with
// an optional description. Supports public (anonymous-readable) and private
// entries.
package configmaps

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

// ValueType describes the type of a config value.
type ValueType string

const (
	ValueString   ValueType = "string"
	ValueNumber   ValueType = "number"
	ValueBoolean  ValueType = "boolean"
	ValueJSON     ValueType = "json"
)

// Entry is a single remote-config key-value pair.
type Entry struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Type        ValueType   `json:"type"`
	Description string      `json:"description,omitempty"`
	Public      bool        `json:"public"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CreatedAt   time.Time   `json:"created_at"`
}

// Store provides CRUD operations for remote config entries.
// It wraps a *pebble.DB (typically a project's engine DB).
type Store struct {
	db *pebble.DB
}

// NewStore creates a configmaps store backed by the given Pebble DB.
func NewStore(db *pebble.DB) *Store {
	return &Store{db: db}
}

const cmPrefix = "__configmap__:"

func cmKey(key string) []byte {
	return []byte(cmPrefix + key)
}

// validateKey checks that a key is non-empty, <= 256 bytes, and contains
// only printable characters (no whitespace-only).
func validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("configmaps: key must not be empty")
	}
	if len(key) > 256 {
		return fmt.Errorf("configmaps: key must be at most 256 bytes")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("configmaps: key must not be whitespace-only")
	}
	return nil
}

// inferType determines the ValueType from a Go interface.
func inferType(v interface{}) ValueType {
	switch v.(type) {
	case bool:
		return ValueBoolean
	case float64, float32, int, int64, int32:
		return ValueNumber
	case string:
		return ValueString
	default:
		return ValueJSON
	}
}

// Get retrieves a single config entry by key.
func (s *Store) Get(key string) (*Entry, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	val, closer, err := s.db.Get(cmKey(key))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("configmaps: key %q not found", key)
	}
	if err != nil {
		return nil, fmt.Errorf("configmaps: get: %w", err)
	}
	defer closer.Close()

	var entry Entry
	if err := json.Unmarshal(val, &entry); err != nil {
		return nil, fmt.Errorf("configmaps: unmarshal: %w", err)
	}
	return &entry, nil
}

// Set creates or updates a config entry. If type is empty, it is inferred.
func (s *Store) Set(key string, value interface{}, valueType ValueType, description string, public bool) (*Entry, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}

	// Check if entry already exists (to preserve CreatedAt).
	now := time.Now().UTC()
	existing, err := s.Get(key)
	createdAt := now
	if err == nil && existing != nil {
		createdAt = existing.CreatedAt
	}

	if valueType == "" {
		valueType = inferType(value)
	}

	entry := &Entry{
		Key:         key,
		Value:       value,
		Type:        valueType,
		Description: description,
		Public:      public,
		UpdatedAt:   now,
		CreatedAt:   createdAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("configmaps: marshal: %w", err)
	}
	if err := s.db.Set(cmKey(key), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("configmaps: set: %w", err)
	}
	return entry, nil
}

// Delete removes a config entry.
func (s *Store) Delete(key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := s.db.Delete(cmKey(key), pebble.Sync); err != nil {
		return fmt.Errorf("configmaps: delete: %w", err)
	}
	return nil
}

// List returns all config entries sorted by key.
func (s *Store) List() ([]*Entry, error) {
	prefix := []byte(cmPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("configmaps: list iter: %w", err)
	}
	defer iter.Close()

	var entries []*Entry
	for iter.First(); iter.Valid(); iter.Next() {
		var entry Entry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue // skip malformed entries
		}
		entries = append(entries, &entry)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("configmaps: list iter error: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})
	return entries, nil
}

// ListPublic returns only entries marked as public (anonymous-readable).
func (s *Store) ListPublic() ([]*Entry, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var public []*Entry
	for _, e := range all {
		if e.Public {
			public = append(public, e)
		}
	}
	return public, nil
}

// prefixUpperBound returns the upper bound for iterating a prefix.
func prefixUpperBound(prefix []byte) []byte {
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
