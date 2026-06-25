package replication

import (
	"context"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/db"
)

// =============================================================================
// TestMasterHeirReaderPipeline — full replication pipeline integration test.
// =============================================================================
//
// Phases:
//   1. Setup: 3 nodes (Master, Heir, Reader), 3 WALs, 3 engines
//   2. Data replication: Master writes, Heir and Reader receive & apply entries
//   3. Consistency: all three engines have identical data
//   4. Failover: Master goes down → Heir detects and promotes to Master
//   5. Reconnect: Reader reconnects to promoted Heir, receives new data
// =============================================================================

func TestMasterHeirReaderPipeline(t *testing.T) {
	// ---- 1. Engines (temp Pebble databases) ----
	masterEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine master: %v", err)
	}
	t.Cleanup(func() { masterEng.Close() })

	heirEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine heir: %v", err)
	}
	t.Cleanup(func() { heirEng.Close() })

	readerEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine reader: %v", err)
	}
	t.Cleanup(func() { readerEng.Close() })

	// ---- 2. WALs (persistent on-disk logs) ----
	masterWal, err := NewReplicationLog(tempDir(t))
	if err != nil {
		t.Fatalf("NewReplicationLog master: %v", err)
	}
	t.Cleanup(func() { masterWal.Close() })

	heirWal, err := NewReplicationLog(tempDir(t))
	if err != nil {
		t.Fatalf("NewReplicationLog heir: %v", err)
	}
	t.Cleanup(func() { heirWal.Close() })

	readerWal, err := NewReplicationLog(tempDir(t))
	if err != nil {
		t.Fatalf("NewReplicationLog reader: %v", err)
	}
	t.Cleanup(func() { readerWal.Close() })

	// ---- 3. Node configs ----
	// Use a short LeaseTTL so failover is fast.
	leaseTTL := 500 * time.Millisecond

	masterCfg := &NodeConfig{
		NodeID:     "master",
		ListenAddr: "127.0.0.1:0",
		Role:       RoleMaster,
		LeaseTTL:   leaseTTL,
	}
	heirCfg := &NodeConfig{
		NodeID:     "heir",
		ListenAddr: "127.0.0.1:0",
		Role:       RoleHeir,
		Peers:      []string{}, // We'll manage heartbeats manually for the test
		LeaseTTL:   leaseTTL,
	}
	readerCfg := &NodeConfig{
		NodeID:     "reader",
		ListenAddr: "127.0.0.1:0",
		Role:       RoleReader,
		LeaseTTL:   leaseTTL,
	}

	// ---- 4. Nodes ----
	master := NewNode(masterCfg, masterEng)
	heir := NewNode(heirCfg, heirEng)
	reader := NewNode(readerCfg, readerEng)

	master.SetLog(masterWal)
	heir.SetLog(heirWal)
	reader.SetLog(readerWal)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Start nodes.
	// Master has no Peers → no LeaseManager started.
	if err := master.Start(ctx); err != nil {
		t.Fatalf("master Start: %v", err)
	}
	t.Cleanup(func() { master.Stop() })

	// Heir with Peers=[] won't auto-start a LeaseManager; we'll create one
	// manually later after we know the Master's address.
	if err := heir.Start(ctx); err != nil {
		t.Fatalf("heir Start: %v", err)
	}
	t.Cleanup(func() { heir.Stop() })

	if err := reader.Start(ctx); err != nil {
		t.Fatalf("reader Start: %v", err)
	}
	t.Cleanup(func() { reader.Stop() })

	// ---- 5. Master StreamServer ----
	// Pre-bind a listener for a known address (port 0 → OS assigns).
	masterSrv, masterStreamAddr := newTestServer(t)
	masterSrv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		entryCh, errCh, cancel := masterWal.StreamFrom(lsn)
		// Discard errors in a goroutine — for test-only we log via t.Log but
		// we don't have t in scope here, so we use a simple drain goroutine.
		go func() {
			for range errCh {
			}
		}()
		return entryCh, cancel
	})
	t.Logf("Master StreamServer listening on %s", masterStreamAddr)
	t.Logf("Master LeaseManager listening on %s", master.config.ListenAddr)

	// ---- 6. StreamClients for Heir and Reader ----
	heirClient := NewStreamClient(masterStreamAddr, "heir", 0)
	if err := heirClient.Connect(ctx); err != nil {
		t.Fatalf("heir StreamClient Connect: %v", err)
	}
	t.Cleanup(func() { heirClient.Close() })

	readerClient := NewStreamClient(masterStreamAddr, "reader", 0)
	if err := readerClient.Connect(ctx); err != nil {
		t.Fatalf("reader StreamClient Connect: %v", err)
	}
	t.Cleanup(func() { readerClient.Close() })

	// ---- 7. Heir LeaseManager (manual, since Peers=[]) ----
	heirLM := NewLeaseManager(heir)
	if err := heirLM.Start(ctx); err != nil {
		t.Fatalf("heir LeaseManager Start: %v", err)
	}
	t.Cleanup(func() { heirLM.Stop() })

	// Seed the Master as alive in the Heir's lease table.
	// In production, the Master would send heartbeats to the Heir. Since the
	// Master has no LeaseManager in this test, we inject the status manually.
	heirLM.mu.Lock()
	heirLM.lastSeen["master"] = &peerStatus{
		LastSeen: time.Now(),
		Role:     RoleMaster,
	}
	heirLM.mu.Unlock()

	// =========================================================================
	// Phase 2 & 3 — Data replication & consistency
	// =========================================================================
	t.Log("=== Phase 2: Writing data via Master ===")

	// Helper: read and apply a given number of entries from a StreamClient.
	receiveAndApply := func(cl *StreamClient, node *Node, count int, label string, timeout time.Duration) []*LogEntry {
		t.Helper()
		var entries []*LogEntry
		deadline := time.After(timeout)
		for len(entries) < count {
			select {
			case entry := <-cl.Entries():
				entries = append(entries, entry)
				if err := node.ApplyEntry(entry); err != nil {
					t.Fatalf("%s ApplyEntry LSN=%d: %v", label, entry.LSN, err)
				}
				t.Logf("%s applied LSN=%d op=%s coll=%s doc=%s", label, entry.LSN, entry.Operation, entry.Collection, entry.DocID)
			case err := <-cl.Errors():
				t.Logf("%s stream error: %v", label, err)
			case <-deadline:
				t.Fatalf("%s timeout: got %d/%d entries", label, len(entries), count)
			}
		}
		return entries
	}

	// Write via Master: create a collection and insert two documents.
	if _, err := master.Write("items", OpCreateCollection, "", nil); err != nil {
		t.Fatalf("Master Write create_collection: %v", err)
	}
	if _, err := master.Write("items", OpInsert, "item-1", map[string]interface{}{"name": "Alpha", "value": float64(10)}); err != nil {
		t.Fatalf("Master Write insert item-1: %v", err)
	}
	if _, err := master.Write("items", OpInsert, "item-2", map[string]interface{}{"name": "Beta", "value": float64(20)}); err != nil {
		t.Fatalf("Master Write insert item-2: %v", err)
	}
	if _, err := master.Write("items", OpInsert, "item-3", map[string]interface{}{"name": "Gamma", "value": float64(30)}); err != nil {
		t.Fatalf("Master Write insert item-3: %v", err)
	}
	// Also write an update.
	if _, err := master.Write("items", OpUpdate, "item-1", map[string]interface{}{"name": "AlphaUpdated", "value": float64(11)}); err != nil {
		t.Fatalf("Master Write update item-1: %v", err)
	}

	// Total entries: 1 create_collection + 3 inserts + 1 update = 5
	const expectedEntryCount = 5

	receiveAndApply(heirClient, heir, expectedEntryCount, "heir", 10*time.Second)
	receiveAndApply(readerClient, reader, expectedEntryCount, "reader", 10*time.Second)

	// ---- Verify consistency ----
	t.Log("=== Phase 3: Verifying data consistency ===")

	checkEngine := func(label string, eng *db.Engine) {
		t.Helper()

		// item-1 should have updated values.
		doc, err := eng.Get("items", "item-1")
		if err != nil {
			t.Fatalf("%s Get item-1: %v", label, err)
		}
		if doc == nil {
			t.Fatalf("%s item-1 not found", label)
		}
		if doc["name"] != "AlphaUpdated" {
			t.Errorf("%s item-1 name = %q, want %q", label, doc["name"], "AlphaUpdated")
		}

		// item-2 should have initial values.
		doc, err = eng.Get("items", "item-2")
		if err != nil {
			t.Fatalf("%s Get item-2: %v", label, err)
		}
		if doc == nil {
			t.Fatalf("%s item-2 not found", label)
		}
		if doc["name"] != "Beta" {
			t.Errorf("%s item-2 name = %q, want %q", label, doc["name"], "Beta")
		}

		// item-3 should have initial values.
		doc, err = eng.Get("items", "item-3")
		if err != nil {
			t.Fatalf("%s Get item-3: %v", label, err)
		}
		if doc == nil {
			t.Fatalf("%s item-3 not found", label)
		}
		if doc["name"] != "Gamma" {
			t.Errorf("%s item-3 name = %q, want %q", label, doc["name"], "Gamma")
		}
	}

	checkEngine("master", masterEng)
	checkEngine("heir", heirEng)
	checkEngine("reader", readerEng)

	t.Log("Data consistency verified: all three engines match")

	// =========================================================================
	// Phase 4 — Failover: Master goes down, Heir promotes
	// =========================================================================
	t.Log("=== Phase 4: Failover ===")

	// Shut down Master's StreamServer first (so clients get errors).
	masterSrv.Shutdown(ctx)

	// Then shut down the Master node itself (stops the log/WAL).
	if err := master.Stop(); err != nil {
		t.Logf("master Stop (expected): %v", err)
	}

	// The Heir's monitorLoop checks every LeaseTTL/2 = 250ms.
	// The Master heartbeat was seeded with 500ms LeaseTTL, so it expires
	// after 2×LeaseTTL = 1s. Wait generously for the monitor to detect loss.
	t.Log("Waiting for Heir to detect Master failure and promote...")
	deadline := time.After(15 * time.Second)
	promoted := false
	for !promoted {
		select {
		case <-deadline:
			t.Fatalf("Heir did not promote within timeout (role=%s)", heir.Role())
		default:
			time.Sleep(100 * time.Millisecond)
			if heir.Role() == RoleMaster {
				promoted = true
			}
		}
	}

	t.Logf("Heir promoted to Master (role=%s)", heir.Role())

	// =========================================================================
	// Phase 5 — Reconnect & write more data
	// =========================================================================
	t.Log("=== Phase 5: Reconnect and write more data ===")

	// The Heir's WAL is independent of the Master's — entries received via
	// ApplyEntry are applied to the engine but NOT appended to the local WAL.
	// After promotion, the Heir writes its own entries to its WAL starting at
	// LSN 1.  The Reader reconnects with lastLSN=0 to receive the Heir's WAL
	// entries from the beginning.
	const readerLastLSN uint64 = 0
	t.Logf("Reader reconnecting with lastLSN=%d (Heir's WAL is independent)", readerLastLSN)

	// Start a StreamServer on the promoted Heir (now Master).
	heirSrv, heirStreamAddr := newTestServer(t)
	heirSrv.SetLogProvider(func(lsn uint64) (<-chan *LogEntry, context.CancelFunc) {
		entryCh, errCh, cancel := heirWal.StreamFrom(lsn)
		go func() {
			for range errCh {
			}
		}()
		return entryCh, cancel
	})
	t.Logf("Heir (new Master) StreamServer listening on %s", heirStreamAddr)

	// Close the old Reader client (it's connected to the dead Master).
	readerClient.Close()

	// Create a new StreamClient connecting to the promoted Heir.
	readerClient2 := NewStreamClient(heirStreamAddr, "reader", readerLastLSN)
	if err := readerClient2.Connect(ctx); err != nil {
		t.Fatalf("reader reconnect to Heir StreamClient Connect: %v", err)
	}
	t.Cleanup(func() { readerClient2.Close() })

	// Write more data from the promoted Heir (now Master).
	if _, err := heir.Write("items", OpInsert, "item-4", map[string]interface{}{"name": "Delta", "value": float64(40)}); err != nil {
		t.Fatalf("Heir (new Master) Write insert item-4: %v", err)
	}
	if _, err := heir.Write("items", OpInsert, "item-5", map[string]interface{}{"name": "Epsilon", "value": float64(50)}); err != nil {
		t.Fatalf("Heir (new Master) Write insert item-5: %v", err)
	}
	// Also write an update to an existing doc.
	if _, err := heir.Write("items", OpUpdate, "item-2", map[string]interface{}{"name": "BetaUpdated", "value": float64(21)}); err != nil {
		t.Fatalf("Heir (new Master) Write update item-2: %v", err)
	}

	// 3 new entries: 2 inserts + 1 update.
	receiveAndApply(readerClient2, reader, 3, "reader-reconnect", 10*time.Second)

	// Verify Reader now has the new data.
	t.Log("=== Verifying data after failover ===")

	// item-4 should exist.
	doc, err := readerEng.Get("items", "item-4")
	if err != nil {
		t.Fatalf("reader Get item-4 after failover: %v", err)
	}
	if doc == nil {
		t.Fatal("reader item-4 not found after failover")
	}
	if doc["name"] != "Delta" {
		t.Errorf("reader item-4 name = %q, want %q", doc["name"], "Delta")
	}

	// item-5 should exist.
	doc, err = readerEng.Get("items", "item-5")
	if err != nil {
		t.Fatalf("reader Get item-5 after failover: %v", err)
	}
	if doc == nil {
		t.Fatal("reader item-5 not found after failover")
	}
	if doc["name"] != "Epsilon" {
		t.Errorf("reader item-5 name = %q, want %q", doc["name"], "Epsilon")
	}

	// item-2 should have been updated by the new Master.
	doc, err = readerEng.Get("items", "item-2")
	if err != nil {
		t.Fatalf("reader Get item-2 after failover: %v", err)
	}
	if doc == nil {
		t.Fatal("reader item-2 not found after failover")
	}
	if doc["name"] != "BetaUpdated" {
		t.Errorf("reader item-2 name after failover = %q, want %q", doc["name"], "BetaUpdated")
	}

	// Original data should still be intact.
	doc, err = readerEng.Get("items", "item-1")
	if err != nil {
		t.Fatalf("reader Get item-1 after failover: %v", err)
	}
	if doc == nil {
		t.Fatal("reader item-1 missing after failover")
	}
	if doc["name"] != "AlphaUpdated" {
		t.Errorf("reader item-1 name after failover = %q, want %q", doc["name"], "AlphaUpdated")
	}

	t.Log("=== ALL PHASES PASSED ===")
}
