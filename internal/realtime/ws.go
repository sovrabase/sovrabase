package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// WSHandler handles WebSocket upgrade and realtime subscription lifecycle.
type WSHandler struct {
	hub            *Hub
	jwtSecret      string
	logger         *slog.Logger
	meterStore     *metering.MeterStore
	projectManager *tenant.ProjectManager
}

// NewWSHandler creates a new WebSocket handler for realtime subscriptions.
func NewWSHandler(hub *Hub, jwtSecret string, meterStore *metering.MeterStore, projectManager *tenant.ProjectManager) *WSHandler {
	return &WSHandler{
		hub:            hub,
		jwtSecret:      jwtSecret,
		logger:         slog.Default().With("module", "realtime-ws"),
		meterStore:     meterStore,
		projectManager: projectManager,
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
	// Close the connection FIRST on exit, before waiting for writeLoop.
	// This unblocks any pending write immediately.
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

	projectKey := am.ProjectKey
	projectID := projectKey
	if h.projectManager != nil && projectKey != "" {
		if proj, err := h.projectManager.GetProjectBySecret(projectKey); err == nil && proj != nil {
			projectID = proj.ID
		}
	}
	if projectID == "" {
		projectID = claims.UserID
	}

	clientID := uuid.New().String()
	client := newClient(clientID, projectID)
	h.hub.Register(client)

	if h.meterStore != nil && projectID != "" {
		_ = h.meterStore.Inc(projectID, metering.MetricRealtimeConnections, 1)
	}

	// Start writer goroutine.
	writeDone := make(chan struct{})
	go h.writeLoop(client, conn, ctx, writeDone)

	// Cleanup: unregister (closes events channel → writeLoop exits), then wait for writeDone.
	defer func() {
		h.hub.Unregister(clientID)
		if h.meterStore != nil && projectID != "" {
			_ = h.meterStore.Inc(projectID, metering.MetricRealtimeConnections, -1)
		}
		<-writeDone
	}()

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

// writeLoop reads pre-marshaled payloads from the client's channel and writes
// them to the WebSocket. Uses the request context for cancellation so writes
// abort immediately when the client disconnects or the server shuts down.
// A periodic ping detects dead connections.
func (h *WSHandler) writeLoop(client *Client, conn *websocket.Conn, ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case payload, ok := <-client.events:
			if !ok {
				// Channel closed by Unregister — clean exit.
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, payload.data); err != nil {
				return
			}
		case <-ticker.C:
			// Heartbeat ping — detects dead connections.
			if err := conn.Ping(ctx); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
