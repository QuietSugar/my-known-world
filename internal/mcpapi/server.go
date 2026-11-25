// Package mcpapi exposes graph queries and safe agent mutation tools over the official MCP Go SDK.
package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GraphLoader func(context.Context) (*graph.Graph, error)
type HistoryReader interface {
	EntityHistory(context.Context, string, string, int) ([]graph.ChangeEvent, error)
}
type AssertionReader interface {
	AssertionViews(context.Context, string) ([]graph.AssertionView, error)
}

// GraphWriter defines mutation capabilities for writable MCP tools.
type GraphWriter interface {
	ApplyCreateBatch(ctx context.Context, batch graph.CreateBatch, expectedRevision string) (graph.CreateResult, string, error)
	PatchNode(ctx context.Context, id string, patch graph.Node, expectedRevision string) (graph.Node, string, error)
	RetireNode(ctx context.Context, id string, expectedRevision string) (graph.Node, string, error)
	DeleteNodeWithRevision(ctx context.Context, id string, expectedRevision string) (string, error)
}

// provisionalReviewer is the optional capability that closes a pending review
// item. The PostgreSQL store implements both methods.
type provisionalReviewer interface {
	ReviewProvisional(ctx context.Context, id string, target graph.NodeType) error
	RejectProvisional(ctx context.Context, id string) error
}

type Server struct {
	load       GraphLoader
	history    HistoryReader
	assertions AssertionReader
	writer     GraphWriter
	usage      *UsageLog
}

// WithUsageLog attaches the append-only usage log. A nil log disables logging.
func (s *Server) WithUsageLog(log *UsageLog) *Server {
	s.usage = log
	return s
}

// NewServer builds the MCP server. implVersion is advertised as
// serverInfo.version in the initialize handshake; callers pass version.Current().
func NewServer(implVersion string, load GraphLoader, history HistoryReader, assertionReaders ...AssertionReader) (*mcp.Server, error) {
	return NewServerWithWriter(implVersion, load, history, nil, assertionReaders...)
}

// NewServerWithWriter builds an MCP server with optional mutation tools.
func NewServerWithWriter(implVersion string, load GraphLoader, history HistoryReader, writer GraphWriter, assertionReaders ...AssertionReader) (*mcp.Server, error) {
	return NewServerWithUsageLog(implVersion, load, history, writer, nil, assertionReaders...)
}

// NewServerWithUsageLog builds an MCP server and, when usage is non-nil,
// records every tool call to it. Recording never changes behaviour.
func NewServerWithUsageLog(implVersion string, load GraphLoader, history HistoryReader, writer GraphWriter, usage *UsageLog, assertionReaders ...AssertionReader) (*mcp.Server, error) {
	if load == nil {
		return nil, errors.New("graph loader is required")
	}
	if strings.TrimSpace(implVersion) == "" {
		implVersion = "dev"
	}
	s := &Server{load: load, history: history, writer: writer, usage: usage}
	if len(assertionReaders) > 0 {
		s.assertions = assertionReaders[0]
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "my-known-world", Version: implVersion}, nil)
	if usage != nil {
		srv.AddReceivingMiddleware(usageMiddleware(usage))
	}
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_query", Description: "Filter graph node summaries by type, lifecycle, certainty, evidence, hosting machine, and case-insensitive text across safe modeled fields."}, s.query)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_search", Description: "Zero-resident-memory instant keyword search across node IDs, names, lifecycles, and typed fields."}, s.search)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_entity", Description: "Get typed node details, direct relations, Self path/category, and value-free assertion metadata."}, s.entity)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_relations", Description: "List direct relations for a node with direction and optional predicate filters."}, s.relations)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_impact", Description: "Return the existing confirmed operational impact paths from Self; knowsAbout is excluded."}, s.impact)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_explore", Description: "Explore a bounded subgraph using the domain graph's direction, depth, and predicate rules."}, s.explore)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_history", Description: "Read append-only history for a node or relation ID."}, s.entityHistory)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "graph_feedback",
		Description: "Optionally report whether a previous lookup was useful, so later offline analysis can tell which data earns its place. Never changes graph content and never alters query results.",
	}, s.feedback)

	if s.writer != nil {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_create_node",
			Description: "Create a new graph node anchored to Self. By default the node is filed into the review queue as a ProvisionalObject and does NOT enter the authoritative graph until a human approves it; the response carries provisional=true and the candidate type. Pass publish=true only when the user explicitly asked to write it straight through. Every published node requires connectivity from Self; specify anchor.predicate ('knowsAbout', 'owns', 'maintains', 'uses', or 'canOperate') to define the relation from Self.",
		}, s.createNode)
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_provisional_queue",
			Description: "List nodes awaiting human review (type ProvisionalObject with reviewStatus pending), including the candidate type each would become. These are invisible to the authoritative graph until reviewed.",
		}, s.provisionalQueue)
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_review_provisional",
			Description: "Close a pending review item. action 'approve' promotes it to targetType (defaults to candidateType); action 'reject' keeps the record as history but marks it rejected. Approving is what makes an agent-proposed node authoritative.",
		}, s.reviewProvisional)
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_patch_node",
			Description: "Update editable attributes of an existing node under optimistic concurrency control. ID and Type are immutable. Requires expectedRevision from a previous read. Self cannot be patched. Returns updated node and new revision.",
		}, s.patchNode)
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_retire_node",
			Description: "Retire (soft decommission) an active entity by setting its lifecycle to 'retired' and stamping lastConfirmed time. Preserves the node and its relations in the graph without breaking connectivity. Prefer retirement over physical deletion for decommissioned hardware, machines, or services. Self cannot be retired. Returns retired node and new revision.",
		}, s.retireNode)
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "graph_delete_node",
			Description: "Physically remove a node and its attached assertions from the graph under optimistic concurrency control. Requires expectedRevision. Self cannot be deleted. Fails if active non-Self relations exist (remove them first) or if deletion would disconnect other nodes. Use graph_retire_node instead when an entity is decommissioned. Returns deleted id and new revision.",
		}, s.deleteNode)
	}

	return srv, nil
}

