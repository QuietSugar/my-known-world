package graph

import "testing"

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
