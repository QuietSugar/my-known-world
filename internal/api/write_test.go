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
func (m *memoryRepository) FieldResolutions(_ context.Context, _ string) ([]graph.FieldResolution, error) {
	return []graph.FieldResolution{}, nil
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
func (m *memoryRepository) PreviewManualMutation(_ context.Context, mutation graph.ManualMutation) (graph.MutationPlan, error) {
	hash, err := mutation.PlanHashValue()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	updated, err := m.g.ApplyManualMutation(mutation)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	return graph.MutationPlan{ExpectedRevision: "memory-revision", PlanHash: hash, Delta: graph.DiffGraphs(m.g, updated)}, nil
}
func (m *memoryRepository) ApplyManualMutation(_ context.Context, mutation graph.ManualMutation, revision, expectedHash string) (graph.MutationPlan, error) {
	if revision != "memory-revision" {
		return graph.MutationPlan{}, errors.New("stale revision")
	}
	hash, err := mutation.PlanHashValue()
	if err != nil {
		return graph.MutationPlan{}, err
	}
	if hash != expectedHash {
		return graph.MutationPlan{}, errors.New("mutation hash mismatch")
	}
	updated, err := m.g.ApplyManualMutation(mutation)
	if err != nil {
		return graph.MutationPlan{}, err
	}
	plan := graph.MutationPlan{ExpectedRevision: revision, PlanHash: hash, Delta: graph.DiffGraphs(m.g, updated)}
	m.g = updated
	return plan, nil
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

func TestManualMutationPreviewAndApplyHMAC(t *testing.T) {
	token := strings.Repeat("p", 32)
	cases := []struct {
		name     string
		graph    func() *graph.Graph
		mutation graph.ManualMutation
	}{
		{"createNode", func() *graph.Graph {
			g := graph.New()
			_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1})
			return g
		}, graph.ManualMutation{Kind: "createNode", Node: &graph.Node{ID: "doc", Type: graph.Document, Name: "Doc", ModelVersion: 1}, Relation: &graph.Relation{ID: "self-doc", From: "self", To: "doc", Predicate: graph.KnowsAbout}}},
		{"reviewProvisional", func() *graph.Graph {
			g := graph.New()
			_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1})
			_ = g.Publish(graph.Node{ID: "candidate", Type: graph.ProvisionalObject, Name: "Candidate", ModelVersion: 1, CandidateType: graph.Machine}, graph.Relation{ID: "self-candidate", From: "self", To: "candidate", Predicate: graph.KnowsAbout})
			return g
		}, graph.ManualMutation{Kind: "reviewProvisional", ID: "candidate", TargetType: graph.Machine}},
		{"updateRelation", func() *graph.Graph {
			g := graph.New()
			_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1})
			_ = g.Publish(graph.Node{ID: "host", Type: graph.Machine, Name: "Host", ModelVersion: 1}, graph.Relation{ID: "rel", From: "self", To: "host", Predicate: graph.KnowsAbout})
			return g
		}, graph.ManualMutation{Kind: "updateRelation", ID: "rel", Relation: &graph.Relation{ID: "rel", From: "self", To: "host", Predicate: graph.KnowsAbout, Certainty: graph.CertaintyConfirmed}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepository{g: tc.graph()}
			h := &Handler{Graph: repo.g, Repository: repo, WriteToken: token}
			previewBody, _ := json.Marshal(map[string]any{"mutation": tc.mutation})
			req := httptest.NewRequest(http.MethodPost, "/api/operations/preview", bytes.NewReader(previewBody))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
			}
			var plan graph.MutationPlan
			if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			apply := func(mutation graph.ManualMutation, revision, mac string) int {
				body, _ := json.Marshal(map[string]any{"mutation": mutation, "expectedRevision": revision, "planHash": mac})
				r := httptest.NewRequest(http.MethodPost, "/api/operations/apply", bytes.NewReader(body))
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w.Code
			}
			before, _ := repo.g.Fingerprint()
			tampered := tc.mutation
			tampered.ID = "tampered"
			if status := apply(tampered, plan.ExpectedRevision, plan.PlanHash); status != http.StatusConflict {
				t.Fatalf("MAC accepted a changed payload: %d", status)
			}
			if status := apply(tc.mutation, "different-revision", plan.PlanHash); status != http.StatusConflict {
				t.Fatalf("MAC accepted a different revision: %d", status)
			}
			after, _ := repo.g.Fingerprint()
			if before != after {
				t.Fatal("rejected manual plan partially mutated graph")
			}
			if status := apply(tc.mutation, plan.ExpectedRevision, plan.PlanHash); status != http.StatusOK {
				t.Fatalf("apply: %d", status)
			}
		})
	}
}

