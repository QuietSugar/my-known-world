package graph

import (
	"errors"
	"fmt"
	"strings"
)

// Limits for one create batch. They keep a single transaction bounded; split a
// larger import into several batches instead of raising them silently.
const (
	MaxCreateNodes     = 500
	MaxCreateRelations = 500
)

// CreateNodeRequest publishes one node together with the relation that anchors
// it to Self. The project requires every published object to be connected, so a
// node is never created without that onboarding relation.
type CreateNodeRequest struct {
	Node   Node          `json:"node"`
	Anchor *CreateAnchor `json:"anchor,omitempty"`
}

// CreateAnchor optionally selects how the new node connects from Self. Empty
// fields fall back to the defaults used by Publish: the first Self node, the
// knowsAbout predicate, and an "anchor-<node id>" relation identifier.
type CreateAnchor struct {
	ID        string    `json:"id,omitempty"`
	Predicate Predicate `json:"predicate,omitempty"`
}

// CreateBatch is the atomic payload for the create API. Nodes and relations are
// applied in order, all or nothing.
type CreateBatch struct {
	Nodes     []CreateNodeRequest `json:"nodes,omitempty"`
	Relations []Relation          `json:"relations,omitempty"`
}

// CreateResult reports exactly what a batch created, plus the diff so callers
// can record or display it.
type CreateResult struct {
	NodeIDs     []string      `json:"nodeIds"`
	RelationIDs []string      `json:"relationIds"`
	Delta       MutationDelta `json:"delta"`
}

// ApplyCreateBatch returns the graph with the batch applied. The receiver is not
// modified; callers commit the returned graph only when the whole batch is valid.
func (g *Graph) ApplyCreateBatch(batch CreateBatch) (*Graph, CreateResult, error) {
	if len(batch.Nodes) == 0 && len(batch.Relations) == 0 {
		return nil, CreateResult{}, errors.New("batch must contain at least one node or relation")
	}
	if len(batch.Nodes) > MaxCreateNodes {
		return nil, CreateResult{}, fmt.Errorf("batch contains %d nodes; the limit is %d", len(batch.Nodes), MaxCreateNodes)
	}
	if len(batch.Relations) > MaxCreateRelations {
		return nil, CreateResult{}, fmt.Errorf("batch contains %d relations; the limit is %d", len(batch.Relations), MaxCreateRelations)
	}
	result := CreateResult{NodeIDs: []string{}, RelationIDs: []string{}}
	candidate := g.Clone()
	seenNodes := map[string]bool{}
	seenRelations := map[string]bool{}
	for _, request := range batch.Nodes {
		node := request.Node
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" {
			return nil, CreateResult{}, errors.New("every node requires an id")
		}
		if seenNodes[node.ID] {
			return nil, CreateResult{}, fmt.Errorf("node %q appears twice in the batch", node.ID)
		}
		seenNodes[node.ID] = true
		relation := Relation{
			ID:           "anchor-" + node.ID,
			Predicate:    KnowsAbout,
			To:           node.ID,
			Source:       node.Source,
			EvidenceKind: node.EvidenceKind,
			Certainty:    node.Certainty,
		}
		if request.Anchor != nil {
			if id := strings.TrimSpace(request.Anchor.ID); id != "" {
				relation.ID = id
			}
			if request.Anchor.Predicate != "" {
				relation.Predicate = request.Anchor.Predicate
			}
		}
		if seenRelations[relation.ID] {
			return nil, CreateResult{}, fmt.Errorf("relation %q appears twice in the batch", relation.ID)
		}
		seenRelations[relation.ID] = true
		if err := candidate.Publish(node, relation); err != nil {
			return nil, CreateResult{}, fmt.Errorf("node %q: %w", node.ID, err)
		}
		result.NodeIDs = append(result.NodeIDs, node.ID)
		result.RelationIDs = append(result.RelationIDs, relation.ID)
	}
	for _, relation := range batch.Relations {
		id := strings.TrimSpace(relation.ID)
		if id == "" {
			return nil, CreateResult{}, errors.New("every relation requires an id")
		}
		if seenRelations[id] {
			return nil, CreateResult{}, fmt.Errorf("relation %q appears twice in the batch", id)
		}
		seenRelations[id] = true
		updated, err := candidate.AddRelationChecked(relation)
		if err != nil {
			return nil, CreateResult{}, fmt.Errorf("relation %q: %w", id, err)
		}
		candidate = updated
		result.RelationIDs = append(result.RelationIDs, id)
	}
	result.Delta = DiffGraphs(g, candidate)
	return candidate, result, nil
}
