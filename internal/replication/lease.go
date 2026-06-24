package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// heartbeatMsg is the JSON payload exchanged during heartbeat requests.
type heartbeatMsg struct {
	NodeID string `json:"node_id"`
	Role   Role   `json:"role"`
	Addr   string `json:"addr"`
}

// peerStatus tracks when a peer was last seen and its current role.
type peerStatus struct {
	LastSeen time.Time
	Role     Role
}

// LeaseManager handles lease-based Heir election via periodic HTTP heartbeats.
// Each node tracks when it last heard from peers. The node with the lowest
// node ID among alive nodes is the designated Heir. If the Heir detects the
// Master has been silent for > 2×LeaseTTL, it promotes itself to Master.
type LeaseManager struct {
	node  *Node
	peers []string

	mu       sync.RWMutex
	lastSeen map[string]*peerStatus // nodeID → status

	httpServer *http.Server
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewLeaseManager creates a LeaseManager for the given node.
func NewLeaseManager(node *Node) *LeaseManager {
	return &LeaseManager{
		node:     node,
		peers:    append([]string(nil), node.config.Peers...),
		lastSeen: make(map[string]*peerStatus),
	}
}

// Start begins the heartbeat loop and HTTP server. It blocks until the server
// is listening, then loops run in background goroutines.
func (lm *LeaseManager) Start(ctx context.Context) error {
	lm.ctx, lm.cancel = context.WithCancel(ctx)

	// Set up the HTTP mux.
	mux := http.NewServeMux()
	mux.HandleFunc("/_replication/heartbeat", lm.heartbeatHandler)

	addr := lm.node.config.ListenAddr

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("lease: listen on %s: %w", addr, err)
	}
	lm.listener = ln

	lm.httpServer = &http.Server{
		Handler: mux,
	}

	lm.wg.Add(1)
	go func() {
		defer lm.wg.Done()
		if err := lm.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("lease: HTTP server error: %v", err)
		}
	}()

	// Start the heartbeat sender.
	lm.wg.Add(1)
	go lm.heartbeatLoop()

	// Start the master-watch loop.
	lm.wg.Add(1)
	go lm.monitorLoop()

	return nil
}

// Stop gracefully shuts down the lease manager.
func (lm *LeaseManager) Stop() {
	if lm.cancel != nil {
		lm.cancel()
	}
	if lm.httpServer != nil {
		_ = lm.httpServer.Shutdown(context.Background())
	}
	lm.wg.Wait()
}

// heartbeatHandler receives heartbeat POSTs from peers and records their
// last-seen timestamp and role.
func (lm *LeaseManager) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg heartbeatMsg
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	lm.mu.Lock()
	lm.lastSeen[msg.NodeID] = &peerStatus{
		LastSeen: time.Now(),
		Role:     msg.Role,
	}
	lm.mu.Unlock()

	// Respond with our own info.
	resp := heartbeatMsg{
		NodeID: lm.node.config.NodeID,
		Role:   lm.node.Role(),
		Addr:   lm.node.config.ListenAddr,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// heartbeatLoop periodically sends heartbeats to all configured peers.
func (lm *LeaseManager) heartbeatLoop() {
	defer lm.wg.Done()

	interval := lm.node.config.LeaseTTL / 2
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send an immediate heartbeat on start.
	lm.sendHeartbeats()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.sendHeartbeats()
		}
	}
}

// sendHeartbeats sends a POST to every peer's /_replication/heartbeat endpoint.
func (lm *LeaseManager) sendHeartbeats() {
	msg := heartbeatMsg{
		NodeID: lm.node.config.NodeID,
		Role:   lm.node.Role(),
		Addr:   lm.node.config.ListenAddr,
	}
	body, err := json.Marshal(msg)
	if err != nil {
		log.Printf("lease: marshal heartbeat: %v", err)
		return
	}

	client := &http.Client{Timeout: 2 * time.Second}

	for _, peer := range lm.peers {
		url := fmt.Sprintf("http://%s/_replication/heartbeat", peer)
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			continue
		}

		var peerMsg heartbeatMsg
		if err := json.NewDecoder(resp.Body).Decode(&peerMsg); err == nil {
			lm.mu.Lock()
			lm.lastSeen[peerMsg.NodeID] = &peerStatus{
				LastSeen: time.Now(),
				Role:     peerMsg.Role,
			}
			lm.mu.Unlock()
		}
		resp.Body.Close()
	}
}

// monitorLoop watches for Master liveness. If this node is the Heir and the
// Master has been silent for > 2×LeaseTTL, it promotes itself to Master.
func (lm *LeaseManager) monitorLoop() {
	defer lm.wg.Done()

	interval := lm.node.config.LeaseTTL / 2
	if interval <= 0 {
		interval = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			if lm.shouldPromote() {
				log.Printf("lease: node %s promoting to Master", lm.node.config.NodeID)
				if err := lm.node.BecomeMaster(); err != nil {
					log.Printf("lease: promote to Master failed: %v", err)
				}
				return // Stop monitoring after promotion.
			}
		}
	}
}

// shouldPromote returns true if this node is the designated Heir and the
// Master is considered dead.
func (lm *LeaseManager) shouldPromote() bool {
	role := lm.node.Role()
	if role != RoleHeir {
		return false
	}

	// Check if we are the designated Heir (lowest node ID among alive peers).
	lm.mu.RLock()
	alive := lm.aliveLocked()
	lm.mu.RUnlock()

	if !lm.isDesignatedHeir(alive) {
		return false
	}

	// Check if Master is dead.
	if lm.IsMasterAlive() {
		return false
	}

	return true
}

// IsMasterAlive returns true if any peer with Master role has sent a heartbeat
// within the last 2×LeaseTTL window.
func (lm *LeaseManager) IsMasterAlive() bool {
	deadline := time.Now().Add(-2 * lm.node.config.LeaseTTL)

	lm.mu.RLock()
	defer lm.mu.RUnlock()

	for _, status := range lm.lastSeen {
		if status.Role == RoleMaster && status.LastSeen.After(deadline) {
			return true
		}
	}
	return false
}

// aliveLocked returns a sorted list of node IDs that have been seen recently
// (within 2×LeaseTTL). Caller must hold lm.mu (read lock is sufficient).
func (lm *LeaseManager) aliveLocked() []string {
	deadline := time.Now().Add(-2 * lm.node.config.LeaseTTL)
	var alive []string
	for nodeID, status := range lm.lastSeen {
		if status.LastSeen.After(deadline) {
			alive = append(alive, nodeID)
		}
	}
	// Always consider ourselves alive.
	alive = append(alive, lm.node.config.NodeID)
	sort.Strings(alive)
	return alive
}

// isDesignatedHeir returns true if this node has the lowest node ID among the
// given set of alive nodes (and is therefore the designated Heir).
func (lm *LeaseManager) isDesignatedHeir(alive []string) bool {
	if len(alive) == 0 {
		return true
	}
	return alive[0] == lm.node.config.NodeID
}

// ClaimLeadership promotes this node to Master. It is called when the Heir
// determines that the Master is unavailable.
func (lm *LeaseManager) ClaimLeadership() error {
	return lm.node.BecomeMaster()
}
