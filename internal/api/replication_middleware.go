package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ketsuna-org/sovrabase/internal/replication"
)

// replicationRoleMiddleware blocks write operations on read-only replication nodes.
func (s *Server) replicationRoleMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s.cluster == nil {
				next.ServeHTTP(w, r)
				return
			}

			role := s.cluster.Role()
			if role == replication.RoleMaster {
				next.ServeHTTP(w, r)
				return
			}

			method := strings.ToUpper(r.Method)
			if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path
			if path == "/health" || path == "/metrics" || strings.HasPrefix(path, "/realtime/") {
				next.ServeHTTP(w, r)
				return
			}

			writeReplicationUnavailable(w, role, s.cluster.Config())
		})
	}
}

// ReplicationMiddleware returns middleware that blocks admin writes on read-only nodes.
func (a *AdminServer) ReplicationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.cluster == nil {
			next.ServeHTTP(w, r)
			return
		}

		role := a.cluster.Role()
		if role == replication.RoleMaster {
			next.ServeHTTP(w, r)
			return
		}

		method := strings.ToUpper(r.Method)
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		writeReplicationUnavailable(w, role, a.cluster.Config())
	})
}

func writeReplicationUnavailable(w http.ResponseWriter, role replication.Role, cfg *replication.NodeConfig) {
	masterAddr := ""
	if cfg != nil && len(cfg.Peers) > 0 {
		masterAddr = cfg.Peers[0]
	}
	w.Header().Set("X-Sovrabase-Role", string(role))
	if masterAddr != "" {
		w.Header().Set("X-Sovrabase-Write-Leader", masterAddr)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   "write_unavailable",
		"message": "this node is read-only; route writes to the master",
		"role":    string(role),
	})
}
