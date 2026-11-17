package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type sourcePolicyRepository struct {
	memoryRepository
	items []graph.SourcePolicy
}

func (r *sourcePolicyRepository) SourcePolicies(context.Context) ([]graph.SourcePolicy, error) {
	return r.items, nil
}
func (r *sourcePolicyRepository) SetSourcePolicy(_ context.Context, p graph.SourcePolicy) error {
	r.items = []graph.SourcePolicy{p}
	return nil
}
func (r *sourcePolicyRepository) PreviewSourcePolicy(_ context.Context, p graph.SourcePolicy) (graph.MutationPlan, *graph.SourcePolicy, error) {
	var old *graph.SourcePolicy
	if len(r.items) > 0 {
		v := r.items[0]
		old = &v
	}
	return graph.MutationPlan{ExpectedRevision: "policy-revision"}, old, p.Validate()
}
func (r *sourcePolicyRepository) ApplySourcePolicy(_ context.Context, p graph.SourcePolicy, revision string) error {
	if revision != "policy-revision" {
		return errors.New("stale policy revision")
	}
	r.items = []graph.SourcePolicy{p}
	return nil
}

func TestSourcePolicyRequiresPreviewAndRevisionBoundApply(t *testing.T) {
	token := strings.Repeat("p", 32)
	repo := &sourcePolicyRepository{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	policy := graph.SourcePolicy{Source: "inventory", Priority: 25, AllowedNodeTypes: []graph.NodeType{graph.Machine}, AllowedPredicates: []graph.Predicate{graph.KnowsAbout}, AllowedFieldPaths: []string{"machine.hostname"}}
	legacy, _ := json.Marshal(policy)
	req := httptest.NewRequest(http.MethodPost, "/api/sync/policies", strings.NewReader(string(legacy)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("legacy direct route status %d: %s", w.Code, w.Body.String())
	}
	previewBody, _ := json.Marshal(map[string]any{"policy": policy})
	req = httptest.NewRequest(http.MethodPost, "/api/sync/policies/preview", strings.NewReader(string(previewBody)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"expectedRevision":"policy-revision"`) {
		t.Fatalf("preview failed: %d %s", w.Code, w.Body.String())
	}
	planHash, _ := guardedPlanMAC(token, "source-policy", "policy-revision", policy)
	applyBody, _ := json.Marshal(map[string]any{"policy": policy, "expectedRevision": "policy-revision", "planHash": planHash})
	req = httptest.NewRequest(http.MethodPost, "/api/sync/policies/apply", strings.NewReader(string(applyBody)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized apply status %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/sync/policies/apply", strings.NewReader(string(applyBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || len(repo.items) != 1 {
		t.Fatalf("apply failed: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/sync/policies", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"allowedNodeTypes":["Machine"]`) {
		t.Fatalf("policy list failed: %d %s", w.Code, w.Body.String())
	}
}
