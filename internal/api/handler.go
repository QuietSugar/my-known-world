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

	"github.com/QuietSugar/my-known-world/internal/cmdb"
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
type UndoPlanWriter interface {
	PreviewUndoOperation(context.Context, int64) (graph.MutationPlan, error)
	UndoOperationPlanned(context.Context, int64, string) error
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
type GuardedSourcePolicyWriter interface {
	PreviewSourcePolicy(context.Context, graph.SourcePolicy) (graph.MutationPlan, *graph.SourcePolicy, error)
	ApplySourcePolicy(context.Context, graph.SourcePolicy, string) error
}
type GuardedFieldResolutionWriter interface {
	PreviewFieldResolution(context.Context, graph.FieldResolution) (graph.MutationPlan, error)
	ApplyFieldResolution(context.Context, graph.FieldResolution, string) (graph.FieldResolution, error)
}
type SyncPlanApplier interface {
	ApplySyncPlan(context.Context, graph.SourceBatch, string) (*graph.SyncPreview, error)
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
	Graph          *graph.Graph
	SyncPreviewer  SyncPreviewer
	Repository     Repository
	WriteToken     string
	SecretProvider cmdb.SecretProvider
	Cache          *SnapshotCache
	TemplateStore  TemplateStore
}

func (h Handler) invalidateCache() {
	if h.Cache != nil {
		h.Cache.Invalidate()
	}
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Match routes against the escaped path so an encoded slash remains inside
	// one identifier segment until that segment is explicitly unescaped.
	path := strings.TrimSuffix(r.URL.EscapedPath(), "/")
	if strings.HasPrefix(path, "/api/workspace/projects/") && strings.Contains(path, "/templates") {
		subPath := strings.TrimPrefix(path, "/api/workspace/projects/")
		h.projectTemplatesRoute(w, r, subPath)
		return
	}
	if strings.HasPrefix(path, "/api/workspace/projects/") && strings.Contains(path, "/export-static") {
		g, err := h.currentGraph(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "graph_load_failed", err.Error())
			return
		}
		subPath := strings.TrimPrefix(path, "/api/workspace/projects/")
		h.projectExportStaticRoute(w, r, g, subPath)
		return
	}
	if r.Method == http.MethodDelete && (strings.HasPrefix(path, "/api/nodes/") || strings.HasPrefix(path, "/api/relations/")) {
		h.rejectUnpreviewedWrite(w, r)
		return
	}
	if r.Method == http.MethodPut && strings.HasPrefix(path, "/api/nodes/") {
		h.rejectUnpreviewedWrite(w, r)
		return
	}
	if r.Method == http.MethodPost {
		switch {
		case strings.HasPrefix(path, "/api/write/nodes/") && strings.HasSuffix(path, "/retire"):
			rawID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/write/nodes/"), "/retire")
			id, err := url.PathUnescape(rawID)
			if err != nil || id == "" {
				writeError(w, http.StatusBadRequest, "invalid_id", "node id is invalid")
				return
			}
			h.retireNodeAPI(w, r, id)
			return
		case path == "/api/write/nodes":
			h.createNodeAPI(w, r)
			return
		case path == "/api/write/relations":
			h.createRelationAPI(w, r)
			return
		case path == "/api/write/batch":
			h.createBatchAPI(w, r)
			return
		case path == "/api/sync/preview":
			h.preview(w, r)
			return
		case path == "/api/relations" || path == "/api/nodes":
			h.rejectUnpreviewedWrite(w, r)
			return
		case path == "/api/operations/preview":
			h.previewManualMutation(w, r)
			return
		case path == "/api/operations/apply":
			h.applyManualMutation(w, r)
			return
		case strings.HasPrefix(path, "/api/operations/") && strings.HasSuffix(path, "/undo/preview"):
			h.previewUndo(w, r, path)
			return
		case strings.HasPrefix(path, "/api/operations/") && strings.HasSuffix(path, "/undo/apply"):
			h.applyUndo(w, r, path)
			return
		case strings.HasPrefix(path, "/api/operations/") && strings.HasSuffix(path, "/undo"):
			h.rejectUnpreviewedWrite(w, r)
			return
		case path == "/api/sync/apply":
			h.apply(w, r)
			return
		case path == "/api/assertions/resolve/preview":
			h.previewAssertionResolution(w, r)
			return
		case path == "/api/assertions/resolve/apply":
			h.applyAssertionResolution(w, r)
			return
		case path == "/api/assertions/resolve":
			h.rejectUnpreviewedWrite(w, r)
			return
		case path == "/api/sync/policies/preview":
			h.previewSourcePolicy(w, r)
			return
		case path == "/api/sync/policies/apply":
			h.applySourcePolicy(w, r)
			return
		case path == "/api/sync/policies":
			h.rejectUnpreviewedWrite(w, r)
			return
		case path == "/api/credentials/sync":
			h.credentialSyncAPI(w, r)
			return
		case path == "/api/credentials/upload-to-upstream" || path == "/api/credentials/upload":
			h.credentialUploadAPI(w, r)
			return
		case strings.HasPrefix(path, "/api/provisionals/"):
			h.rejectUnpreviewedWrite(w, r)
			return
		}
	}
	if r.Method == http.MethodPatch {
		switch {
		case strings.HasPrefix(path, "/api/write/nodes/"):
			id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/write/nodes/"))
			if err != nil || id == "" {
				writeError(w, http.StatusBadRequest, "invalid_id", "node id is invalid")
				return
			}
			h.patchNodeAPI(w, r, id)
			return
		case strings.HasPrefix(path, "/api/write/relations/"):
			id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/write/relations/"))
			if err != nil || id == "" {
				writeError(w, http.StatusBadRequest, "invalid_id", "relation id is invalid")
				return
			}
			h.patchRelationAPI(w, r, id)
			return
		}
	}
	if r.Method == http.MethodDelete {
		switch {
		case strings.HasPrefix(path, "/api/write/nodes/"):
			id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/write/nodes/"))
			if err != nil || id == "" {
				writeError(w, http.StatusBadRequest, "invalid_id", "node id is invalid")
				return
			}
			h.deleteNodeAPI(w, r, id)
			return
		case strings.HasPrefix(path, "/api/write/relations/"):
			id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/write/relations/"))
			if err != nil || id == "" {
				writeError(w, http.StatusBadRequest, "invalid_id", "relation id is invalid")
				return
			}
			h.deleteRelationAPI(w, r, id)
			return
		}
	}
	if r.Method != http.MethodGet {
		// Storing a password is a write and authenticates itself; every other
		// non-GET path is rejected here.
		if r.URL.Path == "/api/credentials/secret" && (r.Method == http.MethodPost || r.Method == http.MethodPut) {
			h.credentialStoreAPI(w, r)
			return
		}
		w.Header().Set("Allow", "GET, POST, PUT, DELETE, PATCH")
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
	case path == "/api/credentials/secret":
		h.credentialSecretAPI(w, r)
		return
	case path == "/api/credentials/status":
		h.credentialStatusAPI(w, r)
		return
	case path == "/metrics" || path == "/api/metrics":
		h.metricsAPI(w, r, g)
		return
	case path == "/api/search":
		h.searchAPI(w, r, g)
		return
	case path == "/api/patrol":
		h.patrolAPI(w, r, g)
		return
	case path == "/api/nodes":
		q := r.URL.Query()
		view := q.Get("view")
		if view != "" && view != "summary" && view != "full" {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid view parameter; must be 'summary' or 'full'")
			return
		}
		includeRetired := true
		if includeRetiredStr := q.Get("includeRetired"); includeRetiredStr != "" {
			parsed, err := strconv.ParseBool(includeRetiredStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "invalid includeRetired parameter; must be a boolean")
				return
			}
			includeRetired = parsed
		}
		allNodes := g.Nodes()
		if !includeRetired {
			filtered := make([]graph.Node, 0, len(allNodes))
			for _, n := range allNodes {
				if n.Lifecycle != "retired" {
					filtered = append(filtered, n)
				}
			}
			allNodes = filtered
		}
		page, total, hasMore, nextCursor, err := paginateSlice(allNodes, func(n graph.Node) string { return n.ID }, q)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		if nextCursor != "" {
			w.Header().Set("X-Next-Cursor", nextCursor)
		}
		var resultItems any = page
		if view == "summary" {
			summaries := make([]graph.NodeSummary, len(page))
			for i, n := range page {
				summaries[i] = n.Summary()
			}
			resultItems = summaries
		}
		resp := map[string]any{
			"items":      resultItems,
			"totalCount": total,
			"count":      len(page),
			"hasMore":    hasMore,
		}
		if nextCursor != "" {
			resp["nextCursor"] = nextCursor
		}
		writeJSON(w, http.StatusOK, resp)
		return
	case path == "/api/relations":
		q := r.URL.Query()
		allRels := g.Relations()
		page, total, hasMore, nextCursor, err := paginateSlice(allRels, func(r graph.Relation) string { return r.ID }, q)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		if nextCursor != "" {
			w.Header().Set("X-Next-Cursor", nextCursor)
		}
		resp := map[string]any{
			"items":      page,
			"totalCount": total,
			"count":      len(page),
			"hasMore":    hasMore,
		}
		if nextCursor != "" {
			resp["nextCursor"] = nextCursor
		}
		writeJSON(w, http.StatusOK, resp)
		return
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
		id, err := url.PathUnescape(id)
		if err != nil || id == "" {
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
		id, err := url.PathUnescape(id)
		if err != nil || id == "" {
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
		id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/nodes/"))
		if err != nil || id == "" {
			writeError(w, http.StatusNotFound, "not_found", "route not found")
			return
		}
		n, ok := g.Node(id)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "node not found")
			return
		}
		writeJSON(w, http.StatusOK, n)
	case strings.HasPrefix(path, "/api/workspace/projects/"):
		id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/workspace/projects/"))
		if err != nil || id == "" {
			writeError(w, http.StatusBadRequest, "invalid_id", "project id is invalid")
			return
		}
		h.projectWorkspaceAPI(w, r, g, id)
		return
	case strings.HasPrefix(path, "/api/explore/"):
		id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/explore/"))
		if err != nil || id == "" {
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
		id, err := url.PathUnescape(strings.TrimPrefix(path, "/api/path/"))
		if err != nil || id == "" {
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
		if h.Cache != nil {
			if cached := h.Cache.Get(); cached != nil {
				return cached, nil
			}
		}
		loaded, err := h.Repository.LoadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		if h.Cache != nil {
			h.Cache.Set(loaded)
		}
		return loaded, nil
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
	if preview.ExpectedRevision != "" {
		preview.PlanHash, err = guardedPlanMAC(h.WriteToken, "sync-batch", preview.ExpectedRevision, batch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "plan_failed", "could not create sync plan")
			return
		}
	}
	writeJSON(w, http.StatusOK, preview)
}

func guardedPlanMAC(token, kind string, revision string, payload any) (string, error) {
	body, err := json.Marshal(struct {
		Kind     string `json:"kind"`
		Revision string `json:"revision"`
		Payload  any    `json:"payload"`
	}{kind, revision, payload})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(token))
	if _, err = mac.Write(body); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func (h *Handler) previewSourcePolicy(w http.ResponseWriter, r *http.Request) {
	planner, ok := h.Repository.(GuardedSourcePolicyWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "policy_unavailable", "source policy preview requires PostgreSQL")
		return
	}
	var body struct {
		Policy graph.SourcePolicy `json:"policy"`
	}
	if err := decodeBody(w, r, 32<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	plan, previous, err := planner.PreviewSourcePolicy(r.Context(), body.Policy)
	if err != nil {
		writeError(w, http.StatusConflict, "preview_rejected", err.Error())
		return
	}
	plan.PlanHash, err = guardedPlanMAC(h.WriteToken, "source-policy", plan.ExpectedRevision, body.Policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_failed", "could not create preview plan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "previousPolicy": previous, "policy": body.Policy})
}

