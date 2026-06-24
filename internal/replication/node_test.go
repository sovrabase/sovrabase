package replication

import (
	"context"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/db"
)

// newTestNode creates a Node backed by a temporary Pebble engine.
func newTestNode(t *testing.T, role Role) *Node {
	t.Helper()

	eng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })

	cfg := &NodeConfig{
		NodeID:     "test-node",
		ListenAddr: ":0", // OS-assigned port
		Role:       role,
		LeaseTTL:   5 * time.Second,
	}

	return NewNode(cfg, eng)
}

// =============================================================================
// TestNodeRoleTransitions
// =============================================================================

func TestNodeRoleTransitions(t *testing.T) {
	t.Run("MasterToHeir", func(t *testing.T) {
		node := newTestNode(t, RoleMaster)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		// Verify initial role.
		if node.Role() != RoleMaster {
			t.Fatalf("expected Master, got %s", node.Role())
		}

		// Transition to Heir.
		if err := node.BecomeHeir(); err != nil {
			t.Fatalf("BecomeHeir: %v", err)
		}
		if node.Role() != RoleHeir {
			t.Fatalf("expected Heir after BecomeHeir, got %s", node.Role())
		}
	})

	t.Run("MasterToReader", func(t *testing.T) {
		node := newTestNode(t, RoleMaster)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		// Transition to Reader.
		if err := node.BecomeReader(); err != nil {
			t.Fatalf("BecomeReader: %v", err)
		}
		if node.Role() != RoleReader {
			t.Fatalf("expected Reader after BecomeReader, got %s", node.Role())
		}
	})

	t.Run("HeirToMaster", func(t *testing.T) {
		node := newTestNode(t, RoleHeir)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		if err := node.BecomeMaster(); err != nil {
			t.Fatalf("BecomeMaster: %v", err)
		}
		if node.Role() != RoleMaster {
			t.Fatalf("expected Master after BecomeMaster, got %s", node.Role())
		}
	})

	t.Run("HeirToReader", func(t *testing.T) {
		node := newTestNode(t, RoleHeir)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		if err := node.BecomeReader(); err != nil {
			t.Fatalf("BecomeReader: %v", err)
		}
		if node.Role() != RoleReader {
			t.Fatalf("expected Reader after BecomeReader, got %s", node.Role())
		}
	})

	t.Run("ReaderToMaster", func(t *testing.T) {
		node := newTestNode(t, RoleReader)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		if err := node.BecomeMaster(); err != nil {
			t.Fatalf("BecomeMaster: %v", err)
		}
		if node.Role() != RoleMaster {
			t.Fatalf("expected Master after BecomeMaster, got %s", node.Role())
		}
	})

	t.Run("ReaderToHeir", func(t *testing.T) {
		node := newTestNode(t, RoleReader)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		if err := node.BecomeHeir(); err != nil {
			t.Fatalf("BecomeHeir: %v", err)
		}
		if node.Role() != RoleHeir {
			t.Fatalf("expected Heir after BecomeHeir, got %s", node.Role())
		}
	})

	t.Run("IdempotentTransitions", func(t *testing.T) {
		node := newTestNode(t, RoleMaster)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx); err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer node.Stop()

		// Multiple BecomeMaster calls should be idempotent.
		if err := node.BecomeMaster(); err != nil {
			t.Fatalf("first BecomeMaster: %v", err)
		}
		if err := node.BecomeMaster(); err != nil {
			t.Fatalf("second BecomeMaster: %v", err)
		}
		if node.Role() != RoleMaster {
			t.Fatalf("expected Master, got %s", node.Role())
		}

		// Multiple BecomeHeir calls should be idempotent.
		if err := node.BecomeHeir(); err != nil {
			t.Fatalf("first BecomeHeir: %v", err)
		}
		if err := node.BecomeHeir(); err != nil {
			t.Fatalf("second BecomeHeir: %v", err)
		}
		if node.Role() != RoleHeir {
			t.Fatalf("expected Heir, got %s", node.Role())
		}
	})
}

// =============================================================================
// TestApplyEntry
// =============================================================================

