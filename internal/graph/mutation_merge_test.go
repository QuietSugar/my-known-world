package graph

import "testing"

// TestUpdateNodeMergesPartialDetail guards the managed-update path used by
// graph_patch_node and the Web preview/apply flow. Sending only one nested
// field must not erase the others; on 2026-11-26 an edit to h618's memory
// silently dropped model, macs, location and status.
func TestUpdateNodeMergesPartialDetail(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	hw := Node{ID: "hw", Type: Hardware, Name: "box", Lifecycle: "active", ModelVersion: 1,
		Hardware: &HardwareDetails{
			Model: "PCDN", Location: "fixed@home", Status: "active",
			MACs:           []string{"02:00:ad:ae:29:f2"},
			Specifications: map[string]string{"cpu": "Cortex-A53", "memory": "4 GB"},
		}}
	if err := g.Publish(hw, Relation{ID: "a", Predicate: KnowsAbout, To: "hw"}); err != nil {
		t.Fatal(err)
	}

	patch := Node{ID: "hw", Type: Hardware, Name: "box", ModelVersion: 1,
		Hardware: &HardwareDetails{Specifications: map[string]string{"memory": "32 GiB"}}}
	next, err := g.ApplyManualMutation(ManualMutation{Kind: "updateNode", ID: "hw", Node: &patch})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := next.Node("hw")
	hwOut := got.Hardware
	if hwOut.Model != "PCDN" {
		t.Errorf("model lost: %q", hwOut.Model)
	}
	if hwOut.Location != "fixed@home" {
		t.Errorf("location lost: %q", hwOut.Location)
	}
	if len(hwOut.MACs) != 1 {
		t.Errorf("macs lost: %v", hwOut.MACs)
	}
	if hwOut.Specifications["cpu"] != "Cortex-A53" {
		t.Errorf("cpu spec lost: %v", hwOut.Specifications)
	}
	if hwOut.Specifications["memory"] != "32 GiB" {
		t.Errorf("memory not updated: %v", hwOut.Specifications)
	}
}
