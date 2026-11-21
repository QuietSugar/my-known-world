package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type SearchResponse struct {
	Items []graph.NodeSummary `json:"items"`
	Query string              `json:"query"`
	Count int                 `json:"count"`
}

func (h *Handler) searchAPI(w http.ResponseWriter, r *http.Request, g *graph.Graph) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 1 {
		writeError(w, http.StatusBadRequest, "invalid_request", "parameter 'q' is required (minimum 1 char)")
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 50 {
			writeError(w, http.StatusBadRequest, "invalid_request", "limit must be an integer between 1 and 50")
			return
		}
		limit = parsed
	}
	items := g.Search(q, limit)
	writeJSON(w, http.StatusOK, SearchResponse{
		Items: items,
		Query: q,
		Count: len(items),
	})
}
