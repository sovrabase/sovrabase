package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ketsuna-org/sovrabase/internal/logdrain"
)

var (
	// logDrainCache caches per-project log drain stores.
	logDrainCacheMu sync.RWMutex
	logDrainCache   = make(map[string]*logdrain.Store)
)

func (a *AdminServer) getProjectLogDrainStore(r *http.Request) (*logdrain.Store, error) {
	projectID := r.PathValue("id")
	logDrainCacheMu.RLock()
	store, ok := logDrainCache[projectID]
	logDrainCacheMu.RUnlock()
	if ok {
		return store, nil
	}

	env, err := a.getProjectEnv(r)
	if err != nil {
		return nil, err
	}
	logDrainCacheMu.Lock()
	defer logDrainCacheMu.Unlock()
	if store, ok := logDrainCache[projectID]; ok {
		return store, nil
	}
	store = logdrain.NewStore(env.Engine.DB())
	logDrainCache[projectID] = store
	return store, nil
}

func (a *AdminServer) handleAdminListLogDrains(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectLogDrainStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	drains := store.List()
	if drains == nil {
		drains = []*logdrain.Drain{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  drains,
		"count": len(drains),
	})
}

func (a *AdminServer) handleAdminCreateLogDrain(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectLogDrainStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	var d logdrain.Drain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	created, err := store.Create(&d)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *AdminServer) handleAdminDeleteLogDrain(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectLogDrainStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	drainID := r.PathValue("drainId")
	if err := store.Delete(drainID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
