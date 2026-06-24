package replication

import (
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sovrabase-repl-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestAppendAndGet(t *testing.T) {
	dir := tempDir(t)
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}
	defer log.Close()

	// Append 5 entries
	var lsns []uint64
	for i := 0; i < 5; i++ {
		entry := &LogEntry{
			Timestamp:  time.Now(),
			Operation:  OpInsert,
			Collection: "test_coll",
			DocID:      "doc-" + string(rune('a'+i)),
			Data:       []byte(`{"value":` + string(rune('1'+i)) + `}`),
		}
		lsn, err := log.Append(entry)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
		lsns = append(lsns, lsn)
	}

	// Verify LSNs are monotonically increasing
	for i := 1; i < len(lsns); i++ {
		if lsns[i] <= lsns[i-1] {
			t.Errorf("LSN not monotonic: %d <= %d", lsns[i], lsns[i-1])
		}
	}

	// Retrieve each entry
	for i, lsn := range lsns {
		entry, err := log.GetLSN(lsn)
		if err != nil {
			t.Fatalf("GetLSN(%d): %v", lsn, err)
		}
		if entry.LSN != lsn {
			t.Errorf("entry.LSN = %d, want %d", entry.LSN, lsn)
		}
		if entry.Collection != "test_coll" {
			t.Errorf("entry.Collection = %q, want %q", entry.Collection, "test_coll")
		}
		expectedDocID := "doc-" + string(rune('a'+i))
		if entry.DocID != expectedDocID {
			t.Errorf("entry.DocID = %q, want %q", entry.DocID, expectedDocID)
		}
	}

	// Verify non-existent LSN returns ErrNotFound
	_, err = log.GetLSN(99999)
	if err != pebble.ErrNotFound {
		t.Errorf("GetLSN(99999) error = %v, want ErrNotFound", err)
	}
}

func TestStreamFrom(t *testing.T) {
	dir := tempDir(t)
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}
	defer log.Close()

	// Append 10 entries
	for i := 0; i < 10; i++ {
		entry := &LogEntry{
			Timestamp:  time.Now(),
			Operation:  OpInsert,
			Collection: "stream_coll",
			DocID:      "sdoc-" + string(rune('0'+i)),
			Data:       []byte(`{"idx":` + string(rune('0'+i)) + `}`),
		}
		_, err := log.Append(entry)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// Stream from LSN 5 (entries 5-10 should arrive)
	entryCh, errCh, cancel := log.StreamFrom(5)
	defer cancel()

	var entries []*LogEntry
	timeout := time.After(3 * time.Second)

collectLoop:
	for {
		select {
		case entry, ok := <-entryCh:
			if !ok {
				break collectLoop
			}
			entries = append(entries, entry)
		case err, ok := <-errCh:
			if ok && err != nil {
				t.Fatalf("stream error: %v", err)
			}
		case <-timeout:
			cancel()
			// Drain channels
			for range entryCh {
			}
			for range errCh {
			}
			break collectLoop
		}
	}

	// We should have at least entries 5-10 (6 entries)
	if len(entries) < 6 {
		t.Fatalf("got %d entries from stream, want at least 6 (LSNs 5-10)", len(entries))
	}

	// Verify the first entry has LSN >= 5
	if entries[0].LSN < 5 {
		t.Errorf("first streamed entry LSN = %d, want >= 5", entries[0].LSN)
	}

	// Verify LSNs are in order
	for i := 1; i < len(entries); i++ {
		if entries[i].LSN <= entries[i-1].LSN {
			t.Errorf("stream LSNs not monotonic: %d after %d", entries[i].LSN, entries[i-1].LSN)
		}
	}
}

func TestLastLSN(t *testing.T) {
	dir := tempDir(t)
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}
	defer log.Close()

	// Empty log returns 0
	last, err := log.LastLSN()
	if err != nil {
		t.Fatalf("LastLSN (empty): %v", err)
	}
	if last != 0 {
		t.Errorf("LastLSN on empty log = %d, want 0", last)
	}

	// Append 3 entries
	for i := 0; i < 3; i++ {
		entry := &LogEntry{
			Timestamp:  time.Now(),
			Operation:  OpInsert,
			Collection: "last_coll",
			DocID:      "last-doc",
		}
		_, err := log.Append(entry)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	last, err = log.LastLSN()
	if err != nil {
		t.Fatalf("LastLSN: %v", err)
	}
	if last != 3 {
		t.Errorf("LastLSN = %d, want 3", last)
	}
}

func TestTruncateBefore(t *testing.T) {
	dir := tempDir(t)
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}
	defer log.Close()

	// Append 5 entries
	for i := 0; i < 5; i++ {
		entry := &LogEntry{
			Timestamp:  time.Now(),
			Operation:  OpInsert,
			Collection: "trunc_coll",
			DocID:      "tdoc-" + string(rune('0'+i)),
		}
		_, err := log.Append(entry)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	// Truncate entries before LSN 3 (i.e., delete LSN 1 and 2)
	err = log.TruncateBefore(3)
	if err != nil {
		t.Fatalf("TruncateBefore(3): %v", err)
	}

	// Entries 1-2 should be gone
	_, err = log.GetLSN(1)
	if err != pebble.ErrNotFound {
		t.Errorf("GetLSN(1) after truncate: error = %v, want ErrNotFound", err)
	}
	_, err = log.GetLSN(2)
	if err != pebble.ErrNotFound {
		t.Errorf("GetLSN(2) after truncate: error = %v, want ErrNotFound", err)
	}

	// Entries 3-5 should still exist
	for lsn := uint64(3); lsn <= 5; lsn++ {
		entry, err := log.GetLSN(lsn)
		if err != nil {
			t.Errorf("GetLSN(%d) after truncate: %v", lsn, err)
		}
		if entry.LSN != lsn {
			t.Errorf("entry.LSN = %d, want %d", entry.LSN, lsn)
		}
	}
}

func TestIdempotentClose(t *testing.T) {
	dir := tempDir(t)
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}

	// First close
	if err := log.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second close should not panic
	if err := log.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestRecoveryPreservesLSN(t *testing.T) {
	dir := tempDir(t)

	// Create log, append entries, close
	log, err := NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("NewReplicationLog: %v", err)
	}

	for i := 0; i < 5; i++ {
		entry := &LogEntry{
			Timestamp:  time.Now(),
			Operation:  OpInsert,
			Collection: "recover_coll",
			DocID:      "rdoc-" + string(rune('0'+i)),
		}
		_, err := log.Append(entry)
		if err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	log.Close()

	// Reopen and verify LSN counter is preserved
	log, err = NewReplicationLog(dir)
	if err != nil {
		t.Fatalf("reopen NewReplicationLog: %v", err)
	}
	defer log.Close()

	last, err := log.LastLSN()
	if err != nil {
		t.Fatalf("LastLSN after recovery: %v", err)
	}
	if last != 5 {
		t.Errorf("LastLSN after recovery = %d, want 5", last)
	}

	// Append one more and verify it gets LSN 6
	entry := &LogEntry{
		Timestamp:  time.Now(),
		Operation:  OpInsert,
		Collection: "recover_coll",
		DocID:      "rdoc-recovered",
	}
	lsn, err := log.Append(entry)
	if err != nil {
		t.Fatalf("Append after recovery: %v", err)
	}
	if lsn != 6 {
		t.Errorf("LSN after recovery = %d, want 6", lsn)
	}
}
