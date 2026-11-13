// Package graph implements the in-memory domain invariants for MyKnownWorld.
package graph

import (
	"errors"
	"fmt"
	"sort"
	"time"
)

type NodeType string

const (
	Self              NodeType = "Self"
	Hardware          NodeType = "Hardware"
	Machine           NodeType = "Machine"
	SoftwareProject   NodeType = "SoftwareProject"
	SoftwareInstance  NodeType = "SoftwareInstance"
	RunningPeriod     NodeType = "RunningPeriod"
	AccountCredential NodeType = "AccountCredential"
	Product           NodeType = "Product"
	Document          NodeType = "Document"
	Runbook           NodeType = "Runbook"
	WorkItem          NodeType = "WorkItem"
	ProvisionalObject NodeType = "ProvisionalObject"
)

var nodeTypes = map[NodeType]bool{Self: true, Hardware: true, Machine: true, SoftwareProject: true, SoftwareInstance: true, RunningPeriod: true, AccountCredential: true, Product: true, Document: true, Runbook: true, WorkItem: true, ProvisionalObject: true}

type Node struct {
	ID            string       `json:"id"`
	Type          NodeType     `json:"type"`
	Name          string       `json:"name"`
	Lifecycle     string       `json:"lifecycle,omitempty"`
	Source        string       `json:"source"`
	EvidenceKind  EvidenceKind `json:"evidenceKind"`
	Certainty     Certainty    `json:"certainty"`
	LastConfirmed *time.Time   `json:"lastConfirmed,omitempty"`
	ModelVersion  int          `json:"modelVersion"`
}

type Predicate string

const (
	Owns             Predicate = "owns"
	Maintains        Predicate = "maintains"
	KnowsAbout       Predicate = "knowsAbout"
	Hosts            Predicate = "hosts"
	ImplementedFrom  Predicate = "implementedFrom"
	HasRunningPeriod Predicate = "hasRunningPeriod"
	HasCredential    Predicate = "hasCredential"
	Documents        Predicate = "documents"
	AppliesTo        Predicate = "appliesTo"
	Targets          Predicate = "targets"
	Supersedes       Predicate = "supersedes"
	DependsOn        Predicate = "dependsOn"
)

type Relation struct {
	ID           string       `json:"id"`
	From         string       `json:"from"`
	Predicate    Predicate    `json:"predicate"`
	To           string       `json:"to"`
	Source       string       `json:"source"`
	EvidenceKind EvidenceKind `json:"evidenceKind"`
	Certainty    Certainty    `json:"certainty"`
	RecordedAt   time.Time    `json:"recordedAt"`
}

type Graph struct {
	nodes     map[string]Node
	relations map[string]Relation
}

func New() *Graph { return &Graph{nodes: map[string]Node{}, relations: map[string]Relation{}} }

func (g *Graph) AddNode(n Node) error {
	if n.ID == "" || n.Name == "" {
		return errors.New("node id and name are required")
	}
	if !nodeTypes[n.Type] {
		return fmt.Errorf("unknown node type %q", n.Type)
	}
	if n.ModelVersion < 1 {
		return errors.New("modelVersion must be positive")
	}
	var err error
	n.Source, n.EvidenceKind, n.Certainty, err = normalizeEvidence(n.Source, n.EvidenceKind, n.Certainty)
	if err != nil {
		return err
	}
	if _, ok := g.nodes[n.ID]; ok {
		return fmt.Errorf("node %q already exists", n.ID)
	}
	if n.Type == Self {
		for _, old := range g.nodes {
			if old.Type == Self {
				return errors.New("graph can contain only one Self")
			}
		}
	}
	g.nodes[n.ID] = n
	return nil
}

