package graph

import "testing"

func TestImpactExcludesKnowledgeAndFollowsOperationalRelations(t *testing.T) {
	g := New()
	for _, n := range []Node{{ID: "self", Type: Self, Name: "Me", ModelVersion: 1}, {ID: "host", Type: Machine, Name: "Host", ModelVersion: 1}, {ID: "service", Type: SoftwareInstance, Name: "Service", ModelVersion: 1}, {ID: "public", Type: SoftwareProject, Name: "Public project", ModelVersion: 1}} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Relation{{ID: "r1", From: "self", Predicate: Maintains, To: "host"}, {ID: "r2", From: "host", Predicate: Hosts, To: "service"}, {ID: "r3", From: "self", Predicate: KnowsAbout, To: "public"}} {
		if err := g.AddRelation(r); err != nil {
			t.Fatal(err)
		}
	}
	impact := g.ImpactFromSelf()
	if len(impact) != 2 {
		t.Fatalf("expected host and service, got %v", impact)
	}
	if impact[0].Node.ID != "host" || impact[1].Node.ID != "service" {
		t.Fatalf("unexpected deterministic ordering: %v", impact)
	}
	if len(impact[1].RelationPath) != 2 {
		t.Fatalf("expected full path, got %v", impact[1].RelationPath)
	}
}
