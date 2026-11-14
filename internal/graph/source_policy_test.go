package graph

import "testing"

func TestSourcePolicyAllowListsNodeRelationAndFieldScope(t *testing.T) {
	p := SourcePolicy{Source: "inventory", Priority: 50, AllowedNodeTypes: []NodeType{Machine}, AllowedPredicates: []Predicate{KnowsAbout}, AllowedFieldPaths: []string{"machine.hostname"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := p.ValidateBatch(SourceBatch{Source: "inventory", Nodes: []Node{{ID: "x", Type: Product}}, Relations: []Relation{{ID: "r", Predicate: Owns}}, Assertions: []FieldAssertion{{ID: "a", FieldPath: "machine.os"}}}); len(got) != 3 {
		t.Fatalf("expected 3 scoped violations, got %v", got)
	}
}
