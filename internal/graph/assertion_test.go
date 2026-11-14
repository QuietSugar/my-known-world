package graph

import "testing"

func TestFieldAssertionValidationAndStaleRetention(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	_ = g.Publish(Node{ID: "host", Type: Machine, Name: "Host", ModelVersion: 1}, Relation{ID: "knows", To: "host"})
	a := FieldAssertion{ID: "host-name-v1", NodeID: "host", FieldPath: "name", ValueHash: HashFieldValue("Host"), Reference: "synthetic://catalog/host", Source: "fixture", EvidenceKind: EvidenceDocument, Certainty: CertaintyConfirmed}
	if err := g.AddAssertion(a); err != nil {
		t.Fatal(err)
	}
	if err := g.AddAssertion(FieldAssertion{ID: "bad-secret", NodeID: "host", FieldPath: "password", ValueHash: HashFieldValue("do-not-store")}); err == nil {
		t.Fatal("unmodeled/secret-like field should not be accepted")
	}
	updated, preview, err := g.ApplySync(SourceBatch{Source: "fixture", Nodes: []Node{}, Relations: []Relation{}, Assertions: []FieldAssertion{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.MissingAssertions) != 1 || preview.MissingAssertions[0] != a.ID {
		t.Fatalf("missing assertion not reported: %+v", preview)
	}
	stale := updated.AssertionsForNode("host")
	if len(stale) != 1 || stale[0].Certainty != CertaintyStale {
		t.Fatalf("assertion should be retained stale: %+v", stale)
	}
	if len(g.AssertionsForNode("host")) != 1 || g.AssertionsForNode("host")[0].Certainty != CertaintyConfirmed {
		t.Fatal("apply mutated receiver graph")
	}
}
