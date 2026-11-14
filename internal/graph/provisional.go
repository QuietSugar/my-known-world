package graph

import (
	"errors"
	"fmt"
)

// ProvisionalQueue returns pending classification candidates in stable ID order.
func (g *Graph) ProvisionalQueue() []Node {
	out := []Node{}
	for _, n := range g.Nodes() {
		if n.Type == ProvisionalObject && n.ReviewStatus == "pending" {
			out = append(out, n)
		}
	}
	return out
}

// ReviewProvisional classifies a pending candidate into an explicit stable type.
// The source, evidence, certainty, ID, and existing relations are retained; the
// operation is rejected if conversion would invalidate any relation endpoint.
func (g *Graph) ReviewProvisional(id string, target NodeType) (*Graph, error) {
	original, ok := g.Node(id)
	if !ok || original.Type != ProvisionalObject {
		return nil, errors.New("pending provisional object not found")
	}
	if original.ReviewStatus != "pending" {
		return nil, errors.New("provisional object is not pending review")
	}
	if target == "" {
		target = original.CandidateType
	}
	if target == "" || !nodeTypes[target] || target == Self || target == ProvisionalObject {
		return nil, fmt.Errorf("invalid stable target type %q", target)
	}
	candidate := New()
	for _, n := range g.Nodes() {
		if n.ID == id {
			n.Type = target
			n.CandidateType = ""
			n.ReviewStatus = ""
			n.ReviewQuestions = nil
		}
		if err := candidate.AddNode(n); err != nil {
			return nil, err
		}
	}
	for _, r := range g.Relations() {
		if err := candidate.AddRelation(r); err != nil {
			return nil, fmt.Errorf("review would invalidate relation %q: %w", r.ID, err)
		}
	}
	for _, a := range g.Assertions() {
		if err := candidate.AddAssertion(a); err != nil {
			return nil, fmt.Errorf("review would invalidate assertion %q: %w", a.ID, err)
		}
	}
	for _, n := range candidate.Nodes() {
		if _, ok := candidate.PathFromSelf(n.ID); !ok {
			return nil, fmt.Errorf("review would disconnect %q", n.ID)
		}
	}
	return candidate, nil
}

// RejectProvisional retains the record and provenance while closing its review.
func (g *Graph) RejectProvisional(id string) (*Graph, error) {
	original, ok := g.Node(id)
	if !ok || original.Type != ProvisionalObject || original.ReviewStatus != "pending" {
		return nil, errors.New("pending provisional object not found")
	}
	candidate := New()
	for _, n := range g.Nodes() {
		if n.ID == id {
			n.ReviewStatus = "rejected"
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
		if err := candidate.AddAssertion(a); err != nil {
			return nil, err
		}
	}
	return candidate, nil
}