func (h *Handler) applySourcePolicy(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(GuardedSourcePolicyWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "policy_unavailable", "source policy apply requires PostgreSQL")
		return
	}
	var body struct {
		Policy           graph.SourcePolicy `json:"policy"`
		ExpectedRevision string             `json:"expectedRevision"`
		PlanHash         string             `json:"planHash"`
	}
	if err := decodeBody(w, r, 32<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	expected, err := guardedPlanMAC(h.WriteToken, "source-policy", body.ExpectedRevision, body.Policy)
	if err != nil || !hmac.Equal([]byte(expected), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "policy_rejected", "plan does not match the previewed policy and revision")
		return
	}
	if err = writer.ApplySourcePolicy(r.Context(), body.Policy, body.ExpectedRevision); err != nil {
		writeError(w, http.StatusConflict, "policy_rejected", err.Error())
		return
	}
	h.invalidateCache()
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "policy": body.Policy})
}

func (h *Handler) previewAssertionResolution(w http.ResponseWriter, r *http.Request) {
	planner, ok := h.Repository.(GuardedFieldResolutionWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "resolution_unavailable", "field resolution preview requires PostgreSQL")
		return
	}
	var body struct {
		Resolution graph.FieldResolution `json:"resolution"`
	}
	if err := decodeBody(w, r, 32<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	plan, err := planner.PreviewFieldResolution(r.Context(), body.Resolution)
	if err != nil {
		writeError(w, http.StatusConflict, "preview_rejected", err.Error())
		return
	}
	plan.PlanHash, err = guardedPlanMAC(h.WriteToken, "field-resolution", plan.ExpectedRevision, body.Resolution)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_failed", "could not create preview plan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan": plan, "resolution": body.Resolution})
}

