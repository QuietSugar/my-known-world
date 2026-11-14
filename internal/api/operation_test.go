package api

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type operationRepo struct {
	memoryRepository
	undone int64
}

func (r *operationRepo) MutationOperations(context.Context, int) ([]graph.MutationOperation, error) {
	return []graph.MutationOperation{{ID: 7, Action: "manual:edit", RecordedAt: time.Now(), Reversible: true}}, nil
}
func (r *operationRepo) UndoOperation(_ context.Context, id int64) error { r.undone = id; return nil }
func TestOperationUndoAPIRequiresAuthentication(t *testing.T) {
	token := strings.Repeat("o", 32)
	repo := &operationRepo{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	req := httptest.NewRequest(http.MethodPost, "/api/operations/7/undo", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated undo returned %d", w.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/operations", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"reversible":true`) {
		t.Fatalf("operation list failed: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/operations/7/undo", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || repo.undone != 7 {
		t.Fatalf("undo not dispatched: %d %s", w.Code, w.Body.String())
	}
}
