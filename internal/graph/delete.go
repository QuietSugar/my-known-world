package graph

import (
	"errors"
	"fmt"
)

// DeleteNode retires a node instead of removing it.
//
// Deleting business data physically is destructive: the row, its relations and
// its field assertions all disappear, and anything that referenced the node —
// external notes, credentials, other systems — silently breaks. Removal is
// therefore expressed as retirement: the node keeps its identity and relations
// but is marked lifecycle=retired, so it stops appearing as current while the
// history stays intact and the decision can be reversed.
//
// purgeNode remains available for the rare case where a row must really go
// (test fixtures, mistaken imports). It is deliberately not reachable from the
// API or MCP surface.
func (g *Graph) DeleteNode(id string) (*Graph, error) {
	return g.RetireNode(id)
}

// PurgeNode removes one node and its directly attached relations and assertions.
// It never cascades through other nodes; if the remaining graph would contain a
// disconnected node, the operation is rejected atomically.
func (g *Graph) PurgeNode(id string) (*Graph, error) {
	n, ok := g.Node(id)
	if !ok {
		return nil, errors.New("node not found")
	}
	if n.Type == Self {
		return nil, errors.New("Self cannot be deleted")
	}
	next := New()
	for _, node := range g.Nodes() {
		if node.ID == id {
			continue
		}
		if err := next.AddNode(node); err != nil {
			return nil, err
		}
	}
	for _, rel := range g.Relations() {
		if rel.From == id || rel.To == id {
			continue
		}
		if err := next.AddRelation(rel); err != nil {
			return nil, err
		}
	}
	for _, a := range g.Assertions() {
		if a.NodeID == id {
			continue
		}
		if err := next.AddAssertion(a); err != nil {
			return nil, err
		}
	}
	for _, node := range next.Nodes() {
		if _, ok := next.PathFromSelf(node.ID); !ok {
			return nil, fmt.Errorf("deleting %q would disconnect node %q", id, node.ID)
		}
	}
	return next, nil
}
