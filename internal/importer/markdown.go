package importer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
	yaml "github.com/goccy/go-yaml"
)

const MarkdownParserVersion = "markdown-frontmatter-v1"
const maxFrontmatterBytes = 1 << 20

// MarkdownConfig is deployment-local source configuration. Keep real wiki paths,
// type mappings and source identifiers outside public repositories.
type MarkdownConfig struct {
	Source              string                    `yaml:"source" json:"source"`
	TypeMappings        map[string]graph.NodeType `yaml:"typeMappings" json:"typeMappings"`
	IdentityMappings    []graph.IdentityMapping   `yaml:"identityMappings,omitempty" json:"identityMappings,omitempty"`
	MachineKindMappings map[string]string         `yaml:"machineKindMappings,omitempty" json:"machineKindMappings,omitempty"`
	MachineKindField    string                    `yaml:"machineKindField,omitempty" json:"machineKindField,omitempty"`
	SecretKeyPatterns   []string                  `yaml:"secretKeyPatterns,omitempty" json:"secretKeyPatterns,omitempty"`
}

type MarkdownExclusion struct {
	SourcePath string `json:"sourcePath"`
	Reason     string `json:"reason"`
	Line       int    `json:"line,omitempty"`
}
type MarkdownReport struct {
	Batch      graph.SourceBatch   `json:"batch"`
	Exclusions []MarkdownExclusion `json:"exclusions"`
}
type markdownFile struct {
	rel  string
	data map[string]any
}

var defaultSecretPatterns = []string{`(?i)password`, `(?i)secret`, `(?i)token`, `(?i)credential`, `(?i)key`, `(?i)private[_-]?key`, `(?i)api[_-]?key`}
var supportedMarkdownTypes = map[graph.NodeType]bool{
	graph.Hardware: true, graph.Machine: true, graph.SoftwareProject: true,
	graph.SoftwareInstance: true, graph.AccountCredential: true, graph.Product: true,
	graph.Document: true, graph.Runbook: true, graph.WorkItem: true, graph.Network: true,
}

