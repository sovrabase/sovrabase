// Package emailtemplates provides per-project email template storage backed
// by Pebble. Templates support {{.Token}}, {{.Email}}, {{.URL}} placeholders.
package emailtemplates

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/pebble"
)

// TemplateType identifies the kind of email template.
type TemplateType string

const (
	TemplateEmailVerification TemplateType = "email_verification"
	TemplatePasswordReset     TemplateType = "password_reset"
	TemplateMagicLink         TemplateType = "magic_link"
	TemplateWelcome           TemplateType = "welcome"
	TemplateInvitation        TemplateType = "invitation"
)

// DefaultTemplates are the built-in fallback templates.
var DefaultTemplates = map[TemplateType]string{
	TemplateEmailVerification: "Hello,\n\nPlease verify your email by clicking the link below:\n{{.URL}}\n\nThis link expires in 24 hours.\n\n— Sovrabase",
	TemplatePasswordReset:     "Hello,\n\nYou requested a password reset. Click the link below:\n{{.URL}}\n\nIf you didn't request this, ignore this email.\n\n— Sovrabase",
	TemplateMagicLink:         "Hello,\n\nClick the link below to sign in:\n{{.URL}}\n\nThis link expires in 15 minutes.\n\n— Sovrabase",
	TemplateWelcome:           "Welcome to our app!\n\nYour account ({{.Email}}) is ready.\n\n— Sovrabase",
	TemplateInvitation:        "Hello,\n\nYou have been invited to join the project \"{{.ProjectName}}\" on Sovrabase.\n\nClick the link below to accept the invitation:\n{{.URL}}\n\nThis invitation expires in 7 days.\n\n— Sovrabase",
}

// Template is a stored email template.
type Template struct {
	Type      TemplateType `json:"type"`
	Subject   string       `json:"subject"`
	Body      string       `json:"body"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// Store manages email templates in a project's Pebble DB.
type Store struct {
	db *pebble.DB
}

const etPrefix = "__email_template__:"

func etKey(t TemplateType) []byte {
	return []byte(etPrefix + string(t))
}

// NewStore creates an email template store.
func NewStore(db *pebble.DB) *Store {
	return &Store{db: db}
}

// Get retrieves a template, falling back to defaults if not stored.
func (s *Store) Get(t TemplateType) (*Template, error) {
	val, closer, err := s.db.Get(etKey(t))
	if err == pebble.ErrNotFound {
		// Return default.
		body, ok := DefaultTemplates[t]
		if !ok {
			return nil, fmt.Errorf("emailtemplates: unknown template type %q", t)
		}
		return &Template{Type: t, Subject: string(t), Body: body}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("emailtemplates: get: %w", err)
	}
	defer closer.Close()
	var tmpl Template
	if err := json.Unmarshal(val, &tmpl); err != nil {
		return nil, fmt.Errorf("emailtemplates: unmarshal: %w", err)
	}
	return &tmpl, nil
}

// Set creates or updates a template.
func (s *Store) Set(t *Template) (*Template, error) {
	t.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("emailtemplates: marshal: %w", err)
	}
	if err := s.db.Set(etKey(t.Type), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("emailtemplates: set: %w", err)
	}
	return t, nil
}

// Reset restores a template to its default.
func (s *Store) Reset(t TemplateType) error {
	return s.db.Delete(etKey(t), pebble.Sync)
}

// List returns all custom templates sorted by type.
func (s *Store) List() ([]*Template, error) {
	prefix := []byte(etPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("emailtemplates: list iter: %w", err)
	}
	defer iter.Close()

	var templates []*Template
	for iter.First(); iter.Valid(); iter.Next() {
		var tmpl Template
		if err := json.Unmarshal(iter.Value(), &tmpl); err != nil {
			continue
		}
		templates = append(templates, &tmpl)
	}
	// Also include defaults that aren't overridden.
	stored := make(map[TemplateType]bool)
	for _, t := range templates {
		stored[t.Type] = true
	}
	for t, body := range DefaultTemplates {
		if !stored[t] {
			templates = append(templates, &Template{Type: t, Subject: string(t), Body: body})
		}
	}
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Type < templates[j].Type
	})
	return templates, nil
}

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
