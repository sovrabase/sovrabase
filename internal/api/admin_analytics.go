package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/ketsuna-org/sovrabase/internal/analytics"
)

var (
	analyticsCacheMu sync.RWMutex
	analyticsCache   = make(map[string]*analytics.Store)
)

func (a *AdminServer) getProjectAnalyticsStore(r *http.Request) (*analytics.Store, error) {
	projectID := r.PathValue("id")
	analyticsCacheMu.RLock()
	store, ok := analyticsCache[projectID]
	analyticsCacheMu.RUnlock()
	if ok {
		return store, nil
	}
	env, err := a.getProjectEnv(r)
	if err != nil {
		return nil, err
	}
	analyticsCacheMu.Lock()
	defer analyticsCacheMu.Unlock()
	if store, ok := analyticsCache[projectID]; ok {
		return store, nil
	}
	store = analytics.NewStore(env.Engine.DB())
	analyticsCache[projectID] = store
	return store, nil
}

func (a *AdminServer) handleAdminAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectAnalyticsStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	summary, err := store.Summary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// handleIngestEvents ingests analytics events from the public API.
// @Summary      Ingest analytics events
// @Description  Accepts a batch of analytics events for processing. Returns 202 with count of accepted events.
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ProjectKey
// @Param        body  body  object{events=[]analytics.Event}  true  "Analytics events"
// @Success      200   {object}  map[string]string
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /api/v1/events [post]
func (s *Server) handleIngestEvents(w http.ResponseWriter, r *http.Request) {
	projectID := getProjectID(r)
	var req struct {
		Events []analytics.Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Events) == 0 {
		writeError(w, http.StatusBadRequest, "at least one event is required")
		return
	}

	analyticsCacheMu.RLock()
	store := analyticsCache[projectID]
	analyticsCacheMu.RUnlock()

	if store == nil {
		env, err := s.projects.GetProjectEnv(projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "project not found")
			return
		}
		analyticsCacheMu.Lock()
		store = analytics.NewStore(env.Engine.DB())
		analyticsCache[projectID] = store
		analyticsCacheMu.Unlock()
	}

	store.Ingest(req.Events)
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted", "count": fmt.Sprintf("%d", len(req.Events))})
}