func TestSlashNodeIDWorksAcrossReadRoutes(t *testing.T) {
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.Publish(graph.Node{ID: "notes/README.md", Type: graph.Document, Name: "Notes", ModelVersion: 1}, graph.Relation{ID: "self-notes", From: "self", To: "notes/README.md", Predicate: graph.KnowsAbout}); err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{g: g}
	h := &Handler{Graph: g, Repository: repo}
	for _, route := range []string{"/api/nodes/notes%2FREADME.md", "/api/nodes/notes%2FREADME.md/assertions", "/api/nodes/notes%2FREADME.md/resolutions", "/api/path/notes%2FREADME.md", "/api/explore/notes%2FREADME.md?depth=1", "/api/entities/node/notes%2FREADME.md/history"} {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		response := httptest.NewRecorder()
		h.ServeHTTP(response, req)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s: status=%d body=%s", route, response.Code, response.Body.String())
		}
	}
}

func TestIdentityMappingSyncPreviewPlanAppliesExactSubmittedBatch(t *testing.T) {
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.Publish(graph.Node{ID: "stable-id", Type: graph.Machine, Name: "Before", Source: "inventory", ModelVersion: 1}, graph.Relation{ID: "knows-stable", From: "self", To: "stable-id", Predicate: graph.KnowsAbout, Source: "inventory"}); err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{g: g}
	token := strings.Repeat("m", 32)
	h := &Handler{Graph: g, Repository: repo, SyncPreviewer: repo, WriteToken: token}
	batch := graph.SourceBatch{Source: "inventory", IdentityMappings: []graph.IdentityMapping{{SourceID: "renamed-id", NodeID: "stable-id"}}, Nodes: []graph.Node{{ID: "renamed-id", Type: graph.Machine, Name: "After", ModelVersion: 1}}, Relations: []graph.Relation{{ID: "knows-stable", From: "self", To: "renamed-id", Predicate: graph.KnowsAbout, Source: "inventory"}}}
	batchJSON, err := json.Marshal(batch)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/sync/preview", bytes.NewReader(batchJSON))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", response.Code, response.Body.String())
	}
	var plan struct {
		ExpectedRevision string `json:"expectedRevision"`
		PlanHash         string `json:"planHash"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.ExpectedRevision == "" || plan.PlanHash == "" {
		t.Fatalf("guarded preview omitted plan fields: %+v", plan)
	}
	applyJSON, err := json.Marshal(map[string]any{"batch": batch, "expectedRevision": plan.ExpectedRevision, "planHash": plan.PlanHash})
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/sync/apply", bytes.NewReader(applyJSON))
	request.Header.Set("Authorization", "Bearer "+token)
	response = httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("exact rename batch rejected: status=%d body=%s", response.Code, response.Body.String())
	}
	node, ok := repo.g.Node("stable-id")
	if !ok || node.Name != "After" {
		t.Fatalf("stable identity was not updated: %+v", node)
	}
	if _, ok = repo.g.Node("renamed-id"); ok {
		t.Fatal("rename created a second node")
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
