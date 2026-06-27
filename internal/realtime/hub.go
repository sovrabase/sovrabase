package realtime

import (
	"encoding/json"
	"log/slog"
	"reflect"
	"sync"
	"time"
)

// EventType describes the kind of data change.
type EventType string

const (
	EventInsert EventType = "insert"
	EventUpdate EventType = "update"
	EventDelete EventType = "delete"
)

// Event represents a data mutation event.
type Event struct {
	Type       EventType              `json:"event_type"`
	Collection string                 `json:"collection"`
	DocID      string                 `json:"id"`
	Data       map[string]interface{} `json:"data,omitempty"`
	ProjectID  string                 `json:"-"`
	Timestamp  time.Time              `json:"timestamp"`
}

// Subscription represents a client's interest in a collection.
type Subscription struct {
	ID           string
	ClientID     string
	ProjectID    string
	Collection   string
	Filter       map[string]interface{} // nil = no filter
	FilterFields []string               // pre-extracted filter keys for quick check
	cancel       func()
}

// broadcastPayload is a pre-marshaled event sent through the client channel.
type broadcastPayload struct {
	data []byte
}

// Client represents a single WebSocket connection.
type Client struct {
	ID            string
	ProjectID     string
	subscriptions map[string]*Subscription
	mu            sync.Mutex
	events        chan *broadcastPayload
	closed        bool
}

func newClient(id, projectID string) *Client {
	return &Client{
		ID:            id,
		ProjectID:     projectID,
		subscriptions: make(map[string]*Subscription),
		events:        make(chan *broadcastPayload, 256),
	}
}

// Hub manages realtime subscriptions and event broadcasting.
type Hub struct {
	mu           sync.RWMutex
	clients      map[string]*Client
	eventChan    chan *Event
	logger       *slog.Logger
	shutdown     chan struct{}
	wg           sync.WaitGroup
}

// NewHub creates a new Hub with a buffered event channel.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		eventChan: make(chan *Event, 1024),
		logger:    slog.Default().With("module", "realtime"),
		shutdown:  make(chan struct{}),
	}
}

// Start launches the event broadcast loop. Blocks until ctx is cancelled.
func (h *Hub) Start() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		for {
			select {
			case event := <-h.eventChan:
				h.broadcast(event)
			case <-h.shutdown:
				return
			}
		}
	}()
}

// Stop gracefully shuts down the hub.
func (h *Hub) Stop() {
	close(h.shutdown)
	h.wg.Wait()
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.ID] = client
	h.logger.Info("realtime client registered", "client_id", client.ID, "project_id", client.ProjectID)
}

// Unregister removes a client and its subscriptions.
// Closes the client's event channel so writeLoop exits cleanly.
func (h *Hub) Unregister(clientID string) {
	h.mu.Lock()
	client, exists := h.clients[clientID]
	if exists {
		delete(h.clients, clientID)
	}
	h.mu.Unlock()

	if exists {
		client.mu.Lock()
		client.closed = true
		close(client.events)
		client.mu.Unlock()
		h.logger.Info("realtime client unregistered", "client_id", clientID)
	}
}

// Subscribe creates a new subscription for a client.
func (h *Hub) Subscribe(clientID, projectID, collection string, filter map[string]interface{}) (*Subscription, error) {
	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	sub := &Subscription{
		ID:         clientID + ":" + collection,
		ClientID:   clientID,
		ProjectID:  projectID,
		Collection: collection,
		Filter:     filter,
	}

	client.mu.Lock()
	client.subscriptions[sub.ID] = sub
	client.mu.Unlock()

	return sub, nil
}

// Unsubscribe removes a subscription.
func (h *Hub) Unsubscribe(clientID, subscriptionID string) {
	h.mu.RLock()
	client, exists := h.clients[clientID]
	h.mu.RUnlock()

	if !exists {
		return
	}

	client.mu.Lock()
	delete(client.subscriptions, subscriptionID)
	client.mu.Unlock()
}

// Publish sends an event to the broadcast channel. Non-blocking; drops if full.
func (h *Hub) Publish(event *Event) {
	select {
	case h.eventChan <- event:
	default:
		h.logger.Warn("realtime event channel full, dropping event",
			"event_type", event.Type,
			"collection", event.Collection,
		)
	}
}

// broadcast dispatches an event to all matching client subscriptions.
// Pre-marshals the event to JSON once instead of per-client.
func (h *Hub) broadcast(event *Event) {
	// Pre-marshal once — avoids N redundant JSON encodes.
	data, _ := json.Marshal(map[string]interface{}{
		"type":       "event",
		"event_type": event.Type,
		"collection": event.Collection,
		"id":         event.DocID,
		"data":       event.Data,
		"timestamp":  event.Timestamp,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, client := range h.clients {
		if client.ProjectID != event.ProjectID {
			continue
		}

		client.mu.Lock()
		if client.closed {
			client.mu.Unlock()
			continue
		}
		matched := false
		for _, sub := range client.subscriptions {
			if sub.Collection != event.Collection {
				continue
			}
			if sub.Filter != nil && !eventMatchesFilter(event, sub.Filter) {
				continue
			}
			matched = true
			break
		}
		if matched {
			select {
			case client.events <- &broadcastPayload{data: data}:
			default:
			}
		}
		client.mu.Unlock()
	}
}

// eventMatchesFilter checks if event data matches a simple equality filter.
func eventMatchesFilter(event *Event, filter map[string]interface{}) bool {
	if event.Data == nil {
		return false
	}
	for k, want := range filter {
		got, ok := event.Data[k]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(got, want) {
			return false
		}
	}
	return true
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SubscriptionCount returns the number of active subscriptions.
func (h *Hub) SubscriptionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, c := range h.clients {
		count += len(c.subscriptions)
	}
	return count
}