// NewHandler wraps NewServer in the streamable HTTP transport.
func NewHandler(implVersion string, load GraphLoader, history HistoryReader, assertionReaders ...AssertionReader) (*mcp.StreamableHTTPHandler, error) {
	return NewHandlerWithWriter(implVersion, load, history, nil, assertionReaders...)
}

// NewHandlerWithWriter wraps NewServerWithWriter in the streamable HTTP transport.
func NewHandlerWithWriter(implVersion string, load GraphLoader, history HistoryReader, writer GraphWriter, assertionReaders ...AssertionReader) (*mcp.StreamableHTTPHandler, error) {
	return NewHandlerWithWriterAndUsage(implVersion, load, history, writer, nil, assertionReaders...)
}

// NewHandlerWithWriterAndUsage is NewHandlerWithWriter with an append-only usage
// log attached. A nil log disables recording.
func NewHandlerWithWriterAndUsage(implVersion string, load GraphLoader, history HistoryReader, writer GraphWriter, usage *UsageLog, assertionReaders ...AssertionReader) (*mcp.StreamableHTTPHandler, error) {
	srv, err := NewServerWithUsageLog(implVersion, load, history, writer, usage, assertionReaders...)
	if err != nil {
		return nil, err
	}
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil), nil
}

type QueryInput struct {
	Type         graph.NodeType     `json:"type,omitempty"`
	Lifecycle    string             `json:"lifecycle,omitempty"`
	Certainty    graph.Certainty    `json:"certainty,omitempty"`
	EvidenceKind graph.EvidenceKind `json:"evidenceKind,omitempty"`
	HostID       string             `json:"hostId,omitempty"`
	Text         string             `json:"text,omitempty"`
	Count        int                `json:"count,omitempty"`
}
type NodeSummary = graph.NodeSummary
type QueryOutput struct {
	Items []NodeSummary `json:"items"`
	Count int           `json:"count"`
}

func (s *Server) query(ctx context.Context, _ *mcp.CallToolRequest, in QueryInput) (*mcp.CallToolResult, QueryOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, QueryOutput{}, err
	}
	if in.Count <= 0 {
		in.Count = 50
	}
	if in.Count > 250 {
		return nil, QueryOutput{}, errors.New("count must be at most 250")
	}
	needle := strings.ToLower(strings.TrimSpace(in.Text))
	out := QueryOutput{Items: []NodeSummary{}}
	for _, n := range g.Nodes() {
		if in.Type != "" && n.Type != in.Type || in.Lifecycle != "" && n.Lifecycle != in.Lifecycle || in.Certainty != "" && n.Certainty != in.Certainty || in.EvidenceKind != "" && n.EvidenceKind != in.EvidenceKind {
			continue
		}
		if in.HostID != "" && !hostedBy(g, n.ID, in.HostID) {
			continue
		}
		if needle != "" && !matchesNodeText(n, needle) {
			continue
		}
		out.Items = append(out.Items, n.Summary())
		if len(out.Items) >= in.Count {
			break
		}
	}
	out.Count = len(out.Items)
	return nil, out, nil
}

