package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/auth"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSHandler handles WebSocket upgrade and realtime subscription lifecycle.
type WSHandler struct {
	hub       *Hub
	jwtSecret string
	logger    *slog.Logger
}

// NewWSHandler creates a new WebSocket handler for realtime subscriptions.
func NewWSHandler(hub *Hub, jwtSecret string) *WSHandler {
	return &WSHandler{
		hub:       hub,
		jwtSecret: jwtSecret,
		logger:    slog.Default().With("module", "realtime-ws"),
	}
}

// authMessage is the first JSON message the client sends after WebSocket upgrade.
type authMessage struct {
	Token      string `json:"token"`
	ProjectKey string `json:"project_key"`
}

// subscribeMsg is sent by the client to subscribe to a collection.
type subscribeMsg struct {
	Type       string                 `json:"type"`
	Collection string                 `json:"collection"`
	Filter     map[string]interface{} `json:"filter,omitempty"`
}

// unsubscribeMsg is sent by the client to unsubscribe.
type unsubscribeMsg struct {
	Type           string `json:"type"`
	SubscriptionID string `json:"subscription_id"`
}

// ServeHTTP upgrades the connection and manages the client lifecycle.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "error", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read auth message.
	var am authMessage
	ctx := r.Context()
	if err := wsjson.Read(ctx, conn, &am); err != nil {
		h.logger.Warn("websocket auth read failed", "error", err)
		conn.Close(websocket.StatusPolicyViolation, "auth required")
		return
	}

	if am.Token == "" {
		conn.Close(websocket.StatusPolicyViolation, "missing token")
		return
	}

	// Validate JWT.
	claims, err := auth.ValidateToken(am.Token, h.jwtSecret)
	if err != nil {
		h.logger.Warn("websocket invalid token", "error", err)
		conn.Close(websocket.StatusPolicyViolation, "invalid token")
		return
	}

	projectID := am.ProjectKey
	if projectID == "" {
		projectID = claims.UserID
	}

	clientID := uuid.New().String()
	client := newClient(clientID, projectID)
	h.hub.Register(client)
	defer h.hub.Unregister(clientID)

	// Start writer goroutine.
	writeDone := make(chan struct{})
	go h.writeLoop(client, conn, writeDone)
	defer func() { <-writeDone }()

	// Read loop for subscribe/unsubscribe messages.
	for {
		var raw json.RawMessage
		if err := wsjson.Read(ctx, conn, &raw); err != nil {
			break
		}

		var base struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &base); err != nil {
			continue
		}

		switch base.Type {
		case "subscribe":
			var msg subscribeMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.Collection == "" {
				continue
			}
			sub, err := h.hub.Subscribe(clientID, projectID, msg.Collection, msg.Filter)
			if err != nil {
				continue
			}
			if err := wsjson.Write(ctx, conn, map[string]interface{}{
				"type":            "subscribed",
				"subscription_id": sub.ID,
				"collection":      msg.Collection,
			}); err != nil {
				break
			}

		case "unsubscribe":
			var msg unsubscribeMsg
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}
			if msg.SubscriptionID == "" {
				continue
			}
			h.hub.Unsubscribe(clientID, msg.SubscriptionID)
		}
	}
}

// writeLoop reads events from the client's channel and writes them to the WebSocket.
func (h *WSHandler) writeLoop(client *Client, conn *websocket.Conn, done chan<- struct{}) {
	defer close(done)
	for event := range client.events {
		if client.closed {
			return
		}
		if err := wsjson.Write(context.Background(), conn, map[string]interface{}{
			"type":       "event",
			"event_type": event.Type,
			"collection": event.Collection,
			"id":         event.DocID,
			"data":       event.Data,
			"timestamp":  event.Timestamp,
		}); err != nil {
			return
		}
	}
}
