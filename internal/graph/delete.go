package graph

import (
	"errors"
	"fmt"
)

// DeleteNode explicitly removes one node and its directly attached relations
// and assertions. It never cascades through other nodes; if the remaining graph
// would contain a disconnected node, the operation is rejected atomically.
func (g *Graph) DeleteNode(id string) (*Graph, error) {
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
