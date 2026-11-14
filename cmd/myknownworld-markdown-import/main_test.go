package main

import (
	"bytes"
	"encoding/json"
	"github.com/QuietSugar/my-known-world/internal/importer"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIEmitsOnlySanitizedReadOnlyBatchJSON(t *testing.T) {
	root := t.TempDir()
	wiki := filepath.Join(root, "wiki")
	if err := os.MkdirAll(wiki, 0700); err != nil {
		t.Fatal(err)
	}
	doc := []byte("---\ntype: vm\nname: Example\npassword: NEVER-EMIT\n---\nBody text NEVER-READ-IN-OUTPUT\n")
	if err := os.WriteFile(filepath.Join(wiki, "vm.md"), doc, 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "source.yaml")
	if err := os.WriteFile(config, []byte("source: local-wiki\ntypeMappings:\n  vm: Machine\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--root", wiki, "--config", config}, &stdout, &stderr); err != nil {
		t.Fatalf("run failed: %v; stderr=%s", err, stderr.String())
	}
	if strings.Contains(stdout.String(), "NEVER-EMIT") || strings.Contains(stdout.String(), "NEVER-READ-IN-OUTPUT") {
		t.Fatalf("secret/body leaked: %s", stdout.String())
	}
	var report importer.MarkdownReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].ID != "vm.md" || report.Batch.Nodes[0].Type != "Machine" {
		t.Fatalf("unexpected batch: %+v", report.Batch)
	}
	if len(report.Exclusions) != 1 || report.Exclusions[0].Reason != "credential-like key excluded" {
		t.Fatalf("secret exclusion missing: %+v", report.Exclusions)
	}
}
func TestCLIRequiresRootAndPrivateConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(nil, &stdout, &stderr); err == nil {
		t.Fatal("missing flags should fail")
	}
}
