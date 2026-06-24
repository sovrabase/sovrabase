package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// RealtimeClient manages a WebSocket connection for realtime subscriptions.
type RealtimeClient struct {
	client *Client
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	mu             sync.Mutex
	subscriptions  map[string]*Subscription
	handlers       map[string]func(*RealtimeEvent) // keyed by subscription ID
	nextSubID      int
}

// Subscription represents an active realtime subscription.
type Subscription struct {
	ID         string
	Collection string
	Filter     map[string]interface{} `json:"filter,omitempty"`
}

// RealtimeEvent is an event received over the WebSocket.
type RealtimeEvent struct {
	Type       string    `json:"type"`
	EventType  string    `json:"event_type"`
	Collection string    `json:"collection"`
	DocID      string    `json:"id"`
	Data       Document  `json:"data,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Realtime returns a new RealtimeClient for managing realtime subscriptions.
func (c *Client) Realtime() *RealtimeClient {
	return &RealtimeClient{
		client:        c,
		subscriptions: make(map[string]*Subscription),
		handlers:      make(map[string]func(*RealtimeEvent)),
	}
}

// Connect opens a WebSocket connection and starts the read loop.
func (rc *RealtimeClient) Connect(ctx context.Context) error {
	rc.ctx, rc.cancel = context.WithCancel(ctx)

	// Build the WebSocket URL.
	wsURL := rc.client.baseURL + "/realtime/v1/ws"
	// Replace http with ws.
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}

	conn, _, err := websocket.Dial(rc.ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("realtime connect: %w", err)
	}
	rc.conn = conn

	// Send auth message.
	rc.client.mu.Lock()
	authMsg := map[string]string{
		"token":       rc.client.accessToken,
		"project_key": rc.client.projectKey,
	}
	rc.client.mu.Unlock()

	if err := wsjson.Write(rc.ctx, conn, authMsg); err != nil {
		conn.Close(websocket.StatusInternalError, "auth failed")
		return fmt.Errorf("realtime auth: %w", err)
	}

	// Start read loop.
	rc.done = make(chan struct{})
	go rc.readLoop()

	return nil
}

// readLoop reads messages from the WebSocket and dispatches events.
func (rc *RealtimeClient) readLoop() {
	defer close(rc.done)
	defer rc.conn.Close(websocket.StatusNormalClosure, "")

	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		var raw json.RawMessage
		if err := wsjson.Read(rc.ctx, rc.conn, &raw); err != nil {
			return
		}

		// Determine message type.
		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}

		switch base.Type {
		case "subscribed":
			var msg struct {
				Type           string `json:"type"`
				SubscriptionID string `json:"subscription_id"`
				Collection     string `json:"collection"`
			}
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			rc.mu.Lock()
			if sub, ok := rc.subscriptions[msg.SubscriptionID]; ok {
				sub.Collection = msg.Collection
			}
			rc.mu.Unlock()

		case "event":
			var event RealtimeEvent
			if err := json.Unmarshal(raw, &event); err != nil {
				continue
			}
			// Dispatch to appropriate handlers.
			rc.mu.Lock()
			for subID, handler := range rc.handlers {
				// Check if this subscription matches the event's collection.
				if sub, ok := rc.subscriptions[subID]; ok && sub.Collection == event.Collection {
					go handler(&event)
				}
			}
			rc.mu.Unlock()
		}
	}
}

// Subscribe subscribes to realtime changes on a collection.
func (rc *RealtimeClient) Subscribe(collection string, handler func(event *RealtimeEvent)) (*Subscription, error) {
	if rc.conn == nil {
		return nil, fmt.Errorf("realtime: not connected")
	}

	msg := map[string]interface{}{
		"type":       "subscribe",
		"collection": collection,
	}

	if err := wsjson.Write(rc.ctx, rc.conn, msg); err != nil {
		return nil, fmt.Errorf("realtime subscribe: %w", err)
	}

	// Read the subscribed response.
	var resp struct {
		Type           string `json:"type"`
		SubscriptionID string `json:"subscription_id"`
		Collection     string `json:"collection"`
	}
	if err := wsjson.Read(rc.ctx, rc.conn, &resp); err != nil {
		return nil, fmt.Errorf("realtime subscribe response: %w", err)
	}

	sub := &Subscription{
		ID:         resp.SubscriptionID,
		Collection: resp.Collection,
	}

	rc.mu.Lock()
	rc.subscriptions[sub.ID] = sub
	rc.handlers[sub.ID] = handler
	rc.mu.Unlock()

	return sub, nil
}

// Unsubscribe removes a subscription.
func (rc *RealtimeClient) Unsubscribe(sub *Subscription) error {
	if rc.conn == nil {
		return fmt.Errorf("realtime: not connected")
	}

	msg := map[string]interface{}{
		"type":            "unsubscribe",
		"subscription_id": sub.ID,
	}

	if err := wsjson.Write(rc.ctx, rc.conn, msg); err != nil {
		return fmt.Errorf("realtime unsubscribe: %w", err)
	}

	rc.mu.Lock()
	delete(rc.subscriptions, sub.ID)
	delete(rc.handlers, sub.ID)
	rc.mu.Unlock()

	return nil
}

// Close closes the WebSocket connection.
func (rc *RealtimeClient) Close() error {
	if rc.cancel != nil {
		rc.cancel()
	}
	if rc.done != nil {
		<-rc.done
	}
	rc.mu.Lock()
	rc.subscriptions = make(map[string]*Subscription)
	rc.handlers = make(map[string]func(*RealtimeEvent))
	rc.mu.Unlock()
	return nil
}
