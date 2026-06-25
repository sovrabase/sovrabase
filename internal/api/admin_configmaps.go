package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ketsuna-org/sovrabase/internal/configmaps"
)

// getProjectConfigStore returns a configmaps.Store for the project in the URL.
func (a *AdminServer) getProjectConfigStore(r *http.Request) (*configmaps.Store, error) {
	projectID := r.PathValue("id")
	if projectID == "" {
		return nil, fmt.Errorf("project not found")
	}
	env, err := a.projects.GetProjectEnv(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found")
	}
	return configmaps.NewStore(env.Engine.DB()), nil
}

// handleAdminListConfig returns all remote config entries for a project.
func (a *AdminServer) handleAdminListConfig(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectConfigStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	entries, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []*configmaps.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  entries,
		"count": len(entries),
	})
}

// handleAdminSetConfig creates or updates a remote config entry.
func (a *AdminServer) handleAdminSetConfig(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectConfigStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Key         string      `json:"key"`
		Value       interface{} `json:"value"`
		Type        string      `json:"type"`
		Description string      `json:"description"`
		Public      bool        `json:"public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	entry, err := store.Set(req.Key, req.Value, configmaps.ValueType(req.Type), req.Description, req.Public)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// handleAdminDeleteConfig removes a remote config entry.
func (a *AdminServer) handleAdminDeleteConfig(w http.ResponseWriter, r *http.Request) {
	store, err := a.getProjectConfigStore(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if err := store.Delete(key); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
