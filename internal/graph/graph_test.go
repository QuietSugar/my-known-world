package graph

import "testing"

func TestPublishEnforcesUniqueSelfAndConnectivity(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(Node{ID: "other", Type: Self, Name: "Other", ModelVersion: 1}); err == nil {
		t.Fatal("expected second Self to fail")
	}
	if err := g.Publish(Node{ID: "machine-1", Type: Machine, Name: "Unknown host", ModelVersion: 1}, Relation{ID: "r1", To: "machine-1"}); err != nil {
		t.Fatal(err)
	}
	path, ok := g.PathFromSelf("machine-1")
	if !ok || len(path) != 2 || path[0] != "self" || path[1] != "machine-1" {
		t.Fatalf("unexpected path: %v, %v", path, ok)
	}
	if links := g.RelationsForPath(path); len(links) != 1 || links[0].ID != "r1" {
		t.Fatalf("unexpected path relations: %v", links)
	}
}

func TestTypedRelationsAndFailedPublishAreAtomic(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	if err := g.Publish(Node{ID: "p", Type: SoftwareProject, Name: "Project", ModelVersion: 1}, Relation{ID: "bad", Predicate: ImplementedFrom, To: "p"}); err == nil {
		t.Fatal("invalid Self-to-project relation accepted")
	}
	if _, ok := g.Node("p"); ok {
		t.Fatal("failed publish left a node behind")
	}
	_ = g.AddNode(Node{ID: "machine", Type: Machine, Name: "Host", ModelVersion: 1})
	if err := g.AddRelation(Relation{ID: "bad-host", From: "self", Predicate: Hosts, To: "machine"}); err == nil {
		t.Fatal("invalid endpoint types accepted")
	}
}

func TestHostsSupportsNestedMachinesButKeepsDirectionTyped(t *testing.T) {
	g := New()
	for _, n := range []Node{{ID: "host", Type: Machine, Name: "Host", ModelVersion: 1}, {ID: "vm", Type: Machine, Name: "VM", ModelVersion: 1}, {ID: "service", Type: SoftwareInstance, Name: "Service", ModelVersion: 1}} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.AddRelation(Relation{ID: "nested", From: "host", Predicate: Hosts, To: "vm"}); err != nil {
		t.Fatalf("machine hosting machine should be valid: %v", err)
	}
	if err := g.AddRelation(Relation{ID: "deployment", From: "vm", Predicate: Hosts, To: "service"}); err != nil {
		t.Fatalf("machine hosting instance should be valid: %v", err)
	}
	if err := g.AddRelation(Relation{ID: "reverse", From: "service", Predicate: Hosts, To: "vm"}); err == nil {
		t.Fatal("software instance hosting a machine should be rejected")
	}
}

func TestPublishUsesActualSelfIdentity(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "anchor", Type: Self, Name: "Me", ModelVersion: 1})
	if err := g.Publish(Node{ID: "object", Type: Product, Name: "Product", ModelVersion: 1}, Relation{ID: "r1", To: "object"}); err != nil {
		t.Fatal(err)
	}
	path, ok := g.PathFromSelf("object")
	if !ok || path[0] != "anchor" {
		t.Fatalf("unexpected path from non-default Self ID: %v", path)
	}
}

func TestNoPathForDisconnectedNode(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	_ = g.AddNode(Node{ID: "p", Type: Product, Name: "Product", ModelVersion: 1})
	if _, ok := g.PathFromSelf("p"); ok {
		t.Fatal("disconnected node reported reachable")
	}
}
