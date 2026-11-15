// Package mcpapi exposes read-only graph queries over the official MCP Go SDK.
// Its server is constructed with query tools only; no mutation tool is registered.
package mcpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

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
type Server struct {
	load       GraphLoader
	history    HistoryReader
	assertions AssertionReader
}

func NewServer(load GraphLoader, history HistoryReader, assertionReaders ...AssertionReader) (*mcp.Server, error) {
	if load == nil {
		return nil, errors.New("graph loader is required")
	}
	s := &Server{load: load, history: history}
	if len(assertionReaders) > 0 {
		s.assertions = assertionReaders[0]
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "my-known-world", Version: "0.0.2"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_query", Description: "Filter graph node summaries by explicit metadata and substring."}, s.query)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_entity", Description: "Get typed node details, direct relations, Self path/category, and value-free assertion metadata."}, s.entity)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_relations", Description: "List direct relations for a node with direction and optional predicate filters."}, s.relations)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_impact", Description: "Return the existing confirmed operational impact paths from Self; knowsAbout is excluded."}, s.impact)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_explore", Description: "Explore a bounded subgraph using the domain graph's direction, depth, and predicate rules."}, s.explore)
	mcp.AddTool(srv, &mcp.Tool{Name: "graph_history", Description: "Read append-only history for a node or relation ID."}, s.entityHistory)
	return srv, nil
}
func NewHandler(load GraphLoader, history HistoryReader, assertionReaders ...AssertionReader) (*mcp.StreamableHTTPHandler, error) {
	srv, err := NewServer(load, history, assertionReaders...)
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
	Text         string             `json:"text,omitempty"`
	Count        int                `json:"count,omitempty"`
}
type NodeSummary struct {
	ID           string             `json:"id"`
	Type         graph.NodeType     `json:"type"`
	Name         string             `json:"name"`
	Lifecycle    string             `json:"lifecycle,omitempty"`
	Source       string             `json:"source"`
	EvidenceKind graph.EvidenceKind `json:"evidenceKind"`
	Certainty    graph.Certainty    `json:"certainty"`
}
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
		if needle != "" && !strings.Contains(strings.ToLower(n.Name), needle) && !strings.Contains(strings.ToLower(n.ID), needle) {
			continue
		}
		out.Items = append(out.Items, NodeSummary{ID: n.ID, Type: n.Type, Name: n.Name, Lifecycle: n.Lifecycle, Source: n.Source, EvidenceKind: n.EvidenceKind, Certainty: n.Certainty})
		if len(out.Items) >= in.Count {
			break
		}
	}
	out.Count = len(out.Items)
	return nil, out, nil
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
type HistoryOutput struct {
	Items []graph.ChangeEvent `json:"items"`
	Count int                 `json:"count"`
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
	return nil, HistoryOutput{Items: items, Count: len(items)}, nil
}
