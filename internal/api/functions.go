package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ketsuna-org/sovrabase/internal/functions"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

var (
	functionExecutor     *functions.Executor
	functionExecutorOnce sync.Once
	functionStoreCacheMu sync.RWMutex
	functionStoreCache   = make(map[string]*functions.Store)
	scheduleWatcher      *functions.ScheduleWatcher
	scheduleWatcherOnce  sync.Once
)

// FunctionExecutor returns the shared function executor singleton.
func FunctionExecutor() *functions.Executor {
	functionExecutorOnce.Do(func() {
		functionExecutor = functions.NewExecutor()
	})
	return functionExecutor
}

// InitFunctionRuntime wires host resolver and schedule watcher.
func InitFunctionRuntime(ctx context.Context, pm *tenant.ProjectManager, meters *metering.MeterStore) {
	exec := FunctionExecutor()
	exec.SetHostResolver(func(projectID string) functions.HostAPI {
		return executorHostResolver(pm, projectID, meters)
	})
	scheduleWatcherOnce.Do(func() {
		scheduleWatcher = functions.NewScheduleWatcher(exec)
		for _, proj := range pm.ListProjects() {
			if store, err := getFunctionStore(proj.ID, pm); err == nil {
				scheduleWatcher.RegisterStore(proj.ID, store)
			}
		}
		go scheduleWatcher.Start(ctx)
	})
}

func refreshFunctionSchedule(projectID string) {
	if scheduleWatcher != nil {
		scheduleWatcher.Refresh(projectID)
	}
}

func getFunctionStore(projectID string, projects *tenant.ProjectManager) (*functions.Store, error) {
	functionStoreCacheMu.RLock()
	store, ok := functionStoreCache[projectID]
	functionStoreCacheMu.RUnlock()
	if ok {
		return store, nil
	}

	env, err := projects.GetProjectEnv(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}

	functionStoreCacheMu.Lock()
	defer functionStoreCacheMu.Unlock()
	if store, ok := functionStoreCache[projectID]; ok {
		return store, nil
	}
	store = functions.NewStore(env.Engine.DB())
	functionStoreCache[projectID] = store
	FunctionExecutor().RegisterStore(projectID, store)
	if scheduleWatcher != nil {
		scheduleWatcher.RegisterStore(projectID, store)
	}
	return store, nil
}

// RegisterFunctionStores initialises function stores for all projects at startup.
func RegisterFunctionStores(ctx context.Context, pm *tenant.ProjectManager, meters *metering.MeterStore) {
	for _, proj := range pm.ListProjects() {
		_, _ = getFunctionStore(proj.ID, pm)
	}
	InitFunctionRuntime(ctx, pm, meters)
}

// handleFunctionHTTP invokes an HTTP-triggered function.
// Public functions skip auth; private functions require Bearer token.
func (s *Server) handleFunctionHTTP(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	projectID := getProjectID(r)
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "missing project context")
		return
	}

	fnStore, err := getFunctionStore(projectID, s.projects)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	fn, err := fnStore.GetByName(name)
	if err != nil {
		writeError(w, http.StatusNotFound, "function not found")
		return
	}

	if !fn.Public {
		if getClaims(r) == nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
	}

	var body interface{}
	if r.Body != nil && r.ContentLength != 0 {
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
	}

	query := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	req := functions.HTTPRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Query:   query,
		Headers: headers,
		Body:    body,
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeLimit(fn))
	defer cancel()

	result, _, err := FunctionExecutor().InvokeHTTP(ctx, projectID, name, req)
	if err != nil {
		switch err {
		case functions.ErrDisabled:
			writeError(w, http.StatusForbidden, "function is disabled")
		case functions.ErrWrongTrigger:
			writeError(w, http.StatusBadRequest, "function is not HTTP-triggered")
		default:
			writeError(w, http.StatusNotFound, err.Error())
		}
		return
	}

	if result.Error != nil {
		writeError(w, http.StatusInternalServerError, result.Error.Error())
		return
	}

	resp := result.HTTPResponse
	if resp == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	status := resp.Status
	if status <= 0 {
		status = http.StatusOK
	}
	writeJSON(w, status, resp.Body)
}

func timeLimit(fn *functions.Function) time.Duration {
	if fn.TimeoutMs <= 0 {
		return 5 * time.Second
	}
	return time.Duration(fn.TimeoutMs) * time.Millisecond
}

// fireFunctionTriggers dispatches event-triggered functions.
func (s *Server) fireFunctionTriggers(eventType, collection, docID string, data map[string]interface{}, r *http.Request) {
	projectID := getProjectID(r)
	if projectID == "" {
		return
	}
	claims := getClaims(r)
	event := functions.EventPayload{
		Type:       eventType,
		Collection: collection,
		ID:         docID,
		Data:       data,
	}
	if claims != nil {
		event.UserID = claims.UserID
		event.UserEmail = claims.Email
	}
	FunctionExecutor().FireEvent(projectID, event)
}