func (h *Handler) applyAssertionResolution(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	writer, ok := h.Repository.(GuardedFieldResolutionWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "resolution_unavailable", "field resolution apply requires PostgreSQL")
		return
	}
	var body struct {
		Resolution       graph.FieldResolution `json:"resolution"`
		ExpectedRevision string                `json:"expectedRevision"`
		PlanHash         string                `json:"planHash"`
	}
	if err := decodeBody(w, r, 32<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	expected, err := guardedPlanMAC(h.WriteToken, "field-resolution", body.ExpectedRevision, body.Resolution)
	if err != nil || !hmac.Equal([]byte(expected), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "resolution_rejected", "plan does not match the previewed decision and revision")
		return
	}
	created, err := writer.ApplyFieldResolution(r.Context(), body.Resolution, body.ExpectedRevision)
	if err != nil {
		writeError(w, http.StatusConflict, "resolution_rejected", err.Error())
		return
	}
	h.invalidateCache()
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) rejectUnpreviewedWrite(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	writeError(w, http.StatusPreconditionRequired, "preview_required", "this write route is retired; use the server-side preview and guarded apply workflow")
}

func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	if h.Repository == nil {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "persistent graph repository is not configured; set DATABASE_URL to a PostgreSQL connection string to enable persistent writes")
		return
	}
	var raw map[string]json.RawMessage
	if err := decodeBody(w, r, 8<<20, &raw); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if raw["batch"] == nil || raw["expectedRevision"] == nil || raw["planHash"] == nil {
		writeError(w, http.StatusPreconditionRequired, "preview_required", "apply requires the exact batch, expectedRevision, and planHash from a server preview")
		return
	}
	if len(raw) != 3 {
		writeError(w, http.StatusBadRequest, "invalid_request", "unexpected apply fields")
		return
	}
	var body struct {
		Batch            graph.SourceBatch
		ExpectedRevision string
		PlanHash         string
	}
	batchDecoder := json.NewDecoder(strings.NewReader(string(raw["batch"])))
	batchDecoder.DisallowUnknownFields()
	if err := batchDecoder.Decode(&body.Batch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("invalid sync batch: %v", err))
		return
	}
	if err := json.Unmarshal(raw["expectedRevision"], &body.ExpectedRevision); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "expectedRevision must be a string")
		return
	}
	if err := json.Unmarshal(raw["planHash"], &body.PlanHash); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "planHash must be a string")
		return
	}
	applier, ok := h.Repository.(SyncPlanApplier)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "write_unavailable", "guarded sync apply requires PostgreSQL")
		return
	}
	expected, err := guardedPlanMAC(h.WriteToken, "sync-batch", body.ExpectedRevision, body.Batch)
	if err != nil || !hmac.Equal([]byte(expected), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "sync_rejected", "plan does not match the previewed batch and revision")
		return
	}
	preview, err := applier.ApplySyncPlan(r.Context(), body.Batch, body.ExpectedRevision)
	if err != nil {
		writeError(w, http.StatusConflict, "sync_rejected", err.Error())
		return
	}
	h.invalidateCache()
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "preview": preview})
}

