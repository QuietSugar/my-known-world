// Package graph implements the in-memory domain invariants for MyKnownWorld.
package graph

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
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
	Network           NodeType = "Network"
	ProvisionalObject NodeType = "ProvisionalObject"
)

var nodeTypes = map[NodeType]bool{Self: true, Hardware: true, Machine: true, SoftwareProject: true, SoftwareInstance: true, RunningPeriod: true, AccountCredential: true, Product: true, Document: true, Runbook: true, WorkItem: true, Network: true, ProvisionalObject: true}

type Node struct {
	ID                 string                    `json:"id"`
	Type               NodeType                  `json:"type"`
	Name               string                    `json:"name"`
	Lifecycle          string                    `json:"lifecycle,omitempty"`
	Source             string                    `json:"source"`
	EvidenceKind       EvidenceKind              `json:"evidenceKind"`
	Certainty          Certainty                 `json:"certainty"`
	LastConfirmed      *time.Time                `json:"lastConfirmed,omitempty"`
	ModelVersion       int                       `json:"modelVersion"`
	CandidateType      NodeType                  `json:"candidateType,omitempty"`
	ReviewStatus       string                    `json:"reviewStatus,omitempty"`
	ReviewQuestions    []string                  `json:"reviewQuestions,omitempty"`
	StartedAt          *time.Time                `json:"startedAt,omitempty"`
	EndedAt            *time.Time                `json:"endedAt,omitempty"`
	DeployedVersion    string                    `json:"deployedVersion,omitempty"`
	VerificationAt     *time.Time                `json:"verificationAt,omitempty"`
	VerificationResult string                    `json:"verificationResult,omitempty"`
	EvidenceRef        string                    `json:"evidenceRef,omitempty"`
	EvidenceReference  string                    `json:"evidenceReference,omitempty"`
	Hardware           *HardwareDetails          `json:"hardware,omitempty"`
	Machine            *MachineDetails           `json:"machine,omitempty"`
	SoftwareProject    *SoftwareProjectDetails   `json:"softwareProject,omitempty"`
	SoftwareInstance   *SoftwareInstanceDetails  `json:"softwareInstance,omitempty"`
	AccountCredential  *AccountCredentialDetails `json:"accountCredential,omitempty"`
	Product            *ProductDetails           `json:"product,omitempty"`
	Document           *DocumentDetails          `json:"document,omitempty"`
	Runbook            *RunbookDetails           `json:"runbook,omitempty"`
	WorkItem           *WorkItemDetails          `json:"workItem,omitempty"`
	Network            *NetworkDetails           `json:"network,omitempty"`
}

type NodeSummary struct {
	ID           string       `json:"id"`
	Type         NodeType     `json:"type"`
	Name         string       `json:"name"`
	Lifecycle    string       `json:"lifecycle,omitempty"`
	Source       string       `json:"source"`
	EvidenceKind EvidenceKind `json:"evidenceKind"`
	Certainty    Certainty    `json:"certainty"`
}

func (n Node) Summary() NodeSummary {
	return NodeSummary{
		ID:           n.ID,
		Type:         n.Type,
		Name:         n.Name,
		Lifecycle:    n.Lifecycle,
		Source:       n.Source,
		EvidenceKind: n.EvidenceKind,
		Certainty:    n.Certainty,
	}
}

type Predicate string

const (
	Owns             Predicate = "owns"
	Maintains        Predicate = "maintains"
	KnowsAbout       Predicate = "knowsAbout"
	Uses             Predicate = "uses"
	CanOperate       Predicate = "canOperate"
	RoutesThrough    Predicate = "routesThrough"
	ProvidesAccessTo Predicate = "providesAccessTo"
	Hosts            Predicate = "hosts"
	ImplementedFrom  Predicate = "implementedFrom"
	HasRunningPeriod Predicate = "hasRunningPeriod"
	HasCredential    Predicate = "hasCredential"
	Documents        Predicate = "documents"
	AppliesTo        Predicate = "appliesTo"
	Targets          Predicate = "targets"
	Supersedes       Predicate = "supersedes"
	DependsOn        Predicate = "dependsOn"
	HasInstance      Predicate = "hasInstance"
	RelatedTo        Predicate = "relatedTo"
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
	nodes      map[string]Node
	relations  map[string]Relation
	assertions map[string]FieldAssertion
}

