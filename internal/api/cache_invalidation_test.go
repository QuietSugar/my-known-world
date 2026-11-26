package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// manualMutationRepo applies a patch straight to its graph, enough to exercise
// the handler's cache invalidation without a database.
type manualMutationRepo struct {
	memoryRepository
}

func (r *manualMutationRepo) ValidateManualMutation(context.Context, graph.ManualMutation, string) (graph.MutationPlan, error) {
	return graph.MutationPlan{}, nil
}

func (r *manualMutationRepo) ApplyManualMutation(_ context.Context, m graph.ManualMutation, _ string, _ string) (graph.MutationPlan, error) {
	if m.Node != nil && m.ID != "" {
		next, err := r.g.UpdateNode(m.ID, *m.Node)
		if err != nil {
			return graph.MutationPlan{}, err
		}
		r.g = next
	}
	return graph.MutationPlan{}, nil
}

// TestApplyManualMutationInvalidatesCache guards the write path's cache
// invalidation. Without it a successful patch leaves every subsequent read
// serving the pre-write snapshot until the TTL expires — on 2026-11-26 that
// made a working fix look broken and sent the diagnosis to the wrong place.
func TestApplyManualMutationInvalidatesCache(t *testing.T) {
	token := strings.Repeat("c", 32)
	repo := &manualMutationRepo{memoryRepository: memoryRepository{g: fixture()}}
	cache := NewSnapshotCache(time.Hour) // 有效期内只有显式失效才更新
	h := &Handler{Graph: repo.g, Repository: repo, WriteToken: token, Cache: cache}

	// 先读一次，把旧快照放进缓存
	first := httptest.NewRequest(http.MethodGet, "/api/nodes?limit=1", nil)
	firstRec := httptest.NewRecorder()
	h.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("initial read failed: %d", firstRec.Code)
	}

	// 写入：把 test 机器改名
	mut := graph.ManualMutation{Kind: "updateNode", ID: "machine",
		Node: &graph.Node{ID: "machine", Type: graph.Machine, Name: "Renamed machine", ModelVersion: 1}}
	revision := "rev-1"
	hash, err := mutationPlanMAC(token, revision, mut)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"mutation": mut, "expectedRevision": revision, "planHash": hash})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/operations/apply", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply failed: %d %s", rec.Code, rec.Body.String())
	}

	// 写入后读到的必须来自新快照
	if got := h.currentGraphName(t); got != "Renamed machine" {
		t.Fatalf("cache served the stale snapshot after a successful write: got %q", got)
	}
}

// currentGraphName 通过一次读请求取回 machine 的名字。
func (h *Handler) currentGraphName(t *testing.T) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/nodes/machine", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode read: %v", err)
	}
	return payload.Name
}
