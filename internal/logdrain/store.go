// Package logdrain provides per-project log forwarding to external
// destinations (Datadog, Sentry, custom HTTP webhook). Drains are stored
// in Pebble and forwarded asynchronously.
package logdrain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// DrainType identifies the forwarding format.
type DrainType string

const (
	DrainHTTP    DrainType = "http"    // Generic HTTP POST (JSON array)
	DrainDatadog DrainType = "datadog" // Datadog Logs API
	DrainSentry  DrainType = "sentry"  // Sentry envelope API
)

// Drain represents a configured log forwarding destination.
type Drain struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      DrainType `json:"type"`
	URL       string    `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Filters   string    `json:"filters,omitempty"` // e.g. "status>=400"
	Enabled   bool      `json:"enabled"`
	BatchSize int       `json:"batch_size,omitempty"` // flush batch (default 50)
	CreatedAt time.Time `json:"created_at"`
}

// LogEntry is a single request log line forwarded to drains.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Duration  string `json:"duration"`
	Email     string `json:"email,omitempty"`
	IP        string `json:"ip,omitempty"`
}

// Store manages log drains in a project's Pebble DB.
type Store struct {
	db     *pebble.DB
	mu     sync.Mutex
	drains map[string]*Drain
	buffer map[string][]LogEntry // drain ID → pending entries
	logger *slog.Logger
	stopCh chan struct{}
}

const drainPrefix = "__logdrain__:"

func drainKey(id string) []byte {
	return []byte(drainPrefix + id)
}

// NewStore creates a log drain store and loads existing drains.
func NewStore(db *pebble.DB) *Store {
	s := &Store{
		db:     db,
		drains: make(map[string]*Drain),
		buffer: make(map[string][]LogEntry),
		logger: slog.Default().With("module", "logdrain"),
		stopCh: make(chan struct{}),
	}
	s.loadAll()
	return s
}

// Create adds a new drain.
func (s *Store) Create(d *Drain) (*Drain, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if d.Name == "" || d.URL == "" {
		return nil, fmt.Errorf("logdrain: name and url are required")
	}
	if d.ID == "" {
		d.ID = fmt.Sprintf("drain_%d", time.Now().UnixNano())
	}
	if d.BatchSize == 0 {
		d.BatchSize = 50
	}
	d.CreatedAt = time.Now().UTC()

	data, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("logdrain: marshal: %w", err)
	}
	if err := s.db.Set(drainKey(d.ID), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("logdrain: set: %w", err)
	}
	s.drains[d.ID] = d
	return d, nil
}

// Delete removes a drain.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.db.Delete(drainKey(id), pebble.Sync); err != nil {
		return fmt.Errorf("logdrain: delete: %w", err)
	}
	delete(s.drains, id)
	delete(s.buffer, id)
	return nil
}

// List returns all drains.
func (s *Store) List() []*Drain {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []*Drain
	for _, d := range s.drains {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Forward queues a log entry for forwarding to all enabled drains.
// Non-blocking — entries are buffered and flushed periodically.
func (s *Store) Forward(entry LogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, d := range s.drains {
		if !d.Enabled {
			continue
		}
		s.buffer[id] = append(s.buffer[id], entry)
		// Flush if batch size reached.
		if len(s.buffer[id]) >= d.BatchSize {
			entries := s.buffer[id]
			s.buffer[id] = nil
			go s.flush(d, entries)
		}
	}
}

// Start begins the periodic flush loop.
func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flushAll()
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop halts the flush loop.
func (s *Store) Stop() {
	close(s.stopCh)
}

func (s *Store) flushAll() {
	s.mu.Lock()
	pending := s.buffer
	s.buffer = make(map[string][]LogEntry)
	s.mu.Unlock()

	for id, entries := range pending {
		if len(entries) == 0 {
			continue
		}
		drain, ok := s.drains[id]
		if !ok || !drain.Enabled {
			continue
		}
		go s.flush(drain, entries)
	}
}

func (s *Store) flush(d *Drain, entries []LogEntry) {
	payload, err := json.Marshal(entries)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", d.URL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range d.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("X-Sovrabase-Drain-ID", d.ID)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Warn("logdrain forward failed", "drain", d.Name, "error", err)
		return
	}
	resp.Body.Close()
}

func (s *Store) loadAll() {
	prefix := []byte(drainPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		var d Drain
		if err := json.Unmarshal(iter.Value(), &d); err != nil {
			continue
		}
		s.drains[d.ID] = &d
	}
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
