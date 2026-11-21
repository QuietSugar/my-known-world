package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func pagedTestGraph() *graph.Graph {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("node-%02d", i)
		node := graph.Node{
			ID:           id,
			Type:         graph.Machine,
			Name:         fmt.Sprintf("Machine %02d", i),
			ModelVersion: 1,
			Source:       "test",
			EvidenceKind: graph.EvidenceObserved,
			Certainty:    graph.CertaintyConfirmed,
			Machine:      &graph.MachineDetails{Hostname: fmt.Sprintf("host-%02d.local", i)},
		}
		_ = g.AddNode(node)
		_ = g.AddRelation(graph.Relation{
			ID:        fmt.Sprintf("rel-%02d", i),
			From:      "self",
			Predicate: graph.KnowsAbout,
			To:        id,
			Source:    "test",
		})
	}
	return g
}

func TestNodesReadAPIUnifiedStructureWithoutQuery(t *testing.T) {
	g := pagedTestGraph()
	h := Handler{Graph: g}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res struct {
		Items      []graph.Node `json:"items"`
		TotalCount int          `json:"totalCount"`
		Count      int          `json:"count"`
		HasMore    bool         `json:"hasMore"`
		NextCursor string       `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 26 || res.TotalCount != 26 || res.Count != 26 || res.HasMore || res.NextCursor != "" {
		t.Fatalf("expected unified standard response structure on empty query: %+v", res)
	}
	if w.Header().Get("X-Total-Count") != "26" {
		t.Fatalf("expected X-Total-Count 26, got %s", w.Header().Get("X-Total-Count"))
	}
}

func TestNodesPaginationAndCursors(t *testing.T) {
	g := pagedTestGraph()
	h := Handler{Graph: g}

	// 1. Page 1: limit=10
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/nodes?limit=10", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("page 1 failed: %d %s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Total-Count") != "26" {
		t.Fatalf("expected X-Total-Count 26, got %s", w.Header().Get("X-Total-Count"))
	}
	var page1 struct {
		Items      []graph.Node `json:"items"`
		TotalCount int          `json:"totalCount"`
		Count      int          `json:"count"`
		HasMore    bool         `json:"hasMore"`
		NextCursor string       `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page1); err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 10 || page1.TotalCount != 26 || !page1.HasMore {
		t.Fatalf("unexpected page 1: %+v", page1)
	}
	firstCursor := page1.NextCursor
	if firstCursor == "" || w.Header().Get("X-Next-Cursor") != firstCursor {
		t.Fatalf("missing or mismatch next cursor: %s vs %s", firstCursor, w.Header().Get("X-Next-Cursor"))
	}

	// 2. Page 2: limit=10&cursor=firstCursor
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/nodes?limit=10&cursor="+firstCursor, nil)
	h.ServeHTTP(w, r)
	var page2 struct {
		Items      []graph.Node `json:"items"`
		HasMore    bool         `json:"hasMore"`
		NextCursor string       `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page2); err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 10 || !page2.HasMore {
		t.Fatalf("unexpected page 2: %+v", page2)
	}

	// 3. Page 3: limit=10&cursor=page2.NextCursor
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/nodes?limit=10&cursor="+page2.NextCursor, nil)
	h.ServeHTTP(w, r)
	var page3 struct {
		Items      []graph.Node `json:"items"`
		HasMore    bool         `json:"hasMore"`
		NextCursor string       `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page3); err != nil {
		t.Fatal(err)
	}
	if len(page3.Items) != 6 || page3.HasMore || page3.NextCursor != "" {
		t.Fatalf("unexpected page 3 (last page): %+v", page3)
	}

	// Combined items check
	var allPaged []string
	for _, it := range page1.Items {
		allPaged = append(allPaged, it.ID)
	}
	for _, it := range page2.Items {
		allPaged = append(allPaged, it.ID)
	}
	for _, it := range page3.Items {
		allPaged = append(allPaged, it.ID)
	}
	if len(allPaged) != 26 {
		t.Fatalf("expected 26 total paged items, got %d", len(allPaged))
	}
}

func TestNodesLimitAndOffsetValidation(t *testing.T) {
	g := pagedTestGraph()
	h := Handler{Graph: g}

	// limit <= 0
	for _, badLimit := range []string{"0", "-5", "abc"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/nodes?limit="+badLimit, nil)
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("limit=%s expected 400, got %d", badLimit, w.Code)
		}
	}

	// limit > 500
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/nodes?limit=501", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "cannot exceed 500") {
		t.Fatalf("expected 400 limit cannot exceed 500, got %d: %s", w.Code, w.Body.String())
	}

	// offset < 0
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/nodes?offset=-1", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("offset=-1 expected 400, got %d", w.Code)
	}

	// valid offset
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/nodes?offset=20&limit=10", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("valid offset failed: %d", w.Code)
	}
	var res struct {
		Items []graph.Node `json:"items"`
		Count int          `json:"count"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Items) != 6 {
		t.Fatalf("expected 6 remaining items from offset 20, got %d", len(res.Items))
	}
}

func TestNodesViewSummaryProjection(t *testing.T) {
	g := pagedTestGraph()
	h := Handler{Graph: g}

	// Invalid view
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/nodes?view=unknown", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown view, got %d", w.Code)
	}

	// view=summary
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/nodes?view=summary&limit=5", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Must NOT contain typed detail object key "machine" or "hostname"
	if strings.Contains(body, `"machine":`) || strings.Contains(body, `"hostname"`) {
		t.Fatalf("view=summary must omit typed detail blocks, but got: %s", body)
	}

	var res struct {
		Items []graph.NodeSummary `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(res.Items))
	}
	if res.Items[0].ID == "" || res.Items[0].Type == "" || res.Items[0].Name == "" {
		t.Fatalf("summary fields missing: %+v", res.Items[0])
	}
}

func TestRelationsPagination(t *testing.T) {
	g := pagedTestGraph()
	h := Handler{Graph: g}

	// Unified structure on empty query
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/relations", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var raw struct {
		Items      []graph.Relation `json:"items"`
		TotalCount int              `json:"totalCount"`
		Count      int              `json:"count"`
		HasMore    bool             `json:"hasMore"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Items) != 25 || raw.TotalCount != 25 || raw.Count != 25 || raw.HasMore {
		t.Fatalf("expected standard unified relations structure: %+v", raw)
	}

	// Paged relations
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/api/relations?limit=10", nil)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res struct {
		Items      []graph.Relation `json:"items"`
		TotalCount int              `json:"totalCount"`
		HasMore    bool             `json:"hasMore"`
		NextCursor string           `json:"nextCursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 10 || res.TotalCount != 25 || !res.HasMore || res.NextCursor == "" {
		t.Fatalf("unexpected relations page: %+v", res)
	}
}
