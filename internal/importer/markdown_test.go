package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
	yaml "github.com/goccy/go-yaml"
)

func markdownConfig() MarkdownConfig {
	return MarkdownConfig{Source: "wiki:private-test", TypeMappings: map[string]graph.NodeType{"hardware": graph.Hardware, "vm": graph.Machine, "rig": graph.Machine, "shared": graph.Machine, "service": graph.SoftwareInstance, "product": graph.Product, "task": graph.WorkItem, "runbook": graph.Runbook, "software_project": graph.SoftwareProject, "document": graph.Document, "network": graph.Network, "credential": graph.AccountCredential}, MachineKindMappings: map[string]string{"vm": "virtual", "shared": "shared"}}
}

func TestHallMCPReplacementCorpusAdmission(t *testing.T) {
	base := filepath.Join("testdata", "hall-mcp-replacement")
	configBytes, err := os.ReadFile(filepath.Join(base, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg MarkdownConfig
	if err := yaml.Unmarshal(configBytes, &cfg); err != nil {
		t.Fatal(err)
	}
	valid, err := MarkdownFrontmatter(filepath.Join(base, "wiki"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(valid.Batch.Nodes) != 10 || len(valid.Batch.Relations) != 16 {
		t.Fatalf("valid corpus batch nodes=%d relations=%d, want 10 and 16", len(valid.Batch.Nodes), len(valid.Batch.Relations))
	}
	invalid, err := MarkdownFrontmatter(filepath.Join(base, "invalid"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(invalid.Batch.Nodes) != 2 || len(invalid.Batch.Relations) != 0 {
		t.Fatalf("invalid corpus batch nodes=%d relations=%d, want 2 support nodes and no relations", len(invalid.Batch.Nodes), len(invalid.Batch.Relations))
	}
	if len(invalid.Exclusions) < 5 {
		t.Fatalf("invalid corpus produced only %d actionable exclusions", len(invalid.Exclusions))
	}
	for _, excluded := range []string{"services/missing-status.md", "services/wrong-host-type.md", "services/broken-host.md", "machine/empty-shell.md", "designs/invalid-status.md"} {
		found := false
		for _, exclusion := range invalid.Exclusions {
			if exclusion.SourcePath == excluded && strings.TrimSpace(exclusion.Reason) != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("missing actionable exclusion for %s", excluded)
		}
	}
}

func assertionPresent(assertions []graph.FieldAssertion, nodeID, fieldPath string) bool {
	for _, assertion := range assertions {
		if assertion.NodeID == nodeID && assertion.FieldPath == fieldPath {
			return true
		}
	}
	return false
}

func TestMarkdownFrontmatterCreatesSafeRepeatableCompleteBatch(t *testing.T) {
	root := filepath.Join("testdata", "wiki")
	a, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	b, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("unchanged wiki must produce byte-stable output")
	}
	if len(a.Batch.Nodes) != 4 {
		t.Fatalf("expected 4 frontmatter documents, got %d", len(a.Batch.Nodes))
	}
	nodes := map[string]graph.Node{}
	for _, n := range a.Batch.Nodes {
		nodes[n.ID] = n
	}
	vm, ok := nodes["machine/example-vm.md"]
	if !ok || vm.Type != graph.Machine || vm.Machine.MachineKind != "virtual" || len(vm.Machine.IPs) != 1 || vm.Machine.IPs[0] != "192.0.2.10" {
		t.Fatalf("machine fields lost: %+v", vm)
	}
	shared := nodes["machine/shared.md"]
	if shared.Machine.MachineKind != "shared" || len(shared.Machine.OperationConstraints) != 2 || len(shared.Machine.Owned) != 2 || shared.Machine.Owned[0] != "application deployments" {
		t.Fatalf("shared machine constraints lost: %+v", shared.Machine)
	}
	candidate := nodes["notes/README.md"]
	if candidate.Type != graph.ProvisionalObject || candidate.ReviewStatus != "pending" {
		t.Fatalf("unmapped type must remain provisional: %+v", candidate)
	}
	if len(a.Batch.Relations) != 1 || a.Batch.Relations[0].From != "hardware/example-host.md" || a.Batch.Relations[0].To != "machine/example-vm.md" || a.Batch.Relations[0].EvidenceKind != graph.EvidenceDocument {
		t.Fatalf("expected only typed frontmatter host relation: %+v", a.Batch.Relations)
	}
	if len(a.Exclusions) < 3 {
		t.Fatalf("expected exclusions for missing/unclosed frontmatter and unmapped type: %+v", a.Exclusions)
	}
	raw := string(ja)
	for _, forbidden := range []string{"body-secret-never-read", "body-password-must-not-leak", "SYNTHETIC-NEVER-EMIT", "Never index this prose", "This prose must not be imported"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("Markdown body or credential leaked in report: %q", forbidden)
		}
	}
	if !strings.Contains(raw, "credential-like key excluded") {
		t.Fatal("credential-like frontmatter exclusion was not reported")
	}
	if len(a.Batch.Assertions) == 0 {
		t.Fatal("mapped safe fields should have field assertions")
	}
	if !assertionPresent(a.Batch.Assertions, "machine/example-vm.md", "lifecycle") {
		t.Fatal("frontmatter status must create a lifecycle field assertion")
	}
	if !assertionPresent(a.Batch.Assertions, "machine/shared.md", "machine.owned") {
		t.Fatal("shared-machine owned field must create typed evidence")
	}
	for _, as := range a.Batch.Assertions {
		if as.EvidenceKind != graph.EvidenceDocument || as.Reference == "" {
			t.Fatalf("assertion provenance missing: %+v", as)
		}
	}
}

func TestMarkdownRigRetainsDistinctMachineKind(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rig.md"), []byte("---\ntype: rig\nname: Synthetic rig\nhostname: rig.example.test\n---\n"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].Machine == nil || report.Batch.Nodes[0].Machine.MachineKind != "rig" {
		t.Fatalf("rig should retain a distinct machine kind: %+v", report.Batch.Nodes)
	}
	if !assertionPresent(report.Batch.Assertions, "rig.md", "machine.machineKind") {
		t.Fatal("rig machine kind should have typed evidence")
	}
}

func TestMarkdownMachineRetainsRole(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: rig\nname: Synthetic dev box\nhostname: devbox.example.test\nrole: Development\n---\nbody ignored\n"
	if err := os.WriteFile(filepath.Join(root, "devbox.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].Machine == nil {
		t.Fatalf("expected one imported machine node: %+v", report.Batch.Nodes)
	}
	roles := report.Batch.Nodes[0].Machine.Roles
	if len(roles) != 1 || roles[0] != "development" {
		t.Fatalf("role should be normalized and retained: %+v", roles)
	}
	if !assertionPresent(report.Batch.Assertions, "devbox.md", "machine.roles") {
		t.Fatal("machine role should have typed evidence")
	}
}

func TestMarkdownMachineAcceptsRoleList(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: rig\nname: Synthetic dual box\nhostname: dual.example.test\nroles:\n  - development\n  - deployment\n---\n"
	if err := os.WriteFile(filepath.Join(root, "dual.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].Machine == nil {
		t.Fatalf("expected one imported machine node: %+v", report.Batch.Nodes)
	}
	roles := report.Batch.Nodes[0].Machine.Roles
	if len(roles) != 2 || roles[0] != "development" || roles[1] != "deployment" {
		t.Fatalf("roles list should be retained in order: %+v", roles)
	}
}

func TestMarkdownServiceImportsContainerAndRecoveryAssets(t *testing.T) {
	root := t.TempDir()
	host := "---\ntype: rig\nname: example\nhostname: example.test\nstatus: active\n---\n"
	if err := os.WriteFile(filepath.Join(root, "example.md"), []byte(host), 0600); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: service\nname: synthetic\nhost: example.md\ndeploy_path: \"/srv/synthetic/\"\ncontainer: hall-synthetic\nrecovery_assets:\n  - compose.yml\n  - nginx.conf\nstatus: active\n---\n# body ignored\n"
	if err := os.WriteFile(filepath.Join(root, "svc.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	var svc *graph.Node
	for i := range report.Batch.Nodes {
		if report.Batch.Nodes[i].ID == "svc.md" {
			svc = &report.Batch.Nodes[i]
			break
		}
	}
	if svc == nil || svc.SoftwareInstance == nil {
		t.Fatalf("expected one service node: %+v exclusions=%+v", report.Batch.Nodes, report.Exclusions)
	}
	si := svc.SoftwareInstance
	if si.RuntimeIdentity != "hall-synthetic" {
		t.Fatalf("container should import into runtimeIdentity: %+v", si)
	}
	if len(si.ConfigurationReferences) != 2 || si.ConfigurationReferences[0] != "compose.yml" {
		t.Fatalf("recovery assets should import into configurationReferences: %+v", si.ConfigurationReferences)
	}
	if !assertionPresent(report.Batch.Assertions, "svc.md", "softwareInstance.runtimeIdentity") {
		t.Fatal("runtime identity should have typed evidence")
	}
	if !assertionPresent(report.Batch.Assertions, "svc.md", "softwareInstance.configurationReferences") {
		t.Fatal("recovery assets should have typed evidence")
	}
}

func TestMarkdownServiceAcceptsQuotedPorts(t *testing.T) {
	root := t.TempDir()
	host := "---\ntype: rig\nname: example\nhostname: example.test\nstatus: active\n---\n"
	if err := os.WriteFile(filepath.Join(root, "example.md"), []byte(host), 0600); err != nil {
		t.Fatal(err)
	}
	content := "---\ntype: service\nname: quoted\nhost: example.md\nports:\n  - \"5432\"\nstatus: active\n---\n"
	if err := os.WriteFile(filepath.Join(root, "quoted.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range report.Batch.Nodes {
		if n.ID != "quoted.md" {
			continue
		}
		if n.SoftwareInstance == nil || len(n.SoftwareInstance.Ports) != 1 || n.SoftwareInstance.Ports[0] != 5432 {
			t.Fatalf("quoted ports should normalise to integers: %+v", n.SoftwareInstance)
		}
		return
	}
	t.Fatalf("quoted service was excluded: %+v", report.Exclusions)
}

func TestMarkdownHardwareImportsLocationAndTypedEvidence(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: hardware\nname: Synthetic host\ncategory: server\nmodel: Example Model\nlocation: lab-rack-2\nstatus: active\n---\nbody ignored\n"
	if err := os.WriteFile(filepath.Join(root, "host.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 {
		t.Fatalf("expected one imported hardware node: %+v", report)
	}
	node := report.Batch.Nodes[0]
	if node.Hardware == nil || node.Hardware.Location != "lab-rack-2" || node.Hardware.Category != "server" || node.Hardware.Model != "Example Model" {
		t.Fatalf("hardware frontmatter fields lost: %+v", node.Hardware)
	}
	for _, field := range []string{"hardware.location", "hardware.category", "hardware.model", "hardware.status"} {
		if !assertionPresent(report.Batch.Assertions, node.ID, field) {
			t.Errorf("expected typed evidence for %s", field)
		}
	}
}

func TestMarkdownFrontmatterKeepsListsTypedAndNeverReturnsCredentialValues(t *testing.T) {
	root := t.TempDir()
	content := `---
type: service
name: Synthetic service
deploy_path: /srv/example
domains:
  - one.example.test
  - two.example.test
ports: [80, 443]
api_token: TOPSECRET-SYNTHETIC
---
Body ignored
`
	if err := os.WriteFile(filepath.Join(root, "service.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	node := report.Batch.Nodes[0]
	if node.SoftwareInstance == nil || node.SoftwareInstance.DeploymentPath != "/srv/example" || len(node.SoftwareInstance.Domains) != 2 || node.SoftwareInstance.Domains[1] != "two.example.test" || len(node.SoftwareInstance.Ports) != 2 || node.SoftwareInstance.Ports[1] != 443 {
		t.Fatalf("list field did not retain type/values: %+v", node.SoftwareInstance)
	}
	if !assertionPresent(report.Batch.Assertions, node.ID, "softwareInstance.ports") {
		t.Fatal("service ports did not produce typed field evidence")
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "TOPSECRET-SYNTHETIC") {
		t.Fatal("credential value escaped exclusion")
	}
}

func TestMarkdownInvalidServicePortsAreExcluded(t *testing.T) {
	root := t.TempDir()
	content := "---\ntype: service\nname: Invalid service\nports: [80, 70000]\n---\n"
	if err := os.WriteFile(filepath.Join(root, "service.md"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 0 {
		t.Fatalf("record with invalid ports must be excluded: %+v", report.Batch.Nodes)
	}
	if len(report.Exclusions) != 1 || !strings.Contains(report.Exclusions[0].Reason, "invalid service ports") {
		t.Fatalf("invalid ports need an actionable safe exclusion: %+v", report.Exclusions)
	}
}

func TestMarkdownSoftwareInstanceDomainAliasAndAmbiguousKeys(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"singular.md":  "---\ntype: service\nname: Singular\ndomain:\n  - one.example.test\n  - two.example.test\n---\n",
		"ambiguous.md": "---\ntype: service\nname: Ambiguous\ndomain: [singular.example.test]\ndomains: [plural.example.test]\n---\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].ID != "singular.md" {
		t.Fatalf("both-key file should be excluded: nodes=%+v exclusions=%+v", report.Batch.Nodes, report.Exclusions)
	}
	node := report.Batch.Nodes[0]
	if node.SoftwareInstance == nil || len(node.SoftwareInstance.Domains) != 2 || node.SoftwareInstance.Domains[1] != "two.example.test" {
		t.Fatalf("singular domain alias not normalized: %+v", node.SoftwareInstance)
	}
	if !assertionPresent(report.Batch.Assertions, node.ID, "softwareInstance.domains") {
		t.Fatal("singular domain alias did not emit the typed field assertion")
	}
	if len(report.Exclusions) != 1 || report.Exclusions[0].SourcePath != "ambiguous.md" || !strings.Contains(report.Exclusions[0].Reason, "both domain and domains") {
		t.Fatalf("ambiguous domain keys need explicit exclusion: %+v", report.Exclusions)
	}
}

func TestMarkdownFrontmatterRejectsInsecureConfigAndDoesNotApplyPartialScan(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.md"), []byte("---\ntype: vm\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := MarkdownFrontmatter(root, MarkdownConfig{Source: "s", TypeMappings: map[string]graph.NodeType{"vm": graph.Self}}); err == nil {
		t.Fatal("must reject mappings to special graph types")
	}
	cfg := markdownConfig()
	if _, err := MarkdownFrontmatter(filepath.Join(root, "missing"), cfg); err == nil {
		t.Fatal("missing root must not produce a partial complete batch")
	}
	if _, err := MarkdownFrontmatter(root, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownProductInstancesAndTaskRelatedReferencesBecomeTypedRelations(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"product/app.md": "---\ntype: product\nname: App\ninstances: ../service/app.md\n---\n",
		"service/app.md": "---\ntype: service\nname: App service\n---\n",
		"task/setup.md":  "---\ntype: task\nname: Setup task\nrelated: ../service/app.md\n---\n",
		"runbook/app.md": "---\ntype: runbook\nname: App runbook\nscope: production service\ntrigger: service outage\n---\n",
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	found := map[graph.Predicate]bool{}
	for _, relation := range report.Batch.Relations {
		found[relation.Predicate] = true
	}
	if !found[graph.HasInstance] || !found[graph.RelatedTo] || len(report.Batch.Relations) != 2 {
		t.Fatalf("expected typed product-instance and task-related references, got %+v", report.Batch.Relations)
	}
	var runbook graph.Node
	foundRunbook := false
	for _, node := range report.Batch.Nodes {
		if node.Type == graph.Runbook {
			runbook = node
			foundRunbook = true
			break
		}
	}
	if !foundRunbook || runbook.Runbook == nil || runbook.Runbook.Scope != "production service" || !assertionPresent(report.Batch.Assertions, runbook.ID, "runbook.scope") {
		t.Fatalf("runbook scope should map with typed evidence: %+v", runbook)
	}
	g := graph.New()
	for _, node := range report.Batch.Nodes {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	for _, relation := range report.Batch.Relations {
		if err := g.AddRelation(relation); err != nil {
			t.Fatalf("imported typed reference rejected by graph model: %+v: %v", relation, err)
		}
	}
}

func TestMarkdownImportsRemainingModeledFrontmatterWithoutSecretValues(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"project/app.md":      "---\ntype: software_project\nname: App source\npurpose: synthetic app\nlanguages: [Go, TypeScript]\nremote_repository: https://example.invalid/app.git\n---\n",
		"document/ops.md":     "---\ntype: document\nname: Operations reference\nrole: runbook-reference\nauthoritative_reference: private://ops\neffective: true\n---\n",
		"network/lab.md":      "---\ntype: network\nname: Synthetic lab\nkind: private\ncidrs: [192.0.2.0/24]\naccess_locations: [local-console]\n---\n",
		"credential/admin.md": "---\ntype: credential\nname: Admin access pointer\naccount: synthetic-admin\ncredential_type: ssh-key\npurpose: maintenance\nscope: lab\naccess_reference: private://vault/item\npassword: SHOULD_NEVER_IMPORT\n---\n",
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	nodes := map[graph.NodeType]graph.Node{}
	for _, node := range report.Batch.Nodes {
		nodes[node.Type] = node
	}
	if nodes[graph.SoftwareProject].SoftwareProject == nil || len(nodes[graph.SoftwareProject].SoftwareProject.Languages) != 2 {
		t.Fatalf("software project fields missing: %+v", nodes[graph.SoftwareProject])
	}
	if nodes[graph.Document].Document == nil || !nodes[graph.Document].Document.Effective {
		t.Fatalf("document frontmatter fields missing: %+v", nodes[graph.Document])
	}
	if nodes[graph.Network].Network == nil || len(nodes[graph.Network].Network.CIDRs) != 1 {
		t.Fatalf("network fields missing: %+v", nodes[graph.Network])
	}
	credential := nodes[graph.AccountCredential].AccountCredential
	if credential == nil || credential.AccessReference != "private://vault/item" || credential.Account != "synthetic-admin" {
		t.Fatalf("safe credential pointers missing: %+v", credential)
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "SHOULD_NEVER_IMPORT") {
		t.Fatal("secret value leaked into Markdown batch")
	}
	if !assertionPresent(report.Batch.Assertions, "network/lab.md", "network.cidrs") || !assertionPresent(report.Batch.Assertions, "document/ops.md", "document.effective") {
		t.Fatal("typed field assertions missing")
	}
}
func TestMarkdownImportsOnlyRelativeLinksAsTypedRelations(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"docs/source.md":    "---\ntype: document\nname: Source record\n---\nPrivate body text MUST NOT APPEAR. [Target](../machine/target.md#details) [same](../machine/target) [[../machine/target.md|wiki]] ![image](../machine/image.md) [external](https://example.invalid/page.md) [missing](../missing.md)\n",
		"machine/target.md": "---\ntype: vm\nname: Target machine\n---\n",
	}
	for name, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Relations) != 1 {
		t.Fatalf("expected one deduplicated internal link relation, got %+v", report.Batch.Relations)
	}
	relation := report.Batch.Relations[0]
	if relation.Predicate != graph.RelatedTo || relation.From != "docs/source.md" || relation.To != "machine/target.md" {
		t.Fatalf("unexpected Markdown link relation: %+v", relation)
	}
	encoded, _ := json.Marshal(report)
	if strings.Contains(string(encoded), "Private body text MUST NOT APPEAR") || strings.Contains(string(encoded), "https://example.invalid") || strings.Contains(string(encoded), "../missing.md") {
		t.Fatal("body text, external URLs, or raw broken-link targets leaked into report")
	}
	foundBroken := false
	for _, exclusion := range report.Exclusions {
		if exclusion.SourcePath == "docs/source.md" && strings.Contains(exclusion.Reason, "relative Markdown link target") && exclusion.Line == 5 {
			foundBroken = true
		}
	}
	if !foundBroken {
		t.Fatalf("broken link should have a safe actionable exclusion: %+v", report.Exclusions)
	}
}

func TestMarkdownBatchAppliesIdempotentlyAndExplicitRenameMappingPreservesIdentity(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	config := markdownConfig()
	write("hardware/host.md", "---\ntype: hardware\nname: Example hardware\n---\n")
	write("machine/old.md", "---\ntype: vm\nname: Old name\nhostname: vm.example.invalid\nparent: hardware/host.md\n---\n")
	first, err := MarkdownFrontmatter(root, config)
	if err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	if err = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	updated, _, err := g.ApplySync(first.Batch)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := MarkdownFrontmatter(root, config)
	if err != nil {
		t.Fatal(err)
	}
	preview := updated.PreviewSync(replay.Batch)
	if len(preview.AddedNodes) != 0 || len(preview.UpdatedNodes) != 0 || len(preview.AddedRelations) != 0 || len(preview.AddedAssertions) != 0 || len(preview.UpdatedAssertions) != 0 {
		t.Fatalf("unchanged full scan must be idempotent: %+v", preview)
	}
	if err = os.Rename(filepath.Join(root, "machine/old.md"), filepath.Join(root, "machine/new.md")); err != nil {
		t.Fatal(err)
	}
	write("machine/new.md", `---
type: vm
name: Renamed VM
hostname: vm.example.invalid
parent: hardware/host.md
---
`)
	renamedCfg := markdownConfig()
	renamedCfg.IdentityMappings = []graph.IdentityMapping{{SourceID: "machine/new.md", NodeID: "machine/old.md"}}
	renamed, err := MarkdownFrontmatter(root, renamedCfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(renamed.Batch.IdentityMappings) != 1 {
		t.Fatal("explicit rename mapping was not included in emitted batch")
	}
	renamedPreview := updated.PreviewSync(renamed.Batch)
	if len(renamedPreview.AddedNodes) != 0 || len(renamedPreview.Conflicts) != 0 {
		t.Fatalf("explicit mapping must prevent duplicate/collision: %+v", renamedPreview)
	}
	if len(renamedPreview.UpdatedNodes) != 1 || renamedPreview.UpdatedNodes[0] != "machine/old.md" {
		t.Fatalf("renamed node should update canonical node: %+v", renamedPreview.UpdatedNodes)
	}
	if len(renamedPreview.AddedRelations) != 0 {
		t.Fatalf("relation identity should remain stable across mapped rename: %+v", renamedPreview.AddedRelations)
	}
}

func TestMarkdownReferenceResolutionAndRenameConfigAreExplicit(t *testing.T) {
	known := map[string]bool{"hardware/host.md": true, "machine/vm.md": true}
	if target, ok := safeReference("machine/vm.md", "hardware/host.md", known); !ok || target != "hardware/host.md" {
		t.Fatalf("root-relative reference did not resolve: %s %v", target, ok)
	}
	if target, ok := safeReference("machine/vm.md", "../hardware/host.md", known); !ok || target != "hardware/host.md" {
		t.Fatalf("document-relative reference did not resolve: %s %v", target, ok)
	}
	if _, ok := safeReference("machine/vm.md", "../../outside.md", known); ok {
		t.Fatal("reference escaping wiki root must be rejected")
	}
	cfg := markdownConfig()
	cfg.IdentityMappings = []graph.IdentityMapping{{SourceID: "missing.md", NodeID: "canonical.md"}}
	if _, err := MarkdownFrontmatter(filepath.Join("testdata", "wiki"), cfg); err == nil {
		t.Fatal("rename mapping for a path absent from this complete scan must fail")
	}
}

func TestMarkdownFrontmatterHandlesCRLFAndExcludesOversizedFrontmatter(t *testing.T) {
	root := t.TempDir()
	crlf := "---\r\ntype: vm\r\nname: CRLF VM\r\n---\r\nBODY-MUST-NOT-BE-READ\r\n"
	if err := os.WriteFile(filepath.Join(root, "crlf.md"), []byte(crlf), 0600); err != nil {
		t.Fatal(err)
	}
	large := "---\n" + strings.Repeat("x", maxFrontmatterBytes+1) + "\n---\n"
	if err := os.WriteFile(filepath.Join(root, "large.md"), []byte(large), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || report.Batch.Nodes[0].ID != "crlf.md" {
		t.Fatalf("unexpected scanned nodes: %+v", report.Batch.Nodes)
	}
	serialized, _ := json.Marshal(report)
	if strings.Contains(string(serialized), "BODY-MUST-NOT-BE-READ") {
		t.Fatal("body was included in report")
	}
	foundLarge := false
	for _, exclusion := range report.Exclusions {
		if exclusion.SourcePath == "large.md" && strings.Contains(exclusion.Reason, "size limit") {
			foundLarge = true
		}
	}
	if !foundLarge {
		t.Fatalf("oversized frontmatter should be a safe exclusion: %+v", report.Exclusions)
	}
}

func TestMarkdownFrontmatterExcludesInvalidIPAddressWithoutEchoingValue(t *testing.T) {
	root := t.TempDir()
	raw := `---
type: vm
name: Synthetic VM
ip: definitely-not-an-address
---
`
	if err := os.WriteFile(filepath.Join(root, "vm.md"), []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := MarkdownFrontmatter(root, markdownConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Batch.Nodes) != 1 || len(report.Batch.Nodes[0].Machine.IPs) != 0 {
		t.Fatalf("invalid IP entered node details: %+v", report.Batch.Nodes)
	}
	body, _ := json.Marshal(report)
	if strings.Contains(string(body), "definitely-not-an-address") {
		t.Fatal("invalid IP value was echoed into report")
	}
	if !strings.Contains(string(body), "invalid IP address excluded") {
		t.Fatal("invalid IP exclusion is missing")
	}
}

func TestIdentityMappingReplacesFilePathAsNodeID(t *testing.T) {
	root := t.TempDir()
	host := "---\ntype: rig\nname: box\nhostname: box.test\nstatus: active\n---\n"
	if err := os.WriteFile(filepath.Join(root, "box.md"), []byte(host), 0600); err != nil {
		t.Fatal(err)
	}
	svc := "---\ntype: service\nname: web\nhost: box.md\nstatus: active\n---\n"
	if err := os.WriteFile(filepath.Join(root, "web.md"), []byte(svc), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := markdownConfig()
	const webUUID = "3f9a1c2e-0000-4000-8000-000000000001"
	const boxUUID = "3f9a1c2e-0000-4000-8000-000000000002"
	cfg.IdentityMappings = []graph.IdentityMapping{
		{SourceID: "web.md", NodeID: webUUID},
		{SourceID: "box.md", NodeID: boxUUID},
	}
	report, err := MarkdownFrontmatter(root, cfg)
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for _, n := range report.Batch.Nodes {
		ids[n.ID] = true
	}
	if !ids[webUUID] || !ids[boxUUID] {
		t.Fatalf("mapped nodes should carry UUIDs, got %v", ids)
	}
	if ids["web.md"] || ids["box.md"] {
		t.Fatalf("file paths must not remain as node IDs: %v", ids)
	}

	for _, a := range report.Batch.Assertions {
		if a.NodeID == "web.md" || a.NodeID == "box.md" {
			t.Fatalf("assertion must reference the mapped ID, got %q", a.NodeID)
		}
	}
	found := false
	for _, r := range report.Batch.Relations {
		if r.Predicate != graph.Hosts {
			continue
		}
		if r.From != boxUUID || r.To != webUUID {
			t.Fatalf("hosts relation should use mapped IDs, got %s -> %s", r.From, r.To)
		}
		found = true
	}
	if !found {
		t.Fatal("expected a hosts relation between the mapped nodes")
	}
}
