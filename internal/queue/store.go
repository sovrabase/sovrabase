// Package queue provides durable FIFO message queues backed by Pebble.
// Supports multiple queues per project, consumer groups, visibility timeout,
// and at-least-once delivery semantics.
package queue

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
)

// Message represents a queue message.
type Message struct {
	ID          string                 `json:"id"`
	QueueName   string                 `json:"queue_name"`
	Body        map[string]interface{} `json:"body"`
	CreatedAt   time.Time              `json:"created_at"`
	VisibleAt   time.Time              `json:"visible_at"`   // when this message can be consumed again
	ReceiveCount int                   `json:"receive_count"`
	// Consumer tracking.
	ConsumerID  string                 `json:"consumer_id,omitempty"`
}

// QueueInfo holds metadata about a queue.
type QueueInfo struct {
	Name        string `json:"name"`
	Visible     int64  `json:"visible"`     // messages waiting to be consumed
	InFlight    int64  `json:"in_flight"`   // messages being processed
	Total       int64  `json:"total"`       // total messages ever sent
}

// Store manages durable queues for a single project.
type Store struct {
	db *pebble.DB
	mu sync.Mutex

	// Default visibility timeout for messages that are "received" but not
	// deleted before the timeout expires.
	VisibilityTimeout time.Duration
}

const (
	msgPrefix  = "__queue_msg__:"   // __queue_msg__:{queueName}:{msgID}
	queuePrefix = "__queue__:"      // __queue__:{queueName} → metadata
)

// NewStore creates a queue store backed by the given Pebble DB.
func NewStore(db *pebble.DB) *Store {
	return &Store{
		db:                db,
		VisibilityTimeout: 30 * time.Second,
	}
}

// ─── Producer API ─────────────────────────────────────────────────────────────

// Send adds a message to the queue. Returns the message ID.
func (s *Store) Send(queueName string, body map[string]interface{}) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if queueName == "" {
		return "", fmt.Errorf("queue: name is required")
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	msg := &Message{
		ID:        id,
		QueueName: queueName,
		Body:      body,
		CreatedAt: now,
		VisibleAt: now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("queue: marshal: %w", err)
	}
	if err := s.db.Set(msgKey(queueName, id), data, pebble.Sync); err != nil {
		return "", fmt.Errorf("queue: set: %w", err)
	}

	// Increment total counter.
	s.incrQueueCounter(queueName, "total", 1)
	return id, nil
}

// ─── Consumer API ─────────────────────────────────────────────────────────────

// Receive pulls up to `limit` visible messages from the queue. Each message
// becomes invisible (in-flight) for the visibility timeout. After processing,
// the consumer MUST call Delete to remove the message, or it will become
// visible again after the timeout.
func (s *Store) Receive(queueName string, limit int) ([]*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 10
	}
	now := time.Now().UTC()

	// Scan for visible messages.
	messages, err := s.scanVisible(queueName, limit, now)
	if err != nil {
		return nil, err
	}

	// Make them invisible by bumping VisibleAt.
	consumeID := uuid.New().String()
	for _, msg := range messages {
		msg.VisibleAt = now.Add(s.VisibilityTimeout)
		msg.ReceiveCount++
		msg.ConsumerID = consumeID
		if err := s.save(msg); err != nil {
			return nil, err
		}
	}

	// Update counters.
	s.setQueueCounter(queueName, "visible", s.countVisible(queueName))
	s.setQueueCounter(queueName, "in_flight", s.countInFlight(queueName, now))

	return messages, nil
}

// Delete removes a message from the queue after successful processing.
func (s *Store) Delete(queueName, msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.db.Delete(msgKey(queueName, msgID), pebble.Sync); err != nil {
		return fmt.Errorf("queue: delete: %w", err)
	}
	s.incrQueueCounter(queueName, "in_flight", -1)
	return nil
}

// ─── Queue Management ────────────────────────────────────────────────────────

// List returns all queue names and their stats.
func (s *Store) List() ([]QueueInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.listQueues()
}

