package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// handshakeMsg is sent by the client on initial WebSocket connection.
type handshakeMsg struct {
	NodeID  string `json:"node_id"`
	LastLSN uint64 `json:"last_lsn"`
}

// controlMsg is used for ping/pong heartbeat messages.
type controlMsg struct {
	Type string `json:"type"`
}

// peerState tracks a connected peer and its WebSocket connection.
type peerState struct {
	info   PeerInfo
	conn   *websocket.Conn
	cancel context.CancelFunc
}

// StreamServer accepts WebSocket connections from Readers and streams WAL
// log entries to them in real time.
type StreamServer struct {
	addr        string
	httpServer  *http.Server
	listener    net.Listener // set during Start(); can be injected for tests
	logProvider func(lsn uint64) (<-chan *LogEntry, context.CancelFunc)

	mu    sync.RWMutex
	peers map[string]*peerState
}

// NewStreamServer creates a new StreamServer that will listen on addr.
func NewStreamServer(addr string) *StreamServer {
	return &StreamServer{
		addr:  addr,
		peers: make(map[string]*peerState),
	}
}

// Addr returns the actual address the server is listening on.
// This is only valid after Start has been called.
func (s *StreamServer) Addr() string {
	return s.addr
}

// SetLogProvider configures the callback used to obtain a stream of log
// entries starting from a given LSN. The returned cancel function stops
// the provider for that particular peer when the peer disconnects.
func (s *StreamServer) SetLogProvider(fn func(lsn uint64) (<-chan *LogEntry, context.CancelFunc)) {
	s.logProvider = fn
}

// Start begins listening for WebSocket connections. Blocks until the server
// is shutdown or ctx is cancelled.
func (s *StreamServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("stream server: listen %s: %w", s.addr, err)
	}
	s.listener = ln
	// Update addr to the actual bound address (useful when using :0 port).
	s.addr = ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/_replication/stream", s.handleStream)

	s.httpServer = &http.Server{
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.httpServer.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	}
}

// Shutdown gracefully stops the server, closing all peer connections.
func (s *StreamServer) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for _, p := range s.peers {
		p.cancel()
		p.conn.Close(websocket.StatusGoingAway, "server shutting down")
	}
	s.peers = make(map[string]*peerState)
	s.mu.Unlock()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// serveWithListener serves HTTP on the provided listener. Used by tests that
// pre-bind a listener to guarantee a specific address.
func (s *StreamServer) serveWithListener(ctx context.Context, ln net.Listener) {
	s.listener = ln
	s.addr = ln.Addr().String()

	mux := http.NewServeMux()
	mux.HandleFunc("/_replication/stream", s.handleStream)

	s.httpServer = &http.Server{
		Handler: mux,
		BaseContext: func(_ net.Listener) context.Context { return ctx },
	}

	s.httpServer.Serve(ln)
}

func (s *StreamServer) ConnectedPeers() []PeerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]PeerInfo, 0, len(s.peers))
	for _, p := range s.peers {
		result = append(result, p.info)
	}
	return result
}

// handleStream handles the WebSocket upgrade and manages a single peer connection.
func (s *StreamServer) handleStream(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // allow non-TLS for dev/internal
	})
	if err != nil {
		return
	}

	// Read handshake.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	var hs handshakeMsg
	if err := wsjson.Read(ctx, conn, &hs); err != nil {
		cancel()
		conn.Close(websocket.StatusInvalidFramePayloadData, "bad handshake")
		return
	}
	cancel()

	if hs.NodeID == "" {
		conn.Close(websocket.StatusInvalidFramePayloadData, "missing node_id")
		return
	}

	peerCtx, peerCancel := context.WithCancel(r.Context())

	ps := &peerState{
		info: PeerInfo{
			ID:       hs.NodeID,
			Role:     RoleReader,
			Addr:     r.RemoteAddr,
			LastSeen: time.Now(),
			LastLSN:  hs.LastLSN,
		},
		conn:   conn,
		cancel: peerCancel,
	}

	s.mu.Lock()
	// If a peer with this ID is already connected, disconnect the old one.
	if old, exists := s.peers[hs.NodeID]; exists {
		old.cancel()
		old.conn.Close(websocket.StatusGoingAway, "duplicate connection")
	}
	s.peers[hs.NodeID] = ps
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		// Only remove if this peerState is still the current one for this ID.
		if s.peers[hs.NodeID] == ps {
			delete(s.peers, hs.NodeID)
		}
		s.mu.Unlock()
		peerCancel()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	s.servePeer(peerCtx, ps, hs.LastLSN)
}

