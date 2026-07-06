// Package functions provides per-project serverless JavaScript functions
// stored in Pebble and executed via the goja runtime.
package functions

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/cronexpr"
)

// Trigger identifies when a function runs.
type Trigger string

const (
	TriggerHTTP         Trigger = "http"
	TriggerRecordCreate Trigger = "record:create"
	TriggerRecordUpdate Trigger = "record:update"
	TriggerRecordDelete Trigger = "record:delete"
	TriggerAuthSignup   Trigger = "auth:signup"
	TriggerAuthSignin   Trigger = "auth:signin"
	TriggerSchedule     Trigger = "schedule"
)

// Function is a deployable serverless function.
type Function struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Language   string    `json:"language,omitempty"` // javascript (default), go, wasm
	Trigger    Trigger   `json:"trigger"`
	Collection string    `json:"collection,omitempty"`
	Schedule   string    `json:"schedule,omitempty"`
	Code       string    `json:"code"`
	Public     bool      `json:"public"`
	TimeoutMs  int       `json:"timeout_ms"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Execution records a single function invocation.
type Execution struct {
	ID         string           `json:"id"`
	FunctionID string           `json:"function_id"`
	Trigger    string           `json:"trigger"`
	Status     string           `json:"status"` // success, error, timeout
	DurationMs int64            `json:"duration_ms"`
	Error      string           `json:"error,omitempty"`
	Output     string           `json:"output,omitempty"`
	Metrics    ExecutionMetrics `json:"metrics,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

