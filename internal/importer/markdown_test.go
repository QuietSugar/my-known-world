package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func markdownConfig() MarkdownConfig {
	return MarkdownConfig{Source: "wiki:private-test", TypeMappings: map[string]graph.NodeType{"hardware": graph.Hardware, "vm": graph.Machine, "rig": graph.Machine, "shared": graph.Machine, "service": graph.SoftwareInstance}, MachineKindMappings: map[string]string{"vm": "virtual", "shared": "shared"}}
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
	if shared.Machine.MachineKind != "shared" || len(shared.Machine.OperationConstraints) != 2 {
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
	for _, as := range a.Batch.Assertions {
		if as.EvidenceKind != graph.EvidenceDocument || as.Reference == "" {
			t.Fatalf("assertion provenance missing: %+v", as)
		}
	}
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
	if node.SoftwareInstance == nil || node.SoftwareInstance.DeploymentPath != "/srv/example" || len(node.SoftwareInstance.Domains) != 2 || node.SoftwareInstance.Domains[1] != "two.example.test" {
		t.Fatalf("list field did not retain type/values: %+v", node.SoftwareInstance)
	}
	raw, _ := json.Marshal(report)
	if strings.Contains(string(raw), "TOPSECRET-SYNTHETIC") {
		t.Fatal("credential value escaped exclusion")
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
