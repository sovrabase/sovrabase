package connections

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRuntimeConn struct {
	closed atomic.Int32
}

func (f *fakeRuntimeConn) Close(_ context.Context) error {
	f.closed.Add(1)
	return nil
}

func TestRegistryConcurrentAcquireOpensOnce(t *testing.T) {
	registry := NewRegistry(10*time.Minute, 1*time.Minute)
	defer func() {
		_ = registry.Stop(context.Background())
	}()

	var opens atomic.Int32
	conn := &fakeRuntimeConn{}

	opener := func(ctx context.Context) (RuntimeConnection, error) {
		opens.Add(1)
		time.Sleep(10 * time.Millisecond)
		return conn, nil
	}

	const workers = 25
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			lease, err := registry.Acquire(context.Background(), "p::db", opener)
			if err != nil {
				t.Errorf("Acquire() error = %v", err)
				return
			}
			lease.Release()
		}()
	}

	wg.Wait()
	if opens.Load() != 1 {
		t.Fatalf("open calls = %d, want 1", opens.Load())
	}
}

func TestRegistryInvalidateWaitsForLeaseRelease(t *testing.T) {
	registry := NewRegistry(10*time.Minute, 1*time.Minute)
	defer func() {
		_ = registry.Stop(context.Background())
	}()

	conn := &fakeRuntimeConn{}
	open := func(context.Context) (RuntimeConnection, error) {
		return conn, nil
	}

	lease1, err := registry.Acquire(context.Background(), "p::db", open)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	lease2, err := registry.Acquire(context.Background(), "p::db", open)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	registry.Invalidate("p::db")
	if conn.closed.Load() != 0 {
		t.Fatalf("connection closed early; got %d closes", conn.closed.Load())
	}

	lease1.Release()
	if conn.closed.Load() != 0 {
		t.Fatalf("connection closed with active lease; got %d closes", conn.closed.Load())
	}

	lease2.Release()
	waitFor(t, 500*time.Millisecond, func() bool { return conn.closed.Load() == 1 })
}

func TestRegistryIdleEviction(t *testing.T) {
	registry := NewRegistry(50*time.Millisecond, 10*time.Millisecond)
	defer func() {
		_ = registry.Stop(context.Background())
	}()

	conn := &fakeRuntimeConn{}
	open := func(context.Context) (RuntimeConnection, error) {
		return conn, nil
	}

	lease, err := registry.Acquire(context.Background(), "p::db", open)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	lease.Release()

	waitFor(t, 1*time.Second, func() bool { return conn.closed.Load() == 1 })
}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