// MetricsBucket holds hourly aggregated metrics for charts.
type MetricsBucket struct {
	Hour        time.Time `json:"hour"`
	FunctionID  string    `json:"function_id"`
	ExecCount   int       `json:"exec_count"`
	DurationP50 int64     `json:"duration_p50_ms"`
	DurationP95 int64     `json:"duration_p95_ms"`
	MemPeakMax  int64     `json:"mem_peak_max_bytes"`
	CPUAvgMs    int64     `json:"cpu_avg_ms"`
	ErrorCount  int       `json:"error_count"`
}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,62}$`)

const (
	fnPrefix       = "__function__:"
	fnNamePrefix   = "__function_name__:"
	execPrefix     = "__function_exec__:"
	metricsPrefix  = "__function_metrics__:"
	maxExecLog     = 200
	metricsRetain  = 7 * 24 * time.Hour
	maxCodeBytes   = 256 * 1024
)

// Store manages functions for a single project.
type Store struct {
	db *pebble.DB
}

// NewStore creates a function store backed by the given Pebble DB.
func NewStore(db *pebble.DB) *Store {
	return &Store{db: db}
}

func fnKey(id string) []byte   { return []byte(fnPrefix + id) }
func fnNameKey(name string) []byte { return []byte(fnNamePrefix + name) }
func metricsKey(functionID string, hour time.Time) []byte {
	return []byte(fmt.Sprintf("%s%s:%s", metricsPrefix, functionID, hour.UTC().Format("2006010215")))
}

func validateLanguage(lang string) error {
	switch NormalizeLanguage(lang) {
	case "javascript", "go", "wasm":
		return nil
	default:
		return fmt.Errorf("functions: invalid language %q", lang)
	}
}

func validateName(name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !namePattern.MatchString(name) {
		return fmt.Errorf("functions: name must be 2-63 chars, lowercase alphanumeric with _ or -")
	}
	return nil
}

func execKey(id string) []byte { return []byte(execPrefix + id) }

func validateSchedule(trigger Trigger, schedule string) error {
	if trigger == TriggerSchedule {
		if strings.TrimSpace(schedule) == "" {
			return fmt.Errorf("functions: schedule is required for trigger schedule")
		}
		return cronexpr.Validate(schedule)
	}
	return nil
}

func validateTrigger(t Trigger) error {
	switch t {
	case TriggerHTTP, TriggerRecordCreate, TriggerRecordUpdate, TriggerRecordDelete,
		TriggerAuthSignup, TriggerAuthSignin, TriggerSchedule:
		return nil
	default:
		return fmt.Errorf("functions: invalid trigger %q", t)
	}
}

// List returns all functions sorted by name.
func (s *Store) List() ([]*Function, error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(fnPrefix),
		UpperBound: []byte(fnPrefix + "\xff"),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var fns []*Function
	for iter.First(); iter.Valid(); iter.Next() {
		var fn Function
		if err := json.Unmarshal(iter.Value(), &fn); err != nil {
			continue
		}
		fns = append(fns, &fn)
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].Name < fns[j].Name })
	return fns, nil
}

// GetByID returns a function by ID.
func (s *Store) GetByID(id string) (*Function, error) {
	val, closer, err := s.db.Get(fnKey(id))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("functions: not found")
	}
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	var fn Function
	if err := json.Unmarshal(val, &fn); err != nil {
		return nil, err
	}
	return &fn, nil
}

// GetByName returns a function by its unique name.
func (s *Store) GetByName(name string) (*Function, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	val, closer, err := s.db.Get(fnNameKey(name))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("functions: not found")
	}
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	id := string(val)
	return s.GetByID(id)
}

// Create stores a new function.
func (s *Store) Create(fn *Function) (*Function, error) {
	if fn == nil {
		return nil, fmt.Errorf("functions: function is required")
	}
	fn.Name = strings.ToLower(strings.TrimSpace(fn.Name))
	if err := validateName(fn.Name); err != nil {
		return nil, err
	}
	if err := validateTrigger(fn.Trigger); err != nil {
		return nil, err
	}
	if err := validateLanguage(fn.Language); err != nil {
		return nil, err
	}
	if err := validateSchedule(fn.Trigger, fn.Schedule); err != nil {
		return nil, err
	}
	if strings.TrimSpace(fn.Code) == "" {
		return nil, fmt.Errorf("functions: code is required")
	}
	if len(fn.Code) > maxCodeBytes {
		return nil, fmt.Errorf("functions: code exceeds %d bytes", maxCodeBytes)
	}
	fn.Language = NormalizeLanguage(fn.Language)
	if fn.TimeoutMs <= 0 {
		fn.TimeoutMs = 5000
	}
	if fn.TimeoutMs > 30000 {
		return nil, fmt.Errorf("functions: timeout_ms must not exceed 30000")
	}

	if _, closer, err := s.db.Get(fnNameKey(fn.Name)); err == nil {
		closer.Close()
		return nil, fmt.Errorf("functions: name %q already exists", fn.Name)
	} else if err != pebble.ErrNotFound {
		return nil, err
	}

	now := time.Now().UTC()
	fn.ID = uuid.New().String()
	fn.Enabled = true
	fn.CreatedAt = now
	fn.UpdatedAt = now

	data, err := json.Marshal(fn)
	if err != nil {
		return nil, err
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := batch.Set(fnKey(fn.ID), data, nil); err != nil {
		return nil, err
	}
	if err := batch.Set(fnNameKey(fn.Name), []byte(fn.ID), nil); err != nil {
		return nil, err
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return nil, err
	}
	return fn, nil
}

// Update modifies an existing function.
func (s *Store) Update(id string, updates *Function) (*Function, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if updates.Name != "" && updates.Name != existing.Name {
		updates.Name = strings.ToLower(strings.TrimSpace(updates.Name))
		if err := validateName(updates.Name); err != nil {
			return nil, err
		}
		if other, closer, err := s.db.Get(fnNameKey(updates.Name)); err == nil {
			closer.Close()
			if string(other) != id {
				return nil, fmt.Errorf("functions: name %q already exists", updates.Name)
			}
		} else if err != nil && err != pebble.ErrNotFound {
			return nil, err
		}
		existing.Name = updates.Name
	}
	if updates.Trigger != "" {
		if err := validateTrigger(updates.Trigger); err != nil {
			return nil, err
		}
		existing.Trigger = updates.Trigger
	}
	if updates.Language != "" {
		if err := validateLanguage(updates.Language); err != nil {
			return nil, err
		}
		existing.Language = NormalizeLanguage(updates.Language)
	}
	if updates.Schedule != "" || updates.Trigger == TriggerSchedule {
		if err := validateSchedule(existing.Trigger, coalesceSchedule(updates.Schedule, existing.Schedule)); err != nil {
			return nil, err
		}
	}
	if updates.Collection != "" {
		existing.Collection = updates.Collection
	}
	if updates.Schedule != "" {
		existing.Schedule = updates.Schedule
	}
	if updates.Code != "" {
		if len(updates.Code) > maxCodeBytes {
			return nil, fmt.Errorf("functions: code exceeds %d bytes", maxCodeBytes)
		}
		existing.Code = updates.Code
	}
	existing.Public = updates.Public
	if updates.TimeoutMs > 0 {
		if updates.TimeoutMs > 30000 {
			return nil, fmt.Errorf("functions: timeout_ms must not exceed 30000")
		}
		existing.TimeoutMs = updates.TimeoutMs
	}
	existing.Enabled = updates.Enabled
	existing.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(existing)
	if err != nil {
		return nil, err
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := batch.Set(fnKey(existing.ID), data, nil); err != nil {
		return nil, err
	}
	if err := batch.Set(fnNameKey(existing.Name), []byte(existing.ID), nil); err != nil {
		return nil, err
	}
	if err := batch.Commit(pebble.Sync); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete removes a function.
func (s *Store) Delete(id string) error {
	fn, err := s.GetByID(id)
	if err != nil {
		return err
	}
	batch := s.db.NewBatch()
	defer batch.Close()
	if err := batch.Delete(fnKey(id), nil); err != nil {
		return err
	}
	if err := batch.Delete(fnNameKey(fn.Name), nil); err != nil {
		return err
	}
	return batch.Commit(pebble.Sync)
}

// ListByTrigger returns enabled functions matching a trigger and optional collection.
func (s *Store) ListByTrigger(trigger Trigger, collection string) ([]*Function, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	var matched []*Function
	for _, fn := range all {
		if !fn.Enabled || fn.Trigger != trigger {
			continue
		}
		if collection != "" && fn.Collection != "" && fn.Collection != collection && fn.Collection != "*" {
			continue
		}
		matched = append(matched, fn)
	}
	return matched, nil
}

// LogExecution stores an execution record (keeps last maxExecLog entries).
func (s *Store) LogExecution(exec *Execution) error {
	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(exec)
	if err != nil {
		return err
	}
	key := execKey(exec.CreatedAt.Format(time.RFC3339Nano) + ":" + exec.ID)
	if err := s.db.Set(key, data, pebble.Sync); err != nil {
		return err
	}
	s.pruneExecutions()
	return nil
}

func (s *Store) pruneExecutions() {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(execPrefix),
		UpperBound: []byte(execPrefix + "\xff"),
	})
	if err != nil {
		return
	}
	defer iter.Close()

	var keys [][]byte
	for iter.Last(); iter.Valid(); iter.Prev() {
		if len(keys) >= maxExecLog {
			key := make([]byte, len(iter.Key()))
			copy(key, iter.Key())
			keys = append(keys, key)
		}
	}
	for _, k := range keys {
		_ = s.db.Delete(k, pebble.Sync)
	}
}

// ListExecutions returns recent executions, optionally filtered by function ID.
func (s *Store) ListExecutions(functionID string, limit int) ([]*Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(execPrefix),
		UpperBound: []byte(execPrefix + "\xff"),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var execs []*Execution
	for iter.Last(); iter.Valid() && len(execs) < limit; iter.Prev() {
		var exec Execution
		if err := json.Unmarshal(iter.Value(), &exec); err != nil {
			continue
		}
		if functionID != "" && exec.FunctionID != functionID {
			continue
		}
		execs = append(execs, &exec)
	}
	return execs, nil
}

func coalesceSchedule(updates, existing string) string {
	if updates != "" {
		return updates
	}
	return existing
}

// GetExecution returns a single execution by ID.
func (s *Store) GetExecution(functionID, execID string) (*Execution, error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(execPrefix),
		UpperBound: []byte(execPrefix + "\xff"),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.Last(); iter.Valid(); iter.Prev() {
		var exec Execution
		if err := json.Unmarshal(iter.Value(), &exec); err != nil {
			continue
		}
		if exec.ID == execID && (functionID == "" || exec.FunctionID == functionID) {
			return &exec, nil
		}
	}
	return nil, fmt.Errorf("functions: execution not found")
}

// ListExecutionsSummary returns executions without sample arrays (lighter list).
func (s *Store) ListExecutionsSummary(functionID string, limit int) ([]*Execution, error) {
	execs, err := s.ListExecutions(functionID, limit)
	if err != nil {
		return nil, err
	}
	for _, e := range execs {
		e.Metrics.Samples = nil
	}
	return execs, nil
}

// UpdateMetricsRollup updates hourly aggregates for dashboard charts.
func (s *Store) UpdateMetricsRollup(functionID string, metrics ExecutionMetrics, isError bool) error {
	hour := time.Now().UTC().Truncate(time.Hour)
	key := metricsKey(functionID, hour)

	var bucket MetricsBucket
	val, closer, err := s.db.Get(key)
	if err == nil {
		_ = json.Unmarshal(val, &bucket)
		closer.Close()
	} else if err != pebble.ErrNotFound {
		return err
	}

	bucket.Hour = hour
	bucket.FunctionID = functionID
	bucket.ExecCount++
	if isError {
		bucket.ErrorCount++
	}
	if metrics.MemPeakBytes > bucket.MemPeakMax {
		bucket.MemPeakMax = metrics.MemPeakBytes
	}
	cpuTotal := metrics.CPUUserMs + metrics.CPUSysMs
	if bucket.ExecCount == 1 {
		bucket.DurationP50 = metrics.DurationMs
		bucket.DurationP95 = metrics.DurationMs
		bucket.CPUAvgMs = cpuTotal
	} else {
		bucket.DurationP50 = (bucket.DurationP50 + metrics.DurationMs) / 2
		if metrics.DurationMs > bucket.DurationP95 {
			bucket.DurationP95 = metrics.DurationMs
		}
		bucket.CPUAvgMs = (bucket.CPUAvgMs*(int64(bucket.ExecCount-1)) + cpuTotal) / int64(bucket.ExecCount)
	}

	data, err := json.Marshal(bucket)
	if err != nil {
		return err
	}
	if err := s.db.Set(key, data, pebble.Sync); err != nil {
		return err
	}
	s.pruneMetrics()
	return nil
}

func (s *Store) pruneMetrics() {
	cutoff := time.Now().UTC().Add(-metricsRetain).Format("2006010215")
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(metricsPrefix),
		UpperBound: []byte(metricsPrefix + "\xff"),
	})
	if err != nil {
		return
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		k := string(iter.Key())
		parts := strings.Split(k, ":")
		if len(parts) < 3 {
			continue
		}
		hourStr := parts[len(parts)-1]
		if hourStr < cutoff {
			_ = s.db.Delete(iter.Key(), pebble.Sync)
		}
	}
}

// ListMetricsBuckets returns hourly buckets for a function in a time range.
func (s *Store) ListMetricsBuckets(functionID string, from, to time.Time) ([]*MetricsBucket, error) {
	prefix := []byte(fmt.Sprintf("%s%s:", metricsPrefix, functionID))
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: append(prefix, 0xff),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var buckets []*MetricsBucket
	for iter.First(); iter.Valid(); iter.Next() {
		var b MetricsBucket
		if err := json.Unmarshal(iter.Value(), &b); err != nil {
			continue
		}
		if b.Hour.Before(from) || b.Hour.After(to) {
			continue
		}
		buckets = append(buckets, &b)
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Hour.Before(buckets[j].Hour) })
	return buckets, nil
}

// MetricsSummary holds project-level function metrics summary.
type MetricsSummary struct {
	FunctionID   string `json:"function_id"`
	FunctionName string `json:"function_name"`
	DurationP95  int64  `json:"duration_p95_ms"`
	MemPeakMax   int64  `json:"mem_peak_max_bytes"`
	ExecCount    int    `json:"exec_count"`
}

// ListMetricsSummary returns top functions by resource usage over range.
func (s *Store) ListMetricsSummary(from, to time.Time) ([]*MetricsSummary, error) {
	fns, err := s.List()
	if err != nil {
		return nil, err
	}
	nameByID := make(map[string]string)
	for _, fn := range fns {
		nameByID[fn.ID] = fn.Name
	}

	agg := make(map[string]*MetricsSummary)
	iter, err := s.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(metricsPrefix),
		UpperBound: []byte(metricsPrefix + "\xff"),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var b MetricsBucket
		if err := json.Unmarshal(iter.Value(), &b); err != nil {
			continue
		}
		if b.Hour.Before(from) || b.Hour.After(to) {
			continue
		}
		s := agg[b.FunctionID]
		if s == nil {
			s = &MetricsSummary{FunctionID: b.FunctionID, FunctionName: nameByID[b.FunctionID]}
			agg[b.FunctionID] = s
		}
		s.ExecCount += b.ExecCount
		if b.DurationP95 > s.DurationP95 {
			s.DurationP95 = b.DurationP95
		}
		if b.MemPeakMax > s.MemPeakMax {
			s.MemPeakMax = b.MemPeakMax
		}
	}

	out := make([]*MetricsSummary, 0, len(agg))
	for _, s := range agg {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MemPeakMax > out[j].MemPeakMax })
	return out, nil
}
