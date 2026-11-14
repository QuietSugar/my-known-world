package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyRequiresExactSourceConfirmation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch.json")
	if err := os.WriteFile(path, []byte(`{"source":"safe-source","nodes":[],"relations":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	getenv := func(string) string { return "" }
	var out, stderr strings.Builder
	err := run(context.Background(), []string{"-input", path, "-apply"}, getenv, &out, &stderr)
	if err == nil || !strings.Contains(err.Error(), `-confirm-source "safe-source"`) {
		t.Fatalf("expected confirmation guard, got %v", err)
	}
}

func TestRejectsUnknownFieldsAndOversizedBatch(t *testing.T) {
	getenv := func(string) string { return "" }
	path := filepath.Join(t.TempDir(), "batch.json")
	if err := os.WriteFile(path, []byte(`{"source":"x","surprise":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr strings.Builder
	if err := run(context.Background(), []string{"-input", path}, getenv, &out, &stderr); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field should fail closed: %v", err)
	}
	large := make([]byte, maxBatchBytes+1)
	for i := range large {
		large[i] = ' '
	}
	if err := os.WriteFile(path, large, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(context.Background(), []string{"-input", path}, getenv, &out, &stderr); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized input should fail: %v", err)
	}
}
