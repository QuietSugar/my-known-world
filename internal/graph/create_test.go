package graph

import (
	"fmt"
	"strings"
	"testing"
)

func createFixture(t *testing.T) *Graph {
	t.Helper()
	g := New()
	if err := g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestApplyCreateBatchPublishesNodesWithSelfAnchors(t *testing.T) {
	g := createFixture(t)
	next, result, err := g.ApplyCreateBatch(CreateBatch{
		Nodes: []CreateNodeRequest{
			{Node: Node{ID: "host-a", Type: Machine, Name: "Host A", ModelVersion: 1}},
			{Node: Node{ID: "project-a", Type: SoftwareProject, Name: "Project A", ModelVersion: 1}, Anchor: &CreateAnchor{Predicate: Owns, ID: "owns-project-a"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NodeIDs) != 2 || result.RelationIDs[0] != "anchor-host-a" || result.RelationIDs[1] != "owns-project-a" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(g.Nodes()) != 1 || len(next.Nodes()) != 3 {
		t.Fatalf("source graph must stay unchanged: before=%d after=%d", len(g.Nodes()), len(next.Nodes()))
	}
	relation, ok := relationByID(next, "anchor-host-a")
	if !ok || relation.From != "self" || relation.To != "host-a" || relation.Predicate != KnowsAbout {
		t.Fatalf("default anchor relation: %+v", relation)
	}
	if relation, ok = relationByID(next, "owns-project-a"); !ok || relation.Predicate != Owns {
		t.Fatalf("explicit anchor relation: %+v", relation)
	}
	if len(result.Delta.AddedNodes) != 2 || len(result.Delta.AddedRelations) != 2 {
		t.Fatalf("delta: %+v", result.Delta)
	}
}

func TestApplyCreateBatchIsAtomicAndValidatesEveryItem(t *testing.T) {
	g := createFixture(t)
	cases := []struct {
		name  string
		batch CreateBatch
		want  string
	}{
		{name: "empty", batch: CreateBatch{}, want: "at least one"},
		{name: "missing id", batch: CreateBatch{Nodes: []CreateNodeRequest{{Node: Node{Type: Machine, Name: "x", ModelVersion: 1}}}}, want: "requires an id"},
		{name: "duplicate node", batch: CreateBatch{Nodes: []CreateNodeRequest{
			{Node: Node{ID: "dup", Type: Machine, Name: "x", ModelVersion: 1}},
			{Node: Node{ID: "dup", Type: Machine, Name: "y", ModelVersion: 1}},
		}}, want: "appears twice"},
		{name: "unknown type", batch: CreateBatch{Nodes: []CreateNodeRequest{{Node: Node{ID: "bad", Type: NodeType("Alien"), Name: "x", ModelVersion: 1}}}}, want: "unknown node type"},
		{name: "existing node", batch: CreateBatch{Nodes: []CreateNodeRequest{{Node: Node{ID: "self", Type: Machine, Name: "self", ModelVersion: 1}}}}, want: "already exists"},
		{name: "invalid anchor predicate", batch: CreateBatch{Nodes: []CreateNodeRequest{{Node: Node{ID: "n1", Type: Machine, Name: "x", ModelVersion: 1}, Anchor: &CreateAnchor{Predicate: Hosts}}}}, want: "invalid hosts relation"},
		{name: "relation with unknown endpoint", batch: CreateBatch{Relations: []Relation{{ID: "r-missing", From: "self", Predicate: Owns, To: "ghost"}}}, want: "not found"},
		{name: "relation duplicates node anchor id", batch: CreateBatch{
			Nodes:     []CreateNodeRequest{{Node: Node{ID: "n2", Type: Machine, Name: "x", ModelVersion: 1}}},
			Relations: []Relation{{ID: "anchor-n2", From: "self", Predicate: Owns, To: "n2"}},
		}, want: "appears twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, _, err := g.ApplyCreateBatch(tc.batch)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
			if next != nil {
				t.Fatal("failed batch returned a graph")
			}
			if len(g.Nodes()) != 1 {
				t.Fatal("failed batch modified the receiver")
			}
		})
	}
}

func TestApplyCreateBatchRejectsOversizedBatches(t *testing.T) {
	g := createFixture(t)
	nodes := make([]CreateNodeRequest, MaxCreateNodes+1)
	for i := range nodes {
		nodes[i] = CreateNodeRequest{Node: Node{ID: fmt.Sprintf("host-%d", i), Type: Machine, Name: "host", ModelVersion: 1}}
	}
	if _, _, err := g.ApplyCreateBatch(CreateBatch{Nodes: nodes}); err == nil || !strings.Contains(err.Error(), "limit is") {
		t.Fatalf("oversized batch: %v", err)
	}
}

func TestApplyCreateBatchAddsRelationsBetweenBatchAndExistingNodes(t *testing.T) {
	g := createFixture(t)
	next, _, err := g.ApplyCreateBatch(CreateBatch{
		Nodes: []CreateNodeRequest{
			{Node: Node{ID: "host-b", Type: Machine, Name: "Host B", ModelVersion: 1}},
			{Node: Node{ID: "project-b", Type: SoftwareProject, Name: "Project B", ModelVersion: 1}},
		},
		Relations: []Relation{{ID: "depends-on", From: "project-b", Predicate: DependsOn, To: "host-b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if relation, ok := relationByID(next, "depends-on"); !ok || relation.From != "project-b" || relation.To != "host-b" {
		t.Fatalf("relation missing or wrong: %+v", relation)
	}
}

func relationByID(g *Graph, id string) (Relation, bool) {
	for _, relation := range g.Relations() {
		if relation.ID == id {
			return relation, true
		}
	}
	return Relation{}, false
}