func mutationPlanMAC(token, revision string, mutation graph.ManualMutation) (string, error) {
	raw, err := json.Marshal(struct {
		Kind     string               `json:"kind"`
		Revision string               `json:"revision"`
		Mutation graph.ManualMutation `json:"mutation"`
	}{"manual-mutation", revision, mutation})
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
		plan.PlanHash, err = mutationPlanMAC(h.WriteToken, plan.ExpectedRevision, body.Mutation)
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
	expectedHash, hashErr := mutationPlanMAC(h.WriteToken, body.ExpectedRevision, body.Mutation)
	if hashErr != nil || !hmac.Equal([]byte(expectedHash), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "mutation_rejected", "plan hash does not match the previewed payload")
		return
	}
	domainHash, err := body.Mutation.PlanHashValue()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_mutation", err.Error())
		return
	}
	plan, err := applier.ApplyManualMutation(r.Context(), body.Mutation, body.ExpectedRevision, domainHash)
	if err != nil {
		writeError(w, http.StatusConflict, "mutation_rejected", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "plan": plan})
}

func operationIDFromPath(path, suffix string) (int64, error) {
	raw := strings.TrimSuffix(strings.TrimPrefix(path, "/api/operations/"), suffix)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("operation ID must be a positive integer")
	}
	return id, nil
}
func (h *Handler) previewUndo(w http.ResponseWriter, r *http.Request, path string) {
	planner, ok := h.Repository.(UndoPlanWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "undo_unavailable", "undo preview requires PostgreSQL")
		return
	}
	id, err := operationIDFromPath(path, "/undo/preview")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_operation", err.Error())
		return
	}
	plan, err := planner.PreviewUndoOperation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusConflict, "undo_rejected", err.Error())
		return
	}
	plan.PlanHash, err = guardedPlanMAC(h.WriteToken, "undo-operation", plan.ExpectedRevision, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_failed", "could not create undo plan")
		return
	}
	writeJSON(w, http.StatusOK, plan)
}
func (h *Handler) applyUndo(w http.ResponseWriter, r *http.Request, path string) {
	if !h.authorized(w, r) {
		return
	}
	planner, ok := h.Repository.(UndoPlanWriter)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "undo_unavailable", "undo apply requires PostgreSQL")
		return
	}
	id, err := operationIDFromPath(path, "/undo/apply")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_operation", err.Error())
		return
	}
	var body struct {
		ExpectedRevision string `json:"expectedRevision"`
		PlanHash         string `json:"planHash"`
	}
	if err = decodeBody(w, r, 16<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	expected, err := guardedPlanMAC(h.WriteToken, "undo-operation", body.ExpectedRevision, id)
	if err != nil || !hmac.Equal([]byte(expected), []byte(body.PlanHash)) {
		writeError(w, http.StatusConflict, "undo_rejected", "plan does not match the previewed operation and revision")
		return
	}
	if err = planner.UndoOperationPlanned(r.Context(), id, body.ExpectedRevision); err != nil {
		writeError(w, http.StatusConflict, "undo_rejected", err.Error())
		return
	}
	h.invalidateCache()
	writeJSON(w, http.StatusOK, map[string]int64{"undone": id})
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
		h.invalidateCache()
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
	h.invalidateCache()
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

const MaxPageLimit = 500

func paginateSlice[T any](items []T, getID func(T) string, values url.Values) ([]T, int, bool, string, error) {
	total := len(items)
	limitStr := values.Get("limit")
	offsetStr := values.Get("offset")
	cursor := values.Get("cursor")

	limit := total
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil || parsedLimit <= 0 {
			return nil, 0, false, "", errors.New("limit must be a positive integer")
		}
		if parsedLimit > MaxPageLimit {
			return nil, 0, false, "", fmt.Errorf("limit cannot exceed %d", MaxPageLimit)
		}
		limit = parsedLimit
	}

	startIdx := 0
	if cursor != "" {
		found := false
		for i, it := range items {
			if getID(it) > cursor {
				startIdx = i
				found = true
				break
			}
		}
		if !found {
			startIdx = total
		}
	} else if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil || parsedOffset < 0 {
			return nil, 0, false, "", errors.New("offset must be a non-negative integer")
		}
		startIdx = parsedOffset
		if startIdx > total {
			startIdx = total
		}
	}

	endIdx := startIdx + limit
	if endIdx > total {
		endIdx = total
	}

	page := items[startIdx:endIdx]
	hasMore := endIdx < total
	nextCursor := ""
	if hasMore && len(page) > 0 {
		nextCursor = getID(page[len(page)-1])
	}
	return page, total, hasMore, nextCursor, nil
}

