package testdata

import (
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestSyntheticWorldCoversCurrentModelAndSync(t *testing.T) {
	batch, err := SyntheticWorld()
	if err != nil {
		t.Fatal(err)
	}
	wantTypes := map[graph.NodeType]bool{graph.Self: true, graph.Hardware: true, graph.Machine: true, graph.SoftwareProject: true, graph.SoftwareInstance: true, graph.RunningPeriod: true, graph.AccountCredential: true, graph.Product: true, graph.Document: true, graph.Runbook: true, graph.WorkItem: true, graph.Network: true, graph.ProvisionalObject: true}
	wantPredicates := map[graph.Predicate]bool{graph.Owns: true, graph.Maintains: true, graph.KnowsAbout: true, graph.Uses: true, graph.CanOperate: true, graph.RoutesThrough: true, graph.ProvidesAccessTo: true, graph.Hosts: true, graph.ImplementedFrom: true, graph.HasRunningPeriod: true, graph.HasCredential: true, graph.Documents: true, graph.AppliesTo: true, graph.Targets: true, graph.Supersedes: true, graph.DependsOn: true}
	seenTypes := map[graph.NodeType]bool{}
	seenPredicates := map[graph.Predicate]bool{}
	for _, n := range batch.Nodes {
		seenTypes[n.Type] = true
	}
	for _, r := range batch.Relations {
		seenPredicates[r.Predicate] = true
	}
	for typ := range wantTypes {
		if !seenTypes[typ] {
			t.Errorf("fixture missing node type %q", typ)
		}
	}
	for pred := range wantPredicates {
		if !seenPredicates[pred] {
			t.Errorf("fixture missing predicate %q", pred)
		}
	}
	g := graph.New()
	updated, preview, err := g.ApplySync(batch)
	if err != nil {
		t.Fatalf("fixture batch rejected: %v", err)
	}
	if len(preview.AddedNodes) != len(batch.Nodes) || len(preview.AddedRelations) != len(batch.Relations) || len(preview.AddedAssertions) != len(batch.Assertions) {
		t.Fatalf("incomplete initial batch report: %+v", preview)
	}
	for _, n := range updated.Nodes() {
		if _, ok := updated.PathFromSelf(n.ID); !ok {
			t.Errorf("fixture node %q is disconnected", n.ID)
		}
	}
	machine, _ := updated.Node("machine-host")
	if machine.Machine == nil || machine.Machine.Hostname == "" || len(machine.Machine.ForbiddenOperations) == 0 {
		t.Fatalf("machine-specific fixture details missing: %+v", machine)
	}
	instance, _ := updated.Node("instance-app")
	if instance.SoftwareInstance == nil || len(instance.SoftwareInstance.Endpoints) == 0 || instance.SoftwareInstance.DeploymentMethod == "" {
		t.Fatalf("instance-specific fixture details missing: %+v", instance)
	}
	if len(updated.Assertions()) != len(batch.Assertions) || len(updated.AssertionsForNode("period-2026")) != 2 {
		t.Fatalf("field provenance assertions missing: %+v", updated.Assertions())
	}
	if len(updated.ProvisionalQueue()) != 1 {
		t.Fatalf("expected one pending provisional candidate, got %d", len(updated.ProvisionalQueue()))
	}
	if _, ok := updated.Node("unrelated-known"); !ok {
		t.Fatal("fixture must include a knowsAbout-only object")
	}
	for _, p := range updated.ImpactFromSelf() {
		if p.Node.ID == "unrelated-known" {
			t.Fatal("knowsAbout-only node leaked into impact analysis")
		}
	}
	repeated, repeatedPreview, err := updated.ApplySync(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(repeatedPreview.AddedNodes) != 0 || len(repeatedPreview.UpdatedNodes) != 0 || len(repeatedPreview.AddedRelations) != 0 || len(repeatedPreview.AddedAssertions) != 0 || len(repeatedPreview.UpdatedAssertions) != 0 || len(repeatedPreview.Conflicts) != 0 {
		t.Fatalf("identical fixture sync is not idempotent: %+v", repeatedPreview)
	}
	reviewed, err := repeated.ReviewProvisional("unclassified", graph.SoftwareInstance)
	if err != nil {
		t.Fatal(err)
	}
	candidate, _ := reviewed.Node("unclassified")
	if candidate.Type != graph.SoftwareInstance || candidate.Source != batch.Source {
		t.Fatalf("review did not preserve fixture provenance: %+v", candidate)
	}
}
