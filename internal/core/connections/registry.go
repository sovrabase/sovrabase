package connections

import (
	"context"
	"errors"
	"sync"
	"time"
)

const closeTimeout = 5 * time.Second

type openCall struct {
	done chan struct{}
	conn RuntimeConnection
	err  error
}

type runtimeEntry struct {
	conn          RuntimeConnection
	refs          int
	lastUsed      time.Time
	evictWhenIdle bool
}

type Registry struct {
	mu            sync.Mutex
	entries       map[string]*runtimeEntry
	inFlight      map[string]*openCall
	cacheTTL      time.Duration
	sweepInterval time.Duration
	stopCh        chan struct{}
	doneCh        chan struct{}
}

func NewRegistry(cacheTTL, sweepInterval time.Duration) *Registry {
	r := &Registry{
		entries:       make(map[string]*runtimeEntry),
		inFlight:      make(map[string]*openCall),
		cacheTTL:      cacheTTL,
		sweepInterval: sweepInterval,
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	go r.runSweeper()
	return r
}

func MappingKey(projectID, slug string) string {
	return projectID + "::" + slug
}

func (r *Registry) Acquire(ctx context.Context, key string, opener func(context.Context) (RuntimeConnection, error)) (Lease, error) {
	if key == "" {
		return nil, errors.New("mapping key is required")
	}
	if opener == nil {
		return nil, errors.New("opener is required")
	}

	for {
		r.mu.Lock()

		if entry, ok := r.entries[key]; ok {
			entry.refs++
			entry.lastUsed = time.Now().UTC()
			l := &registryLease{
				conn:    entry.conn,
				release: func() { r.release(key) },
			}
			r.mu.Unlock()
			return l, nil
		}

		if call, ok := r.inFlight[key]; ok {
			r.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-call.done:
				if call.err != nil {
					return nil, call.err
				}
				continue
			}
		}

		call := &openCall{done: make(chan struct{})}
		r.inFlight[key] = call
		r.mu.Unlock()

		conn, err := opener(ctx)

		r.mu.Lock()
		delete(r.inFlight, key)
		if err == nil {
			r.entries[key] = &runtimeEntry{
				conn:     conn,
				refs:     1,
				lastUsed: time.Now().UTC(),
			}
		}
		call.conn = conn
		call.err = err
		close(call.done)
		r.mu.Unlock()

		if err != nil {
			return nil, err
		}

		return &registryLease{
			conn:    conn,
			release: func() { r.release(key) },
		}, nil
	}
}

func (r *Registry) Invalidate(key string) {
	var conn RuntimeConnection

	r.mu.Lock()
	entry, ok := r.entries[key]
	if ok {
		if entry.refs == 0 {
			conn = entry.conn
			delete(r.entries, key)
		} else {
			entry.evictWhenIdle = true
		}
	}
	r.mu.Unlock()

	if conn != nil {
		closeRuntimeConnection(conn)
	}
}

func (r *Registry) Stop(ctx context.Context) error {
	close(r.stopCh)
	select {
	case <-r.doneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	var toClose []RuntimeConnection

	r.mu.Lock()
	for key, entry := range r.entries {
		toClose = append(toClose, entry.conn)
		delete(r.entries, key)
	}
	r.mu.Unlock()

	for _, conn := range toClose {
		closeRuntimeConnection(conn)
	}

	return nil
}

func (r *Registry) release(key string) {
	var conn RuntimeConnection

	r.mu.Lock()
	entry, ok := r.entries[key]
	if ok {
		if entry.refs > 0 {
			entry.refs--
		}
		if entry.refs == 0 {
			entry.lastUsed = time.Now().UTC()
			if entry.evictWhenIdle {
				conn = entry.conn
				delete(r.entries, key)
			}
		}
	}
	r.mu.Unlock()

	if conn != nil {
		closeRuntimeConnection(conn)
	}
}

func (r *Registry) runSweeper() {
	ticker := time.NewTicker(r.sweepInterval)
	defer func() {
		ticker.Stop()
		close(r.doneCh)
	}()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.sweep()
		}
	}
}

func (r *Registry) sweep() {
	now := time.Now().UTC()
	var toClose []RuntimeConnection

	r.mu.Lock()
	for key, entry := range r.entries {
		if entry.refs > 0 {
			continue
		}
		if now.Sub(entry.lastUsed) <= r.cacheTTL {
			continue
		}
		toClose = append(toClose, entry.conn)
		delete(r.entries, key)
	}
	r.mu.Unlock()

	for _, conn := range toClose {
		closeRuntimeConnection(conn)
	}
}

type registryLease struct {
	conn    RuntimeConnection
	release func()
	once    sync.Once
}

func (l *registryLease) Connection() RuntimeConnection {
	return l.conn
}

func (l *registryLease) Release() {
	l.once.Do(l.release)
}

func closeRuntimeConnection(conn RuntimeConnection) {
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	_ = conn.Close(ctx)
}