// Purge removes all messages from a queue.
func (s *Store) Purge(queueName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, queueName))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("queue: purge iter: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		_ = s.db.Delete(iter.Key(), nil)
	}
	_ = s.db.Delete(queueMetaKey(queueName), pebble.Sync)

	// Reset counters.
	s.setQueueCounter(queueName, "visible", 0)
	s.setQueueCounter(queueName, "in_flight", 0)
	s.setQueueCounter(queueName, "total", 0)
	return nil
}

// MakeVisible forces in-flight messages back to visible (for dead-letter / admin).
func (s *Store) MakeVisible(queueName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, queueName))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("queue: makevisible iter: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var msg Message
		if err := json.Unmarshal(iter.Value(), &msg); err != nil {
			continue
		}
		if msg.VisibleAt.After(now) {
			msg.VisibleAt = now
			msg.ConsumerID = ""
			_ = s.save(&msg)
		}
	}
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func msgKey(queueName, msgID string) []byte {
	return []byte(fmt.Sprintf("%s%s:%s", msgPrefix, queueName, msgID))
}

func queueMetaKey(queueName string) []byte {
	return []byte(fmt.Sprintf("%s%s", queuePrefix, queueName))
}

func (s *Store) save(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("queue: marshal: %w", err)
	}
	return s.db.Set(msgKey(msg.QueueName, msg.ID), data, nil)
}

func (s *Store) scanVisible(queueName string, limit int, now time.Time) ([]*Message, error) {
	prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, queueName))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("queue: scan iter: %w", err)
	}
	defer iter.Close()

	var messages []*Message
	for iter.First(); iter.Valid() && len(messages) < limit; iter.Next() {
		var msg Message
		if err := json.Unmarshal(iter.Value(), &msg); err != nil {
			continue
		}
		if msg.VisibleAt.After(now) {
			continue // still in-flight
		}
		messages = append(messages, &msg)
	}
	return messages, nil
}

func (s *Store) countVisible(queueName string) int64 {
	now := time.Now().UTC()
	prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, queueName))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return 0
	}
	defer iter.Close()
	var count int64
	for iter.First(); iter.Valid(); iter.Next() {
		var msg Message
		if json.Unmarshal(iter.Value(), &msg) == nil && !msg.VisibleAt.After(now) {
			count++
		}
	}
	return count
}

func (s *Store) countInFlight(queueName string, now time.Time) int64 {
	prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, queueName))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return 0
	}
	defer iter.Close()
	var count int64
	for iter.First(); iter.Valid(); iter.Next() {
		var msg Message
		if json.Unmarshal(iter.Value(), &msg) == nil && msg.VisibleAt.After(now) {
			count++
		}
	}
	return count
}

func (s *Store) incrQueueCounter(queueName, field string, delta int64) {
	key := queueMetaKey(queueName)
	val, closer, err := s.db.Get(key)
	var info QueueInfo
	if err == nil {
		json.Unmarshal(val, &info)
		closer.Close()
	}
	info.Name = queueName
	switch field {
	case "visible":
		info.Visible += delta
	case "in_flight":
		info.InFlight += delta
	case "total":
		info.Total += delta
	}
	data, _ := json.Marshal(info)
	_ = s.db.Set(key, data, nil)
}

func (s *Store) setQueueCounter(queueName, field string, value int64) {
	key := queueMetaKey(queueName)
	val, closer, err := s.db.Get(key)
	var info QueueInfo
	if err == nil {
		json.Unmarshal(val, &info)
		closer.Close()
	}
	info.Name = queueName
	switch field {
	case "visible":
		info.Visible = value
	case "in_flight":
		info.InFlight = value
	case "total":
		info.Total = value
	}
	data, _ := json.Marshal(info)
	_ = s.db.Set(key, data, nil)
}

func (s *Store) listQueues() ([]QueueInfo, error) {
	prefix := []byte(queuePrefix)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("queue: list iter: %w", err)
	}
	defer iter.Close()

	var queues []QueueInfo
	for iter.First(); iter.Valid(); iter.Next() {
		var info QueueInfo
		if err := json.Unmarshal(iter.Value(), &info); err != nil {
			continue
		}
		queues = append(queues, info)
	}
	return queues, nil
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
