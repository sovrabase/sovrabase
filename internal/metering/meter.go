// Package metering implements per-project usage counters backed by Pebble.
//
// Hot-path Inc/IncMethod calls use lock-free atomic.Int64 counters in memory.
// A background goroutine flushes dirty counters to Pebble every 10 seconds,
// reducing write contention from one global lock per request to zero.
//
// Each counter is stored as a separate Pebble key for persistence:
//
//	"metering:{projectID}:api_requests"           -> int64 (big-endian)
//	"metering:{projectID}:storage_bytes"          -> int64
//	"metering:{projectID}:bandwidth_up"           -> int64
//	"metering:{projectID}:bandwidth_down"         -> int64
//	"metering:{projectID}:db_reads"               -> int64
//	"metering:{projectID}:db_writes"              -> int64
//	"metering:{projectID}:realtime_connections"   -> int64
//	"metering:{projectID}:method:{METHOD}"        -> int64 (per-HTTP-method counter)
//	"metering:{projectID}:period_start"           -> RFC3339 timestamp
//	"metering:{projectID}:last_updated"           -> RFC3339 timestamp
package metering

import (
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
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
// Hot-path writes go to in-memory atomics; a background goroutine flushes to Pebble.
type MeterStore struct {
	db        *pebble.DB
	deltas    sync.Map // key: string ("projectID:metric") -> *atomic.Int64
	stopCh    chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewMeterStore creates a MeterStore backed by the given Pebble database.
// The caller must call Close() to stop the flush goroutine.
func NewMeterStore(db *pebble.DB) *MeterStore {
	ms := &MeterStore{
		db:     db,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
	go ms.flushLoop()
	return ms
}

// OpenMeterStore opens (or creates) a Pebble database at the given path and
// returns a ready-to-use MeterStore. This is a convenience helper for main.go.
func OpenMeterStore(dbPath string) (*MeterStore, error) {
	db, err := pebble.Open(dbPath, &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("metering: open db: %w", err)
	}
	return NewMeterStore(db), nil
}

// Close flushes pending counters and shuts down the Pebble database.
func (ms *MeterStore) Close() error {
	var err error
	ms.closeOnce.Do(func() {
		close(ms.stopCh)
		<-ms.done
		err = ms.db.Close()
	})
	return err
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

const flushInterval = 10 * time.Second

// getDeltaCounter returns the atomic counter for the given key, creating it if
// necessary. Uses LoadOrStore so concurrent calls for the same key are safe.
func (ms *MeterStore) getDeltaCounter(key string) *atomic.Int64 {
	val, _ := ms.deltas.LoadOrStore(key, new(atomic.Int64))
	return val.(*atomic.Int64)
}

// Inc increments the specified metric for a project by delta.
// Lock-free: just adds to the in-memory atomic counter.
func (ms *MeterStore) Inc(projectID, metric string, delta int64) error {
	ms.getDeltaCounter(deltaKey(projectID, metric)).Add(delta)
	return nil
}

// IncMethod increments the API requests total AND the per-method counter.
func (ms *MeterStore) IncMethod(projectID, method string, delta int64) error {
	ms.getDeltaCounter(deltaKey(projectID, MetricAPIRequests)).Add(delta)
	ms.getDeltaCounter(deltaKey(projectID, "method:"+method)).Add(delta)
	return nil
}

// flushLoop periodically writes in-memory deltas to Pebble.
func (ms *MeterStore) flushLoop() {
	defer close(ms.done)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ms.flush()
		case <-ms.stopCh:
			ms.flush() // final flush on shutdown
			return
		}
	}
}

// flush writes all dirty counters to Pebble in a single batch, then resets the
// in-memory atomics to zero. Uses CAS-style swap to avoid losing increments
// that arrive during the flush.
func (ms *MeterStore) flush() {
	batch := ms.db.NewBatch()
	defer batch.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	flushedProjects := make(map[string]bool)

	ms.deltas.Range(func(key, val any) bool {
		counter := val.(*atomic.Int64)
		delta := counter.Swap(0)
		if delta == 0 {
			return true
		}

		keyStr := key.(string)
		pid, metric := splitDeltaKey(keyStr)

		// Read current Pebble value, add delta, write back
		var current int64
		pebKey := metricKey(pid, metric)
		existing, closer, err := ms.db.Get(pebKey)
		if err == nil {
			current = int64(binary.BigEndian.Uint64(existing))
			closer.Close()
		}

		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(current+delta))
		_ = batch.Set(pebKey, buf, nil)
		flushedProjects[pid] = true
		return true
	})

	// Update last_updated for each dirty project
	for pid := range flushedProjects {
		_ = batch.Set([]byte(fmt.Sprintf("metering:%s:last_updated", pid)), []byte(now), nil)
	}

	if len(flushedProjects) > 0 {
		_ = batch.Commit(pebble.Sync)
	}
}

// --- Read methods ---

// Get retrieves all counters for a project as a MeterRecord.
// Merges in-memory unflushed deltas with persisted Pebble values.
func (ms *MeterStore) Get(projectID string) (*MeterRecord, error) {
	rec := &MeterRecord{
		ProjectID:           projectID,
		APIRequestsByMethod: make(map[string]int64),
		PeriodStart:         time.Now().UTC(),
	}

	var err error
	rec.APIRequestsTotal, err = ms.readInt64WithDelta(projectID, MetricAPIRequests)
	if err != nil {
		return nil, err
	}
	rec.StorageBytes, err = ms.readInt64WithDelta(projectID, MetricStorageBytes)
	if err != nil {
		return nil, err
	}
	rec.BandwidthUploadBytes, err = ms.readInt64WithDelta(projectID, MetricBandwidthUp)
	if err != nil {
		return nil, err
	}
	rec.BandwidthDownloadBytes, err = ms.readInt64WithDelta(projectID, MetricBandwidthDown)
	if err != nil {
		return nil, err
	}
	rec.DBReadsTotal, err = ms.readInt64WithDelta(projectID, MetricDBReads)
	if err != nil {
		return nil, err
	}
	rec.DBWritesTotal, err = ms.readInt64WithDelta(projectID, MetricDBWrites)
	if err != nil {
		return nil, err
	}
	rec.RealtimeConnections, err = ms.readInt64WithDelta(projectID, MetricRealtimeConnections)
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
		val := int64(binary.BigEndian.Uint64(mIter.Value()))
		// Add unflushed delta
		if d, ok := ms.deltas.Load(deltaKey(projectID, "method:"+method)); ok {
			val += d.(*atomic.Int64).Load()
		}
		rec.APIRequestsByMethod[method] = val
	}
	if err := mIter.Error(); err != nil {
		return nil, fmt.Errorf("metering: iterate methods error: %w", err)
	}

	// Scan in-memory deltas for method counters not yet flushed to Pebble
	ms.deltas.Range(func(key, val any) bool {
		keyStr := key.(string)
		pid, metric := splitDeltaKey(keyStr)
		if pid != projectID || len(metric) <= 7 || metric[:7] != "method:" {
			return true
		}
		method := metric[7:]
		if _, exists := rec.APIRequestsByMethod[method]; !exists {
			rec.APIRequestsByMethod[method] = val.(*atomic.Int64).Load()
		}
		return true
	})

	// If there are unflushed deltas, update LastUpdated to reflect recent writes
	hasUnflushed := false
	ms.deltas.Range(func(key, _ any) bool {
		pid, _ := splitDeltaKey(key.(string))
		if pid == projectID {
			hasUnflushed = true
			return false
		}
		return true
	})
	if hasUnflushed {
		rec.LastUpdated = time.Now().UTC()
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

// GetStorageUsage returns the storage_bytes counter for a project.
func (ms *MeterStore) GetStorageUsage(projectID string) (int64, error) {
	return ms.readInt64WithDelta(projectID, MetricStorageBytes)
}

// ListAll returns MeterRecords for every project that has at least one
// counter stored in the metering database.
func (ms *MeterStore) ListAll() ([]*MeterRecord, error) {
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

		rec.APIRequestsTotal, _ = ms.readInt64WithDelta(pid, MetricAPIRequests)
		rec.StorageBytes, _ = ms.readInt64WithDelta(pid, MetricStorageBytes)
		rec.BandwidthUploadBytes, _ = ms.readInt64WithDelta(pid, MetricBandwidthUp)
		rec.BandwidthDownloadBytes, _ = ms.readInt64WithDelta(pid, MetricBandwidthDown)
		rec.DBReadsTotal, _ = ms.readInt64WithDelta(pid, MetricDBReads)
		rec.DBWritesTotal, _ = ms.readInt64WithDelta(pid, MetricDBWrites)
		rec.RealtimeConnections, _ = ms.readInt64WithDelta(pid, MetricRealtimeConnections)

		methodPrefix := []byte(fmt.Sprintf("metering:%s:method:", pid))
		mIter, err := ms.db.NewIter(&pebble.IterOptions{
			LowerBound: methodPrefix,
			UpperBound: keyUpperBound(methodPrefix),
		})
		if err == nil {
			for mIter.First(); mIter.Valid(); mIter.Next() {
				method := string(mIter.Key()[len(methodPrefix):])
				val := int64(binary.BigEndian.Uint64(mIter.Value()))
				if d, ok := ms.deltas.Load(deltaKey(pid, "method:"+method)); ok {
					val += d.(*atomic.Int64).Load()
				}
				rec.APIRequestsByMethod[method] = val
			}
			mIter.Close()
		}

		// Scan in-memory deltas for unflushed method counters
		ms.deltas.Range(func(dk, dv any) bool {
			keyStr := dk.(string)
			dpid, dmetric := splitDeltaKey(keyStr)
			if dpid != pid || len(dmetric) <= 7 || dmetric[:7] != "method:" {
				return true
			}
			method := dmetric[7:]
			if _, exists := rec.APIRequestsByMethod[method]; !exists {
				rec.APIRequestsByMethod[method] = dv.(*atomic.Int64).Load()
			}
			return true
		})

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
	prefix := []byte(fmt.Sprintf("metering:%s:", projectID))
	// Also clear in-memory deltas
	ms.deltas.Range(func(key, _ any) bool {
		ks := key.(string)
		if pid, _ := splitDeltaKey(ks); pid == projectID {
			ms.deltas.Delete(key)
		}
		return true
	})
	return ms.deleteByPrefix(prefix)
}

// ResetAll clears all counters for every project.
func (ms *MeterStore) ResetAll() error {
	ms.deltas.Range(func(key, _ any) bool {
		ms.deltas.Delete(key)
		return true
	})
	return ms.deleteByPrefix([]byte("metering:"))
}

// --- internal helpers ---

// deltaKey builds the in-memory key: "projectID\x00metric"
func deltaKey(projectID, metric string) string {
	return projectID + "\x00" + metric
}

// splitDeltaKey splits "projectID\x00metric" back into its parts.
func splitDeltaKey(key string) (projectID, metric string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

func metricKey(projectID, metric string) []byte {
	if len(metric) > 7 && metric[:7] == "method:" {
		return []byte(fmt.Sprintf("metering:%s:method:%s", projectID, metric[7:]))
	}
	return []byte(fmt.Sprintf("metering:%s:%s", projectID, metric))
}

func periodStartKey(projectID string) []byte {
	return []byte(fmt.Sprintf("metering:%s:period_start", projectID))
}

func lastUpdatedKey(projectID string) []byte {
	return []byte(fmt.Sprintf("metering:%s:last_updated", projectID))
}

// readInt64WithDelta reads a persisted counter and adds any unflushed delta.
func (ms *MeterStore) readInt64WithDelta(projectID, metric string) (int64, error) {
	val, err := ms.readInt64(metricKey(projectID, metric))
	if err != nil {
		return 0, err
	}
	if d, ok := ms.deltas.Load(deltaKey(projectID, metric)); ok {
		val += d.(*atomic.Int64).Load()
	}
	return val, nil
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

// collectProjectIDs iterates all "metering:" keys and returns the unique set
// of project IDs found, including those with only in-memory deltas.
func (ms *MeterStore) collectProjectIDs() ([]string, error) {
	seen := make(map[string]bool)

	// From Pebble
	prefix := []byte("metering:")
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
		seen[pid] = true
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("metering: iter error: %w", err)
	}

	// From in-memory deltas
	ms.deltas.Range(func(key, _ any) bool {
		pid, _ := splitDeltaKey(key.(string))
		seen[pid] = true
		return true
	})

	ids := make([]string, 0, len(seen))
	for pid := range seen {
		ids = append(ids, pid)
	}
	return ids, nil
}

// deleteByPrefix removes all keys that start with the given prefix.
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
		if err := b.Delete(iter.Key(), nil); err != nil {
			return fmt.Errorf("metering: batch delete: %w", err)
		}
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("metering: iter error: %w", err)
	}

	return b.Commit(pebble.Sync)
}

// extractProjectID pulls the project ID from a key like "metering:{id}:..."
func extractProjectID(key []byte) string {
	s := string(key)
	if len(s) <= 9 {
		return s
	}
	rest := s[9:] // Skip "metering:"
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			return rest[:i]
		}
	}
	return rest
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
