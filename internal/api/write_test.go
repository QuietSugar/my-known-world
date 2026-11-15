package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type memoryRepository struct{ g *graph.Graph }

func (m *memoryRepository) PreviewSync(_ context.Context, b graph.SourceBatch) (graph.SyncPreview, error) {
	preview := m.g.PreviewSync(b)
	preview.ExpectedRevision = "memory-revision"
	return preview, nil
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
func (m *memoryRepository) ApplySyncPlan(ctx context.Context, b graph.SourceBatch, revision string) (*graph.SyncPreview, error) {
	if revision != "memory-revision" {
		return nil, errors.New("stale revision")
	}
	return m.ApplySync(ctx, b)
}
func (m *memoryRepository) ReviewProvisional(_ context.Context, id string, target graph.NodeType) error {
	next, err := m.g.ReviewProvisional(id, target)
	if err == nil {
		m.g = next
	}
	return err
}
func (m *memoryRepository) ReviewProvisionalWithEvidence(_ context.Context, id string, target graph.NodeType, evidence graph.ProvisionalReviewEvidence) error {
	next, err := m.g.ReviewProvisionalWithEvidence(id, target, evidence)
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

func TestLegacyReviewAndDeleteRoutesRequirePreview(t *testing.T) {
	g := fixture()
	if err := g.Publish(graph.Node{ID: "pending/with/slash", Type: graph.ProvisionalObject, Name: "Pending", ModelVersion: 1, CandidateType: graph.Machine}, graph.Relation{ID: "pending-slash-link", To: "pending/with/slash"}); err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{g: g}
	token := strings.Repeat("z", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}
	for _, tc := range []struct{ method, path, body string }{{http.MethodPost, "/api/provisionals/pending/with/slash/review", `{"targetType":"Machine"}`}, {http.MethodPost, "/api/provisionals/pending/with/slash/reject", `{}`}, {http.MethodDelete, "/api/nodes/pending/with/slash", ""}} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusPreconditionRequired {
			t.Errorf("%s %s: status %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	n, ok := repo.g.Node("pending/with/slash")
	if !ok || n.Type != graph.ProvisionalObject {
		t.Fatalf("legacy endpoint mutated provisional: %+v", n)
	}
}

func TestWritesRequireConfiguredBearerTokenAndApplyBatch(t *testing.T) {
	g := fixture()
	repo := &memoryRepository{g: g}
	token := strings.Repeat("x", 32)
	h := &Handler{Graph: g, Repository: repo, SyncPreviewer: repo, WriteToken: token}
	batch := graph.SourceBatch{Source: "test-import", Nodes: []graph.Node{{ID: "batch-node", Type: graph.Machine, Name: "Batch node", ModelVersion: 1}}, Relations: []graph.Relation{}}
	batchJSON, _ := json.Marshal(batch)
	body := string(batchJSON)
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
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("legacy sync body must require preview plan: %d %s", w.Code, w.Body.String())
	}
	planHash, _ := guardedPlanMAC(token, "sync-batch", "memory-revision", batch)
	applyBody, _ := json.Marshal(map[string]any{"batch": batch, "expectedRevision": "memory-revision", "planHash": planHash})
	tamperedBatch := batch
	tamperedBatch.Nodes = append([]graph.Node(nil), batch.Nodes...)
	tamperedBatch.Nodes[0].Name = "Different content"
	tamperedBody, _ := json.Marshal(map[string]any{"batch": tamperedBatch, "expectedRevision": "memory-revision", "planHash": planHash})
	req = httptest.NewRequest(http.MethodPost, "/api/sync/apply", bytes.NewReader(tamperedBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict || len(repo.g.Nodes()) != 2 {
		t.Fatalf("tampered sync plan accepted or mutated: %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/sync/apply", bytes.NewReader(applyBody))
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

func TestAuthenticatedProvisionalReviewEndpointRequiresPreview(t *testing.T) {
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
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("review route should require preview, got %d %s", w.Code, w.Body.String())
	}
	n, _ := repo.g.Node("pending")
	if n.Type != graph.ProvisionalObject {
		t.Fatalf("legacy route changed type: %+v", n)
	}
}
