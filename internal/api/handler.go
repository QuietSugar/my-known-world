// Package api exposes read-only HTTP views over the domain graph.
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type Handler struct{ Graph *graph.Graph }

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is supported")
		return
	}
	if h.Graph == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "graph is not configured")
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case path == "/api/nodes":
		writeJSON(w, http.StatusOK, map[string]any{"items": h.Graph.Nodes()})
	case path == "/api/impact":
		writeJSON(w, http.StatusOK, map[string]any{"items": h.Graph.ImpactFromSelf()})
	case strings.HasPrefix(path, "/api/nodes/"):
		id := strings.TrimPrefix(path, "/api/nodes/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		n, ok := h.Graph.Node(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		writeJSON(w, http.StatusOK, n)
	case strings.HasPrefix(path, "/api/path/"):
		id := strings.TrimPrefix(path, "/api/path/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		ids, ok := h.Graph.PathFromSelf(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "node or Self path not found")
			return
		}
		nodes := make([]graph.Node, 0, len(ids))
		for _, nodeID := range ids {
			if n, exists := h.Graph.Node(nodeID); exists {
				nodes = append(nodes, n)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "relations": h.Graph.RelationsForPath(ids)})
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