func (h Handler) credentialSecretAPI(w http.ResponseWriter, r *http.Request) {
	if h.SecretProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "secret_provider_unavailable", "秘密提供者未配置或未启用")
		return
	}
	// POST stores a password; GET reads it back. Writing always requires the
	// write token so an operator action cannot be triggered by a bare link.
	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		h.credentialStoreAPI(w, r)
		return
	}
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "ref 参数不能为空")
		return
	}
	res, err := h.SecretProvider.Read(r.Context(), ref)
	if err != nil {
		writeError(w, http.StatusNotFound, "secret_not_found", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"ref":      ref,
		"password": string(res.Value),
		"opaqueId": res.Metadata.OpaqueID,
	})
}

func (h Handler) credentialSyncAPI(w http.ResponseWriter, r *http.Request) {
	if h.SecretProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "secret_provider_unavailable", "秘密提供者未配置或未启用")
		return
	}
	type syncer interface {
		Sync(context.Context) error
	}
	s, ok := h.SecretProvider.(syncer)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported_operation", "当前秘密提供者不支持手动触发同步")
		return
	}
	if err := s.Sync(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, "sync_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"msg":    "密码库同步成功",
	})
}

// credentialStoreAPI creates or rotates a stored password. It is only reachable
// with the write token, because a stored password is a higher-value target than
// graph content.
func (h Handler) credentialStoreAPI(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(w, r) {
		return
	}
	var body struct {
		Ref      string `json:"ref"`
		Password string `json:"password"`
	}
	if err := decodeBody(w, r, 32<<10, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	ref := strings.TrimSpace(body.Ref)
	if ref == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "ref is required")
		return
	}
	if strings.TrimSpace(body.Password) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "password is required")
		return
	}
	req := cmdb.SecretRequest{Name: ref, Value: []byte(body.Password)}
	// Existing refs are rotated in place so the UUID and references stay valid.
	if _, err := h.SecretProvider.Read(r.Context(), ref); err == nil {
		if _, err := h.SecretProvider.Update(r.Context(), ref, req); err != nil {
			writeError(w, http.StatusInternalServerError, "store_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "ref": ref, "action": "updated"})
		return
	}
	meta, err := h.SecretProvider.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "ref": ref, "action": "created", "opaqueId": meta.OpaqueID})
}

