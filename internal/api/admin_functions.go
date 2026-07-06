package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/functions"
)

// handleAdminListFunctions lists all functions for a project.
func (a *AdminServer) handleAdminListFunctions(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	fns, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if fns == nil {
		fns = []*functions.Function{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  fns,
		"count": len(fns),
	})
}

// handleAdminCreateFunction deploys a new function.
func (a *AdminServer) handleAdminCreateFunction(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var fn functions.Function
	if err := json.NewDecoder(r.Body).Decode(&fn); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := store.Create(&fn)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	refreshFunctionSchedule(r.PathValue("id"))
	writeJSON(w, http.StatusCreated, created)
}

// handleAdminGetFunction returns a single function.
func (a *AdminServer) handleAdminGetFunction(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	fn, err := store.GetByID(r.PathValue("functionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "function not found")
		return
	}
	writeJSON(w, http.StatusOK, fn)
}

// handleAdminUpdateFunction updates a function.
func (a *AdminServer) handleAdminUpdateFunction(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var updates functions.Function
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := store.Update(r.PathValue("functionId"), &updates)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	refreshFunctionSchedule(r.PathValue("id"))
	writeJSON(w, http.StatusOK, updated)
}

// handleAdminDeleteFunction removes a function.
func (a *AdminServer) handleAdminDeleteFunction(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err := store.Delete(r.PathValue("functionId")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	refreshFunctionSchedule(r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleAdminTestFunction invokes a function manually (admin only).
func (a *AdminServer) handleAdminTestFunction(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	fn, err := store.GetByID(r.PathValue("functionId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "function not found")
		return
	}

	projectID := r.PathValue("id")
	exec := FunctionExecutor()

	switch fn.Trigger {
	case functions.TriggerHTTP:
		var req functions.HTTPRequest
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}
		if req.Method == "" {
			req.Method = "POST"
		}
		ctx := r.Context()
		result, _, err := exec.InvokeHTTP(ctx, projectID, fn.Name, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, testRunPayload(result))
	default:
		event := functions.EventPayload{Type: string(fn.Trigger), Collection: fn.Collection}
		if err := json.NewDecoder(r.Body).Decode(&event); err == nil && event.Type == "" {
			event.Type = string(fn.Trigger)
		}
		result := exec.InvokeEvent(r.Context(), projectID, fn, event)
		writeJSON(w, http.StatusOK, testRunPayload(result))
	}
}

// testRunPayload converts a RunResult into the JSON body returned by the
// admin test endpoint. The run always returns HTTP 200 so the dashboard can
// display execution details (status, output, error, metrics) even on failure.
func testRunPayload(result *functions.RunResult) map[string]interface{} {
	status := "success"
	payload := map[string]interface{}{
		"duration_ms": result.Duration.Milliseconds(),
		"metrics":     result.Metrics,
		"output":      result.Output,
	}
	if result.Error != nil {
		status = "error"
		errMsg := result.Error.Error()
		if strings.Contains(errMsg, "timeout") {
			status = "timeout"
		}
		payload["error"] = errMsg
	}
	if result.HTTPResponse != nil {
		payload["response"] = result.HTTPResponse
	}
	payload["status"] = status
	return payload
}

// handleAdminListFunctionExecutions returns recent execution logs.
func (a *AdminServer) handleAdminListFunctionExecutions(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	execs, err := store.ListExecutionsSummary(r.PathValue("functionId"), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if execs == nil {
		execs = []*functions.Execution{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  execs,
		"count": len(execs),
	})
}

// handleAdminGetFunctionExecution returns one execution with samples.
func (a *AdminServer) handleAdminGetFunctionExecution(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	exec, err := store.GetExecution(r.PathValue("functionId"), r.PathValue("execId"))
	if err != nil {
		writeError(w, http.StatusNotFound, "execution not found")
		return
	}
	writeJSON(w, http.StatusOK, exec)
}

// handleAdminFunctionMetrics returns hourly metric buckets.
func (a *AdminServer) handleAdminFunctionMetrics(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	from, to := metricsRange(r.URL.Query().Get("range"))
	buckets, err := store.ListMetricsBuckets(r.PathValue("functionId"), from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if buckets == nil {
		buckets = []*functions.MetricsBucket{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"buckets": buckets,
		"from":    from,
		"to":      to,
	})
}

// handleAdminFunctionsMetricsSummary returns project-wide function metrics.
func (a *AdminServer) handleAdminFunctionsMetricsSummary(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectFunctionStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	from, to := metricsRange(r.URL.Query().Get("range"))
	summary, err := store.ListMetricsSummary(from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if summary == nil {
		summary = []*functions.MetricsSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": summary,
		"from": from,
		"to":   to,
	})
}

func metricsRange(rng string) (time.Time, time.Time) {
	to := time.Now().UTC()
	switch rng {
	case "7d":
		return to.Add(-7 * 24 * time.Hour), to
	default:
		return to.Add(-24 * time.Hour), to
	}
}

func (a *AdminServer) projectFunctionStore(r *http.Request) (*functions.Store, error) {
	projectID := r.PathValue("id")
	if projectID == "" {
		return nil, fmt.Errorf("project not found")
	}
	return getFunctionStore(projectID, a.projects)
}
