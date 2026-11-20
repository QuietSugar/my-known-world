package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/api"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/materialize"
)

func TestMaterializePagedMatchesDirectExport(t *testing.T) {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	for i := 0; i < 45; i++ {
		id := fmt.Sprintf("host-%02d", i)
		_ = g.AddNode(graph.Node{
			ID:           id,
			Type:         graph.Machine,
			Name:         fmt.Sprintf("Host %02d", i),
			ModelVersion: 1,
			Source:       "test-source",
			Machine:      &graph.MachineDetails{Hostname: fmt.Sprintf("%s.internal", id)},
		})
		_ = g.AddRelation(graph.Relation{
			ID:        fmt.Sprintf("knows-%02d", i),
			From:      "self",
			Predicate: graph.KnowsAbout,
			To:        id,
			Source:    "test-source",
		})
	}

	handler := api.Handler{Graph: g}
	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()

	// 1. Fetch via fetchAllPaged (with smaller internal pages to ensure multiple trips)
	pagedNodes, err := fetchAllPaged[graph.Node](client, server.URL, "", "/api/nodes")
	if err != nil {
		t.Fatal(err)
	}
	if len(pagedNodes) != 46 { // self + 45
		t.Fatalf("expected 46 nodes, got %d", len(pagedNodes))
	}
	pagedRels, err := fetchAllPaged[graph.Relation](client, server.URL, "", "/api/relations")
	if err != nil {
		t.Fatal(err)
	}
	if len(pagedRels) != 45 {
		t.Fatalf("expected 45 relations, got %d", len(pagedRels))
	}

	// 2. Reconstruct graph from paged results
	reconstructed := graph.New()
	for _, n := range pagedNodes {
		if err := reconstructed.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range pagedRels {
		if err := reconstructed.AddRelation(r); err != nil {
			t.Fatal(err)
		}
	}

	// 3. Export both to temp directories and verify byte-identical output
	dirDirect := t.TempDir()
	dirPaged := t.TempDir()

	opts := materialize.Options{Overwrite: true}
	manifestDirect, _, err := materialize.Export(dirDirect, g, opts)
	if err != nil {
		t.Fatal(err)
	}
	manifestPaged, _, err := materialize.Export(dirPaged, reconstructed, opts)
	if err != nil {
		t.Fatal(err)
	}

	if len(manifestDirect.Nodes) != len(manifestPaged.Nodes) || len(manifestDirect.Relations) != len(manifestPaged.Relations) {
		t.Fatalf("manifest length mismatch: %d vs %d", len(manifestDirect.Nodes), len(manifestPaged.Nodes))
	}

	// Check file contents
	err = filepath.Walk(dirDirect, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dirDirect, path)
		if err != nil {
			return err
		}
		directBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		pagedPath := filepath.Join(dirPaged, rel)
		pagedBytes, err := os.ReadFile(pagedPath)
		if err != nil {
			return fmt.Errorf("missing paged file %s: %w", rel, err)
		}
		if string(directBytes) != string(pagedBytes) {
			return fmt.Errorf("file %s is not byte-identical", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
