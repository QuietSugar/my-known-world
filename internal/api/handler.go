// Package api exposes graph queries, sync preview, and explicitly authenticated writes.
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type SyncPreviewer interface {
	PreviewSync(context.Context, graph.SourceBatch) (graph.SyncPreview, error)
}
type ManualMutationPlanner interface {
	PreviewManualMutation(context.Context, graph.ManualMutation) (graph.MutationPlan, error)
}
type ManualMutationApplier interface {
	ApplyManualMutation(context.Context, graph.ManualMutation, string, string) (graph.MutationPlan, error)
}
type MutationOperationReader interface {
	MutationOperations(context.Context, int) ([]graph.MutationOperation, error)
}
type OperationUndoer interface {
	UndoOperation(context.Context, int64) error
}
type NodeCreator interface {
	CreateNode(context.Context, graph.Node, graph.Relation) error
}
type NodeEditor interface {
	UpdateNode(context.Context, string, graph.Node) error
}
type RelationEditor interface {
	AddRelation(context.Context, graph.Relation) error
	DeleteRelation(context.Context, string) error
}
type SourcePolicyReader interface {
	SourcePolicies(context.Context) ([]graph.SourcePolicy, error)
}
type SourcePolicyWriter interface {
	SetSourcePolicy(context.Context, graph.SourcePolicy) error
}
type FieldResolutionWriter interface {
	ResolveField(context.Context, graph.FieldResolution) (graph.FieldResolution, error)
}
type ProvisionalEvidenceReviewer interface {
	ReviewProvisionalWithEvidence(context.Context, string, graph.NodeType, graph.ProvisionalReviewEvidence) error
}
type AssertionViewReader interface {
	AssertionViews(context.Context, string) ([]graph.AssertionView, error)
}
type FieldResolutionReader interface {
	FieldResolutions(context.Context, string) ([]graph.FieldResolution, error)
}
type EntityHistoryReader interface {
	EntityHistory(context.Context, string, string, int) ([]graph.ChangeEvent, error)
}
type SyncHistoryReader interface {
	SyncSources(context.Context) ([]graph.SyncSourceState, error)
	SyncHistory(context.Context, string, int) ([]graph.SyncRun, error)
}
type Repository interface {
	SyncPreviewer
	LoadSnapshot(context.Context) (*graph.Graph, error)
	ApplySync(context.Context, graph.SourceBatch) (*graph.SyncPreview, error)
	ReviewProvisional(context.Context, string, graph.NodeType) error
	RejectProvisional(context.Context, string) error
	DeleteNode(context.Context, string) error
}