func hostedBy(g *graph.Graph, nodeID, hostID string) bool {
	if _, ok := g.Node(hostID); !ok {
		return false
	}
	for _, r := range g.Relations() {
		if r.Predicate == graph.Hosts && r.From == hostID && r.To == nodeID {
			return true
		}
	}
	return false
}

func matchesNodeText(n graph.Node, needle string) bool {
	return n.MatchesSearch(needle)
}

type SearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type SearchOutput struct {
	Items []graph.NodeSummary `json:"items"`
	Query string              `json:"query"`
	Count int                 `json:"count"`
}

func (s *Server) search(ctx context.Context, _ *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, SearchOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, SearchOutput{}, err
	}
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return nil, SearchOutput{}, errors.New("query is required")
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		return nil, SearchOutput{}, errors.New("limit must be at most 50")
	}
	items := g.Search(q, limit)
	return nil, SearchOutput{
		Items: items,
		Query: q,
		Count: len(items),
	}, nil
}

type EntityInput struct {
	ID string `json:"id"`
}
type EntityOutput struct {
	Node                   graph.Node            `json:"node"`
	SelfRelationCategories []graph.Predicate     `json:"selfRelationCategories"`
	SelfPathNodes          []graph.Node          `json:"selfPathNodes"`
	SelfPathRelations      []graph.Relation      `json:"selfPathRelations"`
	DirectRelations        []graph.Relation      `json:"directRelations"`
	Assertions             []graph.AssertionView `json:"assertions"`
}

func (s *Server) entity(ctx context.Context, _ *mcp.CallToolRequest, in EntityInput) (*mcp.CallToolResult, EntityOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, EntityOutput{}, err
	}
	n, ok := g.Node(in.ID)
	if !ok {
		return nil, EntityOutput{}, fmt.Errorf("node %q not found", in.ID)
	}
	out := EntityOutput{Node: n, SelfRelationCategories: []graph.Predicate{}, SelfPathNodes: []graph.Node{}, SelfPathRelations: []graph.Relation{}, DirectRelations: []graph.Relation{}, Assertions: []graph.AssertionView{}}
	relations := g.Relations()
	selfID := ""
	for _, x := range g.Nodes() {
		if x.Type == graph.Self {
			selfID = x.ID
			break
		}
	}
	for _, r := range relations {
		if r.From == in.ID || r.To == in.ID {
			out.DirectRelations = append(out.DirectRelations, r)
		}
		if r.From == selfID && r.To == in.ID {
			switch r.Predicate {
			case graph.Owns, graph.Maintains, graph.Uses, graph.CanOperate, graph.KnowsAbout:
				out.SelfRelationCategories = append(out.SelfRelationCategories, r.Predicate)
			}
		}
	}
	sort.Slice(out.SelfRelationCategories, func(i, j int) bool { return out.SelfRelationCategories[i] < out.SelfRelationCategories[j] })
	categories := out.SelfRelationCategories[:0]
	for _, predicate := range out.SelfRelationCategories {
		if len(categories) == 0 || categories[len(categories)-1] != predicate {
			categories = append(categories, predicate)
		}
	}
	out.SelfRelationCategories = categories
	if ids, found := g.PathFromSelf(in.ID); found {
		for _, id := range ids {
			if pn, ok := g.Node(id); ok {
				out.SelfPathNodes = append(out.SelfPathNodes, pn)
			}
		}
		out.SelfPathRelations = g.RelationsForPath(ids)
	}
	out.Assertions = graph.BuildAssertionViews(g.AssertionsForNode(in.ID), nil)
	if s.assertions != nil {
		views, e := s.assertions.AssertionViews(ctx, in.ID)
		if e != nil {
			return nil, EntityOutput{}, e
		}
		if views != nil {
			out.Assertions = views
		}
	}
	return nil, out, nil
}

