package graph

import "testing"

func TestProvenanceDefaultsAreExplicitlyUnknown(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	node, ok := g.Node("self")
	if !ok || node.Source != "unspecified" || node.EvidenceKind != EvidenceUnknown || node.Certainty != CertaintyUnknown {
		t.Fatalf("provenance defaults did not preserve unknown state: %+v", node)
	}
	if err := g.Publish(Node{ID: "project", Type: SoftwareProject, Name: "Project", Source: "catalog:public", EvidenceKind: EvidenceDocument, Certainty: CertaintyProvisional, ModelVersion: 1}, Relation{ID: "knows", To: "project"}); err != nil {
		t.Fatal(err)
	}
	relations := g.Relations()
	if len(relations) != 1 || relations[0].EvidenceKind != EvidenceUnknown || relations[0].Certainty != CertaintyUnknown {
		t.Fatalf("relationship provenance was not normalized: %+v", relations)
	}
}

func TestRejectsUnknownEvidenceAndCertaintyEnums(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "bad", Type: Product, Name: "Invalid evidence", EvidenceKind: "guessed", ModelVersion: 1}); err == nil {
		t.Fatal("expected unknown evidence kind to be rejected")
	}
	if err := g.AddNode(Node{ID: "bad-certainty", Type: Product, Name: "Invalid certainty", Certainty: "certain-ish", ModelVersion: 1}); err == nil {
		t.Fatal("expected invalid certainty to be rejected")
	}
}
