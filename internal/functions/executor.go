package functions

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Executor runs functions and logs executions.
type Executor struct {
	hostResolver func(projectID string) HostAPI
	stores       map[string]*Store
	mu           sync.RWMutex
}

// NewExecutor creates a function executor.
func NewExecutor() *Executor {
	return &Executor{
		stores: make(map[string]*Store),
	}
}

// SetHostResolver sets per-project host API resolver.
func (e *Executor) SetHostResolver(resolver func(projectID string) HostAPI) {
	e.hostResolver = resolver
}

func (e *Executor) hostFor(projectID string) HostAPI {
	if e.hostResolver != nil {
		if h := e.hostResolver(projectID); h != nil {
			return h
		}
	}
	return DefaultHost
}

// RegisterStore associates a project with its function store.
func (e *Executor) RegisterStore(projectID string, store *Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stores[projectID] = store
}

// Store returns the function store for a project.
func (e *Executor) Store(projectID string) *Store {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stores[projectID]
}

func (e *Executor) runHTTP(ctx context.Context, projectID string, fn *Function, req HTTPRequest) *RunResult {
	if NormalizeLanguage(fn.Language) == "wasm" {
		return &RunResult{Error: errWasmNotImplemented}
	}
	return SelectRuntime(fn.Language).RunHTTP(ctx, fn, req, e.hostFor(projectID))
}

func (e *Executor) runEvent(ctx context.Context, projectID string, fn *Function, event EventPayload) *RunResult {
	if NormalizeLanguage(fn.Language) == "wasm" {
		return &RunResult{Error: errWasmNotImplemented}
	}
	return SelectRuntime(fn.Language).RunEvent(ctx, fn, event, e.hostFor(projectID))
}

// InvokeHTTP runs an HTTP-triggered function by name.
func (e *Executor) InvokeHTTP(ctx context.Context, projectID, name string, req HTTPRequest) (*RunResult, *Function, error) {
	store := e.Store(projectID)
	if store == nil {
		return nil, nil, ErrStoreNotFound
	}
	fn, err := store.GetByName(name)
	if err != nil {
		return nil, nil, err
	}
	if fn.Trigger != TriggerHTTP {
		return nil, nil, ErrWrongTrigger
	}
	if !fn.Enabled {
		return nil, nil, ErrDisabled
	}

	result := e.runHTTP(ctx, projectID, fn, req)
	e.logExecution(store, fn, string(TriggerHTTP), result)
	return result, fn, nil
}

// FireEvent runs all matching event-triggered functions asynchronously.
func (e *Executor) FireEvent(projectID string, event EventPayload) {
	store := e.Store(projectID)
	if store == nil {
		return
	}

	trigger := Trigger(event.Type)
	fns, err := store.ListByTrigger(trigger, event.Collection)
	if err != nil {
		return
	}

	for _, fn := range fns {
		go func(fn *Function) {
			ctx, cancel := context.WithTimeout(context.Background(), timeLimit(fn))
			defer cancel()
			result := e.runEvent(ctx, projectID, fn, event)
			e.logExecution(store, fn, string(trigger), result)
		}(fn)
	}
}

// InvokeSchedule runs a scheduled function directly.
func (e *Executor) InvokeSchedule(ctx context.Context, projectID string, fn *Function) *RunResult {
	event := EventPayload{
		Type: string(TriggerSchedule),
		Data: map[string]interface{}{
			"schedule": fn.Schedule,
			"name":     fn.Name,
		},
	}
	result := e.runEvent(ctx, projectID, fn, event)
	if store := e.Store(projectID); store != nil {
		e.logExecution(store, fn, string(TriggerSchedule), result)
	}
	return result
}

func timeLimit(fn *Function) time.Duration {
	return FunctionTimeout(fn)
}

func (e *Executor) logExecution(store *Store, fn *Function, trigger string, result *RunResult) {
	status := "success"
	errMsg := ""
	if result.Error != nil {
		status = "error"
		errMsg = result.Error.Error()
		if strings.Contains(errMsg, "timeout") {
			status = "timeout"
		}
	}

	durationMs := result.Duration.Milliseconds()
	metrics := result.Metrics
	if metrics.DurationMs == 0 {
		metrics.DurationMs = durationMs
	}

	_ = store.LogExecution(&Execution{
		FunctionID: fn.ID,
		Trigger:    trigger,
		Status:     status,
		DurationMs: durationMs,
		Error:      errMsg,
		Output:     result.Output,
		Metrics:    metrics,
	})
	_ = store.UpdateMetricsRollup(fn.ID, metrics, status != "success")
}

// InvokeEvent runs an event-triggered function synchronously (admin test).
func (e *Executor) InvokeEvent(ctx context.Context, projectID string, fn *Function, event EventPayload) *RunResult {
	result := e.runEvent(ctx, projectID, fn, event)
	if store := e.Store(projectID); store != nil {
		e.logExecution(store, fn, string(fn.Trigger), result)
	}
	return result
}

type sentinelError string

func (e sentinelError) Error() string { return string(e) }

const (
	ErrStoreNotFound sentinelError = "functions: store not found"
	ErrWrongTrigger  sentinelError = "functions: function is not HTTP-triggered"
	ErrDisabled      sentinelError = "functions: function is disabled"
	errWasmNotImplemented = sentinelError("functions: wasm runtime not implemented")
)
