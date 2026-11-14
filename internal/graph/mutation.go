package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
)

type ManualMutation struct {
	Kind     string    `json:"kind"`
	Node     *Node     `json:"node,omitempty"`
	Relation *Relation `json:"relation,omitempty"`
	ID       string    `json:"id,omitempty"`
}
type MutationDelta struct {
	AddedNodes        []string `json:"addedNodes"`
	UpdatedNodes      []string `json:"updatedNodes"`
	DeletedNodes      []string `json:"deletedNodes"`
	AddedRelations    []string `json:"addedRelations"`
	UpdatedRelations  []string `json:"updatedRelations"`
	DeletedRelations  []string `json:"deletedRelations"`
	AddedAssertions   []string `json:"addedAssertions"`
	DeletedAssertions []string `json:"deletedAssertions"`
}
type MutationPlan struct {
	ExpectedRevision string        `json:"expectedRevision"`
	PlanHash         string        `json:"planHash"`
	Delta            MutationDelta `json:"delta"`
}

func (m ManualMutation) PlanHashValue() (string, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (g *Graph) ApplyManualMutation(m ManualMutation) (*Graph, error) {
	candidate := g.Clone()
	switch m.Kind {
	case "createNode":
		if m.Node == nil || m.Relation == nil || m.ID != "" {
			return nil, errors.New("createNode requires node and relation only")
		}
		if err := candidate.Publish(*m.Node, *m.Relation); err != nil {
			return nil, err
		}
	case "updateNode":
		if m.Node == nil || m.ID == "" || m.Relation != nil {
			return nil, errors.New("updateNode requires id and node only")
		}
		var err error
		candidate, err = candidate.UpdateNode(m.ID, *m.Node)
		if err != nil {
			return nil, err
		}
	case "addRelation":
		if m.Relation == nil || m.Node != nil || m.ID != "" {
			return nil, errors.New("addRelation requires relation only")
		}
		var err error
		candidate, err = candidate.AddRelationChecked(*m.Relation)
		if err != nil {
			return nil, err
		}
	case "deleteRelation":
		if m.ID == "" || m.Node != nil || m.Relation != nil {
			return nil, errors.New("deleteRelation requires id only")
		}
		var err error
		candidate, err = candidate.DeleteRelation(m.ID)
		if err != nil {
			return nil, err
		}
	case "deleteNode":
		if m.ID == "" || m.Node != nil || m.Relation != nil {
			return nil, errors.New("deleteNode requires id only")
		}
		var err error
		candidate, err = candidate.DeleteNode(m.ID)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported manual mutation %q", m.Kind)
	}
	return candidate, nil
}
func DiffGraphs(before, after *Graph) MutationDelta {
	d := MutationDelta{AddedNodes: []string{}, UpdatedNodes: []string{}, DeletedNodes: []string{}, AddedRelations: []string{}, UpdatedRelations: []string{}, DeletedRelations: []string{}, AddedAssertions: []string{}, DeletedAssertions: []string{}}
	diff := func(old, new map[string]any, added, updated, deleted *[]string) {
		for id, v := range old {
			nv, ok := new[id]
			if !ok {
				*deleted = append(*deleted, id)
			} else if !reflect.DeepEqual(v, nv) {
				*updated = append(*updated, id)
			}
		}
		for id := range new {
			if _, ok := old[id]; !ok {
				*added = append(*added, id)
			}
		}
		sort.Strings(*added)
		sort.Strings(*updated)
		sort.Strings(*deleted)
	}
	oldN, newN := map[string]any{}, map[string]any{}
	for _, v := range before.Nodes() {
		oldN[v.ID] = v
	}
	for _, v := range after.Nodes() {
		newN[v.ID] = v
	}
	diff(oldN, newN, &d.AddedNodes, &d.UpdatedNodes, &d.DeletedNodes)
	oldR, newR := map[string]any{}, map[string]any{}
	for _, v := range before.Relations() {
		oldR[v.ID] = v
	}
	for _, v := range after.Relations() {
		newR[v.ID] = v
	}
	diff(oldR, newR, &d.AddedRelations, &d.UpdatedRelations, &d.DeletedRelations)
	oldA, newA := map[string]any{}, map[string]any{}
	for _, v := range before.Assertions() {
		oldA[v.ID] = v
	}
	for _, v := range after.Assertions() {
		newA[v.ID] = v
	}
	var ignored []string
	diff(oldA, newA, &d.AddedAssertions, &ignored, &d.DeletedAssertions)
	return d
}
