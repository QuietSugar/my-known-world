package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// TestPatchMergesNestedDetails guards the fix for a real data-loss incident:
// patching h618's memory with a partial hardware object erased model, macs,
// location and status because the patch replaced the struct wholesale.
func TestPatchMergesNestedDetails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	const hwID = "e687d73a-e536-4e94-b36c-e35582a2ff6d"
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	orig := graph.Node{ID: hwID, Type: graph.Hardware, Name: "h618", Lifecycle: "active",
		Source: "test", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1,
		Hardware: &graph.HardwareDetails{
			Model:    "PCDN (ARM64 SBC)",
			Location: "fixed@home",
			Status:   "active",
			MACs:     []string{"02:00:ad:ae:29:f2"},
			Specifications: map[string]string{"cpu": "Cortex-A53", "memory": "4 GB"},
		}}
	if err := g.Publish(orig, graph.Relation{ID: "anchor-hw", Predicate: graph.KnowsAbout, To: hwID,
		Source: "test", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rev, err := loaded.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}

	// 只传 specifications.memory，其余字段不应被抹掉
	patch := graph.Node{Hardware: &graph.HardwareDetails{Specifications: map[string]string{"memory": "3.8 GiB"}}}
	patched, _, err := store.PatchNode(ctx, hwID, patch, rev)
	if err != nil {
		t.Fatal(err)
	}
	hw := patched.Hardware
	if hw.Model != "PCDN (ARM64 SBC)" {
		t.Errorf("model lost: %q", hw.Model)
	}
	if hw.Location != "fixed@home" {
		t.Errorf("location lost: %q", hw.Location)
	}
	if len(hw.MACs) != 1 {
		t.Errorf("macs lost: %v", hw.MACs)
	}
	if hw.Specifications["cpu"] != "Cortex-A53" {
		t.Errorf("cpu spec lost: %v", hw.Specifications)
	}
	if hw.Specifications["memory"] != "3.8 GiB" {
		t.Errorf("memory not updated: %v", hw.Specifications)
	}
}