type RelationsInput struct {
	ID        string          `json:"id"`
	Direction string          `json:"direction,omitempty"`
	Predicate graph.Predicate `json:"predicate,omitempty"`
}
type RelationsOutput struct {
	Items []graph.Relation `json:"items"`
	Count int              `json:"count"`
}

func (s *Server) relations(ctx context.Context, _ *mcp.CallToolRequest, in RelationsInput) (*mcp.CallToolResult, RelationsOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, RelationsOutput{}, err
	}
	if _, ok := g.Node(in.ID); !ok {
		return nil, RelationsOutput{}, fmt.Errorf("node %q not found", in.ID)
	}
	if in.Direction == "" {
		in.Direction = "both"
	}
	if in.Direction != "out" && in.Direction != "in" && in.Direction != "both" {
		return nil, RelationsOutput{}, errors.New("direction must be out, in, or both")
	}
	out := RelationsOutput{Items: []graph.Relation{}}
	for _, r := range g.Relations() {
		match := in.Direction == "both" || (in.Direction == "out" && r.From == in.ID) || (in.Direction == "in" && r.To == in.ID)
		if match && (r.From == in.ID || r.To == in.ID) && (in.Predicate == "" || r.Predicate == in.Predicate) {
			out.Items = append(out.Items, r)
		}
	}
	out.Count = len(out.Items)
	return nil, out, nil
}

type EmptyInput struct{}
type ImpactOutput struct {
	Items []graph.ImpactPath `json:"items"`
	Count int                `json:"count"`
}

func (s *Server) impact(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, ImpactOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, ImpactOutput{}, err
	}
	items := g.ImpactFromSelf()
	if items == nil {
		items = []graph.ImpactPath{}
	}
	return nil, ImpactOutput{Items: items, Count: len(items)}, nil
}

type ExploreInput struct {
	CenterID   string            `json:"centerId"`
	Direction  string            `json:"direction,omitempty"`
	Depth      int               `json:"depth,omitempty"`
	Predicates []graph.Predicate `json:"predicates,omitempty"`
}

func (s *Server) explore(ctx context.Context, _ *mcp.CallToolRequest, in ExploreInput) (*mcp.CallToolResult, graph.Exploration, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, graph.Exploration{}, err
	}
	if in.Direction == "" {
		in.Direction = "both"
	}
	if in.Depth == 0 {
		in.Depth = 2
	}
	predicates := append([]graph.Predicate(nil), in.Predicates...)
	sort.Slice(predicates, func(i, j int) bool { return predicates[i] < predicates[j] })
	out, err := g.Explore(graph.ExploreOptions{CenterID: in.CenterID, Direction: in.Direction, Depth: in.Depth, Predicates: predicates})
	return nil, out, err
}

type HistoryInput struct {
	EntityType string `json:"entityType"`
	ID         string `json:"id"`
	Count      int    `json:"count,omitempty"`
}
type HistoryEventOutput struct {
	ID         int64          `json:"id"`
	RunID      *int64         `json:"runId,omitempty"`
	Source     string         `json:"source"`
	EntityType string         `json:"entityType"`
	EntityID   string         `json:"entityId"`
	Action     string         `json:"action"`
	RecordedAt string         `json:"recordedAt"`
	Snapshot   map[string]any `json:"snapshot,omitempty"`
}
type HistoryOutput struct {
	Items []HistoryEventOutput `json:"items"`
	Count int                  `json:"count"`
}

func (s *Server) entityHistory(ctx context.Context, _ *mcp.CallToolRequest, in HistoryInput) (*mcp.CallToolResult, HistoryOutput, error) {
	if s.history == nil {
		return nil, HistoryOutput{}, errors.New("entity history requires PostgreSQL")
	}
	if in.EntityType != "node" && in.EntityType != "relation" {
		return nil, HistoryOutput{}, errors.New("entityType must be node or relation")
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, HistoryOutput{}, errors.New("id is required")
	}
	if in.Count <= 0 {
		in.Count = 100
	}
	if in.Count > 100 {
		return nil, HistoryOutput{}, errors.New("count must be at most 100")
	}
	items, err := s.history.EntityHistory(ctx, in.EntityType, in.ID, in.Count)
	if err != nil {
		return nil, HistoryOutput{}, err
	}
	if items == nil {
		items = []graph.ChangeEvent{}
	}
	out := make([]HistoryEventOutput, 0, len(items))
	for _, item := range items {
		event := HistoryEventOutput{ID: item.ID, RunID: item.RunID, Source: item.Source, EntityType: item.EntityType, EntityID: item.EntityID, Action: item.Action, RecordedAt: item.RecordedAt.Format(time.RFC3339Nano)}
		if len(item.Snapshot) != 0 && string(item.Snapshot) != "null" {
			if err := json.Unmarshal(item.Snapshot, &event.Snapshot); err != nil {
				return nil, HistoryOutput{}, fmt.Errorf("decode history snapshot: %w", err)
			}
		}
		out = append(out, event)
	}
	return nil, HistoryOutput{Items: out, Count: len(out)}, nil
}

