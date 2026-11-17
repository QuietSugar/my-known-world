package api

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mutationRepo struct {
	memoryRepository
	applied bool
}

func (r *mutationRepo) PreviewManualMutation(_ context.Context, m graph.ManualMutation) (graph.MutationPlan, error) {
	return graph.MutationPlan{ExpectedRevision: "revision", Delta: graph.MutationDelta{UpdatedNodes: []string{"m"}}}, nil
}
func (r *mutationRepo) ApplyManualMutation(_ context.Context, _ graph.ManualMutation, revision, hash string) (graph.MutationPlan, error) {
	r.applied = revision == "revision" && hash != ""
	return graph.MutationPlan{ExpectedRevision: revision, PlanHash: hash}, nil
}
func TestMutationPreviewIsReadOnlyAndApplyRequiresAuth(t *testing.T) {
	token := strings.Repeat("m", 32)
	repo := &mutationRepo{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	body := `{"mutation":{"kind":"updateNode","id":"machine","node":{"id":"machine","type":"Machine","name":"Changed","modelVersion":1}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/operations/preview", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"updatedNodes":["m"]`) {
		t.Fatalf("preview failed: %d %s", w.Code, w.Body.String())
	}
	if repo.applied {
		t.Fatal("preview mutated graph")
	}
	req = httptest.NewRequest(http.MethodPost, "/api/operations/apply", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated apply status %d", w.Code)
	}
	mutation := graph.ManualMutation{Kind: "updateNode", ID: "machine", Node: &graph.Node{ID: "machine", Type: graph.Machine, Name: "Changed", ModelVersion: 1}}
	hash, err := mutationPlanMAC(token, "revision", mutation)
	if err != nil {
		t.Fatal(err)
	}
	applyBody := `{"mutation":{"kind":"updateNode","id":"machine","node":{"id":"machine","type":"Machine","name":"Changed","modelVersion":1}},"expectedRevision":"revision","planHash":"` + hash + `"}`
	req = httptest.NewRequest(http.MethodPost, "/api/operations/apply", strings.NewReader(applyBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !repo.applied {
		t.Fatalf("apply was not dispatched: %d %s", w.Code, w.Body.String())
	}
}
