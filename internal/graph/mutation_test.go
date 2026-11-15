package graph

import "testing"

func TestManualRelationEvidenceUpdateIsPreviewableAndKeepsIdentity(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	if err := g.Publish(Node{ID: "machine", Type: Machine, Name: "Machine", ModelVersion: 1}, Relation{ID: "owns-machine", Predicate: Owns, To: "machine", Source: "manual:ui", EvidenceKind: EvidenceManual, Certainty: CertaintyProvisional}); err != nil {
		t.Fatal(err)
	}
	relation := g.Relations()[0]
	relation.EvidenceKind, relation.Certainty = EvidenceObserved, CertaintyConfirmed
	mutation := ManualMutation{Kind: "updateRelation", ID: relation.ID, Relation: &relation}
	updated, err := g.ApplyManualMutation(mutation)
	if err != nil {
		t.Fatal(err)
	}
	got := updated.Relations()[0]
	if got.From != "self" || got.To != "machine" || got.Predicate != Owns || got.Source != "manual:ui" || got.EvidenceKind != EvidenceObserved || got.Certainty != CertaintyConfirmed {
		t.Fatalf("relation identity or provenance was not updated correctly: %+v", got)
	}
	original := g.Relations()[0]
	if original.EvidenceKind != EvidenceManual {
		t.Fatal("preview mutation changed source graph")
	}
	changed := got
	changed.Predicate = Uses
	if _, err := g.UpdateRelation(got.ID, changed); err == nil {
		t.Fatal("relation predicate must be immutable in metadata update")
	}
}

func TestManualMutationPlanHasDeterministicDeltaAndDoesNotMutate(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(Node{ID: "m", Type: Machine, Name: "Before", ModelVersion: 1}, Relation{ID: "knows", To: "m"})
	mutation := ManualMutation{Kind: "updateNode", ID: "m", Node: &Node{ID: "m", Type: Machine, Name: "After", ModelVersion: 1}}
	candidate, err := g.ApplyManualMutation(mutation)
	if err != nil {
		t.Fatal(err)
	}
	delta := DiffGraphs(g, candidate)
	if len(delta.UpdatedNodes) != 1 || delta.UpdatedNodes[0] != "m" {
		t.Fatalf("unexpected plan delta: %+v", delta)
	}
	old, _ := g.Node("m")
	if old.Name != "Before" {
		t.Fatal("preview mutated the graph")
	}
	h1, _ := mutation.PlanHashValue()
	h2, _ := mutation.PlanHashValue()
	if h1 != h2 || h1 == "" {
		t.Fatal("mutation plan hash is not deterministic")
	}
}
