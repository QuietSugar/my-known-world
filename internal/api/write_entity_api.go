package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// EntityWriter defines programmatic update and delete capabilities for the write API.
type EntityWriter interface {
	PatchNode(ctx context.Context, id string, patch graph.Node, expectedRevision string) (graph.Node, string, error)
	DeleteNodeWithRevision(ctx context.Context, id string, expectedRevision string) (string, error)
	PatchRelation(ctx context.Context, id string, patch graph.Relation, expectedRevision string) (graph.Relation, string, error)
	DeleteRelationWithRevision(ctx context.Context, id string, expectedRevision string) (string, error)
	RetireNode(ctx context.Context, id string, expectedRevision string) (graph.Node, string, error)
}

func (h *Handler) patchNodeAPI(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(EntityWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "write API requires PostgreSQL")
		return
	}
	var body struct {
		ExpectedRevision string     `json:"expectedRevision"`
		Node             graph.Node `json:"node"`
	}
	if err := decodeBody(w, r, createBodyLimit, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if body.ExpectedRevision == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedRevision is required")
		return
	}
	updated, revision, err := writer.PatchNode(r.Context(), id, body.Node, body.ExpectedRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "write_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"node": updated, "revision": revision})
}

func (h *Handler) deleteNodeAPI(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(EntityWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "write API requires PostgreSQL")
		return
	}
	expectedRevision := r.URL.Query().Get("expectedRevision")
	if r.Body != nil && r.ContentLength > 0 {
		var body struct {
			ExpectedRevision string `json:"expectedRevision"`
		}
		if err := decodeBody(w, r, createBodyLimit, &body); err == nil && body.ExpectedRevision != "" {
			expectedRevision = body.ExpectedRevision
		}
	}
	if expectedRevision == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedRevision is required")
		return
	}
	revision, err := writer.DeleteNodeWithRevision(r.Context(), id, expectedRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "write_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":  map[string]any{"nodeId": id},
		"revision": revision,
	})
}

func (h *Handler) patchRelationAPI(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(EntityWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "write API requires PostgreSQL")
		return
	}
	var body struct {
		ExpectedRevision string         `json:"expectedRevision"`
		Relation         graph.Relation `json:"relation"`
	}
	if err := decodeBody(w, r, createBodyLimit, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if body.ExpectedRevision == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedRevision is required")
		return
	}
	updated, revision, err := writer.PatchRelation(r.Context(), id, body.Relation, body.ExpectedRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "write_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relation": updated, "revision": revision})
}

func (h *Handler) deleteRelationAPI(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(EntityWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "write API requires PostgreSQL")
		return
	}
	expectedRevision := r.URL.Query().Get("expectedRevision")
	if r.Body != nil && r.ContentLength > 0 {
		var body struct {
			ExpectedRevision string `json:"expectedRevision"`
		}
		if err := decodeBody(w, r, createBodyLimit, &body); err == nil && body.ExpectedRevision != "" {
			expectedRevision = body.ExpectedRevision
		}
	}
	if expectedRevision == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedRevision is required")
		return
	}
	revision, err := writer.DeleteRelationWithRevision(r.Context(), id, expectedRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "write_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted":  map[string]any{"relationId": id},
		"revision": revision,
	})
}

func (h *Handler) retireNodeAPI(w http.ResponseWriter, r *http.Request, id string) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(EntityWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "write API requires PostgreSQL")
		return
	}
	var body struct {
		ExpectedRevision string `json:"expectedRevision,omitempty"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if err := decodeBody(w, r, createBodyLimit, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
	}
	updated, revision, err := writer.RetireNode(r.Context(), id, body.ExpectedRevision)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		writeError(w, http.StatusConflict, "write_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"node": updated, "revision": revision})
}
