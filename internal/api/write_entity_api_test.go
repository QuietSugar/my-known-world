package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type entityWriterMock struct {
	*memoryRepository
	revision string
}

func (m *entityWriterMock) PatchNode(_ context.Context, id string, patch graph.Node, expectedRevision string) (graph.Node, string, error) {
	if expectedRevision != m.revision {
		return graph.Node{}, "", errors.New("graph changed since this request was prepared")
	}
	old, ok := m.g.Node(id)
	if !ok {
		return graph.Node{}, "", fmt.Errorf("node %q not found", id)
	}
	if patch.ID != "" && patch.ID != id {
		return graph.Node{}, "", errors.New("node ID is immutable")
	}
	if patch.Type != "" && patch.Type != old.Type {
		return graph.Node{}, "", errors.New("node type is immutable; use provisional review")
	}
	merged := old
	if patch.Name != "" {
		merged.Name = patch.Name
	}
	next, err := m.g.UpdateNode(id, merged)
	if err != nil {
		return graph.Node{}, "", err
	}
	m.g = next
	m.revision = "rev-after-patch-node"
	return merged, m.revision, nil
}

func (m *entityWriterMock) DeleteNodeWithRevision(_ context.Context, id string, expectedRevision string) (string, error) {
	if expectedRevision != m.revision {
		return "", errors.New("graph changed since this request was prepared")
	}
	old, ok := m.g.Node(id)
	if !ok {
		return "", fmt.Errorf("node %q not found", id)
	}
	if old.Type == graph.Self {
		return "", errors.New("Self cannot be deleted")
	}
	for _, r := range m.g.Relations() {
		if (r.From == id || r.To == id) && !(r.From == "self" && r.To == id) {
			return "", fmt.Errorf("node %q has active relations (%s); remove relations first", id, r.ID)
		}
	}
	next, err := m.g.DeleteNode(id)
	if err != nil {
		return "", err
	}
	m.g = next
	m.revision = "rev-after-delete-node"
	return m.revision, nil
}

func (m *entityWriterMock) RetireNode(_ context.Context, id string, expectedRevision string) (graph.Node, string, error) {
	if expectedRevision != "" && expectedRevision != m.revision {
		return graph.Node{}, "", errors.New("graph changed since this request was prepared")
	}
	old, ok := m.g.Node(id)
	if !ok {
		return graph.Node{}, "", fmt.Errorf("node %q not found", id)
	}
	if old.Type == graph.Self {
		return graph.Node{}, "", errors.New("Self cannot be retired")
	}
	next, err := m.g.RetireNode(id)
	if err != nil {
		return graph.Node{}, "", err
	}
	m.g = next
	m.revision = "rev-after-retire-node"
	retired, _ := m.g.Node(id)
	return retired, m.revision, nil
}

func (m *entityWriterMock) PatchRelation(_ context.Context, id string, patch graph.Relation, expectedRevision string) (graph.Relation, string, error) {
	if expectedRevision != m.revision {
		return graph.Relation{}, "", errors.New("graph changed since this request was prepared")
	}
	old, ok := m.g.Relation(id)
	if !ok {
		return graph.Relation{}, "", fmt.Errorf("relation %q not found", id)
	}
	if (patch.From != "" && patch.From != old.From) || (patch.To != "" && patch.To != old.To) || (patch.Predicate != "" && patch.Predicate != old.Predicate) {
		return graph.Relation{}, "", errors.New("relation endpoints and predicate are immutable; replace through explicit delete/add")
	}
	merged := old
	if patch.Certainty != "" {
		merged.Certainty = patch.Certainty
	}
	next, err := m.g.UpdateRelation(id, merged)
	if err != nil {
		return graph.Relation{}, "", err
	}
	m.g = next
	m.revision = "rev-after-patch-rel"
	return merged, m.revision, nil
}

func (m *entityWriterMock) DeleteRelationWithRevision(_ context.Context, id string, expectedRevision string) (string, error) {
	if expectedRevision != m.revision {
		return "", errors.New("graph changed since this request was prepared")
	}
	if _, ok := m.g.Relation(id); !ok {
		return "", fmt.Errorf("relation %q not found", id)
	}
	next, err := m.g.DeleteRelation(id)
	if err != nil {
		return "", err
	}
	m.g = next
	m.revision = "rev-after-delete-rel"
	return m.revision, nil
}