type Handler struct {
	Graph         *graph.Graph
	SyncPreviewer SyncPreviewer
	Repository    Repository
	WriteToken    string
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	path := strings.TrimSuffix(r.URL.Path, "/")
	if r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/nodes/") {
		h.deleteNode(w, r, path)
		return
	}
	if r.Method == http.MethodDelete && strings.HasPrefix(path, "/api/relations/") {
		h.deleteRelation(w, r, path)
		return
	}
	if r.Method == http.MethodPut && strings.HasPrefix(path, "/api/nodes/") {
		h.updateNode(w, r, path)
		return
	}
	if r.Method == http.MethodPost {
		switch {
		case path == "/api/sync/preview":
			h.preview(w, r)
			return
		case path == "/api/relations":
			h.addRelation(w, r)
			return
		case path == "/api/nodes":
			h.createNode(w, r)
			return
		case path == "/api/operations/preview":
			h.previewManualMutation(w, r)
			return
		case path == "/api/operations/apply":
			h.applyManualMutation(w, r)
			return
		case strings.HasPrefix(path, "/api/operations/") && strings.HasSuffix(path, "/undo"):
			h.undoOperation(w, r, path)
			return
		case path == "/api/sync/apply":
			h.apply(w, r)
			return
		case path == "/api/assertions/resolve":
			h.resolveAssertion(w, r)
			return
		case path == "/api/sync/policies":
			h.setSourcePolicy(w, r)
			return
		case strings.HasPrefix(path, "/api/provisionals/"):
			h.review(w, r, path)
			return
		}
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
		return
	}
	if path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "writesEnabled": len(h.WriteToken) >= 32 && h.Repository != nil})
		return
	}
	g, err := h.currentGraph(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graph_load_failed", err.Error())
		return
	}
	if g == nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "graph is not configured")
		return
	}
	switch {
	case path == "/api/nodes":
		writeJSON(w, http.StatusOK, map[string]any{"items": g.Nodes()})
	case path == "/api/relations":
		writeJSON(w, http.StatusOK, map[string]any{"items": g.Relations()})
	case path == "/api/impact":
		writeJSON(w, http.StatusOK, map[string]any{"items": g.ImpactFromSelf()})
	case path == "/api/provisionals":
		writeJSON(w, http.StatusOK, map[string]any{"items": g.ProvisionalQueue()})
	case strings.HasPrefix(path, "/api/entities/") && strings.HasSuffix(path, "/history"):
		reader, ok := h.Repository.(EntityHistoryReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "history_unavailable", "entity history requires PostgreSQL")
			return
		}
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(path, "/api/entities/"), "/history"), "/")
		if len(parts) != 2 {
			writeError(w, http.StatusBadRequest, "invalid_entity", "entity type and id required")
			return
		}
		kind, err := url.PathUnescape(parts[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_entity", "entity type invalid")
			return
		}
		id, err := url.PathUnescape(parts[1])
		if err != nil || id == "" {
			writeError(w, http.StatusBadRequest, "invalid_entity", "entity id invalid")
			return
		}
		items, err := reader.EntityHistory(r.Context(), kind, id, 100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case path == "/api/operations":
		reader, ok := h.Repository.(MutationOperationReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "operations_unavailable", "operation history requires PostgreSQL")
			return
		}
		items, err := reader.MutationOperations(r.Context(), 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "operations_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case path == "/api/sync/policies":
		reader, ok := h.Repository.(SourcePolicyReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "policy_unavailable", "source policies require PostgreSQL")
			return
		}
		items, err := reader.SourcePolicies(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "policy_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case path == "/api/sync/sources":
		reader, ok := h.Repository.(SyncHistoryReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "history_unavailable", "sync history requires PostgreSQL")
			return
		}
		items, err := reader.SyncSources(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case strings.HasPrefix(path, "/api/sync/sources/") && strings.HasSuffix(path, "/history"):
		reader, ok := h.Repository.(SyncHistoryReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "history_unavailable", "sync history requires PostgreSQL")
			return
		}
		sourcePart := strings.TrimSuffix(strings.TrimPrefix(path, "/api/sync/sources/"), "/history")
		source, err := url.PathUnescape(strings.TrimSuffix(sourcePart, "/"))
		if err != nil || source == "" {
			writeError(w, http.StatusBadRequest, "invalid_source", "source is invalid")
			return
		}
		items, err := reader.SyncHistory(r.Context(), source, 100)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case strings.HasPrefix(path, "/api/nodes/") && strings.HasSuffix(path, "/resolutions"):
		reader, ok := h.Repository.(FieldResolutionReader)
		if !ok {
			writeError(w, http.StatusServiceUnavailable, "resolution_unavailable", "field resolutions require PostgreSQL")
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/nodes/"), "/resolutions")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		items, err := reader.FieldResolutions(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "resolution_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case strings.HasPrefix(path, "/api/nodes/") && strings.HasSuffix(path, "/assertions"):
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/nodes/"), "/assertions")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		if _, ok := g.Node(id); !ok {
			writeError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		var views []graph.AssertionView
		if reader, ok := h.Repository.(AssertionViewReader); ok {
			views, err = reader.AssertionViews(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "assertion_query_failed", err.Error())
				return
			}
		} else {
			views = graph.BuildAssertionViews(g.AssertionsForNode(id), nil)
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": views})
	case strings.HasPrefix(path, "/api/nodes/"):
		id := strings.TrimPrefix(path, "/api/nodes/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		n, ok := g.Node(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		writeJSON(w, http.StatusOK, n)
	case strings.HasPrefix(path, "/api/explore/"):
		id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/explore/"))
		if err != nil || id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusBadRequest, "invalid_center", "center node id is invalid")
			return
		}
		depth := 2
		if raw := r.URL.Query().Get("depth"); raw != "" {
			depth, err = strconv.Atoi(raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_depth", "depth must be an integer")
				return
			}
		}
		predicates := []graph.Predicate{}
		for _, p := range strings.Split(r.URL.Query().Get("predicates"), ",") {
			if p = strings.TrimSpace(p); p != "" {
				predicates = append(predicates, graph.Predicate(p))
			}
		}
		result, err := g.Explore(graph.ExploreOptions{CenterID: id, Direction: r.URL.Query().Get("direction"), Depth: depth, Predicates: predicates})
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "center node not found") {
				status = http.StatusNotFound
			}
			writeError(w, status, "exploration_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	case strings.HasPrefix(path, "/api/path/"):
		id := strings.TrimPrefix(path, "/api/path/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		ids, ok := g.PathFromSelf(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "node or Self path not found")
			return
		}
		nodes := make([]graph.Node, 0, len(ids))
		for _, nodeID := range ids {
			if n, exists := g.Node(nodeID); exists {
				nodes = append(nodes, n)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "relations": g.RelationsForPath(ids)})
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) currentGraph(ctx context.Context) (*graph.Graph, error) {
	if h.Repository != nil {
		return h.Repository.LoadSnapshot(ctx)
	}
	return h.Graph, nil
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	var batch graph.SourceBatch
	if err := decodeBody(w, r, 8<<20, &batch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid sync batch: %v", err))
		return
	}
	previewer := h.SyncPreviewer
	if previewer == nil {
		if h.Graph == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "graph is not configured")
			return
		}
		writeJSON(w, http.StatusOK, h.Graph.PreviewSync(batch))
		return
	}
	preview, err := previewer.PreviewSync(r.Context(), batch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync_preview_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handler) setSourcePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(SourcePolicyWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "policy_unavailable", "source policies require PostgreSQL")
		return
	}
	var policy graph.SourcePolicy
	if err := decodeBody(w, r, 32<<10, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := policy.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_policy", err.Error())
		return
	}
	if err := writer.SetSourcePolicy(r.Context(), policy); err != nil {
		writeError(w, http.StatusInternalServerError, "policy_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "policy": policy})
}

func (h *Handler) resolveAssertion(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(FieldResolutionWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "resolution_unavailable", "field resolutions require PostgreSQL")
		return
	}
	var resolution graph.FieldResolution
	if err := decodeBody(w, r, 32<<10, &resolution); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid field resolution: %v", err))
		return
	}
	if err := resolution.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_resolution", err.Error())
		return
	}
	created, err := writer.ResolveField(r.Context(), resolution)
	if err != nil {
		writeError(w, http.StatusConflict, "resolution_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	if h.Repository == nil {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "persistent graph repository is not configured; set DATABASE_URL to a PostgreSQL connection string to enable persistent writes")
		return
	}
	var batch graph.SourceBatch
	if err := decodeBody(w, r, 8<<20, &batch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid sync batch: %v", err))
		return
	}
	preview, err := h.Repository.ApplySync(r.Context(), batch)
	if err != nil {
		writeError(w, http.StatusConflict, "sync_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "preview": preview})
}

func mutationPlanMAC(token string, mutation graph.ManualMutation) (string, error) {
	raw, err := json.Marshal(mutation)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(token))
	if _, err = mac.Write(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (h *Handler) previewManualMutation(w http.ResponseWriter, r *http.Request) {
	planner, ok := h.Repository.(ManualMutationPlanner)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "preview_unavailable", "manual mutation preview requires PostgreSQL")
		return
	}
	var body struct {
		Mutation graph.ManualMutation `json:"mutation"`
	}
	if err := decodeBody(w, r, 1<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	plan, err := planner.PreviewManualMutation(r.Context(), body.Mutation)
	if err == nil {
		plan.PlanHash, err = mutationPlanMAC(h.WriteToken, body.Mutation)
	}
	if err != nil {
		writeError(w, http.StatusConflict, "preview_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}
func (h *Handler) applyManualMutation(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	applier, ok := h.Repository.(ManualMutationApplier)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "manual mutation apply requires PostgreSQL")
		return
	}
	var body struct {
		Mutation         graph.ManualMutation `json:"mutation"`
		ExpectedRevision string               `json:"expectedRevision"`
		PlanHash         string               `json:"planHash"`
	}
	if err := decodeBody(w, r, 1<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	expectedHash, hashErr := mutationPlanMAC(h.WriteToken, body.Mutation)
	if hashErr != nil || !hmac.Equal([]byte(expectedHash), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "mutation_rejected", "plan hash does not match the previewed payload")
		return
	}
	plan, err := applier.ApplyManualMutation(r.Context(), body.Mutation, body.ExpectedRevision, body.PlanHash)
	if err != nil {
		writeError(w, http.StatusConflict, "mutation_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "plan": plan})
}

func (h *Handler) undoOperation(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	undoer, ok := h.Repository.(OperationUndoer)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "undo_unavailable", "undo requires PostgreSQL")
		return
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(path, "/api/operations/"), "/undo")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_operation", "operation ID must be a positive integer")
		return
	}
	if err = undoer.UndoOperation(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "undo_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"undone": id})
}

func (h *Handler) createNode(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	creator, ok := h.Repository.(NodeCreator)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "node creation requires PostgreSQL")
		return
	}
	var body struct {
		Node     graph.Node     `json:"node"`
		Relation graph.Relation `json:"relation"`
	}
	if err := decodeBody(w, r, 1<<20, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := creator.CreateNode(r.Context(), body.Node, body.Relation); err != nil {
		writeError(w, http.StatusConflict, "create_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"created": body.Node.ID})
}

func (h *Handler) updateNode(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	editor, ok := h.Repository.(NodeEditor)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "node editing requires PostgreSQL")
		return
	}
	id := strings.TrimPrefix(path, "/api/nodes/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	var node graph.Node
	if err := decodeBody(w, r, 1<<20, &node); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if node.ID != id {
		writeError(w, http.StatusBadRequest, "invalid_node", "body ID must match URL ID")
		return
	}
	if err := editor.UpdateNode(r.Context(), id, node); err != nil {
		writeError(w, http.StatusConflict, "update_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": id})
}
func (h *Handler) addRelation(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	editor, ok := h.Repository.(RelationEditor)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "relation editing requires PostgreSQL")
		return
	}
	var relation graph.Relation
	if err := decodeBody(w, r, 64<<10, &relation); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := editor.AddRelation(r.Context(), relation); err != nil {
		writeError(w, http.StatusConflict, "relation_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"created": relation.ID})
}
func (h *Handler) deleteRelation(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	editor, ok := h.Repository.(RelationEditor)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "relation editing requires PostgreSQL")
		return
	}
	id := strings.TrimPrefix(path, "/api/relations/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if err := editor.DeleteRelation(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "delete_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *Handler) deleteNode(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	if h.Repository == nil {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "persistent graph repository is not configured; set DATABASE_URL to a PostgreSQL connection string to enable persistent writes")
		return
	}
	id := strings.TrimPrefix(path, "/api/nodes/")
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if err := h.Repository.DeleteNode(r.Context(), id); err != nil {
		writeError(w, http.StatusConflict, "delete_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	if h.Repository == nil {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "persistent graph repository is not configured; set DATABASE_URL to a PostgreSQL connection string to enable persistent writes")
		return
	}
	rest := strings.TrimPrefix(path, "/api/provisionals/")
	idx := strings.LastIndex(rest, "/")
	if idx <= 0 {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	id, action := rest[:idx], rest[idx+1:]
	if action != "review" && action != "reject" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if action == "reject" {
		if err := h.Repository.RejectProvisional(r.Context(), id); err != nil {
			writeError(w, http.StatusConflict, "review_rejected", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "id": id})
		return
	}
	var body struct {
		TargetType   graph.NodeType     `json:"targetType"`
		Certainty    graph.Certainty    `json:"certainty,omitempty"`
		EvidenceKind graph.EvidenceKind `json:"evidenceKind,omitempty"`
		Source       string             `json:"source,omitempty"`
		Reference    string             `json:"reference,omitempty"`
	}
	if err := decodeBody(w, r, 16<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid review: %v", err))
		return
	}
	var reviewErr error
	if reviewer, ok := h.Repository.(ProvisionalEvidenceReviewer); ok {
		reviewErr = reviewer.ReviewProvisionalWithEvidence(r.Context(), id, body.TargetType, graph.ProvisionalReviewEvidence{Certainty: body.Certainty, EvidenceKind: body.EvidenceKind, Source: body.Source, Reference: body.Reference})
	} else if body.Certainty != "" || body.EvidenceKind != "" || body.Source != "" || body.Reference != "" {
		reviewErr = fmt.Errorf("repository does not support review evidence")
	} else {
		reviewErr = h.Repository.ReviewProvisional(r.Context(), id, body.TargetType)
	}
	if err := reviewErr; err != nil {
		writeError(w, http.StatusConflict, "review_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "classified", "id": id, "type": string(body.TargetType)})
}

func (h *Handler) authorized(w http.ResponseWriter, r *http.Request) bool {
	if len(h.WriteToken) < 32 {
		writeError(w, http.StatusServiceUnavailable, "writes_disabled", "set a stable MYKNOWNWORLD_WRITE_TOKEN of at least 32 bytes to enable writes")
		return false
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "unauthorized", "bearer token required")
		return false
	}
	provided := strings.TrimPrefix(header, "Bearer ")
	if len(provided) != len(h.WriteToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(h.WriteToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
		return false
	}
	return true
}

func decodeBody(w http.ResponseWriter, r *http.Request, limit int64, dst any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request must contain one JSON value")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