func (g *Graph) AddRelation(r Relation) error {
	if r.ID == "" || r.From == "" || r.To == "" {
		return errors.New("relation id and endpoints are required")
	}
	if r.From == r.To {
		return errors.New("relation endpoints must be distinct")
	}
	if _, ok := g.relations[r.ID]; ok {
		return fmt.Errorf("relation %q already exists", r.ID)
	}
	from, ok := g.nodes[r.From]
	if !ok {
		return fmt.Errorf("source node %q not found", r.From)
	}
	to, ok := g.nodes[r.To]
	if !ok {
		return fmt.Errorf("target node %q not found", r.To)
	}
	if !validEndpoints(r.Predicate, from.Type, to.Type) {
		return fmt.Errorf("invalid %s relation: %s -> %s", r.Predicate, from.Type, to.Type)
	}
	var err error
	r.Source, r.EvidenceKind, r.Certainty, err = normalizeEvidence(r.Source, r.EvidenceKind, r.Certainty)
	if err != nil {
		return err
	}
	if r.RecordedAt.IsZero() {
		r.RecordedAt = time.Now().UTC()
	}
	g.relations[r.ID] = r
	return nil
}

func validEndpoints(p Predicate, a, b NodeType) bool {
	switch p {
	case Owns, Maintains:
		return a == Self && b != Self
	case KnowsAbout:
		return a == Self && b != Self
	case Hosts:
		return (a == Hardware && b == Machine) || ((a == Machine || a == SoftwareInstance) && b == SoftwareInstance)
	case ImplementedFrom:
		return a == SoftwareInstance && b == SoftwareProject
	case HasRunningPeriod:
		return a == SoftwareInstance && b == RunningPeriod
	case HasCredential:
		return (a == Machine || a == SoftwareInstance) && b == AccountCredential
	case Documents:
		return a == Document && b != Self
	case AppliesTo:
		return a == Runbook && b != Self
	case Targets:
		return a == WorkItem && b != Self
	case Supersedes:
		return a == b && a != Self
	case DependsOn:
		return a != Self && b != Self
	default:
		return false
	}
}

func (g *Graph) Node(id string) (Node, bool) { n, ok := g.nodes[id]; return n, ok }
func (g *Graph) Nodes() []Node {
	out := make([]Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (g *Graph) Relations() []Relation {
	out := make([]Relation, 0, len(g.relations))
	for _, r := range g.relations {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RelationsForPath returns the relation connecting each adjacent pair in a node path.
func (g *Graph) RelationsForPath(path []string) []Relation {
	result := make([]Relation, 0, max(0, len(path)-1))
	for i := 0; i+1 < len(path); i++ {
		for _, relation := range g.Relations() {
			if (relation.From == path[i] && relation.To == path[i+1]) || (relation.From == path[i+1] && relation.To == path[i]) {
				result = append(result, relation)
				break
			}
		}
	}
	return result
}

// PathFromSelf returns a weakly connected path, ignoring relation direction.
func (g *Graph) PathFromSelf(id string) ([]string, bool) {
	self := ""
	for key, n := range g.nodes {
		if n.Type == Self {
			self = key
			break
		}
	}
	if self == "" {
		return nil, false
	}
	if _, ok := g.nodes[id]; !ok {
		return nil, false
	}
	prev := map[string]string{self: ""}
	queue := []string{self}
	relations := g.Relations()
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == id {
			break
		}
		for _, r := range relations {
			next := ""
			if r.From == cur {
				next = r.To
			} else if r.To == cur {
				next = r.From
			}
			if next != "" {
				if _, seen := prev[next]; !seen {
					prev[next] = cur
					queue = append(queue, next)
				}
			}
		}
	}
	if _, ok := prev[id]; !ok {
		return nil, false
	}
	path := []string{}
	for cur := id; cur != ""; cur = prev[cur] {
		path = append(path, cur)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path, true
}

// Publish rejects nodes not weakly connected to Self; the graph remains unchanged on failure.
func (g *Graph) Publish(n Node, relation Relation) error {
	selfID := ""
	for id, existing := range g.nodes {
		if existing.Type == Self {
			selfID = id
			break
		}
	}
	if selfID == "" {
		return errors.New("cannot publish before Self exists")
	}
	if relation.From == "" {
		relation.From = selfID
	}
	if relation.Predicate == "" {
		relation.Predicate = KnowsAbout
	}
	if relation.To != n.ID {
		return errors.New("onboarding relation must target the new node")
	}
	if relation.From != selfID {
		return errors.New("new nodes must initially connect from Self")
	}
	if err := g.AddNode(n); err != nil {
		return err
	}
	if err := g.AddRelation(relation); err != nil {
		delete(g.nodes, n.ID)
		return err
	}
	return nil
}
