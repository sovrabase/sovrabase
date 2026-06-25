package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ketsuna-org/sovrabase/internal/configmaps"
)

// configmapsStore returns a configmaps.Store for the project in context.
func (s *Server) configmapsStore(r *http.Request) *configmaps.Store {
	if env := getProjectEnv(r); env != nil {
		return configmaps.NewStore(env.Engine.DB())
	}
	return nil
}

// RegisterConfigMapsRoutes registers remote config endpoints under /api/v1/config.
// Public endpoints (/public) bypass auth; all others require it.
func (s *Server) RegisterConfigMapsRoutes() {
	s.router.Route("/api/v1/config", func(r chi.Router) {
		r.Use(s.projectMiddleware)
		r.Use(s.meteringMiddleware)

		// Public endpoint — no auth required, returns only public entries.
		r.Get("/public", s.handleConfigPublic)

		// Authenticated endpoints in a sub-group.
		r.Group(func(r chi.Router) {
			r.Use(s.rateLimitMiddleware)
			r.Use(s.clientRequestLoggerMiddleware)
			r.Use(s.authMiddleware)
			r.Get("/", s.handleConfigList)
			r.Get("/{key}", s.handleConfigGet)
			r.Post("/", s.handleConfigSet)
			r.Put("/{key}", s.handleConfigSet)
			r.Delete("/{key}", s.handleConfigDelete)
		})
	})
}

// ─── Public endpoint ──────────────────────────────────────────────────────────

// handleConfigPublic returns all public config entries as a key→value map.
// @Summary      List public config
// @Description  Returns all public config entries as a simple key→value map. No authentication required.
// @Tags         config
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]string
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config/public [get]
func (s *Server) handleConfigPublic(w http.ResponseWriter, r *http.Request) {
	store := s.configmapsStore(r)
	if store == nil {
		writeError(w, http.StatusInternalServerError, "config store not available")
		return
	}
	entries, err := store.ListPublic()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Return as a simple key→value map for easy client consumption.
	result := make(map[string]interface{}, len(entries))
	for _, e := range entries {
		result[e.Key] = e.Value
	}
	writeJSON(w, http.StatusOK, result)
}

// ─── Authenticated endpoints ──────────────────────────────────────────────────

// handleConfigList lists all config entries.
// @Summary      List config entries
// @Description  Returns all config entries for the project.
// @Tags         config
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]string
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config [get]
func (s *Server) handleConfigList(w http.ResponseWriter, r *http.Request) {
	store := s.configmapsStore(r)
	if store == nil {
		writeError(w, http.StatusInternalServerError, "config store not available")
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

// handleConfigGet returns a single config entry by key.
// @Summary      Get config entry
// @Description  Returns a single config entry by its key.
// @Tags         config
// @Produce      json
// @Security     BearerAuth
// @Param        key  path  string  true  "Config key"
// @Success      200  {object}  configmaps.Entry
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config/{key} [get]
func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	store := s.configmapsStore(r)
	if store == nil {
		writeError(w, http.StatusInternalServerError, "config store not available")
		return
	}
	key := chi.URLParam(r, "key")
	entry, err := store.Get(key)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

type configSetRequest struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Public      bool        `json:"public"`
}

// handleConfigSet creates or updates a config entry.
// @Summary      Create or update config entry
// @Description  Creates a new config entry (POST) or updates an existing one (PUT). On successful create returns 201, on update returns 200.
// @Tags         config
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        key    path      string            false  "Config key (for PUT)"
// @Param        body   body      configSetRequest  true   "Config entry"
// @Success      200    {object}  configmaps.Entry
// @Success      201    {object}  configmaps.Entry
// @Failure      400    {object}  map[string]string
// @Failure      500    {object}  map[string]string
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config [post]
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config/{key} [put]
func (s *Server) handleConfigSet(w http.ResponseWriter, r *http.Request) {
	store := s.configmapsStore(r)
	if store == nil {
		writeError(w, http.StatusInternalServerError, "config store not available")
		return
	}
	var req configSetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Key can come from URL param ({key}) or body.
	key := chi.URLParam(r, "key")
	if key == "" {
		key = req.Key
	}
	if key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if req.Value == nil {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}

	entry, err := store.Set(key, req.Value, configmaps.ValueType(req.Type), req.Description, req.Public)
	if err != nil {
		if strings.HasPrefix(err.Error(), "configmaps: key") {
			writeError(w, http.StatusBadRequest, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Determine if this was a create or update.
	isCreate := r.Method == http.MethodPost
	status := http.StatusOK
	if isCreate {
		if entry.CreatedAt.Equal(entry.UpdatedAt) {
			status = http.StatusCreated
		}
	}
	writeJSON(w, status, entry)
}

// handleConfigDelete deletes a config entry.
// @Summary      Delete config entry
// @Description  Deletes a config entry by its key.
// @Tags         config
// @Produce      json
// @Security     BearerAuth
// @Param        key  path  string  true  "Config key"
// @Success      200  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Param        X-Project-Key  header  string  true  "Project API key for multi-tenant isolation"
// @Router       /api/v1/config/{key} [delete]
func (s *Server) handleConfigDelete(w http.ResponseWriter, r *http.Request) {
	store := s.configmapsStore(r)
	if store == nil {
		writeError(w, http.StatusInternalServerError, "config store not available")
		return
	}
	key := chi.URLParam(r, "key")
	if err := store.Delete(key); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
