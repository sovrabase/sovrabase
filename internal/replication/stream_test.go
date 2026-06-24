package replication

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer creates a StreamServer on a random port and starts it.
func newTestServer(t *testing.T) (*StreamServer, string) {
	t.Helper()

	// Listen on port 0 to get a random available port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	srv := NewStreamServer(addr)
	srv.listener = ln

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start in background with the pre-bound listener.
	go func() {
		srv.serveWithListener(ctx, ln)
	}()

	time.Sleep(100 * time.Millisecond)
	return srv, addr
}

// TestStreamServerClientIntegration tests the full flow:
// server starts, client connects, entries are streamed.
func TestStreamServerClientIntegration(t *testing.T) {
	// Create a log provider that sends 3 entries then stops.
	var logEntries = []*LogEntry{
		{LSN: 1, Operation: OpInsert, Collection: "users", DocID: "u1", Data: []byte(`{"name":"alice"}`)},
		{LSN: 2, Operation: OpInsert, Collection: "users", DocID: "u2", Data: []byte(`{"name":"bob"}`)},
		{LSN: 3, Operation: OpUpdate, Collection: "users", DocID: "u1", Data: []byte(`{"name":"alice2"}`)},
	}

	srv, addr := newTestServer(t)

	srv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan *LogEntry, 10)

		go func() {
			defer close(ch)
			for _, e := range logEntries {
				if e.LSN > lsn {
					select {
					case ch <- e:
					case <-ctx.Done():
						return
					}
				}
			}
			// Keep the channel open briefly so the client can receive.
			<-ctx.Done()
		}()

		return ch, cancel
	})

	client := NewStreamClient(addr, "reader-1", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Collect entries.
	var received []*LogEntry
	timeout := time.After(3 * time.Second)

	for i := 0; i < len(logEntries); i++ {
		select {
		case entry := <-client.Entries():
			received = append(received, entry)
		case err := <-client.Errors():
			t.Logf("client error: %v", err)
		case <-timeout:
			t.Fatalf("timeout waiting for entry %d, got %d so far", i+1, len(received))
		}
	}

	if len(received) != len(logEntries) {
		t.Fatalf("expected %d entries, got %d", len(logEntries), len(received))
	}

	for i, e := range received {
		if e.LSN != logEntries[i].LSN {
			t.Errorf("entry %d: expected LSN %d, got %d", i, logEntries[i].LSN, e.LSN)
		}
		if e.Operation != logEntries[i].Operation {
			t.Errorf("entry %d: expected op %s, got %s", i, logEntries[i].Operation, e.Operation)
		}
	}

	// Verify peer tracking.
	peers := srv.ConnectedPeers()
	if len(peers) != 1 {
		t.Fatalf("expected 1 connected peer, got %d", len(peers))
	}
	if peers[0].ID != "reader-1" {
		t.Errorf("expected peer ID reader-1, got %s", peers[0].ID)
	}
}

// TestClientReconnect tests that the client can reconnect after the server
// goes down and comes back up.
func TestClientReconnect(t *testing.T) {
	srv, addr := newTestServer(t)

	var entryCount atomic.Int64
	entryCount.Store(0)

	srv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan *LogEntry, 10)

		go func() {
			defer close(ch)
			for i := lsn + 1; i <= lsn+5; i++ {
				select {
				case ch <- &LogEntry{LSN: i, Operation: OpInsert, Collection: "test", DocID: "doc"}:
					entryCount.Add(1)
				case <-ctx.Done():
					return
				}
			}
			<-ctx.Done()
		}()

		return ch, cancel
	})

	client := NewStreamClient(addr, "reader-2", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("initial connect failed: %v", err)
	}

	// Receive a few entries.
	received := 0
	timeout := time.After(2 * time.Second)
	for received < 3 {
		select {
		case <-client.Entries():
			received++
		case err := <-client.Errors():
			t.Logf("client error: %v", err)
		case <-timeout:
			t.Fatalf("timeout: received %d entries", received)
		}
	}

	// Shutdown the server.
	srv.Shutdown(context.Background())

	// Wait for the client to detect disconnection.
	time.Sleep(500 * time.Millisecond)

	// Start a new server on the same address.
	srv2 := NewStreamServer(addr)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	var entryCount2 atomic.Int64
	srv2.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan *LogEntry, 10)

		go func() {
			defer close(ch)
			for i := lsn + 1; i <= lsn+3; i++ {
				select {
				case ch <- &LogEntry{LSN: i, Operation: OpInsert, Collection: "test2", DocID: "doc2"}:
					entryCount2.Add(1)
				case <-ctx.Done():
					return
				}
			}
			<-ctx.Done()
		}()

		return ch, cancel
	})

	// Start server 2 in background.
	go func() {
		srv2.Start(ctx2)
	}()
	time.Sleep(200 * time.Millisecond)

	// Reconnect.
	if err := client.Reconnect(ctx); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}

	// Receive entries from the new server.
	timeout2 := time.After(3 * time.Second)
	received2 := 0
	for received2 < 2 {
		select {
		case <-client.Entries():
			received2++
		case err := <-client.Errors():
			t.Logf("client error (after reconnect): %v", err)
		case <-timeout2:
			t.Fatalf("timeout after reconnect: received %d entries", received2)
		}
	}

	if received2 < 2 {
		t.Errorf("expected at least 2 entries after reconnect, got %d", received2)
	}

	client.Close()
}