type CreateNodeData struct {
	ID                 string                          `json:"id"`
	Type               graph.NodeType                  `json:"type"`
	Name               string                          `json:"name"`
	Lifecycle          string                          `json:"lifecycle,omitempty"`
	Source             string                          `json:"source,omitempty"`
	EvidenceKind       graph.EvidenceKind              `json:"evidenceKind,omitempty"`
	Certainty          graph.Certainty                 `json:"certainty,omitempty"`
	ModelVersion       int                             `json:"modelVersion,omitempty"`
	CandidateType      graph.NodeType                  `json:"candidateType,omitempty"`
	ReviewStatus       string                          `json:"reviewStatus,omitempty"`
	ReviewQuestions    []string                        `json:"reviewQuestions,omitempty"`
	StartedAt          *time.Time                      `json:"startedAt,omitempty"`
	EndedAt            *time.Time                      `json:"endedAt,omitempty"`
	DeployedVersion    string                          `json:"deployedVersion,omitempty"`
	VerificationAt     *time.Time                      `json:"verificationAt,omitempty"`
	VerificationResult string                          `json:"verificationResult,omitempty"`
	EvidenceRef        string                          `json:"evidenceRef,omitempty"`
	EvidenceReference  string                          `json:"evidenceReference,omitempty"`
	Hardware           *graph.HardwareDetails          `json:"hardware,omitempty"`
	Machine            *graph.MachineDetails           `json:"machine,omitempty"`
	SoftwareProject    *graph.SoftwareProjectDetails   `json:"softwareProject,omitempty"`
	SoftwareInstance   *graph.SoftwareInstanceDetails  `json:"softwareInstance,omitempty"`
	AccountCredential  *graph.AccountCredentialDetails `json:"accountCredential,omitempty"`
	Product            *graph.ProductDetails           `json:"product,omitempty"`
	Document           *graph.DocumentDetails          `json:"document,omitempty"`
	Runbook            *graph.RunbookDetails           `json:"runbook,omitempty"`
	WorkItem           *graph.WorkItemDetails          `json:"workItem,omitempty"`
	Network            *graph.NetworkDetails           `json:"network,omitempty"`
}

func (d CreateNodeData) toNode() graph.Node {
	mv := d.ModelVersion
	if mv <= 0 {
		mv = 1
	}
	src := d.Source
	if src == "" {
		src = "agent"
	}
	ek := d.EvidenceKind
	if ek == "" {
		ek = graph.EvidenceManual
	}
	cert := d.Certainty
	if cert == "" {
		cert = graph.CertaintyConfirmed
	}
	return graph.Node{
		ID:                 d.ID,
		Type:               d.Type,
		Name:               d.Name,
		Lifecycle:          d.Lifecycle,
		Source:             src,
		EvidenceKind:       ek,
		Certainty:          cert,
		ModelVersion:       mv,
		CandidateType:      d.CandidateType,
		ReviewStatus:       d.ReviewStatus,
		ReviewQuestions:    d.ReviewQuestions,
		StartedAt:          d.StartedAt,
		EndedAt:            d.EndedAt,
		DeployedVersion:    d.DeployedVersion,
		VerificationAt:     d.VerificationAt,
		VerificationResult: d.VerificationResult,
		EvidenceRef:        d.EvidenceRef,
		EvidenceReference:  d.EvidenceReference,
		Hardware:           d.Hardware,
		Machine:            d.Machine,
		SoftwareProject:    d.SoftwareProject,
		SoftwareInstance:   d.SoftwareInstance,
		AccountCredential:  d.AccountCredential,
		Product:            d.Product,
		Document:           d.Document,
		Runbook:            d.Runbook,
		WorkItem:           d.WorkItem,
		Network:            d.Network,
	}
}

