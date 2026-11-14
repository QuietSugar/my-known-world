package graph

import (
	"testing"
	"time"
)

func TestBuildAssertionViewsUsesLatestExplicitResolutionAndHidesHash(t *testing.T) {
	a := []FieldAssertion{{ID: "a1", NodeID: "n", FieldPath: "machine.hostname", ValueHash: "hidden", Source: "source-a"}, {ID: "a2", NodeID: "n", FieldPath: "machine.hostname", ValueHash: "hidden-too", Source: "source-b"}}
	r := []FieldResolution{{ID: 2, NodeID: "n", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"a2"}, RecordedAt: time.Now()}, {ID: 1, NodeID: "n", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"a1"}, RecordedAt: time.Now().Add(-time.Hour)}}
	views := BuildAssertionViews(a, r)
	if len(views) != 2 || views[0].Selected || !views[1].Selected || !views[0].Resolved || views[1].ResolutionID == nil || *views[1].ResolutionID != 2 {
		t.Fatalf("wrong effective assertion view: %+v", views)
	}
}
