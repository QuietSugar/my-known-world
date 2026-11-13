package api

import (
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
	for _, tc := range []struct{ path, contains string }{{"/healthz", `"status":"ok"`}, {"/api/nodes", `"id":"machine"`}, {"/api/nodes/machine", `"name":"Demo host"`}, {"/api/path/machine", `"relations"`}, {"/api/impact", `"items":[]`}} {
		w := request(h, http.MethodGet, tc.path)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status %d", tc.path, w.Code)
		}
		if !strings.Contains(w.Body.String(), tc.contains) {
			t.Errorf("%s: response %s", tc.path, w.Body.String())
		}
	}
}
func TestErrorsAreStructured(t *testing.T) {
	h := Handler{Graph: fixture()}
	if w := request(h, http.MethodGet, "/api/nodes/missing"); w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `"code":"not_found"`) {
		t.Fatalf("unexpected not-found response: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, http.MethodPost, "/api/nodes"); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected method response: %d", w.Code)
	}
}
