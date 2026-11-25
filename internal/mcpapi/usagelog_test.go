package mcpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// readUsageLog parses the append-only JSON-lines file into records.
func readUsageLog(t *testing.T, path string) []UsageRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open usage log: %v", err)
	}
	defer file.Close()
	records := []UsageRecord{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var rec UsageRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			t.Fatalf("usage log line is not JSON: %v", err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan usage log: %v", err)
	}
	return records
}

func usageSession(t *testing.T, usage *UsageLog) *mcp.ClientSession {
	t.Helper()
	g := testGraph()
	handler, err := NewHandlerWithWriterAndUsage(
		"0.3.0",
		func(context.Context) (*graph.Graph, error) { return g.Clone(), nil },
		historyFixture{},
		nil,
		usage,
	)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "usage-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func TestUsageLogRecordsLookups(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "usage.jsonl")
	usage, err := OpenUsageLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer usage.Close()
	session := usageSession(t, usage)

	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "graph_query",
		Arguments: map[string]any{"type": "Machine"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "graph_entity",
		Arguments: map[string]any{"id": "self"},
	}); err != nil {
		t.Fatal(err)
	}

	records := readUsageLog(t, path)
	if len(records) != 2 {
		t.Fatalf("expected one record per tool call, got %d: %+v", len(records), records)
	}
	for _, rec := range records {
		if rec.Timestamp == "" {
			t.Fatalf("record is missing a timestamp: %+v", rec)
		}
		if !rec.OK || rec.Error != "" {
			t.Fatalf("successful call should record ok=true: %+v", rec)
		}
	}
	if records[0].Tool != "graph_query" || records[0].ResultCount == 0 || len(records[0].ResultIDs) == 0 {
		t.Fatalf("query record should carry the returned ids: %+v", records[0])
	}
	if records[1].Tool != "graph_entity" || len(records[1].ResultIDs) != 1 || records[1].ResultIDs[0] != "self" {
		t.Fatalf("entity record should carry the looked-up id: %+v", records[1])
	}
	args, ok := records[0].Arguments.(map[string]any)
	if !ok || args["type"] != "Machine" {
		t.Fatalf("query arguments should be recorded: %+v", records[0].Arguments)
	}
}

func TestUsageLogRecordsFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	usage, err := OpenUsageLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer usage.Close()
	session := usageSession(t, usage)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "graph_entity",
		Arguments: map[string]any{"id": "does-not-exist"},
	})
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	// MCP reports tool-level failures inside the result rather than as a transport
	// error, so the log must still capture them.
	if !res.IsError {
		t.Fatalf("expected a tool-level error for a missing node: %+v", res)
	}

	records := readUsageLog(t, path)
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d: %+v", len(records), records)
	}
	if records[0].OK || records[0].Error == "" {
		t.Fatalf("failed call should record ok=false with an error: %+v", records[0])
	}
}

func TestFeedbackIsOptionalAndRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.jsonl")
	usage, err := OpenUsageLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer usage.Close()
	session := usageSession(t, usage)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "graph_feedback",
		Arguments: map[string]any{
			"useful": true,
			"forId":  "machine/hp/vm128.md",
			"note":   "answered the role question",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out FeedbackOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Recorded {
		t.Fatalf("feedback should be recorded when a log is attached: %+v", out)
	}

	records := readUsageLog(t, path)
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d: %+v", len(records), records)
	}
	rec := records[0]
	if rec.Feedback != "useful" || rec.FeedbackFor != "machine/hp/vm128.md" || rec.Note != "answered the role question" {
		t.Fatalf("feedback record is incomplete: %+v", rec)
	}
}

func TestFeedbackWithoutLogIsNotAnError(t *testing.T) {
	session := usageSession(t, nil)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "graph_feedback",
		Arguments: map[string]any{"useful": false},
	})
	if err != nil {
		t.Fatalf("feedback must stay optional: %v", err)
	}
	raw, _ := json.Marshal(res.StructuredContent)
	var out FeedbackOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Recorded {
		t.Fatalf("no log attached means nothing is recorded: %+v", out)
	}
}

func TestUsageLogDisabledWhenPathEmpty(t *testing.T) {
	usage, err := OpenUsageLog("")
	if err != nil {
		t.Fatalf("blank path must not fail: %v", err)
	}
	if usage != nil {
		t.Fatal("blank path should disable logging")
	}
	if err := usage.Record(UsageRecord{Tool: "graph_query"}); err != nil {
		t.Fatalf("recording on a disabled log must be a no-op: %v", err)
	}
	if err := usage.Close(); err != nil {
		t.Fatalf("closing a disabled log must be a no-op: %v", err)
	}
}
