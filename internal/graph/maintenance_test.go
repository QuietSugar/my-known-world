package graph

import "testing"

func maintenanceFixture() *Graph {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(Node{ID: "m", Type: Machine, Name: "Host", Source: "inventory", ModelVersion: 1}, Relation{ID: "weak", To: "m"})
	return g
}
func TestUpdateNodePreservesIdentitySourceAndReceiver(t *testing.T) {
	g := maintenanceFixture()
	updated, err := g.UpdateNode("m", Node{ID: "m", Type: Machine, Name: "Renamed", Source: "attacker", ModelVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := updated.Node("m")
	if n.Name != "Renamed" || n.Source != "inventory" {
		t.Fatalf("editable update violated identity/provenance: %+v", n)
	}
	old, _ := g.Node("m")
	if old.Name != "Host" {
		t.Fatal("update mutated receiver")
	}
	if _, err = g.UpdateNode("m", Node{ID: "other", Type: Machine, Name: "Bad", ModelVersion: 1}); err == nil {
		t.Fatal("identity replacement accepted")
	}
	if _, err = g.UpdateNode("m", Node{ID: "m", Type: Product, Name: "Bad", ModelVersion: 1}); err == nil {
		t.Fatal("type change accepted")
	}
}
func TestDeleteRelationRejectsDisconnectAndAddRelationIsChecked(t *testing.T) {
	g := maintenanceFixture()
	if _, err := g.DeleteRelation("weak"); err == nil {
		t.Fatal("disconnecting relation delete accepted")
	}
	candidate, err := g.AddRelationChecked(Relation{ID: "maint", From: "self", Predicate: Maintains, To: "m"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := candidate.DeleteRelation("weak")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Relations()) != 1 {
		t.Fatalf("expected replacement relation remains: %+v", updated.Relations())
	}
}

func TestRetireNode(t *testing.T) {
	g := maintenanceFixture()
	// 1. Retiring an existing node succeeds and updates lifecycle & timestamp
	retiredGraph, err := g.RetireNode("m")
	if err != nil {
		t.Fatalf("RetireNode failed: %v", err)
	}
	node, ok := retiredGraph.Node("m")
	if !ok {
		t.Fatal("node m missing after retire")
	}
	if node.Lifecycle != "retired" {
		t.Fatalf("expected lifecycle retired, got %q", node.Lifecycle)
	}
	if node.LastConfirmed == nil {
		t.Fatal("expected non-nil LastConfirmed after retire")
	}
	// Receiver g is not mutated
	origNode, _ := g.Node("m")
	if origNode.Lifecycle == "retired" {
		t.Fatal("RetireNode mutated receiver graph")
	}

	// 2. Retiring Self fails
	if _, err = g.RetireNode("self"); err == nil {
		t.Fatal("expected error retiring self")
	}

	// 3. Retiring unknown node fails
	if _, err = g.RetireNode("nonexistent"); err == nil {
		t.Fatal("expected error retiring nonexistent node")
	}
}

