package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type operationRepo struct {
	memoryRepository
	undone int64
}

func (r *operationRepo) MutationOperations(context.Context, int) ([]graph.MutationOperation, error) {
	return []graph.MutationOperation{{ID: 7, Action: "manual:edit", RecordedAt: time.Now(), Reversible: true}}, nil
}
func (r *operationRepo) PreviewUndoOperation(_ context.Context, id int64) (graph.MutationPlan, error) {
	return graph.MutationPlan{ExpectedRevision: "undo-revision"}, nil
}
func (r *operationRepo) UndoOperationPlanned(_ context.Context, id int64, revision string) error {
	if revision != "undo-revision" {
		return context.Canceled
	}
	r.undone = id
	return nil
}

func TestOperationUndoRequiresPreviewAndAuthentication(t *testing.T) {
	token := strings.Repeat("o", 32)
	repo := &operationRepo{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	req := httptest.NewRequest(http.MethodPost, "/api/operations/7/undo", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("legacy undo status %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/operations", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"reversible":true`) {
		t.Fatalf("operation list failed: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/operations/7/undo/preview", strings.NewReader(`{}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("undo preview failed: %d %s", w.Code, w.Body.String())
	}
	hash, _ := guardedPlanMAC(token, "undo-operation", "undo-revision", int64(7))
	body := `{"expectedRevision":"undo-revision","planHash":"` + hash + `"}`
	req = httptest.NewRequest(http.MethodPost, "/api/operations/7/undo/apply", strings.NewReader(body))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated undo returned %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/operations/7/undo/apply", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || repo.undone != 7 {
		t.Fatalf("undo not dispatched: %d %s", w.Code, w.Body.String())
	}
}
