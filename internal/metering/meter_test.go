package metering

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cockroachdb/pebble"
)

// newTestStore opens a temporary Pebble DB and wraps it in a MeterStore.
func newTestStore(t *testing.T) *MeterStore {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "metering_test", t.Name())
	os.RemoveAll(dir)
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		t.Fatalf("failed to open test pebble db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return NewMeterStore(db)
}

func TestIncAndGet(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-a"

	// Increment a few metrics
	if err := ms.Inc(pid, MetricAPIRequests, 10); err != nil {
		t.Fatalf("Inc api_requests: %v", err)
	}
	if err := ms.Inc(pid, MetricStorageBytes, 2048); err != nil {
		t.Fatalf("Inc storage_bytes: %v", err)
	}
	if err := ms.Inc(pid, MetricBandwidthUp, 512); err != nil {
		t.Fatalf("Inc bandwidth_up: %v", err)
	}

	// Read back
	rec, err := ms.Get(pid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if rec.APIRequestsTotal != 10 {
		t.Errorf("api_requests = %d, want 10", rec.APIRequestsTotal)
	}
	if rec.StorageBytes != 2048 {
		t.Errorf("storage_bytes = %d, want 2048", rec.StorageBytes)
	}
	if rec.BandwidthUploadBytes != 512 {
		t.Errorf("bandwidth_up = %d, want 512", rec.BandwidthUploadBytes)
	}
	if rec.BandwidthDownloadBytes != 0 {
		t.Errorf("bandwidth_down = %d, want 0", rec.BandwidthDownloadBytes)
	}
	if rec.DBReadsTotal != 0 {
		t.Errorf("db_reads = %d, want 0", rec.DBReadsTotal)
	}
	if rec.DBWritesTotal != 0 {
		t.Errorf("db_writes = %d, want 0", rec.DBWritesTotal)
	}
	if rec.RealtimeConnections != 0 {
		t.Errorf("realtime_connections = %d, want 0", rec.RealtimeConnections)
	}

	// PeriodStart should be set to now-ish
	if rec.PeriodStart.IsZero() {
		t.Error("period_start should not be zero")
	}
	if rec.LastUpdated.IsZero() {
		t.Error("last_updated should not be zero")
	}
}

func TestIncMethod(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-b"

	if err := ms.IncMethod(pid, "GET", 5); err != nil {
		t.Fatalf("IncMethod GET: %v", err)
	}
	if err := ms.IncMethod(pid, "POST", 3); err != nil {
		t.Fatalf("IncMethod POST: %v", err)
	}
	if err := ms.IncMethod(pid, "GET", 2); err != nil {
		t.Fatalf("IncMethod GET again: %v", err)
	}

	rec, err := ms.Get(pid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if rec.APIRequestsTotal != 10 {
		t.Errorf("api_requests_total = %d, want 10", rec.APIRequestsTotal)
	}
	if rec.APIRequestsByMethod["GET"] != 7 {
		t.Errorf("api_requests GET = %d, want 7", rec.APIRequestsByMethod["GET"])
	}
	if rec.APIRequestsByMethod["POST"] != 3 {
		t.Errorf("api_requests POST = %d, want 3", rec.APIRequestsByMethod["POST"])
	}
}

func TestMultipleMetrics(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-c"

	metrics := []struct {
		name  string
		delta int64
	}{
		{MetricAPIRequests, 100},
		{MetricStorageBytes, 99999},
		{MetricBandwidthUp, 5000},
		{MetricBandwidthDown, 3000},
		{MetricDBReads, 42},
		{MetricDBWrites, 17},
		{MetricRealtimeConnections, 8},
	}

	for _, m := range metrics {
		if err := ms.Inc(pid, m.name, m.delta); err != nil {
			t.Fatalf("Inc %s: %v", m.name, err)
		}
	}

	rec, err := ms.Get(pid)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if rec.APIRequestsTotal != 100 {
		t.Errorf("api_requests = %d, want 100", rec.APIRequestsTotal)
	}
	if rec.StorageBytes != 99999 {
		t.Errorf("storage_bytes = %d, want 99999", rec.StorageBytes)
	}
	if rec.BandwidthUploadBytes != 5000 {
		t.Errorf("bandwidth_up = %d, want 5000", rec.BandwidthUploadBytes)
	}
	if rec.BandwidthDownloadBytes != 3000 {
		t.Errorf("bandwidth_down = %d, want 3000", rec.BandwidthDownloadBytes)
	}
	if rec.DBReadsTotal != 42 {
		t.Errorf("db_reads = %d, want 42", rec.DBReadsTotal)
	}
	if rec.DBWritesTotal != 17 {
		t.Errorf("db_writes = %d, want 17", rec.DBWritesTotal)
	}
	if rec.RealtimeConnections != 8 {
		t.Errorf("realtime_connections = %d, want 8", rec.RealtimeConnections)
	}
}

func TestGetStorageUsage(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-d"

	if err := ms.Inc(pid, MetricStorageBytes, 500); err != nil {
		t.Fatalf("Inc storage_bytes: %v", err)
	}

	usage, err := ms.GetStorageUsage(pid)
	if err != nil {
		t.Fatalf("GetStorageUsage: %v", err)
	}
	if usage != 500 {
		t.Errorf("usage = %d, want 500", usage)
	}

	// Non-existent project returns 0
	usage, err = ms.GetStorageUsage("nonexistent")
	if err != nil {
		t.Fatalf("GetStorageUsage nonexistent: %v", err)
	}
	if usage != 0 {
		t.Errorf("usage for nonexistent = %d, want 0", usage)
	}
}

func TestListAll(t *testing.T) {
	ms := newTestStore(t)

	// Insert data for several projects
	projects := []string{"p1", "p2", "p3"}
	for i, pid := range projects {
		ms.Inc(pid, MetricAPIRequests, int64((i+1)*10))
		ms.Inc(pid, MetricStorageBytes, int64((i+1)*100))
	}

	records, err := ms.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("ListAll returned %d records, want 3", len(records))
	}

	// Build a map for easy lookup
	byID := make(map[string]*MeterRecord)
	for _, r := range records {
		byID[r.ProjectID] = r
	}

	for _, pid := range projects {
		rec, ok := byID[pid]
		if !ok {
			t.Fatalf("project %q not found in ListAll results", pid)
		}
		if rec.ProjectID != pid {
			t.Errorf("project_id = %q, want %q", rec.ProjectID, pid)
		}
		if rec.APIRequestsTotal < 10 {
			t.Errorf("project %q api_requests = %d, expected >= 10", pid, rec.APIRequestsTotal)
		}
	}
}

func TestReset(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-e"

	ms.Inc(pid, MetricAPIRequests, 50)
	ms.Inc(pid, MetricStorageBytes, 7777)

	// Verify counters exist
	rec, err := ms.Get(pid)
	if err != nil {
		t.Fatalf("Get before reset: %v", err)
	}
	if rec.APIRequestsTotal != 50 || rec.StorageBytes != 7777 {
		t.Fatalf("unexpected counters before reset: %+v", rec)
	}

	// Reset the project
	if err := ms.Reset(pid); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Verify all counters are zero
	rec, err = ms.Get(pid)
	if err != nil {
		t.Fatalf("Get after reset: %v", err)
	}
	if rec.APIRequestsTotal != 0 {
		t.Errorf("api_requests after reset = %d, want 0", rec.APIRequestsTotal)
	}
	if rec.StorageBytes != 0 {
		t.Errorf("storage_bytes after reset = %d, want 0", rec.StorageBytes)
	}
	if rec.BandwidthUploadBytes != 0 {
		t.Errorf("bandwidth_up after reset = %d, want 0", rec.BandwidthUploadBytes)
	}
	if rec.BandwidthDownloadBytes != 0 {
		t.Errorf("bandwidth_down after reset = %d, want 0", rec.BandwidthDownloadBytes)
	}
	if rec.DBReadsTotal != 0 {
		t.Errorf("db_reads after reset = %d, want 0", rec.DBReadsTotal)
	}
	if rec.DBWritesTotal != 0 {
		t.Errorf("db_writes after reset = %d, want 0", rec.DBWritesTotal)
	}
	if rec.RealtimeConnections != 0 {
		t.Errorf("realtime_connections after reset = %d, want 0", rec.RealtimeConnections)
	}
}

func TestResetAll(t *testing.T) {
	ms := newTestStore(t)

	ms.Inc("p-a", MetricAPIRequests, 10)
	ms.Inc("p-b", MetricAPIRequests, 20)

	if err := ms.ResetAll(); err != nil {
		t.Fatalf("ResetAll: %v", err)
	}

	records, err := ms.ListAll()
	if err != nil {
		t.Fatalf("ListAll after ResetAll: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("after ResetAll, ListAll returned %d records, want 0", len(records))
	}
}

func TestNonExistentProject(t *testing.T) {
	ms := newTestStore(t)

	rec, err := ms.Get("no-such-project")
	if err != nil {
		t.Fatalf("Get nonexistent: %v", err)
	}
	if rec.APIRequestsTotal != 0 {
		t.Errorf("api_requests = %d, want 0", rec.APIRequestsTotal)
	}
	if rec.ProjectID != "no-such-project" {
		t.Errorf("project_id = %q, want %q", rec.ProjectID, "no-such-project")
	}
}

func TestThreadSafety(t *testing.T) {
	ms := newTestStore(t)
	pid := "project-g"

	var wg sync.WaitGroup
	n := 50
	delta := int64(1)

	// Storm of concurrent increments
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ms.Inc(pid, MetricAPIRequests, delta); err != nil {
				t.Errorf("concurrent Inc: %v", err)
			}
			if err := ms.Inc(pid, MetricDBReads, delta); err != nil {
				t.Errorf("concurrent Inc db_reads: %v", err)
			}
			if err := ms.IncMethod(pid, "GET", delta); err != nil {
				t.Errorf("concurrent IncMethod: %v", err)
			}
		}()
	}

	// Concurrent reads during writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ms.Get(pid)
			if err != nil {
				t.Errorf("concurrent Get: %v", err)
			}
			_, err = ms.ListAll()
			if err != nil {
				t.Errorf("concurrent ListAll: %v", err)
			}
		}()
	}

	wg.Wait()

	rec, err := ms.Get(pid)
	if err != nil {
		t.Fatalf("Get after storm: %v", err)
	}

	// each goroutine: Inc(api_requests)+Inc(db_reads)+IncMethod(GET)
	// IncMethod also increments api_requests, so api_requests = 2*n
	apiExpected := int64(2 * n)
	dbReadExpected := int64(n)
	methodExpected := int64(n)

	if rec.APIRequestsTotal != apiExpected {
		t.Errorf("api_requests after storm = %d, want %d", rec.APIRequestsTotal, apiExpected)
	}
	if rec.DBReadsTotal != dbReadExpected {
		t.Errorf("db_reads after storm = %d, want %d", rec.DBReadsTotal, dbReadExpected)
	}
	if rec.APIRequestsByMethod["GET"] != methodExpected {
		t.Errorf("method GET after storm = %d, want %d", rec.APIRequestsByMethod["GET"], methodExpected)
	}
}

func TestOpenMeterStore(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "metering_test", t.Name())
	os.RemoveAll(dir)
	t.Cleanup(func() { os.RemoveAll(dir) })

	ms, err := OpenMeterStore(dir)
	if err != nil {
		t.Fatalf("OpenMeterStore: %v", err)
	}
	defer ms.Close()

	if err := ms.Inc("p", MetricAPIRequests, 1); err != nil {
		t.Fatalf("Inc: %v", err)
	}

	rec, err := ms.Get("p")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.APIRequestsTotal != 1 {
		t.Errorf("api_requests = %d, want 1", rec.APIRequestsTotal)
	}
}

func TestClose(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "metering_test", t.Name())
	os.RemoveAll(dir)
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		t.Fatalf("open pebble: %v", err)
	}
	ms := NewMeterStore(db)
	if err := ms.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	os.RemoveAll(dir)
}
