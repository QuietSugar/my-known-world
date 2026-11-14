package graph

import "testing"

func TestProvisionalReviewPreservesProvenanceAndValidatesConversion(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	err := g.Publish(Node{ID: "candidate", Type: ProvisionalObject, Name: "Possibly a machine", Source: "private-import", EvidenceKind: EvidenceDocument, Certainty: CertaintyProvisional, ModelVersion: 1, CandidateType: Machine, ReviewQuestions: []string{"What OS?"}}, Relation{ID: "weak", To: "candidate", Source: "private-import"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(g.ProvisionalQueue()); got != 1 {
		t.Fatalf("pending queue size=%d", got)
	}
	reviewed, err := g.ReviewProvisional("candidate", Machine)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := reviewed.Node("candidate")
	if n.Type != Machine || n.Source != "private-import" || n.EvidenceKind != EvidenceDocument || n.Certainty != CertaintyProvisional || len(reviewed.ProvisionalQueue()) != 0 {
		t.Fatalf("review lost identity/provenance: %+v", n)
	}
	rejected, err := g.RejectProvisional("candidate")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := rejected.Node("candidate")
	if r.ReviewStatus != "rejected" || r.Source != "private-import" {
		t.Fatalf("rejection not retained: %+v", r)
	}
}

func TestProvisionalReviewRejectsInvalidRelationConversionAtomically(t *testing.T) {
	g := New()
	_ = g.AddNode(Node{ID: "self", Type: Self, Name: "Me", ModelVersion: 1})
	_ = g.Publish(Node{ID: "candidate", Type: ProvisionalObject, Name: "Unknown", ModelVersion: 1}, Relation{ID: "weak", To: "candidate"})
	_ = g.AddNode(Node{ID: "instance", Type: SoftwareInstance, Name: "Instance", ModelVersion: 1})
	_ = g.AddRelation(Relation{ID: "deploy", From: "candidate", Predicate: Hosts, To: "instance"})
	if _, err := g.ReviewProvisional("candidate", Product); err == nil {
		t.Fatal("expected incompatible endpoint conversion to fail")
	}
	n, _ := g.Node("candidate")
	if n.Type != ProvisionalObject {
		t.Fatal("rejected review mutated original graph")
	}
}
