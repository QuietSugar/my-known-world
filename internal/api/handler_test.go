package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func fixture() *graph.Graph {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "machine", Type: graph.Machine, Name: "Demo host", ModelVersion: 1}, graph.Relation{ID: "r1", To: "machine"})
	return g
}
func request(h http.Handler, method, path string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestReadEndpoints(t *testing.T) {
	h := Handler{Graph: fixture()}
	for _, tc := range []struct{ path, contains string }{{"/healthz", `"status":"ok"`}, {"/api/nodes", `"id":"machine"`}, {"/api/relations", `"id":"r1"`}, {"/api/nodes/machine", `"name":"Demo host"`}, {"/api/nodes/machine/assertions", `"items"`}, {"/api/path/machine", `"relations"`}, {"/api/explore/machine?direction=both&depth=1", `"centerId":"machine"`}, {"/api/impact", `"items":[]`}, {"/api/provisionals", `"items":[]`}} {
		w := request(h, http.MethodGet, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.contains) {
			t.Errorf("%s: response %s", tc.path, w.Body.String())
		}
	}
}
func TestSyncPreviewEndpointIsReadOnly(t *testing.T) {
	g := fixture()
	h := Handler{Graph: g}
	body := `{"source":"inventory","nodes":[{"id":"new","type":"Machine","name":"New host","modelVersion":1}],"relations":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/sync/preview", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"addedNodes":["new"]`) {
		t.Fatalf("unexpected preview: %d %s", w.Code, w.Body.String())
	}
	if _, ok := g.Node("new"); ok {
		t.Fatal("preview endpoint mutated graph")
	}
	bad := httptest.NewRequest(http.MethodPost, "/api/sync/preview", strings.NewReader(`{} {}`))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, bad)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid batch rejection, got %d", w.Code)
	}
}

func TestErrorsAreStructured(t *testing.T) {
	h := Handler{Graph: fixture()}
	if w := request(h, http.MethodGet, "/api/nodes/missing"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unexpected not-found response: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, http.MethodPost, "/api/not-a-write-route"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected method response: %d", w.Code)
	}
}
