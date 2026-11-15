package api

import (
	"encoding/json"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/importer"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarkdownAdapterBatchPassesSyncPreviewEndpoint(t *testing.T) {
	report, err := importer.MarkdownFrontmatter("../importer/testdata/wiki", importer.MarkdownConfig{Source: "synthetic-wiki", TypeMappings: map[string]graph.NodeType{"hardware": graph.Hardware, "vm": graph.Machine, "rig": graph.Machine, "shared": graph.Machine, "service": graph.SoftwareInstance}, MachineKindMappings: map[string]string{"vm": "virtual", "shared": "shared"}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report.Batch)
	if err != nil {
		t.Fatal(err)
	}
	h := Handler{Graph: fixture()}
	req := httptest.NewRequest(http.MethodPost, "/api/sync/preview", strings.NewReader(string(raw)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("preview returned %d: %s", w.Code, w.Body.String())
	}
	for _, id := range []string{"machine/example-vm.md", "machine/shared.md", "notes/README.md"} {
		if !strings.Contains(w.Body.String(), id) {
			t.Fatalf("preview missing expected source ID %q: %s", id, w.Body.String())
		}
	}
	if !strings.Contains(w.Body.String(), "notes/README.md") {
		t.Fatal("unmapped candidate is not previewed")
	}
	if strings.Contains(w.Body.String(), "SYNTHETIC-NEVER-EMIT") {
		t.Fatal("credential leaked into preview")
	}
}