func TestApplyEntry(t *testing.T) {
	node := newTestNode(t, RoleReader)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer node.Stop()

	eng := newTestEngineForNode(t, node)

	// Create a collection via ApplyEntry.
	err := node.ApplyEntry(&LogEntry{
		LSN:        1,
		Operation:  OpCreateCollection,
		Collection: "users",
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyEntry create_collection: %v", err)
	}

	// Insert a document via ApplyEntry.
	insertData, _ := db.MarshalMap(map[string]interface{}{"name": "Alice", "age": float64(30)})
	err = node.ApplyEntry(&LogEntry{
		LSN:        2,
		Operation:  OpInsert,
		Collection: "users",
		DocID:      "alice",
		Data:       insertData,
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyEntry insert: %v", err)
	}

	// Verify the document exists in the engine.
	doc, err := eng.Get("users", "alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document after ApplyEntry insert")
	}
	if doc["_id"] != "alice" {
		t.Fatalf("expected _id=alice, got %v", doc["_id"])
	}
	if doc["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", doc["name"])
	}

	// Update via ApplyEntry.
	updateData, _ := db.MarshalMap(map[string]interface{}{"name": "Alicia", "age": float64(31)})
	err = node.ApplyEntry(&LogEntry{
		LSN:        3,
		Operation:  OpUpdate,
		Collection: "users",
		DocID:      "alice",
		Data:       updateData,
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyEntry update: %v", err)
	}

	// Verify updated.
	doc, err = eng.Get("users", "alice")
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	if doc["name"] != "Alicia" {
		t.Fatalf("expected name=Alicia, got %v", doc["name"])
	}

	// Delete via ApplyEntry.
	err = node.ApplyEntry(&LogEntry{
		LSN:        4,
		Operation:  OpDelete,
		Collection: "users",
		DocID:      "alice",
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyEntry delete: %v", err)
	}

	// Verify deleted.
	doc, err = eng.Get("users", "alice")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if doc != nil {
		t.Fatal("expected nil document after ApplyEntry delete")
	}

	// Drop the collection.
	err = node.ApplyEntry(&LogEntry{
		LSN:        5,
		Operation:  OpDropCollection,
		Collection: "users",
		Timestamp:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ApplyEntry drop_collection: %v", err)
	}
}

func TestApplyEntryNilEntry(t *testing.T) {
	node := newTestNode(t, RoleReader)

	err := node.ApplyEntry(nil)
	if err == nil {
		t.Fatal("expected error for nil entry")
	}
}

func TestApplyEntryUnknownOp(t *testing.T) {
	node := newTestNode(t, RoleReader)

	err := node.ApplyEntry(&LogEntry{
		LSN:       1,
		Operation: Operation("bogus"),
		Timestamp: time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected error for unknown operation")
	}
}

// =============================================================================
// TestWrite
// =============================================================================

func TestWrite(t *testing.T) {
	node := newTestNode(t, RoleMaster)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer node.Stop()

	eng := newTestEngineForNode(t, node)

	// Create collection via Write.
	lsn, err := node.Write("items", OpCreateCollection, "", nil)
	if err != nil {
		t.Fatalf("Write create_collection: %v", err)
	}
	if lsn != 1 {
		t.Fatalf("expected LSN 1, got %d", lsn)
	}

	// Insert via Write.
	lsn, err = node.Write("items", OpInsert, "item-1", map[string]interface{}{"title": "Hello"})
	if err != nil {
		t.Fatalf("Write insert: %v", err)
	}
	if lsn != 2 {
		t.Fatalf("expected LSN 2, got %d", lsn)
	}

	// Verify the document exists.
	doc, err := eng.Get("items", "item-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc == nil {
		t.Fatal("expected document after Write insert")
	}
	if doc["title"] != "Hello" {
		t.Fatalf("expected title=Hello, got %v", doc["title"])
	}

	// Verify LSN counter.
	if node.CurrentLSN() != 2 {
		t.Fatalf("expected CurrentLSN 2, got %d", node.CurrentLSN())
	}
}

func TestWriteNotMaster(t *testing.T) {
	node := newTestNode(t, RoleReader)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := node.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer node.Stop()

	_, err := node.Write("items", OpInsert, "id", nil)
	if err == nil {
		t.Fatal("expected error writing on non-Master node")
	}
}

// =============================================================================
// TestLeaseHeartbeat
// =============================================================================

func TestLeaseHeartbeat(t *testing.T) {
	// Create a Master node.
	masterEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine master: %v", err)
	}
	defer masterEng.Close()

	masterCfg := &NodeConfig{
		NodeID:     "node-1",
		ListenAddr: "127.0.0.1:0",
		Role:       RoleMaster,
		Peers:      []string{},
		LeaseTTL:   2 * time.Second,
	}
	master := NewNode(masterCfg, masterEng)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := master.Start(ctx); err != nil {
		t.Fatalf("Master Start: %v", err)
	}
	defer master.Stop()

	// We need to start an HTTP server on the Master for heartbeat tests.
	// The LeaseManager starts its own. Let's directly test LeaseManager.

	// Create a Heir with the Master as a peer.
	heirEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine heir: %v", err)
	}
	defer heirEng.Close()

	heirCfg := &NodeConfig{
		NodeID:     "node-2",
		ListenAddr: "127.0.0.1:0",
		Role:       RoleHeir,
		Peers:      []string{},
		LeaseTTL:   500 * time.Millisecond,
	}
	heir := NewNode(heirCfg, heirEng)

	// Create a LeaseManager directly on the Heir.
	lm := NewLeaseManager(heir)

	// Start the lease manager's HTTP server.
	lmCtx, lmCancel := context.WithCancel(ctx)
	defer lmCancel()

	// Use an OS-assigned port — LeaseManager binds to ListenAddr.
	err = lm.Start(lmCtx)
	if err != nil {
		t.Fatalf("LeaseManager Start: %v", err)
	}
	defer lm.Stop()

	// Check initial state: Master should not be alive (no heartbeats received).
	if lm.IsMasterAlive() {
		t.Fatal("expected Master to NOT be alive initially")
	}

	// Manually inject a Master heartbeat to simulate receiving one.
	lm.mu.Lock()
	lm.lastSeen["node-1"] = &peerStatus{
		LastSeen: time.Now(),
		Role:     RoleMaster,
	}
	lm.mu.Unlock()

	// Master should be alive now.
	if !lm.IsMasterAlive() {
		t.Fatal("expected Master to be alive after injecting heartbeat")
	}

	// Verify aliveLocked includes node-1 and node-2 (heir itself).
	lm.mu.RLock()
	alive := lm.aliveLocked()
	lm.mu.RUnlock()

	if len(alive) < 2 {
		t.Fatalf("expected at least 2 alive nodes, got %d: %v", len(alive), alive)
	}

	// node-1 (Master) has lower ID, should be first in sorted order.
	if alive[0] != "node-1" {
		t.Fatalf("expected node-1 first, got %s", alive[0])
	}

	// Heir should not be the designated Heir (since Master node-1 has lower ID).
	if lm.isDesignatedHeir(alive) {
		t.Fatal("expected node-2 NOT to be designated Heir when node-1 is alive")
	}

	// Test ClaimLeadership — should promote Heir to Master.
	// Remove Master from lastSeen (simulate death).
	lm.mu.Lock()
	delete(lm.lastSeen, "node-1")
	lm.mu.Unlock()

	if lm.IsMasterAlive() {
		t.Fatal("expected Master to NOT be alive after removal")
	}

	// Now Heir should be designated Heir.
	lm.mu.RLock()
	alive = lm.aliveLocked()
	lm.mu.RUnlock()

	if len(alive) != 1 || alive[0] != "node-2" {
		t.Fatalf("expected only node-2 alive, got %v", alive)
	}
	if !lm.isDesignatedHeir(alive) {
		t.Fatal("expected node-2 to be designated Heir when alone")
	}

	// Promote to Master via ClaimLeadership.
	heir.BecomeHeir() // Ensure it's Heir first.
	if err := lm.ClaimLeadership(); err != nil {
		t.Fatalf("ClaimLeadership: %v", err)
	}

	if heir.Role() != RoleMaster {
		t.Fatalf("expected Heir promoted to Master, got %s", heir.Role())
	}
}

// =============================================================================
// TestNodeConfig
// =============================================================================

func TestNodeConfig(t *testing.T) {
	node := newTestNode(t, RoleMaster)

	cfg := node.Config()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Role != RoleMaster {
		t.Fatalf("expected Master role, got %s", cfg.Role)
	}

	// Modify the copy — should not affect original.
	cfg.Role = RoleReader
	if node.Role() != RoleMaster {
		t.Fatal("Config() should return a copy, not original pointer")
	}
}

// =============================================================================
// helpers
// =============================================================================

// newTestEngineForNode returns the engine inside the node.
// The engine is already created by newTestNode, this just provides access.
func newTestEngineForNode(t *testing.T, node *Node) *db.Engine {
	t.Helper()
	// We access the engine through the node itself for apply/write tests.
	// The tests call node.ApplyEntry/node.Write which use node.engine internally.
	// For verification, we create a temporary path to access the engine.
	// Since engine is unexported, we use a non-portable approach:

	// Use a temp dir engine for verification inside tests.
	// Actually, the node already has the engine. The tests verify through
	// the node's ApplyEntry/Write, and for direct reads we use the engine
	// that was passed to NewNode. Since newTestNode creates it, we need to
	// return it from newTestNode.

	// Let's just create a separate engine for verification in the test.
	// For now, tests that need to verify engine state use the original engine
	// reference from newTestNode. We'll adjust the helper.
	return node.engine
}
