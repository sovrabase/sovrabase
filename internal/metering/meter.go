// Package metering implements per-project usage counters backed by Pebble.
//
// Each counter is stored as a separate Pebble key for atomic increment:
//
//	"metering:{projectID}:api_requests"           → int64 (big-endian)
//	"metering:{projectID}:storage_bytes"          → int64
//	"metering:{projectID}:bandwidth_up"           → int64
//	"metering:{projectID}:bandwidth_down"         → int64
//	"metering:{projectID}:db_reads"               → int64
//	"metering:{projectID}:db_writes"              → int64
//	"metering:{projectID}:realtime_connections"   → int64
//	"metering:{projectID}:method:{METHOD}"        → int64 (per-HTTP-method counter)
//	"metering:{projectID}:period_start"           → RFC3339 timestamp
//	"metering:{projectID}:last_updated"           → RFC3339 timestamp
package metering

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/cockroachdb/pebble"
)

// MeterRecord holds a snapshot of a project's usage counters.
type MeterRecord struct {
	ProjectID              string           `json:"project_id"`
	APIRequestsTotal       int64            `json:"api_requests_total"`
	APIRequestsByMethod    map[string]int64 `json:"api_requests_by_method"`
	StorageBytes           int64            `json:"storage_bytes"`
	BandwidthUploadBytes   int64            `json:"bandwidth_upload_bytes"`
	BandwidthDownloadBytes int64            `json:"bandwidth_download_bytes"`
	DBReadsTotal           int64            `json:"db_reads_total"`
	DBWritesTotal          int64            `json:"db_writes_total"`
	RealtimeConnections    int64            `json:"realtime_connections"`
	PeriodStart            time.Time        `json:"period_start"`
	LastUpdated            time.Time        `json:"last_updated"`
}

// MeterStore persists per-project usage counters in a Pebble database.
type MeterStore struct {
	mu sync.RWMutex
	db *pebble.DB
}

// NewMeterStore creates a MeterStore backed by the given Pebble database.
func NewMeterStore(db *pebble.DB) *MeterStore {
	return &MeterStore{db: db}
}

// OpenMeterStore opens (or creates) a Pebble database at the given path and
// returns a ready-to-use MeterStore. This is a convenience helper for main.go.
func OpenMeterStore(dbPath string) (*MeterStore, error) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("metering: open db: %w", err)
	}
	return &MeterStore{db: db}, nil
}

// Close shuts down the underlying Pebble database.
func (ms *MeterStore) Close() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.db.Close()
}

// Valid metric names.
const (
	MetricAPIRequests         = "api_requests"
	MetricStorageBytes        = "storage_bytes"
	MetricBandwidthUp         = "bandwidth_up"
	MetricBandwidthDown       = "bandwidth_down"
	MetricDBReads             = "db_reads"
	MetricDBWrites            = "db_writes"
	MetricRealtimeConnections = "realtime_connections"
)

// Inc increments the specified metric for a project by delta.
func (ms *MeterStore) Inc(projectID, metric string, delta int64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	key := metricKey(projectID, metric)
	return ms.addToCounter(key, delta, time.Now())
}

// IncMethod increments the API requests total AND the per-method counter for
// the given HTTP method in a single operation.
func (ms *MeterStore) IncMethod(projectID, method string, delta int64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	now := time.Now()
	// Increment total counter
	if err := ms.addToCounter(metricKey(projectID, MetricAPIRequests), delta, now); err != nil {
		return err
	}
	// Increment per-method counter
	methodKey := []byte(fmt.Sprintf("metering:%s:method:%s", projectID, method))
	return ms.addToCounter(methodKey, delta, now)
}

// addToCounter reads the current value at key, adds delta, writes it back, and
// updates the last_updated timestamp. Caller must hold ms.mu.
func (ms *MeterStore) addToCounter(key []byte, delta int64, now time.Time) error {
	val, closer, err := ms.db.Get(key)
	var current int64
	if err == pebble.ErrNotFound {
		current = 0
	} else if err != nil {
		return fmt.Errorf("metering: get counter: %w", err)
	} else {
		current = int64(binary.BigEndian.Uint64(val))
		closer.Close()
	}

	newVal := current + delta
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(newVal))

	if err := ms.db.Set(key, buf, pebble.Sync); err != nil {
		return fmt.Errorf("metering: set counter: %w", err)
	}

	// Update last_updated timestamp
	tsKey := []byte(fmt.Sprintf("metering:%s:last_updated", extractProjectID(key)))
	return ms.db.Set(tsKey, []byte(now.UTC().Format(time.RFC3339)), pebble.Sync)
}

