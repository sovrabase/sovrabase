package email

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
)

type EmailLogEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Subject   string    `json:"subject"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

type LogStore struct {
	db *pebble.DB
}

const elPrefix = "__email_log__:"

func NewLogStore(db *pebble.DB) *LogStore {
	return &LogStore{db: db}
}

func elKey(t time.Time, id string) []byte {
	// Reverse timestamp so iteration gives newest-first.
	rev := fmt.Sprintf("%020d", math.MaxInt64-t.UnixNano())
	return []byte(elPrefix + rev + ":" + id)
}

func (s *LogStore) Append(entry *EmailLogEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("emaillog: marshal: %w", err)
	}
	return s.db.Set(elKey(entry.Timestamp, entry.ID), data, pebble.Sync)
}

func (s *LogStore) List(offset, limit int) ([]*EmailLogEntry, int, error) {
	prefix := []byte(elPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("emaillog: list iter: %w", err)
	}
	defer iter.Close()

	var all []*EmailLogEntry
	for iter.Last(); iter.Valid(); iter.Prev() {
		var e EmailLogEntry
		if err := json.Unmarshal(iter.Value(), &e); err != nil {
			continue
		}
		all = append(all, &e)
	}
	total := len(all)
	if offset >= total {
		return []*EmailLogEntry{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (s *LogStore) Clear() error {
	prefix := []byte(elPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("emaillog: clear iter: %w", err)
	}
	defer iter.Close()

	var keys [][]byte
	for iter.First(); iter.Valid(); iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		keys = append(keys, key)
	}
	for _, k := range keys {
		if err := s.db.Delete(k, pebble.Sync); err != nil {
			return fmt.Errorf("emaillog: clear delete: %w", err)
		}
	}
	return nil
}

func (s *LogStore) Count() (int, error) {
	prefix := []byte(elPrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("emaillog: count: %w", err)
	}
	defer iter.Close()

	var count int
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	return count, nil
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
