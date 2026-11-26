package materialize

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/importer"
	yaml "github.com/goccy/go-yaml"
)

func timePtr(value time.Time) *time.Time { return &value }

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
	if strings.Contains(text, "ValueHash") || strings.Contains(text, "evidenceKind") {
		t.Fatalf("unsafe or provenance data emitted: %s", text)
	}
	if _, _, err := Export(root, g, Options{}); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}

func TestExportProducesImporterConfigForSupportedRoundTrip(t *testing.T) {
	g := graph.New()
	n := graph.Node{ID: "stable-service-id", Type: graph.SoftwareInstance, Name: "Round trip service", Lifecycle: "stopped", Source: "fixture", ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{ProductName: "Example", Domains: []string{"roundtrip.example.test"}, Ports: []int{8443}, DeploymentPath: "/srv/app", DeploymentMethod: "container", RuntimeIdentity: "app-user", Endpoints: []graph.Endpoint{{Kind: "https", Address: "roundtrip.example.test", Port: 8443}}, ConfigurationReferences: []string{"safe://config-ref"}, DeploymentState: "deployed", OperationalState: "stopped", MaintenanceState: "maintained", VerificationResult: "passed", VerifiedAt: timePtr(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC))}}
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "export")
	if _, _, err := Export(root, g, Options{}); err != nil {
		t.Fatal(err)
	}
	configData, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var config importer.MarkdownConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	report, err := importer.MarkdownFrontmatter(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || len(report.Batch.IdentityMappings) != 1 || report.Batch.IdentityMappings[0].NodeID != n.ID {
		t.Fatalf("unexpected round-trip source batch: %+v", report.Batch)
	}
	got := report.Batch.Nodes[0]
	if got.Name != n.Name || got.Lifecycle != n.Lifecycle || !reflect.DeepEqual(got.SoftwareInstance, n.SoftwareInstance) {
		t.Fatalf("unexpected structured round trip: %+v", got)
	}
}

func TestExportReportsRelationsDroppedByExportFilters(t *testing.T) {
	g := graph.New()
	a := graph.Node{ID: "selected", Type: graph.Product, Name: "Selected", Source: "fixture", ModelVersion: 1}
	b := graph.Node{ID: "excluded", Type: graph.SoftwareInstance, Name: "Excluded", Source: "fixture", ModelVersion: 1}
	if err := g.AddNode(a); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode(b); err != nil {
		t.Fatal(err)
	}
	if err := g.AddRelation(graph.Relation{ID: "cross-filter-edge", From: a.ID, Predicate: graph.HasInstance, To: b.ID, Source: "fixture"}); err != nil {
		t.Fatal(err)
	}
	_, report, err := Export(filepath.Join(t.TempDir(), "filtered"), g, Options{Types: map[graph.NodeType]bool{graph.Product: true}})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(report.Unsupported, "\\n")
	if !strings.Contains(joined, "cross-filter-edge") || !strings.Contains(joined, "excluded by export filters") {
		t.Fatalf("filtered relation omission not reported: %v", report.Unsupported)
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
	if !strings.Contains(string(body), "predicate: hasInstance") || !strings.Contains(string(body), "../SoftwareInstance/") || !strings.Contains(string(body), "id: instance-edge") {
		t.Fatalf("supported relation missing from frontmatter: %s", body)
	}
	configBytes, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var config importer.MarkdownConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := importer.MarkdownFrontmatter(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.Batch.Relations) != 1 || roundTrip.Batch.Relations[0].ID != "instance-edge" || roundTrip.Batch.Relations[0].Predicate != graph.HasInstance {
		t.Fatalf("relation was not round-tripped: %+v", roundTrip.Batch.Relations)
	}
}

func TestExportRecursivelyOmitsSecretLikeDetailValues(t *testing.T) {
	g := graph.New()
	node := graph.Node{ID: "hardware-id", Type: graph.Hardware, Name: "Synthetic", ModelVersion: 1, Hardware: &graph.HardwareDetails{Specifications: map[string]string{"cpu": "four-core", "api_token": "synthetic-secret-value", "notes": "password=embedded-synthetic-value"}}}
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
	if strings.Contains(string(data), "synthetic-secret-value") || strings.Contains(string(data), "embedded-synthetic-value") || !strings.Contains(string(data), "four-core") {
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

func TestExportKeepsNumericFieldsAsIntegers(t *testing.T) {
	notes := []string{}
	cleaned := sanitizeValue(445, "ports[0]", &notes)
	if _, isFloat := cleaned.(float64); isFloat {
		t.Fatalf("integer input must not become a float: %T", cleaned)
	}
	// JSON decoding produces float64 for whole numbers; these must round-trip to int.
	fromJSON := float64(445)
	cleaned = sanitizeValue(fromJSON, "ports[0]", &notes)
	if got, ok := cleaned.(int64); !ok || got != 445 {
		t.Fatalf("whole float from JSON should become int64 445, got %T %v", cleaned, cleaned)
	}
	if got := sanitizeValue(1.5, "ratio", &notes); got != 1.5 {
		t.Fatalf("fractional values must be preserved: %v", got)
	}
}

// TestExportUsesReadableFolderPaths guards the folder-browsable layout: files
// are named after the entity, not a hash, and the root has an index that links
// to every one of them.
func TestExportUsesReadableFolderPaths(t *testing.T) {
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []graph.Node{
		{ID: "aaaaaaaa-1111-4111-8111-111111111111", Type: graph.SoftwareInstance, Name: "homer", Lifecycle: "active", Source: "fixture", ModelVersion: 1,
			SoftwareInstance: &graph.SoftwareInstanceDetails{Ports: []int{10090}}},
		{ID: "bbbbbbbb-2222-4222-8222-222222222222", Type: graph.SoftwareInstance, Name: "homer", Lifecycle: "active", Source: "fixture", ModelVersion: 1,
			SoftwareInstance: &graph.SoftwareInstanceDetails{Ports: []int{10190}}},
		{ID: "cccccccc-3333-4333-8333-333333333333", Type: graph.Machine, Name: "vm247 内部测试机", Lifecycle: "active", Source: "fixture", ModelVersion: 1,
			Machine: &graph.MachineDetails{Hostname: "vm247"}},
	} {
		if err := g.AddNode(n); err != nil {
			t.Fatal(err)
		}
	}

	root := filepath.Join(t.TempDir(), "export")
	manifest, _, err := Export(root, g, Options{})
	if err != nil {
		t.Fatal(err)
	}

	// Paths carry the readable name plus a short id, never a bare hash.
	for _, n := range manifest.Nodes {
		if !strings.Contains(n.Path, n.Name[:4]) && n.Name != "Self" {
			t.Errorf("path %q does not contain the entity name %q", n.Path, n.Name)
		}
		if !strings.HasSuffix(n.Path, ".md") {
			t.Errorf("path %q is not markdown", n.Path)
		}
	}
	// Two nodes sharing a name must not collide.
	seen := map[string]bool{}
	for _, n := range manifest.Nodes {
		if seen[n.Path] {
			t.Fatalf("duplicate path %q", n.Path)
		}
		seen[n.Path] = true
	}
	// The index exists and every link resolves.
	indexPath := filepath.Join(root, "README.md")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("index missing: %v", err)
	}
	for _, n := range manifest.Nodes {
		if !strings.Contains(string(index), n.Path) {
			t.Errorf("index does not link to %q", n.Path)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(n.Path))); err != nil {
			t.Errorf("linked file missing: %q", n.Path)
		}
	}
}