type PatchNodeData struct {
	Name               string                          `json:"name,omitempty"`
	Lifecycle          string                          `json:"lifecycle,omitempty"`
	Source             string                          `json:"source,omitempty"`
	EvidenceKind       graph.EvidenceKind              `json:"evidenceKind,omitempty"`
	Certainty          graph.Certainty                 `json:"certainty,omitempty"`
	ModelVersion       int                             `json:"modelVersion,omitempty"`
	CandidateType      graph.NodeType                  `json:"candidateType,omitempty"`
	ReviewStatus       string                          `json:"reviewStatus,omitempty"`
	ReviewQuestions    []string                        `json:"reviewQuestions,omitempty"`
	StartedAt          *time.Time                      `json:"startedAt,omitempty"`
	EndedAt            *time.Time                      `json:"endedAt,omitempty"`
	DeployedVersion    string                          `json:"deployedVersion,omitempty"`
	VerificationAt     *time.Time                      `json:"verificationAt,omitempty"`
	VerificationResult string                          `json:"verificationResult,omitempty"`
	EvidenceRef        string                          `json:"evidenceRef,omitempty"`
	EvidenceReference  string                          `json:"evidenceReference,omitempty"`
	Hardware           *graph.HardwareDetails          `json:"hardware,omitempty"`
	Machine            *graph.MachineDetails           `json:"machine,omitempty"`
	SoftwareProject    *graph.SoftwareProjectDetails   `json:"softwareProject,omitempty"`
	SoftwareInstance   *graph.SoftwareInstanceDetails  `json:"softwareInstance,omitempty"`
	AccountCredential  *graph.AccountCredentialDetails `json:"accountCredential,omitempty"`
	Product            *graph.ProductDetails           `json:"product,omitempty"`
	Document           *graph.DocumentDetails          `json:"document,omitempty"`
	Runbook            *graph.RunbookDetails           `json:"runbook,omitempty"`
	WorkItem           *graph.WorkItemDetails          `json:"workItem,omitempty"`
	Network            *graph.NetworkDetails           `json:"network,omitempty"`
}

func (d PatchNodeData) toNode(id string) graph.Node {
	return graph.Node{
		ID:                 id,
		Name:               d.Name,
		Lifecycle:          d.Lifecycle,
		Source:             d.Source,
		EvidenceKind:       d.EvidenceKind,
		Certainty:          d.Certainty,
		ModelVersion:       d.ModelVersion,
		CandidateType:      d.CandidateType,
		ReviewStatus:       d.ReviewStatus,
		ReviewQuestions:    d.ReviewQuestions,
		StartedAt:          d.StartedAt,
		EndedAt:            d.EndedAt,
		DeployedVersion:    d.DeployedVersion,
		VerificationAt:     d.VerificationAt,
		VerificationResult: d.VerificationResult,
		EvidenceRef:        d.EvidenceRef,
		EvidenceReference:  d.EvidenceReference,
		Hardware:           d.Hardware,
		Machine:            d.Machine,
		SoftwareProject:    d.SoftwareProject,
		SoftwareInstance:   d.SoftwareInstance,
		AccountCredential:  d.AccountCredential,
		Product:            d.Product,
		Document:           d.Document,
		Runbook:            d.Runbook,
		WorkItem:           d.WorkItem,
		Network:            d.Network,
	}
}

type CreateNodeInput struct {
	Node   CreateNodeData      `json:"node"`
	Anchor *graph.CreateAnchor `json:"anchor,omitempty"`
	// Publish writes the node straight into the authoritative graph. The zero
	// value files it into the review queue instead, so an agent proposal cannot
	// become authoritative without a human decision.
	Publish bool `json:"publish,omitempty"`
}

type CreateNodeOutput struct {
	Node        graph.Node `json:"node"`
	Revision    string     `json:"revision"`
	Provisional bool       `json:"provisional"`
	// CandidateType is the node type the review queue will promote this to.
	CandidateType graph.NodeType `json:"candidateType,omitempty"`
	Note          string         `json:"note,omitempty"`
}