type uploadCredentialsRequest struct {
	Refs     []string `json:"refs"`
	Ref      string   `json:"ref"`
	Upstream string   `json:"upstream"`
}

func (h Handler) credentialUploadAPI(w http.ResponseWriter, r *http.Request) {
	if h.SecretProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "secret_provider_unavailable", "秘密提供者未配置或未启用")
		return
	}
	type migrator interface {
		MigrateToUpstream(ctx context.Context, refs []string, targetUpstream string) (cmdb.BatchMigrateResult, error)
	}
	m, ok := h.SecretProvider.(migrator)
	if !ok {
		writeError(w, http.StatusBadRequest, "unsupported_operation", "当前秘密提供者不支持上传上游")
		return
	}

	var req uploadCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "无效的请求 JSON 体")
		return
	}
	targets := req.Refs
	if len(targets) == 0 && req.Ref != "" {
		targets = []string{req.Ref}
	}
	if len(targets) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "请提供至少一个待上传凭据的 ref")
		return
	}

	upstreamTarget := req.Upstream
	if upstreamTarget == "" {
		upstreamTarget = "bitwarden"
	}

	report, err := m.MigrateToUpstream(r.Context(), targets, upstreamTarget)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "migrate_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"report":  report,
		"message": fmt.Sprintf("批量上传完成: 成功 %d, 失败 %d", report.Succeeded, report.Failed),
	})
}

func (h Handler) credentialStatusAPI(w http.ResponseWriter, r *http.Request) {
	if h.SecretProvider == nil {
		writeError(w, http.StatusServiceUnavailable, "secret_provider_unavailable", "秘密提供者未配置")
		return
	}
	type statusReporter interface {
		GetStatusMap(ctx context.Context) (map[string]string, error)
	}
	s, ok := h.SecretProvider.(statusReporter)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "managed": map[string]string{}})
		return
	}
	m, err := s.GetStatusMap(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "status_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"managed": m,
	})
}
