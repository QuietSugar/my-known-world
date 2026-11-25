package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	_ = g.AddNode(graph.Node{ID: "service", Type: graph.SoftwareInstance, Name: "Fixture service", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{ProductName: "ExampleApp", Domains: []string{"service.example.invalid"}, Ports: []int{80, 443}}})
	_ = g.AddRelation(graph.Relation{ID: "machine-hosts-service", From: "machine", Predicate: graph.Hosts, To: "service", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.AddAssertion(graph.FieldAssertion{ID: "safe-assertion", NodeID: "machine", FieldPath: "machine.hostname", ValueHash: graph.HashFieldValue("example.invalid"), Reference: "wiki/machine.md", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	_ = g.Publish(graph.Node{ID: "known", Type: graph.SoftwareProject, Name: "Known only", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}, graph.Relation{ID: "knows-project", Predicate: graph.KnowsAbout, To: "known", Source: "fixture", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
	return g
}
func TestMCPStreamableHTTPToolsAreTypedReadOnlyAndMatchGraphSemantics(t *testing.T) {
	g := testGraph()
	handler, err := NewHandler("0.1.0", func(context.Context) (*graph.Graph, error) { return g.Clone(), nil }, historyFixture{})
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
	for _, name := range []string{"graph_query", "graph_entity", "graph_relations", "graph_impact", "graph_explore", "graph_history", "graph_search", "graph_feedback"} {
		if !names[name] {
			t.Fatalf("missing read tool %q", name)
		}
	}
	if len(names) != 8 {
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
	byPort, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_query", Arguments: map[string]any{"text": "443"}})
	if err != nil {
		t.Fatal(err)
	}
	byPortJSON, _ := json.Marshal(byPort.StructuredContent)
	if err := json.Unmarshal(byPortJSON, &hostedOut); err != nil || hostedOut.Count != 1 || hostedOut.Items[0].ID != "service" {
		t.Fatalf("service port text filter failed: %s (%v)", byPortJSON, err)
	}
	searchRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "graph_search", Arguments: map[string]any{"query": "example.invalid"}})
	if err != nil {
		t.Fatal(err)
	}
	searchJSON, _ := json.Marshal(searchRes.StructuredContent)
	var searchOut SearchOutput
	if err := json.Unmarshal(searchJSON, &searchOut); err != nil || searchOut.Count == 0 {
		t.Fatalf("graph_search failed: %s (%v)", searchJSON, err)
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

func TestHandshakeAdvertisesInjectedImplementationVersion(t *testing.T) {
	g := testGraph()
	handler, err := NewHandler("0.1.0", func(context.Context) (*graph.Graph, error) { return g.Clone(), nil }, historyFixture{})
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
	info := session.InitializeResult().ServerInfo
	if info == nil || info.Name != "my-known-world" || info.Version != "0.1.0" {
		t.Fatalf("serverInfo = %+v, want my-known-world 0.1.0", info)
	}
}

func TestHandshakeFallsBackToDevForBlankVersion(t *testing.T) {
	g := testGraph()
	srv, err := NewServer("", func(context.Context) (*graph.Graph, error) { return g.Clone(), nil }, historyFixture{})
	if err != nil {
		t.Fatal(err)
	}
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if got := session.InitializeResult().ServerInfo.Version; got != "dev" {
		t.Fatalf("serverInfo.version = %q, want dev fallback", got)
	}
}

type testWriter struct {
	g        *graph.Graph
	revision string
}

func (w *testWriter) ApplyCreateBatch(_ context.Context, batch graph.CreateBatch, expectedRevision string) (graph.CreateResult, string, error) {
	if expectedRevision != "" && expectedRevision != w.revision {
		return graph.CreateResult{}, "", errors.New("stale revision")
	}
	next, res, err := w.g.ApplyCreateBatch(batch)
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	w.g = next
	w.revision = "rev-after-create"
	return res, w.revision, nil
}

func (w *testWriter) PatchNode(_ context.Context, id string, patch graph.Node, expectedRevision string) (graph.Node, string, error) {
	if expectedRevision == "" || expectedRevision != w.revision {
		return graph.Node{}, "", errors.New("stale revision")
	}
	old, ok := w.g.Node(id)
	if !ok {
		return graph.Node{}, "", fmt.Errorf("node %q not found", id)
	}
	if old.Type == graph.Self {
		return graph.Node{}, "", errors.New("Self cannot be patched")
	}
	merged := old
	if patch.Name != "" {
		merged.Name = patch.Name
	}
	next, err := w.g.UpdateNode(id, merged)
	if err != nil {
		return graph.Node{}, "", err
	}
	w.g = next
	w.revision = "rev-after-patch"
	return merged, w.revision, nil
}

func (w *testWriter) RetireNode(_ context.Context, id string, expectedRevision string) (graph.Node, string, error) {
	if expectedRevision != "" && expectedRevision != w.revision {
		return graph.Node{}, "", errors.New("stale revision")
	}
	old, ok := w.g.Node(id)
	if !ok {
		return graph.Node{}, "", fmt.Errorf("node %q not found", id)
	}
	if old.Type == graph.Self {
		return graph.Node{}, "", errors.New("Self cannot be retired")
	}
	next, err := w.g.RetireNode(id)
	if err != nil {
		return graph.Node{}, "", err
	}
	w.g = next
	w.revision = "rev-after-retire"
	retired, _ := w.g.Node(id)
	return retired, w.revision, nil
}

func (w *testWriter) DeleteNodeWithRevision(_ context.Context, id string, expectedRevision string) (string, error) {
	if expectedRevision == "" || expectedRevision != w.revision {
		return "", errors.New("stale revision")
	}
	old, ok := w.g.Node(id)
	if !ok {
		return "", fmt.Errorf("node %q not found", id)
	}
	if old.Type == graph.Self {
		return "", errors.New("Self cannot be deleted")
	}
	next, err := w.g.DeleteNode(id)
	if err != nil {
		return "", err
	}
	w.g = next
	w.revision = "rev-after-delete"
	return w.revision, nil
}

func (w *testWriter) ReviewProvisional(_ context.Context, id string, target graph.NodeType) error {
	next, err := w.g.ReviewProvisional(id, target)
	if err != nil {
		return err
	}
	w.g = next
	w.revision = "rev-after-review"
	return nil
}

func (w *testWriter) RejectProvisional(_ context.Context, id string) error {
	next, err := w.g.RejectProvisional(id)
	if err != nil {
		return err
	}
	w.g = next
	w.revision = "rev-after-reject"
	return nil
}

func TestMCPWritableToolsWhenConfigured(t *testing.T) {
	g := testGraph()
	writer := &testWriter{g: g, revision: "rev-0"}
	handler, err := NewHandlerWithWriter("0.2.0", func(context.Context) (*graph.Graph, error) { return writer.g.Clone(), nil }, historyFixture{}, writer)
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
	expectedTools := []string{
		"graph_query", "graph_entity", "graph_relations", "graph_impact", "graph_explore", "graph_history", "graph_search",
		"graph_create_node", "graph_patch_node", "graph_retire_node", "graph_delete_node",
		"graph_provisional_queue", "graph_review_provisional", "graph_feedback",
	}
	for _, name := range expectedTools {
		if !names[name] {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
	if len(names) != len(expectedTools) {
		t.Fatalf("expected exactly %d tools, got %d: %v", len(expectedTools), len(names), names)
	}

	// 1. graph_create_node
	createRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_create_node",
		Arguments: map[string]any{
			"node": map[string]any{
				"id":           "mcp-machine",
				"type":         "Machine",
				"name":         "MCP Machine",
				"modelVersion": 1,
			},
			"anchor": map[string]any{
				"predicate": "owns",
			},
		},
	})
	if err != nil {
		t.Fatalf("graph_create_node failed: %v", err)
	}
	createJSON, _ := json.Marshal(createRes.StructuredContent)
	var createOut CreateNodeOutput
	if err := json.Unmarshal(createJSON, &createOut); err != nil {
		t.Fatalf("unmarshal create output: %v", err)
	}
	if createOut.Node.ID != "mcp-machine" || createOut.Revision != "rev-after-create" {
		t.Fatalf("unexpected create output: %+v", createOut)
	}
	// An agent proposal must not become authoritative without review.
	if !createOut.Provisional || createOut.CandidateType != graph.Machine {
		t.Fatalf("create should file into the review queue: %+v", createOut)
	}
	if createOut.Node.Type != graph.ProvisionalObject || createOut.Node.ReviewStatus != "pending" {
		t.Fatalf("created node should be a pending provisional: %+v", createOut.Node)
	}

	// 1b. graph_provisional_queue must surface the pending item.
	queueRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "graph_provisional_queue",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("graph_provisional_queue failed: %v", err)
	}
	queueJSON, _ := json.Marshal(queueRes.StructuredContent)
	var queueOut ProvisionalQueueOutput
	if err := json.Unmarshal(queueJSON, &queueOut); err != nil {
		t.Fatalf("unmarshal queue output: %v", err)
	}
	if queueOut.Count != 1 || queueOut.Items[0].ID != "mcp-machine" || queueOut.Items[0].CandidateType != graph.Machine {
		t.Fatalf("unexpected queue output: %+v", queueOut)
	}

	// 1c. Approving the review promotes it to the candidate type.
	reviewRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_review_provisional",
		Arguments: map[string]any{
			"id":     "mcp-machine",
			"action": "approve",
		},
	})
	if err != nil {
		t.Fatalf("graph_review_provisional failed: %v", err)
	}
	reviewJSON, _ := json.Marshal(reviewRes.StructuredContent)
	var reviewOut ReviewProvisionalOutput
	if err := json.Unmarshal(reviewJSON, &reviewOut); err != nil {
		t.Fatalf("unmarshal review output: %v", err)
	}
	if reviewOut.Status != "classified" || reviewOut.Type != graph.Machine {
		t.Fatalf("unexpected review output: %+v", reviewOut)
	}
	if promoted, ok := writer.g.Node("mcp-machine"); !ok || promoted.Type != graph.Machine || promoted.ReviewStatus != "" {
		t.Fatalf("approval should promote the node: %+v", promoted)
	}

	// 2. graph_patch_node
	patchRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_patch_node",
		Arguments: map[string]any{
			"id":               "mcp-machine",
			"expectedRevision": "rev-after-review",
			"node": map[string]any{
				"name": "MCP Machine Renamed",
			},
		},
	})
	if err != nil {
		t.Fatalf("graph_patch_node failed: %v", err)
	}
	patchJSON, _ := json.Marshal(patchRes.StructuredContent)
	var patchOut PatchNodeOutput
	if err := json.Unmarshal(patchJSON, &patchOut); err != nil {
		t.Fatalf("unmarshal patch output: %v", err)
	}
	if patchOut.Node.Name != "MCP Machine Renamed" || patchOut.Revision != "rev-after-patch" {
		t.Fatalf("unexpected patch output: %+v", patchOut)
	}

	// 3. graph_retire_node
	retireRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_retire_node",
		Arguments: map[string]any{
			"id":               "mcp-machine",
			"expectedRevision": "rev-after-patch",
		},
	})
	if err != nil {
		t.Fatalf("graph_retire_node failed: %v", err)
	}
	retireJSON, _ := json.Marshal(retireRes.StructuredContent)
	var retireOut RetireNodeOutput
	if err := json.Unmarshal(retireJSON, &retireOut); err != nil {
		t.Fatalf("unmarshal retire output: %v", err)
	}
	if retireOut.Node.Lifecycle != "retired" || retireOut.Revision != "rev-after-retire" {
		t.Fatalf("unexpected retire output: %+v", retireOut)
	}

	// 4. graph_retire_node on Self returns error
	selfRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_retire_node",
		Arguments: map[string]any{
			"id": "self",
		},
	})
	if err == nil && (selfRes == nil || !selfRes.IsError) {
		t.Fatal("expected error retiring Self")
	}

	// 5. graph_delete_node
	deleteRes, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_delete_node",
		Arguments: map[string]any{
			"id":               "mcp-machine",
			"expectedRevision": "rev-after-retire",
		},
	})
	if err != nil {
		t.Fatalf("graph_delete_node failed: %v", err)
	}
	deleteJSON, _ := json.Marshal(deleteRes.StructuredContent)
	var deleteOut DeleteNodeOutput
	if err := json.Unmarshal(deleteJSON, &deleteOut); err != nil {
		t.Fatalf("unmarshal delete output: %v", err)
	}
	if deleteOut.DeletedID != "mcp-machine" || deleteOut.Revision != "rev-after-delete" {
		t.Fatalf("unexpected delete output: %+v", deleteOut)
	}
}
