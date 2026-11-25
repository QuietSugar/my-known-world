package materialize

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
	"github.com/QuietSugar/my-known-world/internal/importer"
	yaml "github.com/goccy/go-yaml"
)

func TestEverySupportedTypedModelRoundTripsThroughGeneratedImportConfig(t *testing.T) {
	stamp := time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
	nodes := []graph.Node{
		{ID: "hardware-id", Type: graph.Hardware, Name: "Hardware", Lifecycle: "active", ModelVersion: 1, Hardware: &graph.HardwareDetails{Category: "server", Model: "X", Location: "rack", Specifications: map[string]string{"cpu": "12", "memory": "48G"}, Status: "online", AccessReference: "safe://access", MaintenanceReference: "safe://maint"}},
		{ID: "machine-id", Type: graph.Machine, Name: "Machine", Lifecycle: "active", ModelVersion: 1, Machine: &graph.MachineDetails{MachineKind: "physical", Owned: []string{"app"}, Hostname: "host.example.test", OS: "Linux", IPs: []string{"192.0.2.12"}, Network: "private", Status: "ready", AccessMethod: "safe://ssh", AccessPrerequisites: []string{"approval"}, AllowedOperations: []string{"inspect"}, OperationConstraints: []string{"read-only"}, ForbiddenOperations: []string{"reboot"}, ResponsibilityBoundary: "platform", Resources: []graph.ResourceClaim{{Kind: "port", Name: "https", Scope: "internal", Port: 443}}, LastVerified: &stamp}},
		{ID: "project-id", Type: graph.SoftwareProject, Name: "Project", Lifecycle: "maintained", ModelVersion: 1, SoftwareProject: &graph.SoftwareProjectDetails{Purpose: "test", Languages: []string{"Go", "TypeScript"}, Form: "service", RemoteRepository: "https://example.test/repo", Upstream: "https://example.test/upstream", MaintenanceState: "active", RecommendedVersion: "1.2", SuccessorID: "next"}},
		{ID: "service-id", Type: graph.SoftwareInstance, Name: "Service", Lifecycle: "active", ModelVersion: 1, SoftwareInstance: &graph.SoftwareInstanceDetails{ProductName: "app", Domains: []string{"app.example.test"}, Ports: []int{443}, DeploymentPath: "/srv/app", DeploymentMethod: "container", RuntimeIdentity: "app", Endpoints: []graph.Endpoint{{Kind: "https", Address: "app.example.test", Port: 443}}, ConfigurationReferences: []string{"safe://config"}, DeploymentState: "deployed", OperationalState: "healthy", MaintenanceState: "maintained", VerificationResult: "passed", VerifiedAt: &stamp}},
		{ID: "credential-id", Type: graph.AccountCredential, Name: "Account metadata", Lifecycle: "active", ModelVersion: 1, AccountCredential: &graph.AccountCredentialDetails{Account: "svc-account", CredentialType: "vault-entry", Purpose: "runtime", Scope: "app", AccessReference: "vault://opaque-entry", RotationAt: &stamp, Status: "enabled", LastVerified: &stamp}},
		{ID: "product-id", Type: graph.Product, Name: "Product", Lifecycle: "active", ModelVersion: 1, Product: &graph.ProductDetails{Purpose: "test", CommonConfigurationReference: "safe://common", RunbookReference: "safe://runbook"}},
		{ID: "document-id", Type: graph.Document, Name: "Design", Lifecycle: "draft", ModelVersion: 1, Document: &graph.DocumentDetails{Role: "design", AuthoritativeReference: "safe://design", Effective: true, LastVerified: &stamp, SupersededBy: "next"}},
		{ID: "runbook-id", Type: graph.Runbook, Name: "Runbook", Lifecycle: "ready", ModelVersion: 1, Runbook: &graph.RunbookDetails{Scope: "service", Trigger: "alert", Preconditions: []string{"approved"}, StepsReference: "safe://steps", StopConditionReference: "safe://stop", RollbackReference: "safe://rollback", VerificationMethod: "health check", LastVerified: &stamp, ExecutableStatus: "reviewed", ApprovalRequired: true}},
		{ID: "task-id", Type: graph.WorkItem, Name: "Task", Lifecycle: "open", ModelVersion: 1, WorkItem: &graph.WorkItemDetails{Kind: "change", Status: "open", RationaleReference: "safe://why", DecisionEvidenceReference: "safe://decision"}},
		{ID: "network-id", Type: graph.Network, Name: "Network", Lifecycle: "active", ModelVersion: 1, Network: &graph.NetworkDetails{Kind: "lan", CIDRs: []string{"192.0.2.0/24"}, GatewayReference: "safe://gateway", AccessLocations: []string{"office"}, VPNReference: "safe://vpn", Status: "ready"}},
	}
	g := graph.New()
	for _, node := range nodes {
		if err := g.AddNode(node); err != nil {
			t.Fatalf("add %s: %v", node.ID, err)
		}
	}
	for _, relation := range []graph.Relation{{ID: "hosts-edge", From: "machine-id", Predicate: graph.Hosts, To: "service-id"}, {ID: "credential-edge", From: "service-id", Predicate: graph.HasCredential, To: "credential-id"}, {ID: "instance-edge", From: "product-id", Predicate: graph.HasInstance, To: "service-id"}} {
		if err := g.AddRelation(relation); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(t.TempDir(), "generated")
	manifest, _, err := Export(root, g, Options{})
	if err != nil {
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
	imported, err := importer.MarkdownFrontmatter(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Batch.Nodes) != len(nodes) {
		t.Fatalf("nodes imported=%d want=%d exclusions=%+v", len(imported.Batch.Nodes), len(nodes), imported.Exclusions)
	}
	// identityMappings now apply during import, so imported nodes already carry
	// the stable ID; no reverse lookup through the file path is needed.
	byStableID := map[string]graph.Node{}
	for _, node := range imported.Batch.Nodes {
		byStableID[node.ID] = node
	}
	for _, want := range nodes {
		got, ok := byStableID[want.ID]
		if !ok {
			t.Errorf("stable ID mapping missing for %s", want.ID)
			continue
		}
		if got.Type != want.Type || got.Name != want.Name || got.Lifecycle != want.Lifecycle {
			t.Errorf("base fields changed for %s: got=%+v", want.ID, got)
			continue
		}
		// Import evidence/source defaults are intentionally different; compare all actual type-specific detail payloads.
		wantDetails, _ := want.MarshalModelDetails()
		gotDetails, _ := got.MarshalModelDetails()
		if string(wantDetails) != string(gotDetails) {
			t.Errorf("typed details changed for %s: want=%s got=%s", want.ID, wantDetails, gotDetails)
		}
	}
	if len(imported.Batch.Relations) != 3 {
		t.Fatalf("relations imported=%d want=3 exclusions=%+v", len(imported.Batch.Relations), imported.Exclusions)
	}
	stableBySource := map[string]string{}
	for _, mapping := range manifest.IdentityMappings {
		stableBySource[mapping.SourceID] = mapping.NodeID
	}
	for _, relation := range imported.Batch.Relations {
		var expected *ManifestRelation
		for i := range manifest.Relations {
			if manifest.Relations[i].ID == relation.ID {
				expected = &manifest.Relations[i]
				break
			}
		}
		if expected == nil {
			t.Errorf("relation identity %s absent from manifest", relation.ID)
			continue
		}
		// Imported relations already carry stable IDs; fall back to the path map
		// only if a legacy importer left a file path in place.
		from, to := relation.From, relation.To
		if mapped, ok := stableBySource[from]; ok {
			from = mapped
		}
		if mapped, ok := stableBySource[to]; ok {
			to = mapped
		}
		if from != expected.From || to != expected.To || relation.Predicate != expected.Predicate {
			t.Errorf("relation changed: imported=%+v manifest=%+v", relation, *expected)
		}
	}
}