// servePeer runs the read and write loops for a connected peer.
func (s *StreamServer) servePeer(ctx context.Context, ps *peerState, lastLSN uint64) {
	// Channel for pong responses from the client.
	pongCh := make(chan struct{}, 1)
	var lastPong time.Time
	var pongMu sync.Mutex

	// Read loop: handles pong responses from client.
	go func() {
		for {
			var msg controlMsg
			if err := wsjson.Read(ctx, ps.conn, &msg); err != nil {
				return
			}
			if msg.Type == "pong" {
				pongMu.Lock()
				lastPong = time.Now()
				pongMu.Unlock()
				select {
				case pongCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	// Get the log stream from the provider.
	var logCh <-chan *LogEntry
	var logCancel context.CancelFunc
	if s.logProvider != nil {
		logCh, logCancel = s.logProvider(lastLSN)
		if logCancel != nil {
			defer logCancel()
		}
	}

	pingTicker := time.NewTicker(10 * time.Second)
	defer pingTicker.Stop()

	pongMu.Lock()
	lastPong = time.Now()
	pongMu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return

		case <-pingTicker.C:
			// Send ping.
			if err := wsjson.Write(ctx, ps.conn, controlMsg{Type: "ping"}); err != nil {
				return
			}

			// Check if we've received a pong recently.
			pongMu.Lock()
			pongAge := time.Since(lastPong)
			pongMu.Unlock()
			if pongAge > 30*time.Second {
				ps.conn.Close(websocket.StatusPolicyViolation, "ping timeout")
				return
			}

		case entry, ok := <-logCh:
			if !ok {
				// Log stream ended.
				return
			}
			if err := wsjson.Write(ctx, ps.conn, entry); err != nil {
				return
			}
			// Update last seen LSN.
			s.mu.Lock()
			if p, exists := s.peers[ps.info.ID]; exists {
				p.info.LastLSN = entry.LSN
				p.info.LastSeen = time.Now()
			}
			s.mu.Unlock()
		}
	}
}

// ---------------------------------------------------------------------------
// StreamClient
// ---------------------------------------------------------------------------

// StreamClient connects to a Master's StreamServer and receives replicated
// log entries over WebSocket.
type StreamClient struct {
	masterAddr string
	nodeID     string
	lastLSN    uint64

	conn *websocket.Conn

	entriesCh chan *LogEntry
	errorsCh  chan error

	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	closed     bool
	reconnect  bool // whether auto-reconnect is enabled

	backoff time.Duration // current backoff for reconnection

	// lastRecvTime is an atomic UnixNano timestamp updated on every
	// successful WebSocket read (entries and control messages).
	lastRecvTime atomic.Int64
}

// NewStreamClient creates a new StreamClient. It does not connect until
// Connect is called.
func NewStreamClient(masterAddr, nodeID string, lastLSN uint64) *StreamClient {
	return &StreamClient{
		masterAddr: masterAddr,
		nodeID:     nodeID,
		lastLSN:    lastLSN,
		entriesCh:  make(chan *LogEntry, 256),
		errorsCh:   make(chan error, 16),
	}
}

// Connect establishes a WebSocket connection to the Master and begins
// receiving log entries.
func (c *StreamClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client is closed")
	}
	if c.conn != nil {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	c.mu.Unlock()

	return c.dial(ctx)
}

// dial performs the actual WebSocket dial and starts the read loop.
func (c *StreamClient) dial(ctx context.Context) error {
	url := fmt.Sprintf("ws://%s/_replication/stream", c.masterAddr)

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if err != nil {
		return fmt.Errorf("dial master %s: %w", c.masterAddr, err)
	}

	// Send handshake.
	hs := handshakeMsg{
		NodeID:  c.nodeID,
		LastLSN: c.lastLSN,
	}
	if err := wsjson.Write(ctx, conn, hs); err != nil {
		conn.Close(websocket.StatusInternalError, "")
		return fmt.Errorf("send handshake: %w", err)
	}

	readCtx, readCancel := context.WithCancel(ctx)

	c.mu.Lock()
	c.conn = conn
	c.cancel = readCancel
	c.backoff = 1 * time.Second // reset backoff on successful connect
	c.lastRecvTime.Store(time.Now().UnixNano())
	c.mu.Unlock()

	// Start read loop.
	go c.readLoop(readCtx, conn)

	return nil
}

// readLoop reads messages from the WebSocket and dispatches them.
func (c *StreamClient) readLoop(ctx context.Context, conn *websocket.Conn) {
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		shouldReconnect := c.reconnect && !c.closed
		c.mu.Unlock()

		if shouldReconnect {
			// Reconnect with backoff.
			c.mu.Lock()
			backoff := c.backoff
			c.mu.Unlock()
			time.Sleep(backoff)
			c.mu.Lock()
			c.backoff = time.Duration(math.Min(float64(c.backoff*2), float64(30*time.Second)))
			c.mu.Unlock()
			if err := c.dial(context.Background()); err != nil {
				c.errorsCh <- fmt.Errorf("reconnect failed: %w", err)
			}
		}
	}()

	for {
		// Read raw message to determine type.
		_, data, err := conn.Read(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				c.errorsCh <- fmt.Errorf("read error: %w", err)
				return
			}
		}

		// Update last receive time on any successful read.
		c.lastRecvTime.Store(time.Now().UnixNano())

		// Try to parse as control message first.
		var ctrl controlMsg
		if json.Unmarshal(data, &ctrl) == nil && ctrl.Type == "ping" {
			// Respond with pong.
			if err := wsjson.Write(ctx, conn, controlMsg{Type: "pong"}); err != nil {
				c.errorsCh <- fmt.Errorf("write pong: %w", err)
				return
			}
			continue
		}

		// Parse as log entry.
		var entry LogEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			c.errorsCh <- fmt.Errorf("decode entry: %w", err)
			continue
		}

		// Update last known LSN.
		c.mu.Lock()
		if entry.LSN > c.lastLSN {
			c.lastLSN = entry.LSN
		}
		c.mu.Unlock()

		select {
		case c.entriesCh <- &entry:
		case <-ctx.Done():
			return
		default:
			c.errorsCh <- fmt.Errorf("entries channel full, dropping entry LSN=%d", entry.LSN)
		}
	}
}