// extractProjectID pulls the project ID from a key like "metering:{id}:..."
func extractProjectID(key []byte) string {
	s := string(key)
	// Skip "metering:" (9 bytes)
	rest := s[9:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			return rest[:i]
		}
	}
	return rest
}

// Get retrieves all counters for a project as a MeterRecord.
func (ms *MeterStore) Get(projectID string) (*MeterRecord, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	rec := &MeterRecord{
		ProjectID:           projectID,
		APIRequestsByMethod: make(map[string]int64),
		PeriodStart:         time.Now().UTC(),
	}

	var err error
	rec.APIRequestsTotal, err = ms.readInt64(metricKey(projectID, MetricAPIRequests))
	if err != nil {
		return nil, err
	}
	rec.StorageBytes, err = ms.readInt64(metricKey(projectID, MetricStorageBytes))
	if err != nil {
		return nil, err
	}
	rec.BandwidthUploadBytes, err = ms.readInt64(metricKey(projectID, MetricBandwidthUp))
	if err != nil {
		return nil, err
	}
	rec.BandwidthDownloadBytes, err = ms.readInt64(metricKey(projectID, MetricBandwidthDown))
	if err != nil {
		return nil, err
	}
	rec.DBReadsTotal, err = ms.readInt64(metricKey(projectID, MetricDBReads))
	if err != nil {
		return nil, err
	}
	rec.DBWritesTotal, err = ms.readInt64(metricKey(projectID, MetricDBWrites))
	if err != nil {
		return nil, err
	}
	rec.RealtimeConnections, err = ms.readInt64(metricKey(projectID, MetricRealtimeConnections))
	if err != nil {
		return nil, err
	}

	// Read per-method counters by iterating over the method key prefix
	methodPrefix := []byte(fmt.Sprintf("metering:%s:method:", projectID))
	mIter, err := ms.db.NewIter(&pebble.IterOptions{
		LowerBound: methodPrefix,
		UpperBound: keyUpperBound(methodPrefix),
	})
	if err != nil {
		return nil, fmt.Errorf("metering: iterate methods: %w", err)
	}
	defer mIter.Close()
	for mIter.First(); mIter.Valid(); mIter.Next() {
		method := string(mIter.Key()[len(methodPrefix):])
		rec.APIRequestsByMethod[method] = int64(binary.BigEndian.Uint64(mIter.Value()))
	}
	if err := mIter.Error(); err != nil {
		return nil, fmt.Errorf("metering: iterate methods error: %w", err)
	}

	// Read timestamps
	if ps, err := ms.readString(periodStartKey(projectID)); err == nil && ps != "" {
		if t, err := time.Parse(time.RFC3339, ps); err == nil {
			rec.PeriodStart = t
		}
	}
	if lu, err := ms.readString(lastUpdatedKey(projectID)); err == nil && lu != "" {
		if t, err := time.Parse(time.RFC3339, lu); err == nil {
			rec.LastUpdated = t
		}
	}

	return rec, nil
}

// GetStorageUsage returns the storage_bytes counter for a project. Useful for
// quick quota checks without building the full MeterRecord.
func (ms *MeterStore) GetStorageUsage(projectID string) (int64, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.readInt64(metricKey(projectID, MetricStorageBytes))
}

