package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/ketsuna-org/sovrabase/internal/scheduler"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// schedulerCache caches per-project scheduler.Store instances so we don't
// re-load jobs from Pebble on every request. Each store runs its own tick
// goroutine.
var (
	schedulerCacheMu sync.RWMutex
	schedulerCache   = make(map[string]*scheduler.Store)
)

// StartProjectSchedulers initialises and starts cron schedulers for all
// active projects. Called once at server startup.
func StartProjectSchedulers(ctx context.Context, pm *tenant.ProjectManager) {
	for _, proj := range pm.ListProjects() {
		env, err := pm.GetProjectEnv(proj.ID)
		if err != nil {
			continue
		}
		store := scheduler.NewStore(env.Engine.DB())
		schedulerCacheMu.Lock()
		schedulerCache[proj.ID] = store
		schedulerCacheMu.Unlock()
		go store.Start(ctx)
	}
}

// projectSchedulerCached returns a cached scheduler.Store for the project,
// creating and starting it on first access.
func (a *AdminServer) projectSchedulerCached(r *http.Request) (*scheduler.Store, error) {
	projectID := r.PathValue("id")
	if projectID == "" {
		return nil, fmt.Errorf("project not found")
	}

	// Fast path: read lock.
	schedulerCacheMu.RLock()
	store, ok := schedulerCache[projectID]
	schedulerCacheMu.RUnlock()
	if ok {
		return store, nil
	}

	// Slow path: create, cache, and start.
	schedulerCacheMu.Lock()
	defer schedulerCacheMu.Unlock()

	// Double-check after acquiring write lock.
	if store, ok := schedulerCache[projectID]; ok {
		return store, nil
	}

	env, err := a.projects.GetProjectEnv(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}
	store = scheduler.NewStore(env.Engine.DB())
	schedulerCache[projectID] = store
	return store, nil
}

// handleAdminListCronJobs returns all scheduled jobs for a project.
func (a *AdminServer) handleAdminListCronJobs(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectSchedulerCached(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	jobs, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if jobs == nil {
		jobs = []*scheduler.Job{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  jobs,
		"count": len(jobs),
	})
}

// handleAdminCreateCronJob creates a new scheduled job.
func (a *AdminServer) handleAdminCreateCronJob(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectSchedulerCached(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var job scheduler.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := store.Create(&job)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// handleAdminUpdateCronJob updates an existing scheduled job.
func (a *AdminServer) handleAdminUpdateCronJob(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectSchedulerCached(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var job scheduler.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job.ID = r.PathValue("jobId")
	updated, err := store.Update(&job)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// handleAdminDeleteCronJob removes a scheduled job.
func (a *AdminServer) handleAdminDeleteCronJob(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectSchedulerCached(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	jobID := r.PathValue("jobId")
	if err := store.Delete(jobID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleAdminGetCronExecutions returns recent execution history for a job.
func (a *AdminServer) handleAdminGetCronExecutions(w http.ResponseWriter, r *http.Request) {
	store, err := a.projectSchedulerCached(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	jobID := r.PathValue("jobId")
	execs, err := store.GetExecutions(jobID, 20)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if execs == nil {
		execs = []*scheduler.JobExecution{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  execs,
		"count": len(execs),
	})
}