// Entries returns a channel of log entries received from the Master.
func (c *StreamClient) Entries() <-chan *LogEntry {
	return c.entriesCh
}

// Errors returns a channel of stream errors.
func (c *StreamClient) Errors() <-chan error {
	return c.errorsCh
}

// Close disconnects from the Master and stops the client.
func (c *StreamClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true
	c.reconnect = false

	if c.cancel != nil {
		c.cancel()
	}
	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "client closing")
		c.conn = nil
	}

	return nil
}

// LastRecvTime returns the time of the last successful WebSocket read.
// Returns the zero time if no data has been received yet.
func (c *StreamClient) LastRecvTime() time.Time {
	nano := c.lastRecvTime.Load()
	if nano == 0 {
		return time.Time{}
	}
	return time.Unix(0, nano)
}

// SetReconnect enables or disables automatic reconnection on disconnect.
func (c *StreamClient) SetReconnect(enabled bool) {
	c.mu.Lock()
	c.reconnect = enabled
	c.mu.Unlock()
}

// Reconnect reconnects to the Master with exponential backoff. On success,
// the backoff is reset.
func (c *StreamClient) Reconnect(ctx context.Context) error {
	c.mu.Lock()
	c.reconnect = true
	if c.conn != nil {
		c.conn.Close(websocket.StatusGoingAway, "reconnecting")
		c.conn = nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	c.mu.Unlock()

	return c.dial(ctx)
}
