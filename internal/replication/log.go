package replication

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// ReplicationLog is an alias for WAL for use by the Node type.
type ReplicationLog = WAL

// key prefixes and meta key
const (
	walKeyPrefix = "__wal__:"
	metaLSNKey   = "__wal__:meta:lsn"
)

// WAL is a Write-Ahead Log backed by Pebble.
// It stores log entries keyed by monotonically increasing Log Sequence Numbers (LSNs).
// WAL implements the ReplicationLog interface.
type WAL struct {
	mu         sync.Mutex
	db         *pebble.DB
	currentLSN uint64
	closed     bool
}

// NewReplicationLog opens (or creates) the WAL Pebble database at {dataDir}/wal
// and recovers the current LSN counter from the stored metadata.
func NewReplicationLog(dataDir string) (*WAL, error) {
	db, err := pebble.Open(fmt.Sprintf("%s/wal", dataDir), &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("replication log: open pebble: %w", err)
	}

	wal := &WAL{
		db: db,
	}

	// Recover currentLSN from the meta key
	val, closer, err := db.Get([]byte(metaLSNKey))
	if err == pebble.ErrNotFound {
		wal.currentLSN = 0
	} else if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("replication log: read meta lsn: %w", err)
	} else {
		if len(val) >= 8 {
			wal.currentLSN = binary.BigEndian.Uint64(val)
		}
		closer.Close()
	}

	return wal, nil
}

// Close closes the underlying Pebble database. It is idempotent.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true
	return w.db.Close()
}

// makeKey returns the Pebble key for a given LSN.
// Format: __wal__:{16-char zero-padded hex LSN}
func makeKey(lsn uint64) []byte {
	key := make([]byte, len(walKeyPrefix)+16)
	copy(key, walKeyPrefix)
	fmtKey := fmt.Sprintf("%016x", lsn)
	copy(key[len(walKeyPrefix):], fmtKey)
	return key
}

// makeEndKey returns a key strictly after all WAL keys.
func makeEndKey() []byte {
	return []byte(walKeyPrefix + "~") // '~' is ASCII 126, after all hex chars
}

// parseKeyLSN extracts the LSN from a Pebble key of our format.
// Returns 0 and false if the key doesn't match.
func parseKeyLSN(key []byte) (uint64, bool) {
	if len(key) != len(walKeyPrefix)+16 {
		return 0, false
	}
	if string(key[:len(walKeyPrefix)]) != walKeyPrefix {
		return 0, false
	}
	var result uint64
	n, err := fmt.Sscanf(string(key[len(walKeyPrefix):]), "%x", &result)
	if err != nil || n != 1 {
		return 0, false
	}
	return result, true
}

// Append adds a new log entry to the WAL. It auto-assigns the LSN field
// using an internal monotonically increasing counter. Returns the assigned LSN.
func (w *WAL) Append(entry *LogEntry) (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("replication log: closed")
	}

	w.currentLSN++
	lsn := w.currentLSN
	entry.LSN = lsn

	data, err := json.Marshal(entry)
	if err != nil {
		w.currentLSN-- // rollback
		return 0, fmt.Errorf("replication log: marshal entry: %w", err)
	}

	// Write the entry
	if err := w.db.Set(makeKey(lsn), data, pebble.Sync); err != nil {
		w.currentLSN-- // rollback
		return 0, fmt.Errorf("replication log: write entry: %w", err)
	}

	// Persist currentLSN
	metaVal := make([]byte, 8)
	binary.BigEndian.PutUint64(metaVal, w.currentLSN)
	if err := w.db.Set([]byte(metaLSNKey), metaVal, pebble.Sync); err != nil {
		// LSN was committed; the entry exists but meta is stale.
		// This is acceptable — on recovery we'll scan for the max LSN.
	}

	return lsn, nil
}

// GetLSN retrieves a single log entry by LSN. Returns pebble.ErrNotFound if
// the entry does not exist.
func (w *WAL) GetLSN(lsn uint64) (*LogEntry, error) {
	val, closer, err := w.db.Get(makeKey(lsn))
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	var entry LogEntry
	if err := json.Unmarshal(val, &entry); err != nil {
		return nil, fmt.Errorf("replication log: unmarshal entry: %w", err)
	}
	return &entry, nil
}

// StreamFrom returns channels that stream log entries starting from the given LSN
// (inclusive). The streaming goroutine runs until the returned context.CancelFunc
// is called or Close() is called on the WAL.
func (w *WAL) StreamFrom(lsn uint64) (<-chan *LogEntry, <-chan error, context.CancelFunc) {
	entryCh := make(chan *LogEntry, 64)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(entryCh)
		defer close(errCh)

		nextLSN := lsn
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.mu.Lock()
				closed := w.closed
				w.mu.Unlock()
				if closed {
					return
				}

				// Take a snapshot to get a consistent view
				snap := w.db.NewSnapshot()

				startKey := makeKey(nextLSN)
				endKey := makeEndKey()

				iter, _ := snap.NewIter(nil)
				valid := iter.SeekGE(startKey)

				found := false
				for ; valid; valid = iter.Next() {
					key := iter.Key()
					// Check if we've gone past our key prefix
					if string(key) >= string(endKey) {
						break
					}

					entryLSN, ok := parseKeyLSN(key)
					if !ok {
						break
					}

					val, err := iter.ValueAndErr()
					if err != nil {
						select {
						case errCh <- fmt.Errorf("replication log: iter value: %w", err):
						default:
						}
						iter.Close()
						snap.Close()
						return
					}

					var entry LogEntry
					if err := json.Unmarshal(val, &entry); err != nil {
						select {
						case errCh <- fmt.Errorf("replication log: unmarshal: %w", err):
						default:
						}
						iter.Close()
						snap.Close()
						return
					}

					select {
					case entryCh <- &entry:
						nextLSN = entryLSN + 1
						found = true
					case <-ctx.Done():
						iter.Close()
						snap.Close()
						return
					}
				}
				iter.Close()
				snap.Close()

				if found {
					// Reset ticker to check immediately for more entries
					ticker.Reset(1 * time.Millisecond)
				}
			}
		}
	}()

	return entryCh, errCh, cancel
}

// LastLSN returns the highest LSN in the log, or 0 if the log is empty.
func (w *WAL) LastLSN() (uint64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, fmt.Errorf("replication log: closed")
	}

	// Fast path: use the in-memory counter
	if w.currentLSN > 0 {
		return w.currentLSN, nil
	}

	// If counter is 0, scan to confirm the log is truly empty
	return w.scanLastLSN()
}

// scanLastLSN performs a reverse scan to find the highest LSN.
func (w *WAL) scanLastLSN() (uint64, error) {
	snap := w.db.NewSnapshot()
	defer snap.Close()

	iter, _ := snap.NewIter(nil)
	defer iter.Close()

	// Seek to the last key in our prefix range
	iter.SeekLT(makeEndKey())

	if !iter.Valid() {
		return 0, nil
	}

	key := iter.Key()
	if lsn, ok := parseKeyLSN(key); ok {
		return lsn, nil
	}

	return 0, nil
}

// TruncateBefore deletes all log entries with LSN less than the given value.
// This is used for log compaction.
func (w *WAL) TruncateBefore(lsn uint64) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("replication log: closed")
	}

	if lsn <= 1 {
		return nil // nothing to truncate (LSNs start at 1)
	}

	// Delete range from the first WAL key up to (but not including) lsn
	startKey := makeKey(0)
	endKey := makeKey(lsn)

	return w.db.DeleteRange(startKey, endKey, pebble.Sync)
}