func TestWriteEntityAPINodeLifecycle(t *testing.T) {
	g := fixture()
	// Add an isolated node with only its self anchor for clean deletion testing
	if err := g.AddNode(graph.Node{ID: "leaf-machine", Type: graph.Machine, Name: "Leaf Machine", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "anchor-leaf", From: "self", Predicate: graph.KnowsAbout, To: "leaf-machine"}); err != nil {
		t.Fatal(err)
	}

	// Add a dependent node and relation targeting "leaf-machine" to test referential integrity
	if err := g.AddNode(graph.Node{ID: "dependent-proj", Type: graph.SoftwareProject, Name: "Dependent Project", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "anchor-dep", From: "self", Predicate: graph.Maintains, To: "dependent-proj"}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "dep-runs-on-leaf", From: "dependent-proj", Predicate: graph.DependsOn, To: "leaf-machine"}); err != nil {
		t.Fatal(err)
	}

	repo := &entityWriterMock{memoryRepository: &memoryRepository{g: g}, revision: "rev-initial"}
	token := strings.Repeat("k", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}

	// 1. PATCH without token => 401
	req := httptest.NewRequest(http.MethodPatch, "/api/write/nodes/leaf-machine", strings.NewReader(`{"expectedRevision":"rev-initial","node":{"name":"Updated"}}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", w.Code)
	}

	// 2. PATCH without expectedRevision => 400
	req = httptest.NewRequest(http.MethodPatch, "/api/write/nodes/leaf-machine", strings.NewReader(`{"node":{"name":"Updated"}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing expectedRevision, got %d", w.Code)
	}

	// 3. PATCH with stale revision => 409
	req = httptest.NewRequest(http.MethodPatch, "/api/write/nodes/leaf-machine", strings.NewReader(`{"expectedRevision":"wrong-rev","node":{"name":"Updated"}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict for stale revision, got %d", w.Code)
	}

	// 4. PATCH non-existent node => 404
	req = httptest.NewRequest(http.MethodPatch, "/api/write/nodes/non-existent", strings.NewReader(`{"expectedRevision":"rev-initial","node":{"name":"Updated"}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 not found, got %d", w.Code)
	}

	// 5. PATCH successful update
	req = httptest.NewRequest(http.MethodPatch, "/api/write/nodes/leaf-machine", strings.NewReader(`{"expectedRevision":"rev-initial","node":{"name":"Renamed Leaf Machine"}}`))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	var patchResp struct {
		Node     graph.Node `json:"node"`
		Revision string     `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &patchResp); err != nil {
		t.Fatal(err)
	}
	if patchResp.Node.Name != "Renamed Leaf Machine" || patchResp.Revision != "rev-after-patch-node" {
		t.Fatalf("unexpected patch response: %+v", patchResp)
	}

	// 6. DELETE node with active relations => 409
	// "leaf-machine" has dependent relation "dep-runs-on-leaf"
	req = httptest.NewRequest(http.MethodDelete, "/api/write/nodes/leaf-machine?expectedRevision="+patchResp.Revision, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 deleting node with active relations, got %d: %s", w.Code, w.Body.String())
	}

	// 7. DELETE Self => 409
	req = httptest.NewRequest(http.MethodDelete, "/api/write/nodes/self?expectedRevision="+patchResp.Revision, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 deleting Self, got %d", w.Code)
	}

	// 8. Remove the dependent relation first, then DELETE leaf node successfully
	req = httptest.NewRequest(http.MethodDelete, "/api/write/relations/dep-runs-on-leaf?expectedRevision="+patchResp.Revision, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting dependent relation, got %d: %s", w.Code, w.Body.String())
	}
	var delRelResp struct {
		Revision string `json:"revision"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &delRelResp)

	// Now delete leaf node (supports query param expectedRevision)
	req = httptest.NewRequest(http.MethodDelete, "/api/write/nodes/leaf-machine?expectedRevision="+delRelResp.Revision, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting leaf node, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWriteEntityAPIRelationLifecycle(t *testing.T) {
	g := fixture()
	// Add an extra non-critical relation for patch/delete testing
	extraRel := graph.Relation{ID: "extra-rel", From: "self", Predicate: graph.KnowsAbout, To: "machine"}
	if err := g.AddRelation(extraRel); err != nil {
		t.Fatal(err)
	}

	repo := &entityWriterMock{memoryRepository: &memoryRepository{g: g}, revision: "rev-initial"}
	token := strings.Repeat("m", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}

	// 1. PATCH relation modifying endpoints => 409
	body, _ := json.Marshal(map[string]any{
		"expectedRevision": "rev-initial",
		"relation":         map[string]any{"from": "different"},
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/write/relations/extra-rel", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 modifying relation endpoints, got %d", w.Code)
	}

	// 2. PATCH relation certainty successfully => 200
	body, _ = json.Marshal(map[string]any{
		"expectedRevision": "rev-initial",
		"relation":         map[string]any{"certainty": "confirmed"},
	})
	req = httptest.NewRequest(http.MethodPatch, "/api/write/relations/extra-rel", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating relation, got %d: %s", w.Code, w.Body.String())
	}

	// 3. DELETE relation successfully with body expectedRevision => 200
	deleteBody, _ := json.Marshal(map[string]any{"expectedRevision": "rev-after-patch-rel"})
	req = httptest.NewRequest(http.MethodDelete, "/api/write/relations/extra-rel", bytes.NewReader(deleteBody))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting extra relation, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWriteEntityAPIRetireNode(t *testing.T) {
	g := fixture()
	if err := g.AddNode(graph.Node{ID: "node-to-retire", Type: graph.Machine, Name: "Machine Active", Lifecycle: "active", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "anchor-retire", From: "self", Predicate: graph.Owns, To: "node-to-retire"}); err != nil {
		t.Fatal(err)
	}

	repo := &entityWriterMock{memoryRepository: &memoryRepository{g: g}, revision: "rev-1"}
	token := strings.Repeat("r", 32)
	h := &Handler{Graph: g, Repository: repo, WriteToken: token}

	// 1. Without auth token => 401
	req := httptest.NewRequest(http.MethodPost, "/api/write/nodes/node-to-retire/retire", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", w.Code)
	}

	// 2. Non-existent node => 404
	req = httptest.NewRequest(http.MethodPost, "/api/write/nodes/nonexistent/retire", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 not found, got %d", w.Code)
	}

	// 3. Retiring Self => 409
	req = httptest.NewRequest(http.MethodPost, "/api/write/nodes/self/retire", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict retiring self, got %d", w.Code)
	}

	// 4. Stale expectedRevision => 409
	body, _ := json.Marshal(map[string]any{"expectedRevision": "stale-revision"})
	req = httptest.NewRequest(http.MethodPost, "/api/write/nodes/node-to-retire/retire", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict on stale revision, got %d", w.Code)
	}

	// 5. Successful retire with matching expectedRevision => 200
	body, _ = json.Marshal(map[string]any{"expectedRevision": "rev-1"})
	req = httptest.NewRequest(http.MethodPost, "/api/write/nodes/node-to-retire/retire", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK retiring node, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Node     graph.Node `json:"node"`
		Revision string     `json:"revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal retire response: %v", err)
	}
	if resp.Node.Lifecycle != "retired" {
		t.Fatalf("expected lifecycle retired, got %q", resp.Node.Lifecycle)
	}
	if resp.Revision != "rev-after-retire-node" {
		t.Fatalf("unexpected revision: %q", resp.Revision)
	}

	// 6. Successful retire without expectedRevision body => 200
	req = httptest.NewRequest(http.MethodPost, "/api/write/nodes/node-to-retire/retire", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK retiring node without expectedRevision, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetNodesIncludeRetiredFilter(t *testing.T) {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "active-node", Type: graph.Machine, Name: "Active Machine", Lifecycle: "active", ModelVersion: 1}, graph.Relation{ID: "a1", To: "active-node"})
	_ = g.Publish(graph.Node{ID: "retired-node", Type: graph.Machine, Name: "Retired Machine", Lifecycle: "retired", ModelVersion: 1}, graph.Relation{ID: "a2", To: "retired-node"})

	h := &Handler{Graph: g}

	// 1. Default (no includeRetired param) includes all nodes (3 total)
	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var respAll struct {
		TotalCount int          `json:"totalCount"`
		Items      []graph.Node `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &respAll)
	if respAll.TotalCount != 3 {
		t.Fatalf("expected totalCount 3, got %d", respAll.TotalCount)
	}

	// 2. includeRetired=true includes all nodes (3 total)
	req = httptest.NewRequest(http.MethodGet, "/api/nodes?includeRetired=true", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var respTrue struct {
		TotalCount int          `json:"totalCount"`
		Items      []graph.Node `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &respTrue)
	if respTrue.TotalCount != 3 {
		t.Fatalf("expected totalCount 3, got %d", respTrue.TotalCount)
	}

	// 3. includeRetired=false excludes retired nodes (2 total: self and active-node)
	req = httptest.NewRequest(http.MethodGet, "/api/nodes?includeRetired=false", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var respFalse struct {
		TotalCount int          `json:"totalCount"`
		Items      []graph.Node `json:"items"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &respFalse)
	if respFalse.TotalCount != 2 {
		t.Fatalf("expected totalCount 2, got %d", respFalse.TotalCount)
	}
	for _, item := range respFalse.Items {
		if item.Lifecycle == "retired" {
			t.Fatalf("retired node returned when includeRetired=false: %+v", item)
		}
	}

	// 4. Invalid includeRetired => 400
	req = httptest.NewRequest(http.MethodGet, "/api/nodes?includeRetired=invalid", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid includeRetired, got %d", w.Code)
	}
}
