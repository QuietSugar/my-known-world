package graph

import "testing"

func TestNetworkDetailsValidateCIDRAndType(t *testing.T) {
	g := New()
	if err := g.AddNode(Node{ID: "n", Type: Network, Name: "Subnet", ModelVersion: 1, Network: &NetworkDetails{Kind: "subnet", CIDRs: []string{"192.0.2.0/24"}}}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(Node{ID: "bad", Type: Network, Name: "Bad", ModelVersion: 1, Network: &NetworkDetails{CIDRs: []string{"not-a-cidr"}}}); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
	if err := g.AddNode(Node{ID: "wrong", Type: Machine, Name: "Wrong details", ModelVersion: 1, Network: &NetworkDetails{Kind: "vpn"}}); err == nil {
		t.Fatal("network details accepted for another node type")
	}
}
func TestNetworkRelationSemanticsAndConnectivity(t *testing.T) {
	g := New()
	for _, n := range []Node{{ID: "self", Type: Self, Name: "Self", ModelVersion: 1}, {ID: "n1", Type: Network, Name: "Home", ModelVersion: 1}, {ID: "n2", Type: Network, Name: "VPN", ModelVersion: 1}, {ID: "host", Type: Machine, Name: "Host", ModelVersion: 1}} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range []Relation{{ID: "uses", From: "self", Predicate: Uses, To: "n1"}, {ID: "route", From: "n1", Predicate: RoutesThrough, To: "n2"}, {ID: "access", From: "n2", Predicate: ProvidesAccessTo, To: "host"}} {
		if err := g.AddRelation(r); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := g.PathFromSelf("host"); !ok {
		t.Fatal("network route should connect host to Self")
	}
	impact := g.ImpactFromSelf()
	if len(impact) != 3 {
		t.Fatalf("network access chain should appear in impact: %+v", impact)
	}
}
