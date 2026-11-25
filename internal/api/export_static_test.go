package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestProjectExportStaticRoute(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mkw-static-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("STATIC_EXPORT_DIR", tempDir)
	defer os.Unsetenv("STATIC_EXPORT_DIR")

	g := graph.New()
	proj := graph.Node{
		ID:           "projects/test-proj",
		Type:         graph.SoftwareProject,
		Name:         "Test Project",
		ModelVersion: 1,
		SoftwareProject: &graph.SoftwareProjectDetails{
			Purpose: "Testing static export",
		},
	}
	if err := g.AddNode(proj); err != nil {
		t.Fatalf("add project: %v", err)
	}

	cred := graph.Node{
		ID:           "credentials/test-cred",
		Type:         graph.AccountCredential,
		Name:         "Test Credential",
		ModelVersion: 1,
		AccountCredential: &graph.AccountCredentialDetails{
			Account: "admin",
		},
	}
	if err := g.AddNode(cred); err != nil {
		t.Fatalf("add credential: %v", err)
	}

	inst := graph.Node{
		ID:           "services/test-inst",
		Type:         graph.SoftwareInstance,
		Name:         "Test Service",
		ModelVersion: 1,
		SoftwareInstance: &graph.SoftwareInstanceDetails{
			Domains: []string{"test.example.com"},
			Ports:   []int{8080},
		},
	}
	if err := g.AddNode(inst); err != nil {
		t.Fatalf("add inst: %v", err)
	}

	rel1 := graph.Relation{
		ID:        "rel-1",
		From:      inst.ID,
		Predicate: graph.ImplementedFrom,
		To:        proj.ID,
	}
	if err := g.AddRelation(rel1); err != nil {
		t.Fatalf("add rel1: %v", err)
	}

	rel2 := graph.Relation{
		ID:        "rel-2",
		From:      inst.ID,
		Predicate: graph.HasCredential,
		To:        cred.ID,
	}
	if err := g.AddRelation(rel2); err != nil {
		t.Fatalf("add rel2: %v", err)
	}

	handler := Handler{
		Graph: g,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/workspace/projects/projects%2Ftest-proj/export-static", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal resp: %v", err)
	}

	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}

	expectedIndex := filepath.Join(tempDir, "projects", "test-proj", "index.html")
	if _, err := os.Stat(expectedIndex); err != nil {
		t.Fatalf("expected index.html at %s, got: %v", expectedIndex, err)
	}

	content, err := os.ReadFile(expectedIndex)
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}

	if len(content) == 0 {
		t.Fatalf("expected index.html not to be empty")
	}
}
