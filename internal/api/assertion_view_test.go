package api

import (
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"strings"
	"testing"
)

func TestAssertionReadIsValueFreeAndExposesDecisionState(t *testing.T) {
	g := fixture()
	hash := strings.Repeat("a", 64)
	if err := g.AddAssertion(graph.FieldAssertion{ID: "assertion-1", NodeID: "machine", FieldPath: "machine.hostname", ValueHash: hash, Source: "inventory", EvidenceKind: graph.EvidenceObserved, Certainty: graph.CertaintyConfirmed}); err != nil {
		t.Fatal(err)
	}
	h := Handler{Graph: g}
	w := request(h, http.MethodGet, "/api/nodes/machine/assertions")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), hash) || strings.Contains(w.Body.String(), "valueHash") {
		t.Fatalf("field assertion API exposed digest or failed: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"resolved":false`) {
		t.Fatalf("resolution state missing: %s", w.Body.String())
	}
}
