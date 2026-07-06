package functions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/cronexpr"
)

// ScheduleWatcher fires functions with trigger=schedule.
type ScheduleWatcher struct {
	executor  *Executor
	stores    map[string]*Store
	next      map[string]map[string]time.Time // projectID -> functionID -> next
	mu        sync.Mutex
	logger    *slog.Logger
	started   map[string]bool
}

// NewScheduleWatcher creates a watcher for scheduled functions.
func NewScheduleWatcher(exec *Executor) *ScheduleWatcher {
	return &ScheduleWatcher{
		executor: exec,
		stores:   make(map[string]*Store),
		next:     make(map[string]map[string]time.Time),
		logger:   slog.Default().With("component", "function-scheduler"),
		started:  make(map[string]bool),
	}
}

// RegisterStore adds a project's function store.
func (w *ScheduleWatcher) RegisterStore(projectID string, store *Store) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stores[projectID] = store
	w.next[projectID] = make(map[string]time.Time)
	w.refreshProjectLocked(projectID)
}

func (w *ScheduleWatcher) refreshProjectLocked(projectID string) {
	store := w.stores[projectID]
	if store == nil {
		return
	}
	fns, err := store.ListByTrigger(TriggerSchedule, "")
	if err != nil {
		return
	}
	now := time.Now().UTC()
	next := w.next[projectID]
	if next == nil {
		next = make(map[string]time.Time)
		w.next[projectID] = next
	}
	seen := make(map[string]bool)
	for _, fn := range fns {
		seen[fn.ID] = true
		if fn.Enabled && fn.Schedule != "" {
			next[fn.ID] = cronexpr.Next(fn.Schedule, now)
		} else {
			delete(next, fn.ID)
		}
	}
	for id := range next {
		if !seen[id] {
			delete(next, id)
		}
	}
}

// Start runs the tick loop for all registered projects.
func (w *ScheduleWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *ScheduleWatcher) tick(ctx context.Context) {
	w.mu.Lock()
	now := time.Now().UTC()
	type job struct {
		projectID string
		fn        *Function
	}
	var due []job
	for projectID, times := range w.next {
		store := w.stores[projectID]
		if store == nil {
			continue
		}
		for fnID, nextTime := range times {
			if now.Before(nextTime) {
				continue
			}
			fn, err := store.GetByID(fnID)
			if err != nil || !fn.Enabled || fn.Trigger != TriggerSchedule {
				delete(times, fnID)
				continue
			}
			due = append(due, job{projectID: projectID, fn: fn})
			times[fnID] = cronexpr.Next(fn.Schedule, now)
		}
	}
	w.mu.Unlock()

	for _, j := range due {
		fn := j.fn
		go func(projectID string, fn *Function) {
			runCtx, cancel := context.WithTimeout(ctx, timeLimit(fn))
			defer cancel()
			w.executor.InvokeSchedule(runCtx, projectID, fn)
			w.logger.Info("scheduled function executed", "project", projectID, "function", fn.Name)
		}(j.projectID, fn)
	}
}

// Refresh reloads schedules for a project (call after function CRUD).
func (w *ScheduleWatcher) Refresh(projectID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.refreshProjectLocked(projectID)
}