// MarkdownFrontmatter walks a complete directory tree and reads only the YAML
// frontmatter of .md files. It stops reading each file at the closing delimiter;
// Markdown bodies are never retained, parsed, logged, or emitted.
func MarkdownFrontmatter(root string, cfg MarkdownConfig) (MarkdownReport, error) {
	if strings.TrimSpace(cfg.Source) == "" {
		return MarkdownReport{}, errors.New("source is required")
	}
	if len(cfg.TypeMappings) == 0 {
		return MarkdownReport{}, errors.New("explicit typeMappings are required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return MarkdownReport{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return MarkdownReport{}, fmt.Errorf("stat wiki root: %w", err)
	}
	if !info.IsDir() {
		return MarkdownReport{}, errors.New("wiki root must be a directory")
	}
	seenMappings := map[string]bool{}
	for _, mapping := range cfg.IdentityMappings {
		if strings.TrimSpace(mapping.SourceID) == "" || strings.TrimSpace(mapping.NodeID) == "" || mapping.SourceID == mapping.NodeID || seenMappings[mapping.SourceID] {
			return MarkdownReport{}, errors.New("identityMappings require unique, distinct sourceId and nodeId values")
		}
		seenMappings[mapping.SourceID] = true
	}
	for from, to := range cfg.TypeMappings {
		if strings.TrimSpace(from) == "" || !supportedMarkdownTypes[to] {
			return MarkdownReport{}, fmt.Errorf("type mapping %q has unsupported target %q", from, to)
		}
	}
	patterns := make([]*regexp.Regexp, 0, len(defaultSecretPatterns)+len(cfg.SecretKeyPatterns))
	for _, raw := range append(append([]string{}, defaultSecretPatterns...), cfg.SecretKeyPatterns...) {
		re, e := regexp.Compile(raw)
		if e != nil {
			return MarkdownReport{}, fmt.Errorf("invalid secret key pattern: %w", e)
		}
		patterns = append(patterns, re)
	}
	cfgMap := make(map[string]graph.NodeType, len(cfg.TypeMappings))
	for k, v := range cfg.TypeMappings {
		cfgMap[strings.ToLower(strings.TrimSpace(k))] = v
	}
	kindMap := make(map[string]string, len(cfg.MachineKindMappings))
	for k, v := range cfg.MachineKindMappings {
		key, value := strings.ToLower(strings.TrimSpace(k)), strings.ToLower(strings.TrimSpace(v))
		if cfgMap[key] != graph.Machine || (value != "physical" && value != "virtual" && value != "shared") {
			return MarkdownReport{}, fmt.Errorf("invalid machineKindMappings entry for %q", k)
		}
		kindMap[key] = value
	}
	kindField := cfg.MachineKindField
	if kindField == "" {
		kindField = "machine_kind"
	}
	kindField = strings.ToLower(strings.TrimSpace(kindField))
	report := MarkdownReport{Batch: graph.SourceBatch{Source: cfg.Source, ParserVersion: MarkdownParserVersion, IdentityMappings: append([]graph.IdentityMapping(nil), cfg.IdentityMappings...), Nodes: []graph.Node{}, Relations: []graph.Relation{}, Assertions: []graph.FieldAssertion{}}, Exclusions: []MarkdownExclusion{}}
	files := []markdownFile{}
	walkErr := filepath.WalkDir(root, func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		rel, e := filepath.Rel(root, full)
		if e != nil {
			return e
		}
		rel = path.Clean(filepath.ToSlash(rel))
		data, closed, closeLine, e := readFrontmatter(full)
		if e != nil {
			return fmt.Errorf("read frontmatter %s: %w", rel, e)
		}
		if !closed {
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: rel, Reason: "missing or unclosed YAML frontmatter", Line: closeLine})
			return nil
		}
		var values map[string]any
		if e = yaml.Unmarshal(data, &values); e != nil || values == nil {
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: rel, Reason: "invalid YAML frontmatter", Line: 2})
			return nil
		}
		files = append(files, markdownFile{rel: rel, data: normalizeMap(values)})
		return nil
	})
	if walkErr != nil {
		return MarkdownReport{}, fmt.Errorf("incomplete wiki scan; batch must not be applied: %w", walkErr)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	filePaths := make(map[string]bool, len(files))
	for _, f := range files {
		filePaths[f.rel] = true
	}
	canonicalIDs := map[string]string{}
	for _, f := range files {
		id := canonicalSourceID(f.rel, cfg.IdentityMappings)
		if previous, exists := canonicalIDs[id]; exists {
			return MarkdownReport{}, fmt.Errorf("multiple wiki files map to canonical node %q (%s and %s)", id, previous, f.rel)
		}
		canonicalIDs[id] = f.rel
	}
	for _, m := range cfg.IdentityMappings {
		if !filePaths[m.SourceID] {
			return MarkdownReport{}, fmt.Errorf("identity mapping source path %q is not present in this complete scan", m.SourceID)
		}
	}
	mapped := map[string]graph.Node{}
	for _, f := range files {
		collectSecretExclusions(f.rel, f.data, patterns, &report.Exclusions)
		typeValue, _ := stringValue(f.data["type"])
		target, hasMapping := cfgMap[strings.ToLower(strings.TrimSpace(typeValue))]
		name := firstString(f.data, "name", "title")
		if name == "" {
			name = strings.TrimSuffix(path.Base(f.rel), path.Ext(f.rel))
		}
		node := graph.Node{ID: f.rel, Name: name, Lifecycle: firstString(f.data, "status"), Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}
		if !hasMapping {
			node.Type = graph.ProvisionalObject
			node.Certainty = graph.CertaintyProvisional
			node.ReviewStatus = "pending"
			node.ReviewQuestions = []string{"Review the document's role and map its frontmatter type explicitly."}
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "unmapped type; emitted provisional candidate"})
		} else {
			node.Type = target
			if target == graph.Machine {
				details, ex := machineDetails(f.data, kindField, kindMap[strings.ToLower(strings.TrimSpace(typeValue))])
				node.Machine = details
				if ex != "" {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: ex})
				}
			}
			if target == graph.SoftwareInstance {
				node.SoftwareInstance = instanceDetails(f.data)
			}
			for _, field := range mappedFields(node, f.data, kindField) {
				raw, e := json.Marshal(field.value)
				if e != nil {
					continue
				}
				ref := f.rel
				report.Batch.Assertions = append(report.Batch.Assertions, graph.FieldAssertion{ID: assertionID(canonicalSourceID(f.rel, cfg.IdentityMappings), field.path), NodeID: f.rel, FieldPath: field.path, ValueHash: graph.HashFieldValue(string(raw)), Reference: ref, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
			}
			mapped[f.rel] = node
		}
		report.Batch.Nodes = append(report.Batch.Nodes, node)
	}
	relationSeen := map[string]bool{}
	for _, f := range files {
		child, ok := mapped[f.rel]
		if !ok {
			continue
		}
		for _, key := range []string{"parent", "host"} {
			refs := stringList(f.data[key])
			for _, rawRef := range refs {
				targetPath, ok := safeReference(f.rel, rawRef, filePaths)
				if !ok {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter host reference is invalid or outside the wiki"})
					continue
				}
				parent, exists := mapped[targetPath]
				if !exists {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter host reference is missing or not explicitly mapped"})
					continue
				}
				if !validHosts(parent.Type, child.Type) {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter host reference has incompatible mapped types"})
					continue
				}
				rid := relationID(canonicalSourceID(parent.ID, cfg.IdentityMappings), canonicalSourceID(child.ID, cfg.IdentityMappings))
				if !relationSeen[rid] {
					relationSeen[rid] = true
					report.Batch.Relations = append(report.Batch.Relations, graph.Relation{ID: rid, From: parent.ID, Predicate: graph.Hosts, To: child.ID, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
				}
			}
		}
	}
	return report, nil
}

func readFrontmatter(file string) ([]byte, bool, int, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, false, 0, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), maxFrontmatterBytes)
	if !s.Scan() {
		if s.Err() != nil {
			return nil, false, 1, s.Err()
		}
		return nil, false, 1, nil
	}
	if strings.TrimPrefix(s.Text(), "\ufeff") != "---" {
		return nil, false, 1, nil
	}
	var b strings.Builder
	line := 1
	for s.Scan() {
		line++
		text := s.Text()
		if text == "---" || text == "..." {
			return []byte(b.String()), true, line, nil
		}
		b.WriteString(text)
		b.WriteByte('\n')
		if b.Len() > maxFrontmatterBytes {
			return nil, false, line, errors.New("frontmatter exceeds size limit")
		}
	}
	if s.Err() != nil {
		return nil, false, line, s.Err()
	}
	return nil, false, line, nil
}
func normalizeMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[strings.ToLower(strings.TrimSpace(k))] = v
	}
	return out
}
func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := stringValue(m[k]); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func stringValue(v any) (string, bool) { s, ok := v.(string); return s, ok }
func stringList(v any) []string {
	if s, ok := stringValue(v); ok {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		return []string{strings.TrimSpace(s)}
	}
	switch xs := v.(type) {
	case []any:
		out := []string{}
		for _, x := range xs {
			if s, ok := stringValue(x); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return append([]string(nil), xs...)
	}
	return nil
}
func machineDetails(m map[string]any, kindField, configuredKind string) (*graph.MachineDetails, string) {
	d := &graph.MachineDetails{Hostname: firstString(m, "hostname"), OS: firstString(m, "os"), Status: firstString(m, "status"), MachineKind: configuredKind}
	if v, ok := m[kindField]; ok {
		d.MachineKind = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
	}
	invalidKind := false
	if d.MachineKind != "" && d.MachineKind != "physical" && d.MachineKind != "virtual" && d.MachineKind != "shared" {
		d.MachineKind = ""
		invalidKind = true
	}
	rawIPs := stringList(m["ip"])
	if len(rawIPs) == 0 {
		rawIPs = stringList(m["ips"])
	}
	invalidIP := false
	for _, rawIP := range rawIPs {
		parsed := net.ParseIP(rawIP)
		if parsed == nil {
			invalidIP = true
			continue
		}
		d.IPs = append(d.IPs, parsed.String())
	}
	d.OperationConstraints = stringList(m["constraints"])
	d.AllowedOperations = stringList(m["allowed_operations"])
	d.ForbiddenOperations = stringList(m["forbidden_operations"])
	reasons := []string{}
	if invalidKind {
		reasons = append(reasons, "invalid machine kind excluded")
	}
	if invalidIP {
		reasons = append(reasons, "invalid IP address excluded")
	}
	return d, strings.Join(reasons, "; ")
}
func instanceDetails(m map[string]any) *graph.SoftwareInstanceDetails {
	return &graph.SoftwareInstanceDetails{DeploymentPath: firstString(m, "deploy_path", "deployment_path"), Domains: stringList(m["domains"]), OperationalState: firstString(m, "status")}
}

type markdownMappedField struct {
	path  string
	value any
}

func mappedFields(n graph.Node, m map[string]any, kindField string) []markdownMappedField {
	out := []markdownMappedField{}
	add := func(field string, value any) {
		if value != nil {
			out = append(out, markdownMappedField{field, value})
		}
	}
	addString := func(key, field string) {
		if v := firstString(m, key); v != "" {
			add(field, v)
		}
	}
	if d := n.Machine; d != nil {
		addString("hostname", "machine.hostname")
		addString("os", "machine.os")
		addString("status", "machine.status")
		if _, ok := m[kindField]; ok && d.MachineKind != "" {
			add("machine.machineKind", d.MachineKind)
		}
		if len(d.IPs) > 0 {
			add("machine.ips", append([]string{}, d.IPs...))
		}
		if len(d.OperationConstraints) > 0 {
			add("machine.operationConstraints", append([]string{}, d.OperationConstraints...))
		}
		if len(d.AllowedOperations) > 0 {
			add("machine.allowedOperations", append([]string{}, d.AllowedOperations...))
		}
		if len(d.ForbiddenOperations) > 0 {
			add("machine.forbiddenOperations", append([]string{}, d.ForbiddenOperations...))
		}
	}
	if d := n.SoftwareInstance; d != nil {
		if d.DeploymentPath != "" {
			add("softwareInstance.deploymentPath", d.DeploymentPath)
		}
		if len(d.Domains) > 0 {
			add("softwareInstance.domains", append([]string{}, d.Domains...))
		}
		if d.OperationalState != "" {
			add("softwareInstance.operationalState", d.OperationalState)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}
func collectSecretExclusions(sourcePath string, value any, patterns []*regexp.Regexp, out *[]MarkdownExclusion) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			secret := false
			for _, re := range patterns {
				if re.MatchString(k) {
					secret = true
					break
				}
			}
			if secret {
				*out = append(*out, MarkdownExclusion{SourcePath: sourcePath, Reason: "credential-like key excluded"})
				continue
			}
			collectSecretExclusions(sourcePath, v[k], patterns, out)
		}
	case []any:
		for _, x := range v {
			collectSecretExclusions(sourcePath, x, patterns, out)
		}
	}
}
func safeReference(sourcePath, raw string, known map[string]bool) (string, bool) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || strings.Contains(raw, ":") || strings.HasPrefix(raw, "/") {
		return "", false
	}
	clean := path.Clean(raw)
	clean = strings.TrimSuffix(clean, path.Ext(clean)) + ".md"
	if known[clean] {
		return clean, true
	}
	relative := path.Clean(path.Join(path.Dir(sourcePath), clean))
	if relative == ".." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	if known[relative] {
		return relative, true
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}
func validHosts(a, b graph.NodeType) bool {
	return (a == graph.Hardware && b == graph.Machine) || (a == graph.Machine && (b == graph.Machine || b == graph.SoftwareInstance)) || (a == graph.SoftwareInstance && b == graph.SoftwareInstance)
}
func canonicalSourceID(id string, mappings []graph.IdentityMapping) string {
	for _, mapping := range mappings {
		if mapping.SourceID == id {
			return mapping.NodeID
		}
	}
	return id
}
func assertionID(sourcePath, field string) string {
	sum := sha256.Sum256([]byte(sourcePath + "\x00" + field))
	return "md-" + hex.EncodeToString(sum[:16])
}
func relationID(from, to string) string {
	sum := sha256.Sum256([]byte(from + "\x00hosts\x00" + to))
	return "md-rel-" + hex.EncodeToString(sum[:16])
}
