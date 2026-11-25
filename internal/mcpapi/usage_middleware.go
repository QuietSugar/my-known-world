package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// usageMiddleware records every tools/call after it completes. It never blocks
// a response on a logging failure: the graph keeps serving even if the log
// volume is full or read-only.
func usageMiddleware(usage *UsageLog) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if usage == nil || method != "tools/call" {
				return next(ctx, method, req)
			}
			start := time.Now()
			result, err := next(ctx, method, req)

			rec := UsageRecord{
				Tool:       toolName(req),
				Arguments:  toolArguments(req),
				DurationMs: time.Since(start).Milliseconds(),
				OK:         err == nil,
			}
			if err != nil {
				rec.Error = err.Error()
			} else {
				rec.ResultIDs, rec.ResultCount = summarizeResult(result)
				// MCP carries tool-level failures inside the result, not as a
				// transport error, so a failed lookup must be recorded as such.
				if call, ok := result.(*mcp.CallToolResult); ok && call != nil && call.IsError {
					rec.OK = false
					rec.Error = toolErrorText(call)
				}
			}
			// graph_feedback carries the caller's own usefulness verdict; fold it
			// into this single line instead of writing a second record.
			if rec.Tool == "graph_feedback" {
				if in, ok := feedbackFields(req); ok {
					rec.Feedback = FeedbackVerdict(in)
					rec.FeedbackFor = strings.TrimSpace(in.ForID)
					note := strings.TrimSpace(in.Note)
					if len(note) > 500 {
						note = note[:500]
					}
					rec.Note = note
				}
			}
			_ = usage.Record(rec)
			return result, err
		}
	}
}

func toolName(req mcp.Request) string {
	type named interface{ GetName() string }
	if r, ok := req.(*mcp.CallToolRequest); ok && r.Params != nil {
		return r.Params.Name
	}
	if n, ok := req.(named); ok {
		return n.GetName()
	}
	return ""
}

// toolArguments decodes the raw arguments into a generic value so the log stays
// readable without coupling to each tool's input struct.
func toolArguments(req mcp.Request) any {
	r, ok := req.(*mcp.CallToolRequest)
	if !ok || len(r.Params.Arguments) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(r.Params.Arguments, &out); err != nil {
		return nil
	}
	return out
}

// toolErrorText extracts a readable message from an error result.
func toolErrorText(call *mcp.CallToolResult) string {
	for _, c := range call.Content {
		if text, ok := c.(*mcp.TextContent); ok && text != nil {
			if msg := strings.TrimSpace(text.Text); msg != "" {
				return msg
			}
		}
	}
	return "tool reported an error"
}

// summarizeResult extracts the node IDs a lookup returned, so later analysis can
// answer "which entities were actually pulled" without parsing every payload.
func summarizeResult(result mcp.Result) ([]string, int) {
	call, ok := result.(*mcp.CallToolResult)
	if !ok || call == nil {
		return nil, 0
	}
	raw, err := json.Marshal(call.StructuredContent)
	if err != nil || len(raw) == 0 {
		return nil, 0
	}
	var shape struct {
		Items []struct {
			ID   string `json:"id"`
			Node struct {
				ID string `json:"id"`
			} `json:"node"`
			Count int `json:"count"`
		} `json:"items"`
		Count int `json:"count"`
		Node  struct {
			ID string `json:"id"`
		} `json:"node"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return nil, 0
	}
	ids := make([]string, 0, len(shape.Items))
	for _, item := range shape.Items {
		switch {
		case item.ID != "":
			ids = append(ids, item.ID)
		case item.Node.ID != "":
			ids = append(ids, item.Node.ID)
		}
	}
	if shape.Node.ID != "" {
		ids = append(ids, shape.Node.ID)
	}
	count := shape.Count
	if count == 0 {
		count = len(ids)
	}
	return ids, count
}

// FeedbackInput is the voluntary usefulness report attached to a lookup.
type FeedbackInput struct {
	Useful bool   `json:"useful"`
	ForID  string `json:"forId,omitempty"`
	Note   string `json:"note,omitempty"`
	Tool   string `json:"tool,omitempty"`
}

type FeedbackOutput struct {
	Recorded bool   `json:"recorded"`
	Verdict  string `json:"verdict,omitempty"`
}

// feedbackFields decodes a graph_feedback call's arguments for the usage log.
func feedbackFields(req mcp.Request) (FeedbackInput, bool) {
	raw, ok := toolArguments(req).(map[string]any)
	if !ok {
		return FeedbackInput{}, false
	}
	in := FeedbackInput{}
	if useful, ok := raw["useful"].(bool); ok {
		in.Useful = useful
	}
	if v, ok := raw["forId"].(string); ok {
		in.ForID = v
	}
	if v, ok := raw["note"].(string); ok {
		in.Note = v
	}
	return in, true
}

func (s *Server) feedback(_ context.Context, _ *mcp.CallToolRequest, in FeedbackInput) (*mcp.CallToolResult, FeedbackOutput, error) {
	if s.usage == nil {
		// Not an error: feedback is optional and a server may run without a log.
		return nil, FeedbackOutput{Recorded: false}, nil
	}
	// The usage middleware appends the line for every tool call; the verdict rides
	// along in the structured output so there is exactly one record per call.
	return nil, FeedbackOutput{Recorded: true, Verdict: FeedbackVerdict(in)}, nil
}

// FeedbackVerdict renders a caller's judgement in a stable, analysable word.
func FeedbackVerdict(in FeedbackInput) string {
	if in.Useful {
		return "useful"
	}
	return "not_useful"
}
