package materialize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestExportWritesDeterministicSafeCopiesAndRefusesOverwrite(t *testing.T) {
	g := graph.New()
	n := graph.Node{ID: "private/source/id", Type: graph.SoftwareInstance, Name: "Synthetic service", Lifecycle: "stopped", Source: "fixture", ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{Domains: []string{"service.example.test"}, Ports: []int{8080}, ConfigurationReferences: []string{"vault://nonsecret-pointer"}}}
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "export")
	manifest, report, err := Export(root, g, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Nodes) != 1 || manifest.Nodes[0].ID != n.ID {
		t.Fatalf("manifest: %+v", manifest)
	}
	if len(report.Unsupported) == 0 {
		t.Fatal("expected explicit unsupported body/history report")
	}
	path := filepath.Join(root, filepath.FromSlash(manifest.Nodes[0].Path))
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{"Synthetic service", "service.example.test", "8080", "stopped"} {
		if !strings.Contains(text, want) {
			t.Errorf("materialization lacks %q: %s", want, text)
		}
	}
	if strings.Contains(text, "vault://") || strings.Contains(text, "ValueHash") || strings.Contains(text, "evidenceKind") {
		t.Fatalf("unsafe or provenance data emitted: %s", text)
	}
	if _, _, err := Export(root, g, Options{}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}

func TestExportMaterializesSupportedRelationsAndStableIdentityMappings(t *testing.T) {
	g := graph.New()
	product := graph.Node{ID: "product-id", Type: graph.Product, Name: "Synthetic product", Source: "fixture", ModelVersion: 1, Product: &graph.ProductDetails{Purpose: "fixture"}}
	service := graph.Node{ID: "service-id", Type: graph.SoftwareInstance, Name: "Synthetic service", Source: "fixture", ModelVersion: 1}
	if err := g.AddNode(product); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(service); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "instance-edge", From: product.ID, Predicate: graph.HasInstance, To: service.ID, Source: "fixture"}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "export")
	manifest, _, err := Export(root, g, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.IdentityMappings) != 2 || len(manifest.Relations) != 1 {
		t.Fatalf("manifest missing identity/relation records: %+v", manifest)
	}
	productPath := filepath.Join(root, filepath.FromSlash(manifest.IdentityMappings[0].SourceID))
	for _, mapping := range manifest.IdentityMappings {
		if mapping.NodeID == product.ID {
			productPath = filepath.Join(root, filepath.FromSlash(mapping.SourceID))
		}
	}
	body, err := os.ReadFile(productPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "instances:") || !strings.Contains(string(body), "../SoftwareInstance/") {
		t.Fatalf("supported relation missing from frontmatter: %s", body)
	}
}

func TestExportRecursivelyOmitsSecretLikeDetailValues(t *testing.T) {
	g := graph.New()
	node := graph.Node{ID: "hardware-id", Type: graph.Hardware, Name: "Synthetic", ModelVersion: 1, Hardware: &graph.HardwareDetails{Specifications: map[string]string{"cpu": "four-core", "api_token": "synthetic-secret-value"}}}
	if err := g.AddNode(node); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "safe")
	manifest, _, err := Export(root, g, Options{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(manifest.Nodes[0].Path)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "synthetic-secret-value") || !strings.Contains(string(data), "four-core") {
		t.Fatalf("recursive secret filtering failed: %s", data)
	}
}

func TestSecretKeyClassificationCoversCommonSecretNames(t *testing.T) {
	for _, key := range []string{"password", "api_key", "private-key", "secretToken", "credential_value"} {
		if !isSecretKey(key) {
			t.Errorf("did not classify %q as secret-like", key)
		}
	}
	for _, key := range []string{"purpose", "status", "domain", "credentialType", "accessReference"} {
		if isSecretKey(key) {
			t.Errorf("classified ordinary field %q as secret-like", key)
		}
	}
}
