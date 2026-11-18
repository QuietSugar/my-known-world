package mcpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type historyFixture struct{}

func (historyFixture) EntityHistory(_ context.Context, kind, id string, _ int) ([]graph.ChangeEvent, error) {
	return []graph.ChangeEvent{{EntityType: kind, EntityID: id, Action: "updated", Snapshot: json.RawMessage(`{"id":"machine","type":"Machine","name":"Fixture machine"}`)}}, nil
}

func testGraph() *graph.Graph {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", Source: "fixture", EvidenceKind: graph.EvidenceManual, Certainty: graph.CertaintyConfirmed, ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "machine", Type: graph.Machine, Name: "Fixture machine", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1, Machine: &graph.MachineDetails{Hostname: "example.invalid"}}, graph.Relation{ID: "owns-machine", Predicate: graph.Owns, To: "machine", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.AddRelation(graph.Relation{ID: "maintains-machine", From: "self", Predicate: graph.Maintains, To: "machine", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.Publish(graph.Node{ID: "service", Type: graph.SoftwareInstance, Name: "Fixture service", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{ProductName: "ExampleApp", Domains: []string{"service.example.invalid"}}}, graph.Relation{ID: "machine-hosts-service", From: "machine", Predicate: graph.Hosts, To: "service", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.AddAssertion(graph.FieldAssertion{ID: "safe-assertion", NodeID: "machine", FieldPath: "machine.hostname", ValueHash: graph.HashFieldValue("example.invalid"), Reference: "wiki/machine.md", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.Publish(graph.Node{ID: "known", Type: graph.SoftwareProject, Name: "Known only", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}, graph.Relation{ID: "knows-project", Predicate: graph.KnowsAbout, To: "known", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	return g
}
func TestMCPStreamableHTTPToolsAreTypedReadOnlyAndMatchGraphSemantics(t *testing.T) {
	g := testGraph()
	handler, err := NewHandler(func(context.Context) (*graph.Graph, error) { return g.Clone(), nil }, historyFixture{})
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range listed.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"graph_query", "graph_entity", "graph_relations", "graph_impact", "graph_explore", "graph_history"} {
		if !names[name] {
			t.Fatalf("missing read tool %q", name)
		}
	}
	if len(names) != 6 {
		t.Fatalf("unexpected tool registered (must not expose writes): %v", names)
	}
	empty, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_query", Arguments: map[string]any{"text": "no matches"}})
	if err != nil {
		t.Fatal(err)
	}
	emptyJSON, _ := json.Marshal(empty.StructuredContent)
	var emptyOut QueryOutput
	if err := json.Unmarshal(emptyJSON, &emptyOut); err != nil || emptyOut.Items == nil || len(emptyOut.Items) != 0 || emptyOut.Count != 0 {
		t.Fatalf("empty query must return structured empty array, got %s (%v)", emptyJSON, err)
	}
	hosted, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_query", Arguments: map[string]any{"hostId": "machine", "type": "SoftwareInstance", "text": "EXAMPLE.INVALID"}})
	if err != nil {
		t.Fatal(err)
	}
	hostedJSON, _ := json.Marshal(hosted.StructuredContent)
	var hostedOut QueryOutput
	if err := json.Unmarshal(hostedJSON, &hostedOut); err != nil || hostedOut.Count != 1 || hostedOut.Items[0].ID != "service" {
		t.Fatalf("host and case-insensitive domain filter failed: %s (%v)", hostedJSON, err)
	}
	entity, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_entity", Arguments: map[string]any{"id": "machine"}})
	if err != nil {
		t.Fatal(err)
	}
	entityJSON, _ := json.Marshal(entity.StructuredContent)
	if !containsAll(string(entityJSON), `"selfRelationCategories":["maintains","owns"]`, `"fieldPath":"machine.hostname"`, `"source":"fixture"`) {
		t.Fatalf("entity result missing typed provenance: %s", entityJSON)
	}
	if containsAll(string(entityJSON), `"valueHash"`) {
		t.Fatalf("entity output leaked assertion hash: %s", entityJSON)
	}
	impact, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_impact", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	impactJSON, _ := json.Marshal(impact.StructuredContent)
	if containsAll(string(impactJSON), `"id":"known"`) {
		t.Fatalf("impact should exclude knowsAbout: %s", impactJSON)
	}
	var impactOut ImpactOutput
	if err := json.Unmarshal(impactJSON, &impactOut); err != nil || !reflect.DeepEqual(impactOut.Items, g.ImpactFromSelf()) {
		t.Fatalf("MCP impact diverged from graph/API semantics: %+v err=%v", impactOut, err)
	}
	relations, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_relations", Arguments: map[string]any{"id": "machine", "direction": "in", "predicate": "owns"}})
	if err != nil {
		t.Fatal(err)
	}
	if relations.StructuredContent == nil {
		t.Fatal("tool must return structured output")
	}
	exploration, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_explore", Arguments: map[string]any{"centerId": "machine", "depth": 1, "direction": "both"}})
	if err != nil || exploration.StructuredContent == nil {
		t.Fatalf("graph_explore failed: %v", err)
	}
	history, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_history", Arguments: map[string]any{"entityType": "node", "id": "machine"}})
	if err != nil || history.StructuredContent == nil {
		t.Fatalf("graph_history failed: %v", err)
	}
	historyJSON, err := json.Marshal(history.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var historyOut HistoryOutput
	if err = json.Unmarshal(historyJSON, &historyOut); err != nil {
		t.Fatalf("decode structured history: %s: %v", historyJSON, err)
	}
	if historyOut.Count != 1 || len(historyOut.Items) != 1 || historyOut.Items[0].Snapshot["id"] != "machine" {
		t.Fatalf("history snapshot missing or malformed: %s", historyJSON)
	}
}
func containsAll(s string, needles ...string) bool {
	for _, n := range needles {
		if !strings.Contains(s, n) {
			return false
		}
	}
	return true
}
