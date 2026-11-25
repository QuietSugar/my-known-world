package api

import (
	"context"
	"net/http"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// CreateBatchWriter is implemented by the PostgreSQL store. It applies a create
// batch atomically and returns the applied result plus the resulting revision.
type CreateBatchWriter interface {
	ApplyCreateBatch(context.Context, graph.CreateBatch, string) (graph.CreateResult, string, error)
}

const (
	createBodyLimit  = 256 << 10
	createBatchLimit = 1 << 20
)

// createNode creates one node together with its Self anchor relation. The
// project never publishes a node without that relation, so callers do not have
// to build it themselves.
func (h *Handler) createNodeAPI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Node             graph.Node          `json:"node"`
		Anchor           *graph.CreateAnchor `json:"anchor,omitempty"`
		ExpectedRevision string              `json:"expectedRevision,omitempty"`
	}
	if err := decodeBody(w, r, createBodyLimit, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	h.applyCreate(w, r, graph.CreateBatch{
		Nodes: []graph.CreateNodeRequest{{Node: body.Node, Anchor: body.Anchor}},
	}, body.ExpectedRevision, http.StatusCreated)
}

// createRelation creates one relation between existing nodes.
func (h *Handler) createRelationAPI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Relation         graph.Relation `json:"relation"`
		ExpectedRevision string         `json:"expectedRevision,omitempty"`
	}
	if err := decodeBody(w, r, createBodyLimit, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	h.applyCreate(w, r, graph.CreateBatch{Relations: []graph.Relation{body.Relation}}, body.ExpectedRevision, http.StatusCreated)
}

// createBatch creates many nodes and relations in one transaction.
func (h *Handler) createBatchAPI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Nodes            []graph.CreateNodeRequest `json:"nodes,omitempty"`
		Relations        []graph.Relation          `json:"relations,omitempty"`
		ExpectedRevision string                    `json:"expectedRevision,omitempty"`
	}
	if err := decodeBody(w, r, createBatchLimit, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	h.applyCreate(w, r, graph.CreateBatch{Nodes: body.Nodes, Relations: body.Relations}, body.ExpectedRevision, http.StatusCreated)
}

func (h *Handler) applyCreate(w http.ResponseWriter, r *http.Request, batch graph.CreateBatch, expectedRevision string, status int) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(CreateBatchWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "create API requires PostgreSQL")
		return
	}
	result, revision, err := writer.ApplyCreateBatch(r.Context(), batch, expectedRevision)
	if err != nil {
		// Domain rejections (duplicate ids, invalid endpoints, stale revision)
		// are conflicts rather than malformed requests; the message says which.
		writeError(w, http.StatusConflict, "create_rejected", err.Error())
		return
	}
	h.invalidateCache()
	writeJSON(w, status, map[string]any{"created": result, "revision": revision})
}
