package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type resolutionRepository struct {
	memoryRepository
	stored graph.FieldResolution
}

func (r *resolutionRepository) PreviewFieldResolution(_ context.Context, v graph.FieldResolution) (graph.MutationPlan, error) {
	if err := v.Validate(); err != nil {
		return graph.MutationPlan{}, err
	}
	return graph.MutationPlan{ExpectedRevision: "assertions-revision"}, nil
}
func (r *resolutionRepository) ApplyFieldResolution(_ context.Context, v graph.FieldResolution, revision string) (graph.FieldResolution, error) {
	if revision != "assertions-revision" {
		return v, context.Canceled
	}
	r.stored = v
	r.stored.ID = 9
	return r.stored, nil
}
func (r *resolutionRepository) FieldResolutions(context.Context, string) ([]graph.FieldResolution, error) {
	return []graph.FieldResolution{r.stored}, nil
}

func TestFieldResolutionRequiresPreviewAndExactPlan(t *testing.T) {
	repo := &resolutionRepository{memoryRepository: memoryRepository{g: fixture()}}
	token := strings.Repeat("t", 32)
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	decision := graph.FieldResolution{NodeID: "m", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"a1"}, RationaleReference: "ticket:12"}
	legacy, _ := json.Marshal(decision)
	req := httptest.NewRequest(http.MethodPost, "/api/assertions/resolve", strings.NewReader(string(legacy)))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("legacy route status %d", w.Code)
	}
	previewBody, _ := json.Marshal(map[string]any{"resolution": decision})
	req = httptest.NewRequest(http.MethodPost, "/api/assertions/resolve/preview", strings.NewReader(string(previewBody)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "valueHash") {
		t.Fatalf("unsafe or failed preview: %d %s", w.Code, w.Body.String())
	}
	hash, _ := guardedPlanMAC(token, "field-resolution", "assertions-revision", decision)
	bad := decision
	bad.RationaleReference = "ticket:tampered"
	badBody, _ := json.Marshal(map[string]any{"resolution": bad, "expectedRevision": "assertions-revision", "planHash": hash})
	req = httptest.NewRequest(http.MethodPost, "/api/assertions/resolve/apply", strings.NewReader(string(badBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict || repo.stored.ID != 0 {
		t.Fatalf("tampered plan accepted: %d %s", w.Code, w.Body.String())
	}
	applyBody, _ := json.Marshal(map[string]any{"resolution": decision, "expectedRevision": "assertions-revision", "planHash": hash})
	req = httptest.NewRequest(http.MethodPost, "/api/assertions/resolve/apply", strings.NewReader(string(applyBody)))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized apply status %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/assertions/resolve/apply", strings.NewReader(string(applyBody)))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"id":9`) {
		t.Fatalf("resolution apply failed: %d %s", w.Code, w.Body.String())
	}
}
