package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type memoryRepository struct{ g *graph.Graph }

func (m *memoryRepository) PreviewSync(_ context.Context, b graph.SourceBatch) (graph.SyncPreview, error) {
	return m.g.PreviewSync(b), nil
}
func (m *memoryRepository) LoadSnapshot(context.Context) (*graph.Graph, error) { return m.g, nil }
func (m *memoryRepository) EntityHistory(_ context.Context, kind, id string, _ int) ([]graph.ChangeEvent, error) {
	return []graph.ChangeEvent{{ID: 1, EntityType: kind, EntityID: id, Source: "fixture", Action: "created"}}, nil
}
func (m *memoryRepository) ApplySync(_ context.Context, b graph.SourceBatch) (*graph.SyncPreview, error) {
	next, p, err := m.g.ApplySync(b)
	if err == nil {
		m.g = next
	}
	return &p, err
}
func (m *memoryRepository) ReviewProvisional(_ context.Context, id string, target graph.NodeType) error {
	next, err := m.g.ReviewProvisional(id, target)
	if err == nil {
		m.g = next
	}
	return err
}
func (m *memoryRepository) RejectProvisional(_ context.Context, id string) error {
	next, err := m.g.RejectProvisional(id)
	if err == nil {
		m.g = next
	}
	return err
}
func (m *memoryRepository) DeleteNode(_ context.Context, id string) error {
	next, err := m.g.DeleteNode(id)
	if err == nil {
		m.g = next
	}
	return err
}

func TestWritesRequireConfiguredBearerTokenAndApplyBatch(t *testing.T) {
	g := fixture()
	repo := &memoryRepository{g: g}
	token := strings.Repeat("x", 32)
	h := &Handler{Graph: g, Repository: repo, SyncPreviewer: repo, WriteToken: token}
	body := `{"source":"test-import","nodes":[{"id":"batch-node","type":"Machine","name":"Batch node","modelVersion":1}],"relations":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/apply", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer token status=%d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/sync/apply", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("batch apply status=%d body=%s", w.Code, w.Body.String())
	}
	if _, ok := repo.g.Node("batch-node"); !ok {
		t.Fatal("authorized batch wasn't applied")
	}
}

func TestAuthenticatedProvisionalReviewEndpoint(t *testing.T) {
	g := fixture()
	if err := g.Publish(graph.Node{ID: "pending", Type: graph.ProvisionalObject, Name: "Pending", ModelVersion: 1, CandidateType: graph.Machine}, graph.Relation{ID: "pending-link", To: "pending"}); err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{g: g}
	token := strings.Repeat("y", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}
	req := httptest.NewRequest(http.MethodPost, "/api/provisionals/pending/review", strings.NewReader(`{"targetType":"Machine"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("review failed: %d %s", w.Code, w.Body.String())
	}
	n, _ := repo.g.Node("pending")
	if n.Type != graph.Machine {
		t.Fatalf("node wasn't classified: %+v", n)
	}
}
