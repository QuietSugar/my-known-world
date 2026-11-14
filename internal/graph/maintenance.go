package graph

import (
	"errors"
	"fmt"
)

// UpdateNode replaces editable attributes while preserving a node's stable ID,
// type and source ownership. Classification changes must use provisional review.
func (g *Graph) UpdateNode(id string, updated Node) (*Graph, error) {
	old, ok := g.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %q not found", id)
	}
	if updated.ID != id {
		return nil, errors.New("node ID is immutable")
	}
	if updated.Type != old.Type {
		return nil, errors.New("node type is immutable; use provisional review")
	}
	updated.Source = old.Source
	candidate := New()
	for _, n := range g.Nodes() {
		if n.ID == id {
			n = updated
		}
		if err := candidate.AddNode(n); err != nil {
			return nil, err
		}
	}
	for _, r := range g.Relations() {
		if err := candidate.AddRelation(r); err != nil {
			return nil, err
		}
	}
	for _, a := range g.Assertions() {
		candidate.assertions[a.ID] = a
	}
	if err := ensureConnected(candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (g *Graph) AddRelationChecked(relation Relation) (*Graph, error) {
	candidate := cloneGraph(g)
	if err := candidate.AddRelation(relation); err != nil {
		return nil, err
	}
	if err := ensureConnected(candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (g *Graph) DeleteRelation(id string) (*Graph, error) {
	if _, ok := g.relations[id]; !ok {
		return nil, fmt.Errorf("relation %q not found", id)
	}
	candidate := New()
	for _, n := range g.Nodes() {
		if err := candidate.AddNode(n); err != nil {
			return nil, err
		}
	}
	for _, r := range g.Relations() {
		if r.ID == id {
			continue
		}
		if err := candidate.AddRelation(r); err != nil {
			return nil, err
		}
	}
	for _, a := range g.Assertions() {
		candidate.assertions[a.ID] = a
	}
	if err := ensureConnected(candidate); err != nil {
		return nil, err
	}
	return candidate, nil
}

func (g *Graph) Clone() *Graph { return cloneGraph(g) }

func cloneGraph(g *Graph) *Graph {
	candidate := New()
	for _, n := range g.Nodes() {
		_ = candidate.AddNode(n)
	}
	for _, r := range g.Relations() {
		_ = candidate.AddRelation(r)
	}
	for _, a := range g.Assertions() {
		candidate.assertions[a.ID] = a
	}
	return candidate
}
func ensureConnected(g *Graph) error {
	for _, n := range g.Nodes() {
		if _, ok := g.PathFromSelf(n.ID); !ok {
			return fmt.Errorf("mutation would disconnect node %q from Self", n.ID)
		}
	}
	return nil
}
