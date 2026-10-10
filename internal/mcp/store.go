package mcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// Scopes a token can hold. A token with no scopes gets ScopeRead.
const (
	ScopeRead      = "read"
	ScopeWrite     = "write"
	ScopeSchema    = "schema"
	ScopeFunctions = "functions"
	ScopeLogs      = "logs"
)

// AllScopes lists every valid scope, in display order.
var AllScopes = []string{ScopeRead, ScopeWrite, ScopeSchema, ScopeFunctions, ScopeLogs}

// TokenPrefix starts every MCP access token.
const TokenPrefix = "sbmcp_"

// displayPrefixLen is how many leading characters of a token are kept
// in clear so owners can recognise it in the dashboard.
const displayPrefixLen = 12

// ErrTokenNotFound is returned when a token ID does not exist.
var ErrTokenNotFound = errors.New("mcp: token not found")

// Config is the per-project MCP configuration.
type Config struct {
	Enabled       bool     `json:"enabled"`
	DisabledTools []string `json:"disabled_tools"`
}

// Token is a stored MCP access token. Only the SHA-256 hash of the secret
// is persisted; the plaintext is returned once, at creation.
type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Hash       string     `json:"hash"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// HasScope reports whether the token grants scope.
func (t *Token) HasScope(scope string) bool {
	for _, s := range t.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// Store persists MCP config and tokens in a project's Pebble DB, following
// the same "__name__:" key-prefix convention as logdrain and configmaps.
type Store struct {
	db     *pebble.DB
	mu     sync.RWMutex
	config Config
	tokens map[string]*Token // by ID
	// lastPersistedUse throttles writes of LastUsedAt.
	lastPersistedUse map[string]time.Time
}

const (
	configKey   = "__mcp__:config"
	tokenPrefix = "__mcp__:token:"
)

// NewStore loads the MCP state from db.
func NewStore(db *pebble.DB) (*Store, error) {
	s := &Store{
		db:               db,
		tokens:           make(map[string]*Token),
		lastPersistedUse: make(map[string]time.Time),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	val, closer, err := s.db.Get([]byte(configKey))
	switch {
	case err == nil:
		uerr := json.Unmarshal(val, &s.config)
		closer.Close()
		if uerr != nil {
			return fmt.Errorf("mcp: decode config: %w", uerr)
		}
	case errors.Is(err, pebble.ErrNotFound):
	default:
		return fmt.Errorf("mcp: read config: %w", err)
	}

	lower := []byte(tokenPrefix)
	upper := append([]byte(tokenPrefix[:len(tokenPrefix)-1]), tokenPrefix[len(tokenPrefix)-1]+1)
	iter, err := s.db.NewIter(&pebble.IterOptions{LowerBound: lower, UpperBound: upper})
	if err != nil {
		return fmt.Errorf("mcp: iterate tokens: %w", err)
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		var t Token
		if err := json.Unmarshal(iter.Value(), &t); err != nil {
			continue
		}
		s.tokens[t.ID] = &t
	}
	return nil
}

// Config returns a copy of the current configuration.
func (s *Store) Config() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.config
	c.DisabledTools = append([]string{}, s.config.DisabledTools...)
	return c
}

// SetConfig persists a new configuration.
func (s *Store) SetConfig(c Config) error {
	if c.DisabledTools == nil {
		c.DisabledTools = []string{}
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.Set([]byte(configKey), data, pebble.Sync); err != nil {
		return fmt.Errorf("mcp: save config: %w", err)
	}
	s.config = c
	return nil
}

// NormalizeScopes validates and de-duplicates scopes. Empty means read-only.
func NormalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return []string{ScopeRead}, nil
	}
	seen := make(map[string]bool)
	for _, sc := range scopes {
		sc = strings.ToLower(strings.TrimSpace(sc))
		valid := false
		for _, v := range AllScopes {
			if v == sc {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("mcp: unknown scope %q (valid: %s)", sc, strings.Join(AllScopes, ", "))
		}
		seen[sc] = true
	}
	out := make([]string, 0, len(seen))
	for _, v := range AllScopes {
		if seen[v] {
			out = append(out, v)
		}
	}
	return out, nil
}

// HashToken returns the hex SHA-256 of a plaintext token.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateToken mints a new token and returns the stored record plus the
// plaintext secret (which is never stored).
func (s *Store) CreateToken(name string, scopes []string) (*Token, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", fmt.Errorf("mcp: token name is required")
	}
	if len(name) > 100 {
		return nil, "", fmt.Errorf("mcp: token name must be at most 100 characters")
	}
	norm, err := NormalizeScopes(scopes)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomString(32)
	if err != nil {
		return nil, "", fmt.Errorf("mcp: generate token: %w", err)
	}
	idPart, err := randomString(9)
	if err != nil {
		return nil, "", fmt.Errorf("mcp: generate id: %w", err)
	}
	plain := TokenPrefix + secret
	t := &Token{
		ID:        "mcpt_" + idPart,
		Name:      name,
		Prefix:    plain[:displayPrefixLen],
		Hash:      HashToken(plain),
		Scopes:    norm,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.saveToken(t); err != nil {
		return nil, "", err
	}
	s.mu.Lock()
	s.tokens[t.ID] = t
	s.mu.Unlock()
	cp := *t
	return &cp, plain, nil
}

func (s *Store) saveToken(t *Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := s.db.Set([]byte(tokenPrefix+t.ID), data, pebble.Sync); err != nil {
		return fmt.Errorf("mcp: save token: %w", err)
	}
	return nil
}

// DeleteToken revokes a token. It returns the deleted record.
func (s *Store) DeleteToken(id string) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[id]
	if !ok {
		return nil, ErrTokenNotFound
	}
	if err := s.db.Delete([]byte(tokenPrefix+id), pebble.Sync); err != nil {
		return nil, fmt.Errorf("mcp: delete token: %w", err)
	}
	delete(s.tokens, id)
	delete(s.lastPersistedUse, id)
	return t, nil
}

// ListTokens returns all tokens, newest first.
func (s *Store) ListTokens() []Token {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Token, 0, len(s.tokens))
	for _, t := range s.tokens {
		cp := *t
		cp.Scopes = append([]string{}, t.Scopes...)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// TokenByHash finds a token by the hash of its secret.
func (s *Store) TokenByHash(hash string) (*Token, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tokens {
		if t.Hash == hash {
			cp := *t
			cp.Scopes = append([]string{}, t.Scopes...)
			return &cp, true
		}
	}
	return nil, false
}

// Touch records a token use. The timestamp is updated in memory on every
// call and persisted at most once a minute.
func (s *Store) Touch(id string, at time.Time) {
	s.mu.Lock()
	t, ok := s.tokens[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	at = at.UTC()
	t.LastUsedAt = &at
	persist := at.Sub(s.lastPersistedUse[id]) >= time.Minute
	var cp Token
	if persist {
		s.lastPersistedUse[id] = at
		cp = *t
	}
	s.mu.Unlock()
	if persist {
		_ = s.saveToken(&cp)
	}
}
