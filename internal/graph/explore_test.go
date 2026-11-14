package graph

import (
	"fmt"
	"testing"
)

func TestExploreCapsLargeNeighborhoodAndReportsTruncation(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	for i := 0; i < 260; i++ {
		id := fmt.Sprintf("p-%03d", i)
		if err := g.AddNode(Node{ID: id, Type: Product, Name: id, ModelVersion: 1}); err != nil {
			t.Fatal(err)
		}
		if err := g.AddRelation(Relation{ID: "r-" + id, From: "self", Predicate: KnowsAbout, To: id}); err != nil {
			t.Fatal(err)
		}
	}
	result, err := g.Explore(ExploreOptions{CenterID: "self", Direction: "out", Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) != 250 || !result.Truncated {
		t.Fatalf("expected bounded truncated subgraph: nodes=%d truncated=%v", len(result.Nodes), result.Truncated)
	}
}

func TestExploreFiltersDirectionAndDepth(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(Node{ID: "host", Type: Machine, Name: "Host", ModelVersion: 1}, Relation{ID: "maintains", Predicate: Maintains, To: "host"})
	_ = g.AddNode(Node{ID: "instance", Type: SoftwareInstance, Name: "Service", ModelVersion: 1})
	_ = g.AddRelation(Relation{ID: "hosts", From: "host", Predicate: Hosts, To: "instance"})
	_ = g.Publish(Node{ID: "known", Type: Product, Name: "Known", ModelVersion: 1}, Relation{ID: "knows", To: "known"})
	out, err := g.Explore(ExploreOptions{CenterID: "self", Direction: "out", Depth: 2, Predicates: []Predicate{Maintains, Hosts}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 3 || len(out.Relations) != 2 {
		t.Fatalf("unexpected filtered exploration: %+v", out)
	}
	in, err := g.Explore(ExploreOptions{CenterID: "instance", Direction: "in", Depth: 1, Predicates: []Predicate{Hosts}})
	if err != nil {
		t.Fatal(err)
	}
	if len(in.Nodes) != 2 || in.Nodes[0].ID != "host" && in.Nodes[1].ID != "host" {
		t.Fatalf("incoming traversal failed: %+v", in)
	}
	if _, err = g.Explore(ExploreOptions{CenterID: "self", Direction: "sideways", Depth: 1}); err == nil {
		t.Fatal("invalid direction accepted")
	}
}
