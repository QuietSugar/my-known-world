package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// SourceBatch is one complete observation from a named, authoritative source.
// Records absent from the batch are reported as missing, never deleted.
type IdentityMapping struct {
	SourceID string `json:"sourceId"`
	NodeID   string `json:"nodeId"`
}

type SourceBatch struct {
	Source           string            `json:"source"`
	SourceHash       string            `json:"sourceHash,omitempty"`
	ParserVersion    string            `json:"parserVersion,omitempty"`
	IdentityMappings []IdentityMapping `json:"identityMappings,omitempty"`
	Nodes            []Node            `json:"nodes"`
	Relations        []Relation        `json:"relations"`
	Assertions       []FieldAssertion  `json:"assertions,omitempty"`
}

type SyncSourceState struct {
	Source        string    `json:"source"`
	SourceHash    string    `json:"sourceHash"`
	ParserVersion string    `json:"parserVersion"`
	AppliedAt     time.Time `json:"appliedAt"`
}
type SyncRun struct {
	ID            int64       `json:"id"`
	Source        string      `json:"source"`
	SourceHash    string      `json:"sourceHash"`
	ParserVersion string      `json:"parserVersion"`
	AppliedAt     time.Time   `json:"appliedAt"`
	Preview       SyncPreview `json:"preview"`
	Undone        bool        `json:"undone"`
}

type FieldConflict struct {
	NodeID       string   `json:"nodeId"`
	FieldPath    string   `json:"fieldPath"`
	AssertionIDs []string `json:"assertionIds"`
	Sources      []string `json:"sources"`
}

type SyncPreview struct {
	Source            string          `json:"source"`
	SourceHash        string          `json:"sourceHash"`
	ParserVersion     string          `json:"parserVersion,omitempty"`
	AddedNodes        []string        `json:"addedNodes"`
	UpdatedNodes      []string        `json:"updatedNodes"`
	AddedRelations    []string        `json:"addedRelations"`
	UpdatedRels       []string        `json:"updatedRelations"`
	MissingNodes      []string        `json:"missingNodes"`
	MissingRels       []string        `json:"missingRelations"`
	Conflicts         []string        `json:"conflicts"`
	AddedAssertions   []string        `json:"addedAssertions"`
	UpdatedAssertions []string        `json:"updatedAssertions"`
	MissingAssertions []string        `json:"missingAssertions"`
	ValueConflicts    []FieldConflict `json:"valueConflicts"`
}

func canonicalizeBatch(batch SourceBatch) (SourceBatch, error) {
	mapping := map[string]string{}
	for _, m := range batch.IdentityMappings {
		if m.SourceID == "" || m.NodeID == "" || m.SourceID == m.NodeID {
			return batch, errors.New("identity mapping requires distinct sourceId and nodeId")
		}
		if _, exists := mapping[m.SourceID]; exists {
			return batch, fmt.Errorf("duplicate identity mapping for %q", m.SourceID)
		}
		mapping[m.SourceID] = m.NodeID
	}
	for i := range batch.Nodes {
		if id, ok := mapping[batch.Nodes[i].ID]; ok {
			batch.Nodes[i].ID = id
		}
	}
	for i := range batch.Assertions {
		if id, ok := mapping[batch.Assertions[i].NodeID]; ok {
			batch.Assertions[i].NodeID = id
		}
	}
	for i := range batch.Relations {
		if id, ok := mapping[batch.Relations[i].From]; ok {
			batch.Relations[i].From = id
		}
		if id, ok := mapping[batch.Relations[i].To]; ok {
			batch.Relations[i].To = id
		}
	}
	return batch, nil
}

