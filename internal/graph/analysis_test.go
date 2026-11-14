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

func TestUsageAndRestrictedOperationRemainDistinctInImpact(t *testing.T) {
	g := New()
	for _, n := range []Node{{ID: "self", Type: Self, Name: "Self", ModelVersion: 1}, {ID: "uses", Type: Machine, Name: "Used machine", ModelVersion: 1}, {ID: "operate", Type: Machine, Name: "Restricted machine", ModelVersion: 1}, {ID: "knows", Type: Machine, Name: "Known machine", ModelVersion: 1}} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Relation{{ID: "use", From: "self", Predicate: Uses, To: "uses"}, {ID: "operate", From: "self", Predicate: CanOperate, To: "operate"}, {ID: "know", From: "self", Predicate: KnowsAbout, To: "knows"}} {
		if err := g.AddRelation(r); err != nil {
			t.Fatal(err)
		}
	}
	impact := g.ImpactFromSelf()
	if len(impact) != 1 || impact[0].Node.ID != "uses" {
		t.Fatalf("uses should be explicit impact, while limited operation/knowledge stay excluded: %+v", impact)
	}
}
