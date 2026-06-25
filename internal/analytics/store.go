// Package analytics provides per-project event ingestion and aggregation.
// Events are stored as batches and aggregated hourly for dashboard display.
package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// Event is a user-submitted analytics event.
type Event struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	UserID     string                 `json:"user_id,omitempty"`
}

// Aggregate holds hourly aggregated stats for a single event name.
type Aggregate struct {
	Name    string    `json:"name"`
	Hour    time.Time `json:"hour"` // truncated to hour boundary
	Count   int64     `json:"count"`
	Updated time.Time `json:"updated"`
}

// Summary is returned by the stats endpoint.
type Summary struct {
	TotalEvents  int64            `json:"total_events"`
	TopEvents    []EventCount     `json:"top_events"`
	HourlyCounts []HourlyCount    `json:"hourly_counts"`
}

type EventCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type HourlyCount struct {
	Hour  string `json:"hour"` // RFC3339
	Count int64  `json:"count"`
}

// Store manages event ingestion and aggregation.
type Store struct {
	db     *pebble.DB
	mu     sync.Mutex
	buffer []Event
	logger *slog.Logger
	stopCh chan struct{}
}

const (
	aggPrefix    = "__analytics_agg__:"
	batchSize    = 100
	flushInterval = 5 * time.Second
)

func aggKey(name string, hour time.Time) []byte {
	return []byte(fmt.Sprintf("%s%s:%d", aggPrefix, name, hour.Truncate(time.Hour).Unix()))
}

// NewStore creates an analytics event store.
func NewStore(db *pebble.DB) *Store {
	return &Store{
		db:     db,
		buffer: make([]Event, 0, batchSize),
		logger: slog.Default().With("module", "analytics"),
		stopCh: make(chan struct{}),
	}
}

// Ingest records one or more analytics events.
func (s *Store) Ingest(events []Event) {
	s.mu.Lock()
	s.buffer = append(s.buffer, events...)
	shouldFlush := len(s.buffer) >= batchSize
	s.mu.Unlock()

	if shouldFlush {
		s.flush()
	}
}

// Start begins the periodic flush loop.
func (s *Store) Start(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flush()
		case <-s.stopCh:
			s.flush() // final flush
			return
		case <-ctx.Done():
			s.flush()
			return
		}
	}
}

// Stop ends the flush loop.
func (s *Store) Stop() {
	close(s.stopCh)
}

func (s *Store) flush() {
	s.mu.Lock()
	events := s.buffer
	s.buffer = nil
	s.mu.Unlock()

	if len(events) == 0 {
		return
	}

	// Aggregate by (name, hour).
	hourAgg := make(map[string]*Aggregate)
	for _, e := range events {
		hour := e.Timestamp.Truncate(time.Hour)
		key := aggKey(e.Name, hour)
		kstr := string(key)
		if a, ok := hourAgg[kstr]; ok {
			a.Count++
		} else {
			hourAgg[kstr] = &Aggregate{Name: e.Name, Hour: hour, Count: 1}
		}
	}

	// Read existing aggregates from Pebble and merge.
	batch := s.db.NewBatch()
	for kstr, a := range hourAgg {
		key := []byte(kstr)
		val, closer, err := s.db.Get(key)
		if err == nil {
			var existing Aggregate
			if json.Unmarshal(val, &existing) == nil {
				a.Count += existing.Count
			}
			closer.Close()
		}
		a.Updated = time.Now().UTC()
		data, _ := json.Marshal(a)
		_ = batch.Set(key, data, nil)
	}
	_ = batch.Commit(pebble.NoSync)
	batch.Close()
}

// Summary returns aggregate statistics for the last 24 hours.
func (s *Store) Summary() (*Summary, error) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour).Truncate(time.Hour)

	prefix := []byte(aggPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("analytics: iter: %w", err)
	}
	defer iter.Close()

	type nameCount struct {
		count int64
		hour  map[time.Time]int64
	}
	byName := make(map[string]*nameCount)
	var total int64

	for iter.First(); iter.Valid(); iter.Next() {
		var a Aggregate
		if err := json.Unmarshal(iter.Value(), &a); err != nil {
			continue
		}
		if a.Hour.Before(since) {
			continue
		}
		total += a.Count
		nc, ok := byName[a.Name]
		if !ok {
			nc = &nameCount{hour: make(map[time.Time]int64)}
			byName[a.Name] = nc
		}
		nc.count += a.Count
		nc.hour[a.Hour] += a.Count
	}

	// Top 10 events.
	var top []EventCount
	for name, nc := range byName {
		top = append(top, EventCount{Name: name, Count: nc.count})
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Count > top[j].Count })
	if len(top) > 10 {
		top = top[:10]
	}

	// Hourly totals.
	hourTotals := make(map[time.Time]int64)
	for _, nc := range byName {
		for h, c := range nc.hour {
			hourTotals[h] += c
		}
	}
	var hourly []HourlyCount
	for h, c := range hourTotals {
		hourly = append(hourly, HourlyCount{Hour: h.Format(time.RFC3339), Count: c})
	}
	sort.Slice(hourly, func(i, j int) bool { return hourly[i].Hour < hourly[j].Hour })

	return &Summary{
		TotalEvents:  total,
		TopEvents:    top,
		HourlyCounts: hourly,
	}, nil
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
