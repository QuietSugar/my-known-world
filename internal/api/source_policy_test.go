package api

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
func TestSourcePolicyAPIRequiresAuthAndListsSavedAllowList(t *testing.T) {
	token := strings.Repeat("p", 32)
	repo := &sourcePolicyRepository{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	body := `{"source":"inventory","priority":25,"allowedNodeTypes":["Machine"],"allowedPredicates":["knowsAbout"],"allowedFieldPaths":["machine.hostname"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/policies", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized policy write status: %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/sync/policies", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("policy write failed: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/sync/policies", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"allowedNodeTypes":["Machine"]`) {
		t.Fatalf("policy list failed: %d %s", w.Code, w.Body.String())
	}
}