func New() *Graph {
	return &Graph{nodes: map[string]Node{}, relations: map[string]Relation{}, assertions: map[string]FieldAssertion{}}
}

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
	if n.Type == ProvisionalObject {
		if n.CandidateType != "" && (!nodeTypes[n.CandidateType] || n.CandidateType == Self || n.CandidateType == ProvisionalObject) {
			return fmt.Errorf("invalid provisional candidate type %q", n.CandidateType)
		}
		if n.ReviewStatus == "" {
			n.ReviewStatus = "pending"
		}
		if n.ReviewStatus != "pending" && n.ReviewStatus != "rejected" {
			return fmt.Errorf("invalid provisional review status %q", n.ReviewStatus)
		}
	} else if n.CandidateType != "" || n.ReviewStatus != "" || len(n.ReviewQuestions) != 0 {
		return errors.New("review metadata is only valid for provisional objects")
	}
	if n.Type == RunningPeriod {
		if n.StartedAt != nil && n.EndedAt != nil && n.EndedAt.Before(*n.StartedAt) {
			return errors.New("running period end must not precede start")
		}
		if (n.VerificationAt == nil) != (n.VerificationResult == "") {
			return errors.New("verification time and result must be supplied together")
		}
		if len(n.EvidenceRef) > 1024 {
			return errors.New("running period evidence reference exceeds 1024 bytes")
		}
	} else if n.StartedAt != nil || n.EndedAt != nil || n.DeployedVersion != "" || n.VerificationAt != nil || n.VerificationResult != "" || n.EvidenceRef != "" {
		return errors.New("running-period attributes are only valid for RunningPeriod nodes")
	}
	if err := n.ValidateModelDetails(); err != nil {
		return err
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
	case KnowsAbout, Uses:
		return a == Self && b != Self
	case CanOperate:
		return a == Self && (b == Hardware || b == Machine || b == SoftwareInstance)
	case RoutesThrough:
		return a == Network && b == Network
	case ProvidesAccessTo:
		return a == Network && (b == Machine || b == SoftwareInstance)
	case Hosts:
		return (a == Hardware && b == Machine) || (a == Machine && (b == Machine || b == SoftwareInstance))
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
	case DependsOn, RelatedTo:
		return a != Self && b != Self
	case HasInstance:
		return a == Product && b == SoftwareInstance
	default:
		return false
	}
}

func (g *Graph) Node(id string) (Node, bool)         { n, ok := g.nodes[id]; return n, ok }
func (g *Graph) Relation(id string) (Relation, bool) { r, ok := g.relations[id]; return r, ok }
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

// MatchesSearch checks if node matches needle against identity and typed detail fields.
func (n Node) MatchesSearch(needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return false
	}
	contains := func(s string) bool {
		return strings.Contains(strings.ToLower(s), needle)
	}
	if contains(n.ID) || contains(n.Name) || contains(n.Lifecycle) || contains(n.Source) {
		return true
	}
	if contains(n.EvidenceRef) || contains(n.EvidenceReference) {
		return true
	}
	if n.Machine != nil {
		if contains(n.Machine.Hostname) || contains(n.Machine.OS) {
			return true
		}
		for _, ip := range n.Machine.IPs {
			if contains(ip) {
				return true
			}
		}
	}
	if n.Hardware != nil {
		if contains(n.Hardware.Model) || contains(n.Hardware.Category) || contains(n.Hardware.Location) {
			return true
		}
	}
	if n.SoftwareProject != nil {
		if contains(n.SoftwareProject.RemoteRepository) || contains(n.SoftwareProject.Purpose) {
			return true
		}
	}
	if n.SoftwareInstance != nil {
		if contains(n.SoftwareInstance.ProductName) || contains(n.SoftwareInstance.DeploymentPath) {
			return true
		}
		for _, domain := range n.SoftwareInstance.Domains {
			if contains(domain) {
				return true
			}
		}
		for _, port := range n.SoftwareInstance.Ports {
			if contains(strconv.Itoa(port)) {
				return true
			}
		}
	}
	if n.AccountCredential != nil {
		if contains(n.AccountCredential.Account) || contains(n.AccountCredential.Purpose) {
			return true
		}
	}
	return false
}

// Search performs on-demand, streaming matching over the graph's nodes with zero resident memory index.
func (g *Graph) Search(query string, limit int) []NodeSummary {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []NodeSummary{}
	}
	results := make([]NodeSummary, 0)
	for _, n := range g.Nodes() {
		if n.MatchesSearch(query) {
			results = append(results, n.Summary())
			if len(results) >= limit {
				break
			}
		}
	}
	return results
}
