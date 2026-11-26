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
	Kind           string                     `json:"kind"`
	Node           *Node                      `json:"node,omitempty"`
	Relation       *Relation                  `json:"relation,omitempty"`
	ID             string                     `json:"id,omitempty"`
	TargetType     NodeType                   `json:"targetType,omitempty"`
	ReviewEvidence *ProvisionalReviewEvidence `json:"reviewEvidence,omitempty"`
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
		existing, ok := candidate.Node(m.ID)
		if !ok {
			return nil, fmt.Errorf("node %q not found", m.ID)
		}
		merged, err := mergeNodeDetail(existing, *m.Node)
		if err != nil {
			return nil, err
		}
		candidate, err = candidate.UpdateNode(m.ID, merged)
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
	case "updateRelation":
		if m.Relation == nil || m.ID == "" || m.Node != nil {
			return nil, errors.New("updateRelation requires id and relation only")
		}
		var err error
		candidate, err = candidate.UpdateRelation(m.ID, *m.Relation)
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
	case "reviewProvisional":
		if m.ID == "" || m.Node != nil || m.Relation != nil || m.TargetType == "" {
			return nil, errors.New("reviewProvisional requires id and targetType only")
		}
		evidence := ProvisionalReviewEvidence{}
		if m.ReviewEvidence != nil {
			evidence = *m.ReviewEvidence
		}
		var err error
		candidate, err = candidate.ReviewProvisionalWithEvidence(m.ID, m.TargetType, evidence)
		if err != nil {
			return nil, err
		}
	case "rejectProvisional":
		if m.ID == "" || m.Node != nil || m.Relation != nil || m.TargetType != "" || m.ReviewEvidence != nil {
			return nil, errors.New("rejectProvisional requires id only")
		}
		var err error
		candidate, err = candidate.RejectProvisional(m.ID)
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

// mergeNodeDetail overlays the fields a caller actually supplied onto an
// existing node's typed details.
//
// The managed-update surface lets callers send a partial nested object, e.g.
// only {"hardware": {"specifications": {"memory": "..."}}}. Passing that to
// UpdateNode directly replaces the whole detail struct, dropping every field
// the caller did not mention — on 2026-11-26 an edit to h618's memory silently
// erased model, macs, location and status while still reporting success.
//
// Only keys present in patch are applied, and nested maps merge key by key so
// that changing one specification keeps the others. Clearing a field requires
// sending an explicit empty value at the top level.
func mergeNodeDetail(current, patch Node) (Node, error) {
	merged := patch
	merged.ID = current.ID
	merged.Type = current.Type
	if merged.Name == "" {
		merged.Name = current.Name
	}
	if merged.Lifecycle == "" {
		merged.Lifecycle = current.Lifecycle
	}
	if merged.ModelVersion == 0 {
		merged.ModelVersion = current.ModelVersion
	}
	if merged.Source == "" {
		merged.Source = current.Source
	}
	if merged.EvidenceKind == "" {
		merged.EvidenceKind = current.EvidenceKind
	}
	if merged.Certainty == "" {
		merged.Certainty = current.Certainty
	}
	merged.Hardware = mergeDetailStruct(current.Hardware, patch.Hardware)
	merged.Machine = mergeDetailStruct(current.Machine, patch.Machine)
	merged.SoftwareInstance = mergeDetailStruct(current.SoftwareInstance, patch.SoftwareInstance)
	merged.SoftwareProject = mergeDetailStruct(current.SoftwareProject, patch.SoftwareProject)
	merged.Product = mergeDetailStruct(current.Product, patch.Product)
	merged.Document = mergeDetailStruct(current.Document, patch.Document)
	merged.Runbook = mergeDetailStruct(current.Runbook, patch.Runbook)
	merged.WorkItem = mergeDetailStruct(current.WorkItem, patch.WorkItem)
	merged.Network = mergeDetailStruct(current.Network, patch.Network)
	merged.AccountCredential = mergeDetailStruct(current.AccountCredential, patch.AccountCredential)
	return merged, nil
}

// mergeDetailStruct merges patch over current by JSON key, recursing into maps.
func mergeDetailStruct[T any](current, patch *T) *T {
	if patch == nil {
		return current
	}
	if current == nil {
		return patch
	}
	base, err := json.Marshal(current)
	if err != nil {
		return patch
	}
	overlay, err := json.Marshal(patch)
	if err != nil {
		return patch
	}
	var merged map[string]any
	if err = json.Unmarshal(base, &merged); err != nil {
		return patch
	}
	var changes map[string]any
	if err = json.Unmarshal(overlay, &changes); err != nil {
		return patch
	}
	applyDetailChanges(merged, changes)
	raw, err := json.Marshal(merged)
	if err != nil {
		return patch
	}
	out := new(T)
	if err = json.Unmarshal(raw, out); err != nil {
		return patch
	}
	return out
}

// applyDetailChanges writes non-empty patch keys into target, merging nested
// maps key by key instead of replacing them.
func applyDetailChanges(target, patch map[string]any) {
	for key, value := range patch {
		if detailValueEmpty(value) {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			if existing, ok := target[key].(map[string]any); ok {
				applyDetailChanges(existing, nested)
				continue
			}
		}
		target[key] = value
	}
}

func detailValueEmpty(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	}
	return false
}