// TestHeartbeat verifies that ping/pong keeps the connection alive.
func TestHeartbeat(t *testing.T) {
	srv, addr := newTestServer(t)

	// Provider that sends entries very slowly (every 15s) so that only
	// heartbeats keep the connection alive between entries.
	providerCalled := make(chan struct{})
	srv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		close(providerCalled)
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan *LogEntry, 1)

		go func() {
			defer close(ch)
			// Send one entry immediately.
			select {
			case ch <- &LogEntry{LSN: 1, Operation: OpInsert, Collection: "hb", DocID: "h1"}:
			case <-ctx.Done():
				return
			}
			// Then wait 12 seconds before the next one to force heartbeats.
			select {
			case <-time.After(12 * time.Second):
			case <-ctx.Done():
				return
			}
			select {
			case ch <- &LogEntry{LSN: 2, Operation: OpInsert, Collection: "hb", DocID: "h2"}:
			case <-ctx.Done():
			}
		}()

		return ch, cancel
	})

	client := NewStreamClient(addr, "reader-hb", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer client.Close()

	// Receive the first entry quickly.
	select {
	case entry := <-client.Entries():
		if entry.LSN != 1 {
			t.Errorf("expected LSN 1, got %d", entry.LSN)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first entry")
	}

	// Now wait for the second entry which comes after 12s.
	// The connection should stay alive because of heartbeats.
	select {
	case entry := <-client.Entries():
		if entry.LSN != 2 {
			t.Errorf("expected LSN 2, got %d", entry.LSN)
		}
	case err := <-client.Errors():
		t.Fatalf("unexpected error (heartbeat should have kept alive): %v", err)
	case <-time.After(18 * time.Second):
		t.Fatal("timeout waiting for second entry; heartbeat may have failed")
	}
}

// TestDuplicatePeerDisconnect verifies that when a peer reconnects with the
// same node ID, the old connection is dropped and the new one takes over.
func TestDuplicatePeerDisconnect(t *testing.T) {
	providerMu := sync.Mutex{}
	providerChs := make([]chan *LogEntry, 0)

	srv, addr := newTestServer(t)
	srv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		providerMu.Lock()
		defer providerMu.Unlock()
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan *LogEntry, 1)
		providerChs = append(providerChs, ch)
		go func() {
			<-ctx.Done()
		}()
		return ch, cancel
	})

	// Connect first client.
	client1 := NewStreamClient(addr, "dup-peer", 0)
	ctx := context.Background()
	if err := client1.Connect(ctx); err != nil {
		t.Fatalf("client1 connect: %v", err)
	}
	defer client1.Close()

	// Wait for it to register.
	time.Sleep(200 * time.Millisecond)

	// Connect second client with same node ID.
	client2 := NewStreamClient(addr, "dup-peer", 0)
	if err := client2.Connect(ctx); err != nil {
		t.Fatalf("client2 connect: %v", err)
	}
	defer client2.Close()

	// Wait for the old connection to be dropped and new one registered.
	// Use a retry loop to handle timing variations.
	var peers []PeerInfo
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		peers = srv.ConnectedPeers()
		if len(peers) >= 1 {
			break
		}
	}

	if len(peers) != 1 {
		t.Fatalf("expected exactly 1 peer, got %d", len(peers))
	}
	if peers[0].ID != "dup-peer" {
		t.Errorf("expected peer ID dup-peer, got %s", peers[0].ID)
	}
}
