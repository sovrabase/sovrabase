package adapters

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrAdapterNotFound = errors.New("adapter not found")

type RuntimeConnection interface {
	Close(ctx context.Context) error
}

type TargetAdapter interface {
	Engine() string
	Ping(ctx context.Context, dsn string, opts map[string]string) error
	Open(ctx context.Context, dsn string, opts map[string]string) (RuntimeConnection, error)
}

type Registry struct {
	mu       sync.RWMutex
	adapters map[string]TargetAdapter
}

func NewRegistry(list ...TargetAdapter) (*Registry, error) {
	r := &Registry{
		adapters: make(map[string]TargetAdapter, len(list)),
	}
	for _, adapter := range list {
		if err := r.Register(adapter); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(adapter TargetAdapter) error {
	if adapter == nil {
		return errors.New("adapter is nil")
	}
	engine := adapter.Engine()
	if engine == "" {
		return errors.New("adapter engine is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.adapters[engine]; ok {
		return fmt.Errorf("adapter for engine %q already registered", engine)
	}
	r.adapters[engine] = adapter
	return nil
}

func (r *Registry) Get(engine string) (TargetAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	adapter, ok := r.adapters[engine]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAdapterNotFound, engine)
	}
	return adapter, nil
}
