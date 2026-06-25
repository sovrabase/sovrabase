package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/ketsuna-org/sovrabase/internal/queue"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

var (
	queueCacheMu sync.RWMutex
	queueCache   = make(map[string]*queue.Store)
)

func getQueueStore(projectID string, projects *tenant.ProjectManager) (*queue.Store, error) {
	queueCacheMu.RLock()
	store, ok := queueCache[projectID]
	queueCacheMu.RUnlock()
	if ok {
		return store, nil
	}

	env, err := projects.GetProjectEnv(projectID)
	if err != nil {
		return nil, err
	}

	queueCacheMu.Lock()
	defer queueCacheMu.Unlock()
	if store, ok := queueCache[projectID]; ok {
		return store, nil
	}
	store = queue.NewStore(env.Engine.DB())
	queueCache[projectID] = store
	return store, nil
}

// ─── Public API ──────────────────────────────────────────────────────────────

func (s *Server) handleQueueSend(w http.ResponseWriter, r *http.Request) {
	projectID := getProjectID(r)
	store, err := getQueueStore(projectID, s.projects)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "queue store not available")
		return
	}

	var req struct {
		Queue string                 `json:"queue"`
		Body  map[string]interface{} `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Queue == "" {
		writeError(w, http.StatusBadRequest, "queue name is required")
		return
	}

	id, err := store.Send(req.Queue, req.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "queue": req.Queue})
}

func (s *Server) handleQueueReceive(w http.ResponseWriter, r *http.Request) {
	projectID := getProjectID(r)
	store, err := getQueueStore(projectID, s.projects)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "queue store not available")
		return
	}

	var req struct {
		Queue string `json:"queue"`
		Limit int    `json:"limit"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Queue == "" {
		writeError(w, http.StatusBadRequest, "queue name is required")
		return
	}

	messages, err := store.Receive(req.Queue, req.Limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []*queue.Message{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  messages,
		"count": len(messages),
	})
}

func (s *Server) handleQueueDelete(w http.ResponseWriter, r *http.Request) {
	projectID := getProjectID(r)
	store, err := getQueueStore(projectID, s.projects)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "queue store not available")
		return
	}

	var req struct {
		Queue string `json:"queue"`
		ID    string `json:"id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := store.Delete(req.Queue, req.ID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Admin API ──────────────────────────────────────────────────────────────

func (a *AdminServer) handleAdminListQueues(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	store, err := getQueueStore(projectID, a.projects)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	queues, err := store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if queues == nil {
		queues = []queue.QueueInfo{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  queues,
		"count": len(queues),
	})
}

func (a *AdminServer) handleAdminPurgeQueue(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	store, err := getQueueStore(projectID, a.projects)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		Queue string `json:"queue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := store.Purge(req.Queue); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "purged"})
}

func (a *AdminServer) handleAdminMakeVisible(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	store, err := getQueueStore(projectID, a.projects)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	queueName := r.PathValue("queueName")
	if err := store.MakeVisible(queueName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "made_visible"})
}
