package mcpapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UsageLog appends one JSON line per MCP tool call so a later offline pass can
// analyse which entities were actually useful. It only records; it never
// aggregates, scores, or feeds anything back into query results.
type UsageLog struct {
	mu   sync.Mutex
	path string
	file *os.File
}

// UsageRecord is one appended line. Fields are deliberately flat and boring so
// that an external analyser can read them without knowing this package.
type UsageRecord struct {
	Timestamp   string   `json:"ts"`
	Tool        string   `json:"tool"`
	Caller      string   `json:"caller,omitempty"`
	Arguments   any      `json:"args,omitempty"`
	ResultIDs   []string `json:"resultIds,omitempty"`
	ResultCount int      `json:"resultCount,omitempty"`
	DurationMs  int64    `json:"durationMs"`
	OK          bool     `json:"ok"`
	Error       string   `json:"error,omitempty"`
	// Feedback is the caller's own judgement of whether the result was useful,
	// supplied voluntarily through graph_feedback.
	Feedback    string `json:"feedback,omitempty"`
	FeedbackFor string `json:"feedbackFor,omitempty"`
	Note        string `json:"note,omitempty"`
}

// OpenUsageLog opens the append-only usage log at path, creating parent
// directories. A blank path disables logging and returns nil.
func OpenUsageLog(path string) (*UsageLog, error) {
	if path == "" {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create usage log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open usage log: %w", err)
	}
	return &UsageLog{path: path, file: file}, nil
}

// Record appends rec. Logging must never break a query, so callers treat a
// returned error as advisory; the in-memory server keeps serving regardless.
func (l *UsageLog) Record(rec UsageRecord) error {
	if l == nil {
		return nil
	}
	if rec.Timestamp == "" {
		rec.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// Close releases the underlying file.
func (l *UsageLog) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Path reports the log location, for tests and diagnostics.
func (l *UsageLog) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
