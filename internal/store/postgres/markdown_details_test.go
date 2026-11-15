package postgres

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"reflect"
	"testing"
)

func TestMarkdownTypedListAndMachineKindDetailsPersistWithoutSchemaRebuild(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.Publish(graph.Node{ID: "machine/vm.md", Type: graph.Machine, Name: "Example VM", ModelVersion: 1, Machine: &graph.MachineDetails{MachineKind: "shared", IPs: []string{"192.0.2.1"}, OperationConstraints: []string{"read-only"}}}, graph.Relation{ID: "host-knows", Predicate: graph.KnowsAbout, To: "machine/vm.md"}); err != nil {
		t.Fatal(err)
	}
	if err := g.Publish(graph.Node{ID: "service/api.md", Type: graph.SoftwareInstance, Name: "Example service", ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{Domains: []string{"one.example.test", "two.example.test"}, DeploymentPath: "/srv/demo"}}, graph.Relation{ID: "service-knows", Predicate: graph.KnowsAbout, To: "service/api.md"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	machine, _ := loaded.Node("machine/vm.md")
	if machine.Machine == nil || machine.Machine.MachineKind != "shared" || !reflect.DeepEqual(machine.Machine.OperationConstraints, []string{"read-only"}) {
		t.Fatalf("machine details lost: %+v", machine.Machine)
	}
	service, _ := loaded.Node("service/api.md")
	if service.SoftwareInstance == nil || !reflect.DeepEqual(service.SoftwareInstance.Domains, []string{"one.example.test", "two.example.test"}) {
		t.Fatalf("domain list lost: %+v", service.SoftwareInstance)
	}
}