func batchHash(batch SourceBatch) string {
	batch.SourceHash = ""
	batch.Nodes = append([]Node(nil), batch.Nodes...)
	batch.Relations = append([]Relation(nil), batch.Relations...)
	batch.Assertions = append([]FieldAssertion(nil), batch.Assertions...)
	batch.IdentityMappings = append([]IdentityMapping(nil), batch.IdentityMappings...)
	sort.Slice(batch.IdentityMappings, func(i, j int) bool { return batch.IdentityMappings[i].SourceID < batch.IdentityMappings[j].SourceID })
	sort.Slice(batch.Nodes, func(i, j int) bool { return batch.Nodes[i].ID < batch.Nodes[j].ID })
	sort.Slice(batch.Relations, func(i, j int) bool { return batch.Relations[i].ID < batch.Relations[j].ID })
	sort.Slice(batch.Assertions, func(i, j int) bool { return batch.Assertions[i].ID < batch.Assertions[j].ID })
	raw, _ := json.Marshal(batch)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// PreviewSync compares a source's complete batch with the current graph without
// changing either. IDs are the source's stable identity mapping.
func (g *Graph) PreviewSync(batch SourceBatch) SyncPreview {
	source := strings.TrimSpace(batch.Source)
	p := SyncPreview{Source: source, ParserVersion: batch.ParserVersion, SourceHash: batch.SourceHash, AddedNodes: []string{}, UpdatedNodes: []string{}, AddedRelations: []string{}, UpdatedRels: []string{}, MissingNodes: []string{}, MissingRels: []string{}, Conflicts: []string{}, AddedAssertions: []string{}, UpdatedAssertions: []string{}, MissingAssertions: []string{}, ValueConflicts: []FieldConflict{}}
	if p.SourceHash == "" {
		p.SourceHash = batchHash(batch)
	}
	canonical, mapErr := canonicalizeBatch(batch)
	if mapErr != nil {
		p.Conflicts = append(p.Conflicts, mapErr.Error())
		return p
	}
	batch = canonical
	if source == "" {
		p.Conflicts = append(p.Conflicts, "source is required")
		return p
	}
	nodeIDs, relationIDs := map[string]bool{}, map[string]bool{}
	for _, n := range batch.Nodes {
		if n.ID == "" {
			p.Conflicts = append(p.Conflicts, "node with empty id")
			continue
		}
		nodeIDs[n.ID] = true
		n.Source = source
		n.Source, n.EvidenceKind, n.Certainty, _ = normalizeEvidence(n.Source, n.EvidenceKind, n.Certainty)
		if old, ok := g.nodes[n.ID]; !ok {
			p.AddedNodes = append(p.AddedNodes, n.ID)
		} else if old.Source != source || old.Type != n.Type {
			p.Conflicts = append(p.Conflicts, fmt.Sprintf("node %q identity/type conflict", n.ID))
		} else if old.ID != n.ID || old.Type != n.Type || old.Name != n.Name || old.Lifecycle != n.Lifecycle || old.EvidenceKind != n.EvidenceKind || old.Certainty != n.Certainty || old.ModelVersion != n.ModelVersion || old.CandidateType != n.CandidateType || old.ReviewStatus != n.ReviewStatus || !reflect.DeepEqual(old.ReviewQuestions, n.ReviewQuestions) || !sameTime(old.LastConfirmed, n.LastConfirmed) || !sameTime(old.StartedAt, n.StartedAt) || !sameTime(old.EndedAt, n.EndedAt) || old.DeployedVersion != n.DeployedVersion || !sameTime(old.VerificationAt, n.VerificationAt) || old.VerificationResult != n.VerificationResult || old.EvidenceRef != n.EvidenceRef || !reflect.DeepEqual(old.Hardware, n.Hardware) || !reflect.DeepEqual(old.Machine, n.Machine) || !reflect.DeepEqual(old.SoftwareProject, n.SoftwareProject) || !reflect.DeepEqual(old.SoftwareInstance, n.SoftwareInstance) || !reflect.DeepEqual(old.AccountCredential, n.AccountCredential) || !reflect.DeepEqual(old.Product, n.Product) || !reflect.DeepEqual(old.Document, n.Document) || !reflect.DeepEqual(old.Runbook, n.Runbook) || !reflect.DeepEqual(old.WorkItem, n.WorkItem) || !reflect.DeepEqual(old.Network, n.Network) {
			p.UpdatedNodes = append(p.UpdatedNodes, n.ID)
		}
	}
	for _, r := range batch.Relations {
		if r.ID == "" {
			p.Conflicts = append(p.Conflicts, "relation with empty id")
			continue
		}
		relationIDs[r.ID] = true
		r.Source = source
		r.Source, r.EvidenceKind, r.Certainty, _ = normalizeEvidence(r.Source, r.EvidenceKind, r.Certainty)
		if old, ok := g.relations[r.ID]; !ok {
			p.AddedRelations = append(p.AddedRelations, r.ID)
		} else if old.Source != source || old.From != r.From || old.To != r.To || old.Predicate != r.Predicate {
			p.Conflicts = append(p.Conflicts, fmt.Sprintf("relation %q identity/endpoints conflict", r.ID))
		} else if old.EvidenceKind != r.EvidenceKind || old.Certainty != r.Certainty || (!r.RecordedAt.IsZero() && !old.RecordedAt.Equal(r.RecordedAt)) {
			p.UpdatedRels = append(p.UpdatedRels, r.ID)
		}
	}
	assertionIDs := map[string]bool{}
	for _, a := range batch.Assertions {
		if a.ID == "" {
			p.Conflicts = append(p.Conflicts, "assertion with empty id")
			continue
		}
		assertionIDs[a.ID] = true
		a.Source, a.EvidenceKind, a.Certainty, _ = normalizeEvidence(source, a.EvidenceKind, a.Certainty)
		if old, ok := g.assertions[a.ID]; !ok {
			p.AddedAssertions = append(p.AddedAssertions, a.ID)
		} else if old.Source != source || old.NodeID != a.NodeID || old.FieldPath != a.FieldPath {
			p.Conflicts = append(p.Conflicts, fmt.Sprintf("assertion %q identity conflict", a.ID))
		} else if old.ValueHash != a.ValueHash || old.Reference != a.Reference || old.EvidenceKind != a.EvidenceKind || old.Certainty != a.Certainty {
			p.UpdatedAssertions = append(p.UpdatedAssertions, a.ID)
		}
	}
	// Surface cross-source disagreements without returning field hashes or values.
	byField := map[string][]FieldAssertion{}
	for _, a := range g.Assertions() {
		byField[a.NodeID+"\x00"+a.FieldPath] = append(byField[a.NodeID+"\x00"+a.FieldPath], a)
	}
	for _, a := range batch.Assertions {
		a.Source = source
		key := a.NodeID + "\x00" + a.FieldPath
		replaced := false
		for i, old := range byField[key] {
			if old.ID == a.ID {
				byField[key][i] = a
				replaced = true
				break
			}
		}
		if !replaced {
			byField[key] = append(byField[key], a)
		}
	}
	for key, items := range byField {
		different := false
		for i := 1; i < len(items); i++ {
			if items[i].ValueHash != items[0].ValueHash || items[i].Reference != items[0].Reference {
				different = true
				break
			}
		}
		if !different {
			continue
		}
		parts := strings.SplitN(key, "\x00", 2)
		conflict := FieldConflict{NodeID: parts[0], FieldPath: parts[1], AssertionIDs: []string{}, Sources: []string{}}
		idSet, sourceSet := map[string]bool{}, map[string]bool{}
		for _, a := range items {
			idSet[a.ID] = true
			sourceSet[a.Source] = true
		}
		for id := range idSet {
			conflict.AssertionIDs = append(conflict.AssertionIDs, id)
		}
		for src := range sourceSet {
			conflict.Sources = append(conflict.Sources, src)
		}
		sort.Strings(conflict.AssertionIDs)
		sort.Strings(conflict.Sources)
		if len(conflict.Sources) > 1 {
			p.ValueConflicts = append(p.ValueConflicts, conflict)
		}
	}
	sort.Slice(p.ValueConflicts, func(i, j int) bool {
		if p.ValueConflicts[i].NodeID == p.ValueConflicts[j].NodeID {
			return p.ValueConflicts[i].FieldPath < p.ValueConflicts[j].FieldPath
		}
		return p.ValueConflicts[i].NodeID < p.ValueConflicts[j].NodeID
	})
	for id, a := range g.assertions {
		if a.Source == source && !assertionIDs[id] {
			p.MissingAssertions = append(p.MissingAssertions, id)
		}
	}
	for id, n := range g.nodes {
		if n.Source == source && n.Type != Self && !nodeIDs[id] {
			p.MissingNodes = append(p.MissingNodes, id)
		}
	}
	for id, r := range g.relations {
		if r.Source == source && !relationIDs[id] {
			p.MissingRels = append(p.MissingRels, id)
		}
	}
	for _, ids := range [][]string{p.AddedNodes, p.UpdatedNodes, p.AddedRelations, p.UpdatedRels, p.MissingNodes, p.MissingRels, p.Conflicts, p.AddedAssertions, p.UpdatedAssertions, p.MissingAssertions} {
		sort.Strings(ids)
	}
	return p
}

// ApplySync validates and applies a source batch atomically, returning a new
// graph. Missing records are marked stale; they are never removed. New objects
// get a weak Self knowsAbout link unless the batch explicitly connects them.
func (g *Graph) ApplySync(batch SourceBatch) (*Graph, SyncPreview, error) {
	preview := g.PreviewSync(batch)
	if len(preview.Conflicts) != 0 {
		return nil, preview, fmt.Errorf("sync has conflicts: %s", strings.Join(preview.Conflicts, "; "))
	}
	if strings.TrimSpace(batch.Source) == "" {
		return nil, preview, errors.New("source is required")
	}
	batch, err := canonicalizeBatch(batch)
	if err != nil {
		return nil, preview, err
	}
	candidate := New()
	for _, n := range g.Nodes() {
		if err := candidate.AddNode(n); err != nil {
			return nil, preview, err
		}
	}
	for _, r := range g.Relations() {
		if err := candidate.AddRelation(r); err != nil {
			return nil, preview, err
		}
	}
	for _, a := range g.Assertions() {
		candidate.assertions[a.ID] = a
	}
	for _, n := range batch.Nodes {
		n.Source = batch.Source
		if existing, ok := candidate.nodes[n.ID]; ok {
			if existing.Type != n.Type {
				return nil, preview, fmt.Errorf("node %q type cannot change", n.ID)
			}
			n.Source, n.EvidenceKind, n.Certainty, err = normalizeEvidence(existing.Source, n.EvidenceKind, n.Certainty)
			if err != nil {
				return nil, preview, err
			}
			if err := n.ValidateModelDetails(); err != nil {
				return nil, preview, err
			}
			candidate.nodes[n.ID] = n
		} else if err := candidate.AddNode(n); err != nil {
			return nil, preview, fmt.Errorf("add node %q: %w", n.ID, err)
		}
	}
	for _, r := range batch.Relations {
		r.Source = batch.Source
		if old, ok := candidate.relations[r.ID]; ok {
			if old.From != r.From || old.To != r.To || old.Predicate != r.Predicate {
				return nil, preview, fmt.Errorf("relation %q endpoints cannot change", r.ID)
			}
			r.Source, r.EvidenceKind, r.Certainty, err = normalizeEvidence(r.Source, r.EvidenceKind, r.Certainty)
			if err != nil {
				return nil, preview, err
			}
			if r.RecordedAt.IsZero() {
				r.RecordedAt = old.RecordedAt
			}
			candidate.relations[r.ID] = r
		} else if err := candidate.AddRelation(r); err != nil {
			return nil, preview, fmt.Errorf("add relation %q: %w", r.ID, err)
		}
	}
	for _, a := range batch.Assertions {
		a.Source = batch.Source
		if err := candidate.AddAssertion(a); err != nil {
			return nil, preview, fmt.Errorf("apply assertion %q: %w", a.ID, err)
		}
	}
	for _, id := range preview.MissingAssertions {
		a := candidate.assertions[id]
		a.Certainty = CertaintyStale
		candidate.assertions[id] = a
	}
	for _, id := range preview.MissingNodes {
		n := candidate.nodes[id]
		n.Certainty = CertaintyStale
		candidate.nodes[id] = n
	}
	for _, id := range preview.MissingRels {
		r := candidate.relations[id]
		r.Certainty = CertaintyStale
		candidate.relations[id] = r
	}
	// Ensure every newly introduced node is weakly connected, without inferring a stronger relation.
	selfID := ""
	for id, n := range candidate.nodes {
		if n.Type == Self {
			selfID = id
			break
		}
	}
	if selfID == "" && len(preview.AddedNodes) > 0 {
		return nil, preview, errors.New("cannot sync new nodes before Self exists")
	}
	for _, id := range preview.AddedNodes {
		if _, ok := candidate.PathFromSelf(id); ok {
			continue
		}
		rid := "sync-knows-" + id
		if _, exists := candidate.relations[rid]; exists {
			return nil, preview, fmt.Errorf("generated relation id %q already exists", rid)
		}
		if err := candidate.AddRelation(Relation{ID: rid, From: selfID, Predicate: KnowsAbout, To: id, Source: "system:connectivity", EvidenceKind: EvidenceDerived, Certainty: CertaintyProvisional}); err != nil {
			return nil, preview, err
		}
	}
	for _, n := range candidate.Nodes() {
		if _, ok := candidate.PathFromSelf(n.ID); !ok {
			return nil, preview, fmt.Errorf("sync would disconnect node %q", n.ID)
		}
	}
	return candidate, preview, nil
}