func (s *Server) createNode(ctx context.Context, _ *mcp.CallToolRequest, in CreateNodeInput) (*mcp.CallToolResult, CreateNodeOutput, error) {
	if s.writer == nil {
		return nil, CreateNodeOutput{}, errors.New("write operations are not enabled")
	}
	if in.Node.ID == "" {
		return nil, CreateNodeOutput{}, errors.New("node id is required")
	}
	node := in.Node.toNode()
	provisional := !in.Publish
	if provisional {
		// The review queue holds the proposal verbatim and promotes it to
		// candidateType once a human approves.
		candidateType := node.Type
		if candidateType == graph.Self || candidateType == graph.ProvisionalObject {
			return nil, CreateNodeOutput{}, fmt.Errorf("node type %q cannot be proposed for review", candidateType)
		}
		node.CandidateType = candidateType
		node.Type = graph.ProvisionalObject
		node.ReviewStatus = "pending"
	}
	batch := graph.CreateBatch{
		Nodes: []graph.CreateNodeRequest{{Node: node, Anchor: in.Anchor}},
	}
	result, rev, err := s.writer.ApplyCreateBatch(ctx, batch, "")
	if err != nil {
		return nil, CreateNodeOutput{}, err
	}
	createdNode := node
	if len(result.NodeIDs) > 0 {
		createdID := result.NodeIDs[0]
		if g, loadErr := s.load(ctx); loadErr == nil && g != nil {
			if n, ok := g.Node(createdID); ok {
				createdNode = n
			}
		}
	}
	out := CreateNodeOutput{Node: createdNode, Revision: rev, Provisional: provisional}
	if provisional {
		out.CandidateType = node.CandidateType
		out.Note = "Filed into the review queue. It becomes authoritative only after graph_review_provisional approves it."
	}
	return nil, out, nil
}

type ProvisionalQueueInput struct {
	CandidateType graph.NodeType `json:"candidateType,omitempty"`
}

type ProvisionalItem struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	CandidateType  graph.NodeType `json:"candidateType"`
	Source         string         `json:"source"`
	Certainty      string         `json:"certainty"`
	EvidenceKind   string         `json:"evidenceKind"`
	ReviewQuestion []string       `json:"reviewQuestions,omitempty"`
}

type ProvisionalQueueOutput struct {
	Items []ProvisionalItem `json:"items"`
	Count int               `json:"count"`
}

func (s *Server) provisionalQueue(ctx context.Context, _ *mcp.CallToolRequest, in ProvisionalQueueInput) (*mcp.CallToolResult, ProvisionalQueueOutput, error) {
	g, err := s.load(ctx)
	if err != nil {
		return nil, ProvisionalQueueOutput{}, err
	}
	items := []ProvisionalItem{}
	for _, n := range g.ProvisionalQueue() {
		if in.CandidateType != "" && n.CandidateType != in.CandidateType {
			continue
		}
		items = append(items, ProvisionalItem{
			ID:             n.ID,
			Name:           n.Name,
			CandidateType:  n.CandidateType,
			Source:         n.Source,
			Certainty:      string(n.Certainty),
			EvidenceKind:   string(n.EvidenceKind),
			ReviewQuestion: n.ReviewQuestions,
		})
	}
	return nil, ProvisionalQueueOutput{Items: items, Count: len(items)}, nil
}

type ReviewProvisionalInput struct {
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	TargetType graph.NodeType `json:"targetType,omitempty"`
}

type ReviewProvisionalOutput struct {
	ID     string         `json:"id"`
	Action string         `json:"action"`
	Status string         `json:"status"`
	Type   graph.NodeType `json:"type,omitempty"`
}

