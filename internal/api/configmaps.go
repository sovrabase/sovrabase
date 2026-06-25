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
