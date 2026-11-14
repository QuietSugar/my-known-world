package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSyncPreviewApplyIsConservativeAndRepeatable(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := g.Publish(Node{ID: "old", Type: Machine, Name: "Old", Source: "inventory", ModelVersion: 1}, Relation{ID: "old-link", To: "old", Source: "inventory"}); err != nil {
		t.Fatal(err)
	}
	batch := SourceBatch{Source: "inventory", Nodes: []Node{{ID: "new", Type: Machine, Name: "New", ModelVersion: 1}}, Relations: []Relation{}}
	preview := g.PreviewSync(batch)
	if len(preview.AddedNodes) != 1 || preview.AddedNodes[0] != "new" || len(preview.MissingNodes) != 1 || preview.MissingNodes[0] != "old" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if _, ok := g.Node("new"); ok {
		t.Fatal("preview mutated graph")
	}
	updated, p, err := g.ApplySync(batch)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.MissingNodes) != 1 {
		t.Fatalf("missing report lost: %+v", p)
	}
	old, _ := updated.Node("old")
	if old.Certainty != CertaintyStale {
		t.Fatalf("missing node not marked stale: %+v", old)
	}
	if _, ok := updated.PathFromSelf("new"); !ok {
		t.Fatal("new node was not weakly connected")
	}
	repeated := updated.PreviewSync(batch)
	if len(repeated.AddedNodes) != 0 || len(repeated.UpdatedNodes) != 0 {
		t.Fatalf("identical re-sync should be idempotent: %+v", repeated)
	}
	if len(g.Nodes()) != 2 {
		t.Fatal("ApplySync mutated original graph")
	}
}

func TestSyncRenameMappingPreservesCanonicalIdentity(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	_ = g.Publish(Node{ID: "stable-id", Type: Machine, Name: "Before", Source: "inventory", ModelVersion: 1}, Relation{ID: "knows-stable", To: "stable-id", Source: "inventory"})
	batch := SourceBatch{Source: "inventory", IdentityMappings: []IdentityMapping{{SourceID: "renamed-id", NodeID: "stable-id"}}, Nodes: []Node{{ID: "renamed-id", Type: Machine, Name: "After", ModelVersion: 1}}}
	preview := g.PreviewSync(batch)
	if len(preview.AddedNodes) != 0 || len(preview.UpdatedNodes) != 1 || preview.UpdatedNodes[0] != "stable-id" || len(preview.MissingNodes) != 0 {
		t.Fatalf("rename was treated as add/missing: %+v", preview)
	}
	updated, _, err := g.ApplySync(batch)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updated.Node("renamed-id"); ok {
		t.Fatal("rename replaced stable graph identity")
	}
	n, _ := updated.Node("stable-id")
	if n.Name != "After" {
		t.Fatalf("mapped node wasn't updated: %+v", n)
	}
}

func TestSyncPreviewReportsCrossSourceFieldConflictWithoutValuesOrHashes(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Self", ModelVersion: 1})
	_ = g.AddNode(Node{ID: "m", Type: Machine, Name: "Machine", ModelVersion: 1})
	existing := FieldAssertion{ID: "a1", NodeID: "m", FieldPath: "machine.hostname", ValueHash: strings.Repeat("a", 64), Source: "inventory-a", EvidenceKind: EvidenceObserved, Certainty: CertaintyConfirmed}
	if err := g.AddAssertion(existing); err != nil {
		t.Fatal(err)
	}
	batch := SourceBatch{Source: "inventory-b", Assertions: []FieldAssertion{{ID: "a2", NodeID: "m", FieldPath: "machine.hostname", ValueHash: strings.Repeat("b", 64), EvidenceKind: EvidenceObserved, Certainty: CertaintyConfirmed}}}
	preview := g.PreviewSync(batch)
	if len(preview.ValueConflicts) != 1 {
		t.Fatalf("expected field disagreement: %+v", preview)
	}
	c := preview.ValueConflicts[0]
	if c.NodeID != "m" || c.FieldPath != "machine.hostname" || len(c.Sources) != 2 || len(c.AssertionIDs) != 2 {
		t.Fatalf("incorrect conflict metadata: %+v", c)
	}
	raw, _ := json.Marshal(preview)
	if strings.Contains(string(raw), strings.Repeat("a", 64)) || strings.Contains(string(raw), strings.Repeat("b", 64)) {
		t.Fatal("preview exposed assertion hashes")
	}
}

func TestSyncHashIsStableAcrossInputOrdering(t *testing.T) {
	a := SourceBatch{Source: "inventory", ParserVersion: "v1", Nodes: []Node{{ID: "b", Type: Machine, Name: "B", ModelVersion: 1}, {ID: "a", Type: Machine, Name: "A", ModelVersion: 1}}}
	b := SourceBatch{Source: "inventory", ParserVersion: "v1", Nodes: []Node{{ID: "a", Type: Machine, Name: "A", ModelVersion: 1}, {ID: "b", Type: Machine, Name: "B", ModelVersion: 1}}}
	g := New()
	pa := g.PreviewSync(a)
	pb := g.PreviewSync(b)
	if pa.SourceHash == "" || pa.SourceHash != pb.SourceHash {
		t.Fatalf("hash should be deterministic: %q vs %q", pa.SourceHash, pb.SourceHash)
	}
	if pa.ParserVersion != "v1" {
		t.Fatalf("parser version not reported: %+v", pa)
	}
}

func TestSyncConflictsAreAtomic(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	_ = g.Publish(Node{ID: "id", Type: Machine, Name: "Host", Source: "manual", ModelVersion: 1}, Relation{ID: "knows", To: "id", Source: "manual"})
	candidate, preview, err := g.ApplySync(SourceBatch{Source: "other", Nodes: []Node{{ID: "id", Type: Machine, Name: "Changed", ModelVersion: 1}}})
	if err == nil || candidate != nil || len(preview.Conflicts) == 0 {
		t.Fatalf("expected source ownership conflict, got graph=%v preview=%+v err=%v", candidate, preview, err)
	}
	original, _ := g.Node("id")
	if original.Name != "Host" {
		t.Fatal("conflicted sync changed graph")
	}
}