// ListAll returns MeterRecords for every project that has at least one
// counter stored in the metering database.
func (ms *MeterStore) ListAll() ([]*MeterRecord, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	projectIDs, err := ms.collectProjectIDs()
	if err != nil {
		return nil, err
	}

	records := make([]*MeterRecord, 0, len(projectIDs))
	for _, pid := range projectIDs {
		rec := &MeterRecord{
			ProjectID:           pid,
			APIRequestsByMethod: make(map[string]int64),
			PeriodStart:         time.Now().UTC(),
		}

		rec.APIRequestsTotal, _ = ms.readInt64(metricKey(pid, MetricAPIRequests))
		rec.StorageBytes, _ = ms.readInt64(metricKey(pid, MetricStorageBytes))
		rec.BandwidthUploadBytes, _ = ms.readInt64(metricKey(pid, MetricBandwidthUp))
		rec.BandwidthDownloadBytes, _ = ms.readInt64(metricKey(pid, MetricBandwidthDown))
		rec.DBReadsTotal, _ = ms.readInt64(metricKey(pid, MetricDBReads))
		rec.DBWritesTotal, _ = ms.readInt64(metricKey(pid, MetricDBWrites))
		rec.RealtimeConnections, _ = ms.readInt64(metricKey(pid, MetricRealtimeConnections))

		methodPrefix := []byte(fmt.Sprintf("metering:%s:method:", pid))
		mIter, err := ms.db.NewIter(&pebble.IterOptions{
			LowerBound: methodPrefix,
			UpperBound: keyUpperBound(methodPrefix),
		})
		if err == nil {
			for mIter.First(); mIter.Valid(); mIter.Next() {
				method := string(mIter.Key()[len(methodPrefix):])
				rec.APIRequestsByMethod[method] = int64(binary.BigEndian.Uint64(mIter.Value()))
			}
			mIter.Close()
		}

		if ps, err := ms.readString(periodStartKey(pid)); err == nil && ps != "" {
			if t, err := time.Parse(time.RFC3339, ps); err == nil {
				rec.PeriodStart = t
			}
		}
		if lu, err := ms.readString(lastUpdatedKey(pid)); err == nil && lu != "" {
			if t, err := time.Parse(time.RFC3339, lu); err == nil {
				rec.LastUpdated = t
			}
		}

		records = append(records, rec)
	}
	return records, nil
}

// Reset clears all counters for a project.
func (ms *MeterStore) Reset(projectID string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	prefix := []byte(fmt.Sprintf("metering:%s:", projectID))
	return ms.deleteByPrefix(prefix)
}

// ResetAll clears all counters for every project.
func (ms *MeterStore) ResetAll() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	prefix := []byte("metering:")
	return ms.deleteByPrefix(prefix)
}

// deleteByPrefix removes all keys that start with the given prefix. Caller
// must hold ms.mu.
func (ms *MeterStore) deleteByPrefix(prefix []byte) error {
	iter, err := ms.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("metering: iter for delete: %w", err)
	}
	defer iter.Close()

	b := ms.db.NewBatch()
	defer b.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if err := b.Delete(iter.Key(), pebble.Sync); err != nil {
			return fmt.Errorf("metering: batch delete: %w", err)
		}
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("metering: iter error: %w", err)
	}

	return b.Commit(pebble.Sync)
}

// collectProjectIDs iterates all "metering:" keys and returns the unique set
// of project IDs found.
func (ms *MeterStore) collectProjectIDs() ([]string, error) {
	prefix := []byte("metering:")
	seen := make(map[string]struct{})

	iter, err := ms.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("metering: iter for list: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		pid := extractProjectID(iter.Key())
		seen[pid] = struct{}{}
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("metering: iter error: %w", err)
	}

	ids := make([]string, 0, len(seen))
	for pid := range seen {
		ids = append(ids, pid)
	}
	return ids, nil
}

// --- internal helpers ---

func metricKey(projectID, metric string) []byte {
	return []byte(fmt.Sprintf("metering:%s:%s", projectID, metric))
}

func periodStartKey(projectID string) []byte {
	return []byte(fmt.Sprintf("metering:%s:period_start", projectID))
}

func lastUpdatedKey(projectID string) []byte {
	return []byte(fmt.Sprintf("metering:%s:last_updated", projectID))
}

// readInt64 reads a big-endian int64 from the DB. Returns 0 if key not found.
func (ms *MeterStore) readInt64(key []byte) (int64, error) {
	val, closer, err := ms.db.Get(key)
	if err == pebble.ErrNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("metering: read int64: %w", err)
	}
	defer closer.Close()
	return int64(binary.BigEndian.Uint64(val)), nil
}

// readString reads a string value from the DB. Returns empty string if not found.
func (ms *MeterStore) readString(key []byte) (string, error) {
	val, closer, err := ms.db.Get(key)
	if err == pebble.ErrNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("metering: read string: %w", err)
	}
	defer closer.Close()
	return string(val), nil
}

// keyUpperBound returns an exclusive upper bound for iteration.
func keyUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return append(prefix, 0x00)
}