func (s *Server) reviewProvisional(ctx context.Context, _ *mcp.CallToolRequest, in ReviewProvisionalInput) (*mcp.CallToolResult, ReviewProvisionalOutput, error) {
	if s.writer == nil {
		return nil, ReviewProvisionalOutput{}, errors.New("write operations are not enabled")
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, ReviewProvisionalOutput{}, errors.New("id is required")
	}
	g, err := s.load(ctx)
	if err != nil {
		return nil, ReviewProvisionalOutput{}, err
	}
	node, ok := g.Node(in.ID)
	if !ok || node.Type != graph.ProvisionalObject || node.ReviewStatus != "pending" {
		return nil, ReviewProvisionalOutput{}, fmt.Errorf("no pending review item %q", in.ID)
	}

	var next *graph.Graph
	switch in.Action {
	case "approve":
		next, err = g.ReviewProvisional(in.ID, in.TargetType)
	case "reject":
		next, err = g.RejectProvisional(in.ID)
	default:
		return nil, ReviewProvisionalOutput{}, errors.New("action must be approve or reject")
	}
	if err != nil {
		return nil, ReviewProvisionalOutput{}, err
	}
	_ = next // validated in memory; the store persists the decision atomically

	reviewer, ok := s.writer.(provisionalReviewer)
	if !ok {
		return nil, ReviewProvisionalOutput{}, errors.New("this writer does not support provisional review")
	}
	if in.Action == "reject" {
		err = reviewer.RejectProvisional(ctx, in.ID)
	} else {
		err = reviewer.ReviewProvisional(ctx, in.ID, in.TargetType)
	}
	if err != nil {
		return nil, ReviewProvisionalOutput{}, err
	}

	out := ReviewProvisionalOutput{ID: in.ID, Action: in.Action, Status: "classified"}
	if in.Action == "reject" {
		out.Status = "rejected"
	} else {
		target := in.TargetType
		if target == "" {
			target = node.CandidateType
		}
		out.Type = target
	}
	return nil, out, nil
}

type PatchNodeInput struct {
	ID               string        `json:"id"`
	Node             PatchNodeData `json:"node"`
	ExpectedRevision string        `json:"expectedRevision"`
}

type PatchNodeOutput struct {
	Node     graph.Node `json:"node"`
	Revision string     `json:"revision"`
}

func (s *Server) patchNode(ctx context.Context, _ *mcp.CallToolRequest, in PatchNodeInput) (*mcp.CallToolResult, PatchNodeOutput, error) {
	if s.writer == nil {
		return nil, PatchNodeOutput{}, errors.New("write operations are not enabled")
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, PatchNodeOutput{}, errors.New("id is required")
	}
	if strings.TrimSpace(in.ExpectedRevision) == "" {
		return nil, PatchNodeOutput{}, errors.New("expectedRevision is required")
	}
	patch := in.Node.toNode(in.ID)
	updated, rev, err := s.writer.PatchNode(ctx, in.ID, patch, in.ExpectedRevision)
	if err != nil {
		return nil, PatchNodeOutput{}, err
	}
	return nil, PatchNodeOutput{Node: updated, Revision: rev}, nil
}

type RetireNodeInput struct {
	ID               string `json:"id"`
	ExpectedRevision string `json:"expectedRevision,omitempty"`
}

type RetireNodeOutput struct {
	Node     graph.Node `json:"node"`
	Revision string     `json:"revision"`
}

func (s *Server) retireNode(ctx context.Context, _ *mcp.CallToolRequest, in RetireNodeInput) (*mcp.CallToolResult, RetireNodeOutput, error) {
	if s.writer == nil {
		return nil, RetireNodeOutput{}, errors.New("write operations are not enabled")
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, RetireNodeOutput{}, errors.New("id is required")
	}
	retired, rev, err := s.writer.RetireNode(ctx, in.ID, in.ExpectedRevision)
	if err != nil {
		return nil, RetireNodeOutput{}, err
	}
	return nil, RetireNodeOutput{Node: retired, Revision: rev}, nil
}

type DeleteNodeInput struct {
	ID               string `json:"id"`
	ExpectedRevision string `json:"expectedRevision"`
}

type DeleteNodeOutput struct {
	ID        string `json:"id"`
	DeletedID string `json:"deletedId"`
	Revision  string `json:"revision"`
}

func (s *Server) deleteNode(ctx context.Context, _ *mcp.CallToolRequest, in DeleteNodeInput) (*mcp.CallToolResult, DeleteNodeOutput, error) {
	if s.writer == nil {
		return nil, DeleteNodeOutput{}, errors.New("write operations are not enabled")
	}
	if strings.TrimSpace(in.ID) == "" {
		return nil, DeleteNodeOutput{}, errors.New("id is required")
	}
	if strings.TrimSpace(in.ExpectedRevision) == "" {
		return nil, DeleteNodeOutput{}, errors.New("expectedRevision is required")
	}
	rev, err := s.writer.DeleteNodeWithRevision(ctx, in.ID, in.ExpectedRevision)
	if err != nil {
		return nil, DeleteNodeOutput{}, err
	}
	return nil, DeleteNodeOutput{ID: in.ID, DeletedID: in.ID, Revision: rev}, nil
}
