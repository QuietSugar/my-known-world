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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuietSugar/my-known-world/internal/graph"
	yaml "github.com/goccy/go-yaml"
)

const MarkdownParserVersion = "markdown-frontmatter-v4"
const maxFrontmatterBytes = 1 << 20

var errFrontmatterTooLarge = errors.New("frontmatter exceeds size limit")
var markdownInlineLinkPattern = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)
var markdownWikiLinkPattern = regexp.MustCompile(`\[\[([^\]|#]+)(?:[|#][^\]]*)?\]\]`)

// MarkdownConfig is deployment-local source configuration. Keep real wiki paths,
// type mappings and source identifiers outside public repositories.
type MarkdownConfig struct {
	Source              string                    `yaml:"source" json:"source"`
	TypeMappings        map[string]graph.NodeType `yaml:"typeMappings" json:"typeMappings"`
	IdentityMappings    []graph.IdentityMapping   `yaml:"identityMappings,omitempty" json:"identityMappings,omitempty"`
	MachineKindMappings map[string]string         `yaml:"machineKindMappings,omitempty" json:"machineKindMappings,omitempty"`
	MachineKindField    string                    `yaml:"machineKindField,omitempty" json:"machineKindField,omitempty"`
	RequiredFields      map[string][]string       `yaml:"requiredFields,omitempty" json:"requiredFields,omitempty"`
	RequiredAnyFields   map[string][]string       `yaml:"requiredAnyFields,omitempty" json:"requiredAnyFields,omitempty"`
	StatusValues        map[string][]string       `yaml:"statusValues,omitempty" json:"statusValues,omitempty"`
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
	rel   string
	data  map[string]any
	links []markdownLink
}

type markdownLink struct {
	target string
	line   int
}
type markdownRelationReference struct {
	ID        string          `json:"id"`
	Predicate graph.Predicate `json:"predicate"`
	Direction string          `json:"direction"`
	Target    string          `json:"target"`
}

var defaultSecretPatterns = []string{`(?i)password`, `(?i)secret`, `(?i)token`, `(?i)credential`, `(?i)key`, `(?i)private[_-]?key`, `(?i)api[_-]?key`}
var supportedMarkdownTypes = map[graph.NodeType]bool{
	graph.Hardware: true, graph.Machine: true, graph.SoftwareProject: true,
	graph.SoftwareInstance: true, graph.AccountCredential: true, graph.Product: true,
	graph.Document: true, graph.Runbook: true, graph.WorkItem: true, graph.Network: true,
}

// MarkdownFrontmatter walks a complete directory tree and reads YAML
// frontmatter from .md files. It scans the body only to extract relative link
// targets; body text is never retained, logged, or emitted.
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
	for _, rules := range []map[string][]string{cfg.RequiredFields, cfg.RequiredAnyFields, cfg.StatusValues} {
		for sourceType, values := range rules {
			if strings.TrimSpace(sourceType) == "" {
				return MarkdownReport{}, errors.New("invalid validation rule source type")
			}
			if _, exists := cfg.TypeMappings[sourceType]; !exists {
				mapped := false
				for mapping := range cfg.TypeMappings {
					if strings.EqualFold(strings.TrimSpace(mapping), strings.TrimSpace(sourceType)) {
						mapped = true
						break
					}
				}
				if !mapped {
					return MarkdownReport{}, fmt.Errorf("validation rule has no type mapping for %q", sourceType)
				}
			}
			if len(values) == 0 {
				return MarkdownReport{}, fmt.Errorf("validation rule for %q must contain values", sourceType)
			}
			seen := map[string]bool{}
			for _, value := range values {
				value = strings.ToLower(strings.TrimSpace(value))
				if value == "" || seen[value] {
					return MarkdownReport{}, fmt.Errorf("validation rule for %q contains an empty or duplicate value", sourceType)
				}
				seen[value] = true
			}
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
		if cfgMap[key] != graph.Machine || (value != "physical" && value != "virtual" && value != "shared" && value != "rig") {
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
			if errors.Is(e, errFrontmatterTooLarge) {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: rel, Reason: "YAML frontmatter exceeds size limit", Line: closeLine})
				return nil
			}
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
		links, linkErr := readMarkdownLinks(full)
		if linkErr != nil {
			return fmt.Errorf("scan Markdown links %s: %w", rel, linkErr)
		}
		files = append(files, markdownFile{rel: rel, data: normalizeMap(values), links: links})
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
		if _, hasDomain := f.data["domain"]; hasDomain {
			if _, hasDomains := f.data["domains"]; hasDomains {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "both domain and domains are present; file excluded to avoid ambiguous service domains"})
				continue
			}
		}
		if reason := invalidMarkdownFieldReason(f.data); reason != "" {
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: reason})
			continue
		}
		typeValue, _ := stringValue(f.data["type"])
		target, hasMapping := cfgMap[strings.ToLower(strings.TrimSpace(typeValue))]
		if hasMapping {
			if reason := validateMappedSourceRecord(typeValue, f.data, cfg); reason != "" {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: reason})
				continue
			}
		}
		collectSecretExclusions(f.rel, f.data, patterns, &report.Exclusions)
		name := firstString(f.data, "name", "title")
		if name == "" {
			name = strings.TrimSuffix(path.Base(f.rel), path.Ext(f.rel))
		}
		node := graph.Node{ID: f.rel, Name: name, Lifecycle: firstString(f.data, "lifecycle", "status"), Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}
		if !hasMapping {
			node.Type = graph.ProvisionalObject
			node.Certainty = graph.CertaintyProvisional
			node.ReviewStatus = "pending"
			node.ReviewQuestions = []string{"Review the document's role and map its frontmatter type explicitly."}
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "unmapped type; emitted provisional candidate"})
		} else {
			node.Type = target
			if target == graph.Hardware {
				node.Hardware = &graph.HardwareDetails{Category: firstString(f.data, "category"), Model: firstString(f.data, "model"), Location: firstString(f.data, "location"), Status: firstString(f.data, "status")}
			}
			if target == graph.Machine {
				configuredKind := kindMap[strings.ToLower(strings.TrimSpace(typeValue))]
				if configuredKind == "" && strings.EqualFold(strings.TrimSpace(typeValue), "rig") {
					configuredKind = "rig"
				}
				details, ex := machineDetails(f.data, kindField, configuredKind)
				node.Machine = details
				if ex != "" {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: ex})
				}
			}
			if target == graph.SoftwareInstance {
				details, ex := instanceDetails(f.data)
				node.SoftwareInstance = details
				if ex != "" {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: ex})
				}
			}
			if target == graph.SoftwareProject {
				node.SoftwareProject = &graph.SoftwareProjectDetails{Purpose: firstString(f.data, "purpose"), Languages: stringList(f.data["languages"]), Form: firstString(f.data, "form"), RemoteRepository: firstString(f.data, "remote_repository", "repository"), Upstream: firstString(f.data, "upstream"), MaintenanceState: firstString(f.data, "maintenance_state"), RecommendedVersion: firstString(f.data, "recommended_version"), SuccessorID: firstString(f.data, "successor_id")}
			}
			if target == graph.Product {
				node.Product = &graph.ProductDetails{Purpose: firstString(f.data, "purpose"), CommonConfigurationReference: firstString(f.data, "common_configuration_reference"), RunbookReference: firstString(f.data, "runbook_reference")}
			}
			if target == graph.Document {
				node.Document = &graph.DocumentDetails{Role: firstString(f.data, "role"), AuthoritativeReference: firstString(f.data, "authoritative_reference"), Effective: boolValue(f.data["effective"]), SupersededBy: firstString(f.data, "superseded_by")}
			}
			if target == graph.Runbook {
				node.Runbook = &graph.RunbookDetails{Scope: firstString(f.data, "scope"), Trigger: firstString(f.data, "trigger"), Preconditions: stringList(f.data["preconditions"]), StepsReference: firstString(f.data, "steps_reference"), StopConditionReference: firstString(f.data, "stop_condition_reference"), RollbackReference: firstString(f.data, "rollback_reference"), VerificationMethod: firstString(f.data, "verification_method"), ExecutableStatus: firstString(f.data, "executable_status"), ApprovalRequired: boolValue(f.data["approval_required"])}
			}
			if target == graph.WorkItem {
				node.WorkItem = &graph.WorkItemDetails{Kind: firstString(f.data, "kind"), Status: firstString(f.data, "status"), RationaleReference: firstString(f.data, "rationale_reference"), DecisionEvidenceReference: firstString(f.data, "decision_evidence_reference")}
			}
			if target == graph.Network {
				node.Network = &graph.NetworkDetails{Kind: firstString(f.data, "kind"), CIDRs: stringList(f.data["cidrs"]), GatewayReference: firstString(f.data, "gateway_reference"), AccessLocations: stringList(f.data["access_locations"]), VPNReference: firstString(f.data, "vpn_reference"), Status: firstString(f.data, "status")}
			}
			if target == graph.AccountCredential {
				node.AccountCredential = &graph.AccountCredentialDetails{Account: firstString(f.data, "account"), CredentialType: firstString(f.data, "credential_type"), Purpose: firstString(f.data, "purpose"), Scope: firstString(f.data, "scope"), AccessReference: firstString(f.data, "access_reference"), Status: firstString(f.data, "status")}
			}
			if err := decodeMarkdownModelDetails(f.data, &node); err != nil {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "invalid typed detail field; record excluded"})
				continue
			}
			for _, field := range mappedFields(node, f.data) {
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
	invalidRecords := map[string]string{}
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			child, ok := mapped[f.rel]
			if !ok || child.Type == graph.ProvisionalObject || invalidRecords[f.rel] != "" {
				continue
			}
			for _, key := range []string{"parent", "host"} {
				for _, rawRef := range stringList(f.data[key]) {
					targetPath, valid := safeReference(f.rel, rawRef, filePaths)
					if !valid || !filePaths[targetPath] {
						invalidRecords[f.rel] = "frontmatter " + key + " reference is invalid or missing; record excluded"
						break
					}
					parent, exists := mapped[targetPath]
					if !exists || invalidRecords[targetPath] != "" {
						invalidRecords[f.rel] = "frontmatter " + key + " target is not a valid mapped entity; record excluded"
						break
					}
					if !validHosts(parent.Type, child.Type) {
						invalidRecords[f.rel] = "frontmatter " + key + " target has an incompatible mapped type; record excluded"
						break
					}
				}
				if invalidRecords[f.rel] != "" {
					changed = true
					break
				}
			}
		}
	}
	if len(invalidRecords) > 0 {
		for _, f := range files {
			if reason := invalidRecords[f.rel]; reason != "" {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: reason})
				delete(mapped, f.rel)
			}
		}
		validNodes := report.Batch.Nodes[:0]
		for _, node := range report.Batch.Nodes {
			if invalidRecords[node.ID] == "" {
				validNodes = append(validNodes, node)
			}
		}
		report.Batch.Nodes = validNodes
		validAssertions := report.Batch.Assertions[:0]
		for _, assertion := range report.Batch.Assertions {
			if invalidRecords[assertion.NodeID] == "" {
				validAssertions = append(validAssertions, assertion)
			}
		}
		report.Batch.Assertions = validAssertions
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
				rid := relationID(canonicalSourceID(parent.ID, cfg.IdentityMappings), graph.Hosts, canonicalSourceID(child.ID, cfg.IdentityMappings))
				if !relationSeen[rid] {
					relationSeen[rid] = true
					report.Batch.Relations = append(report.Batch.Relations, graph.Relation{ID: rid, From: parent.ID, Predicate: graph.Hosts, To: child.ID, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
				}
			}
		}
	}
	for _, f := range files {
		node, ok := mapped[f.rel]
		if !ok {
			continue
		}
		for _, refSpec := range []struct {
			key       string
			predicate graph.Predicate
		}{
			{"instances", graph.HasInstance},
			{"related", graph.RelatedTo},
		} {
			key, predicate := refSpec.key, refSpec.predicate
			if key == "instances" && node.Type != graph.Product || key == "related" && node.Type != graph.WorkItem {
				continue
			}
			for _, rawRef := range stringList(f.data[key]) {
				targetPath, valid := safeReference(f.rel, rawRef, filePaths)
				if !valid || !filePaths[targetPath] {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter " + key + " reference is invalid or missing"})
					continue
				}
				target, exists := mapped[targetPath]
				if !exists {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter " + key + " reference target is not a mapped valid entity"})
					continue
				}
				if predicate == graph.HasInstance && target.Type != graph.SoftwareInstance {
					report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "product instances reference must target a mapped service"})
					continue
				}
				fromID, toID := node.ID, target.ID
				if predicate == graph.RelatedTo {
					fromID, toID = node.ID, target.ID
				}
				rid := relationID(canonicalSourceID(fromID, cfg.IdentityMappings), predicate, canonicalSourceID(toID, cfg.IdentityMappings))
				if relationSeen[rid] {
					continue
				}
				relationSeen[rid] = true
				report.Batch.Relations = append(report.Batch.Relations, graph.Relation{ID: rid, From: fromID, Predicate: predicate, To: toID, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
			}
		}
	}
	for _, f := range files {
		node, ok := mapped[f.rel]
		if !ok {
			continue
		}
		references, parseErr := markdownRelationReferences(f.data["relations"])
		if parseErr != nil {
			report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relations list is invalid"})
			continue
		}
		for _, ref := range references {
			targetPath, valid := safeReference(f.rel, ref.Target, filePaths)
			if !valid || !filePaths[targetPath] {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation target is invalid or missing"})
				continue
			}
			target, exists := mapped[targetPath]
			if !exists {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation target is not a mapped valid entity"})
				continue
			}
			fromID, toID := node.ID, target.ID
			if ref.Direction == "in" {
				fromID, toID = target.ID, node.ID
			} else if ref.Direction != "out" {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation direction must be in or out"})
				continue
			}
			rid := strings.TrimSpace(ref.ID)
			if rid == "" {
				rid = relationID(canonicalSourceID(fromID, cfg.IdentityMappings), ref.Predicate, canonicalSourceID(toID, cfg.IdentityMappings))
			}
			if len(rid) > 512 || strings.ContainsAny(rid, "\r\n") {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation identity is invalid"})
				continue
			}
			if relationSeen[rid] {
				continue
			}
			candidate := graph.Relation{ID: rid, From: fromID, Predicate: ref.Predicate, To: toID, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed}
			check := graph.New()
			if err := check.AddNode(mapped[fromID]); err != nil {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation endpoint is invalid"})
				continue
			}
			if err := check.AddNode(mapped[toID]); err != nil {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation endpoint is invalid"})
				continue
			}
			if err := check.AddRelation(candidate); err != nil {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "frontmatter relation predicate or endpoints are incompatible"})
				continue
			}
			relationSeen[rid] = true
			report.Batch.Relations = append(report.Batch.Relations, candidate)
		}
	}
	for _, f := range files {
		node, ok := mapped[f.rel]
		if !ok {
			continue
		}
		for _, link := range f.links {
			targetPath, valid := safeReference(f.rel, link.target, filePaths)
			if !valid || !filePaths[targetPath] {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "relative Markdown link target is invalid or missing", Line: link.line})
				continue
			}
			target, exists := mapped[targetPath]
			if !exists {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "relative Markdown link target is not a mapped valid entity", Line: link.line})
				continue
			}
			if node.ID == target.ID {
				report.Exclusions = append(report.Exclusions, MarkdownExclusion{SourcePath: f.rel, Reason: "self-referential Markdown link was ignored", Line: link.line})
				continue
			}
			rid := relationID(canonicalSourceID(node.ID, cfg.IdentityMappings), graph.RelatedTo, canonicalSourceID(target.ID, cfg.IdentityMappings))
			if relationSeen[rid] {
				continue
			}
			relationSeen[rid] = true
			report.Batch.Relations = append(report.Batch.Relations, graph.Relation{ID: rid, From: node.ID, Predicate: graph.RelatedTo, To: target.ID, Source: cfg.Source, EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed})
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
	s.Buffer(make([]byte, 4096), maxFrontmatterBytes*2)
	if !s.Scan() {
		if s.Err() != nil {
			return nil, false, 1, s.Err()
		}
		return nil, false, 1, nil
	}
	if strings.TrimSuffix(strings.TrimPrefix(s.Text(), "\ufeff"), "\r") != "---" {
		return nil, false, 1, nil
	}
	var b strings.Builder
	line := 1
	for s.Scan() {
		line++
		text := strings.TrimSuffix(s.Text(), "\r")
		if text == "---" || text == "..." {
			return []byte(b.String()), true, line, nil
		}
		b.WriteString(text)
		b.WriteByte('\n')
		if b.Len() > maxFrontmatterBytes {
			return nil, false, line, errFrontmatterTooLarge
		}
	}
	if s.Err() != nil {
		if errors.Is(s.Err(), bufio.ErrTooLong) {
			return nil, false, line, errFrontmatterTooLarge
		}
		return nil, false, line, s.Err()
	}
	return nil, false, line, nil
}
func addAllPresentTypedFields(out *[]markdownMappedField, n graph.Node, fields map[string]any) {
	prefix := ""
	var detail any
	switch n.Type {
	case graph.Hardware:
		prefix, detail = "hardware", n.Hardware
	case graph.Machine:
		prefix, detail = "machine", n.Machine
	case graph.SoftwareProject:
		prefix, detail = "softwareProject", n.SoftwareProject
	case graph.SoftwareInstance:
		prefix, detail = "softwareInstance", n.SoftwareInstance
	case graph.AccountCredential:
		prefix, detail = "accountCredential", n.AccountCredential
	case graph.Product:
		prefix, detail = "product", n.Product
	case graph.Document:
		prefix, detail = "document", n.Document
	case graph.Runbook:
		prefix, detail = "runbook", n.Runbook
	case graph.WorkItem:
		prefix, detail = "workItem", n.WorkItem
	case graph.Network:
		prefix, detail = "network", n.Network
	}
	if prefix == "" || detail == nil {
		return
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return
	}
	var values map[string]any
	if json.Unmarshal(raw, &values) != nil {
		return
	}
	present := map[string]bool{}
	for _, item := range *out {
		present[item.path] = true
	}
	for field, value := range values {
		if _, ok := fields[camelToSnake(field)]; !ok {
			continue
		}
		fieldPath := prefix + "." + field
		if !present[fieldPath] {
			*out = append(*out, markdownMappedField{path: fieldPath, value: value})
			present[fieldPath] = true
		}
	}
}
func camelToSnake(value string) string {
	var out strings.Builder
	for i, r := range value {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			r += 32
		}
		out.WriteRune(r)
	}
	return out.String()
}

func readMarkdownLinks(file string) ([]markdownLink, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	line := 0
	if !scanner.Scan() {
		return nil, scanner.Err()
	}
	line++
	if strings.TrimSuffix(strings.TrimPrefix(scanner.Text(), "\ufeff"), "\r") != "---" {
		return nil, nil
	}
	for scanner.Scan() {
		line++
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if text == "---" || text == "..." {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	links := []markdownLink{}
	add := func(raw string, line int, image bool) {
		if image {
			return
		}
		raw = strings.TrimSpace(raw)
		if strings.HasPrefix(raw, "<") && strings.HasSuffix(raw, ">") {
			raw = strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">")
		} else if i := strings.IndexAny(raw, " \t"); i >= 0 {
			raw = raw[:i]
		}
		if raw == "" || strings.Contains(raw, ":") || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "/") {
			return
		}
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			raw = raw[:i]
		}
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			return
		}
		ext := strings.ToLower(path.Ext(decoded))
		if ext != "" && ext != ".md" {
			return
		}
		links = append(links, markdownLink{target: decoded, line: line})
	}
	for scanner.Scan() {
		line++
		text := scanner.Text()
		for _, match := range markdownInlineLinkPattern.FindAllStringSubmatchIndex(text, -1) {
			image := match[0] > 0 && text[match[0]-1] == '!'
			add(text[match[2]:match[3]], line, image)
		}
		for _, match := range markdownWikiLinkPattern.FindAllStringSubmatch(text, -1) {
			add(match[1], line, false)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return links, nil
}

func validateMappedSourceRecord(sourceType string, fields map[string]any, cfg MarkdownConfig) string {
	for _, required := range configValuesForType(cfg.RequiredFields, sourceType) {
		if !frontmatterFieldPresent(fields[required]) {
			return "missing required field " + required + "; record excluded"
		}
	}
	if requiredAny := configValuesForType(cfg.RequiredAnyFields, sourceType); len(requiredAny) > 0 {
		present := false
		for _, field := range requiredAny {
			if frontmatterFieldPresent(fields[field]) {
				present = true
				break
			}
		}
		if !present {
			return "missing required identity field (" + strings.Join(requiredAny, ", ") + "); record excluded"
		}
	}
	if allowed := configValuesForType(cfg.StatusValues, sourceType); len(allowed) > 0 {
		status := strings.ToLower(strings.TrimSpace(firstString(fields, "status")))
		if status == "" {
			return "missing required status; record excluded"
		}
		for _, value := range allowed {
			if status == strings.ToLower(strings.TrimSpace(value)) {
				return ""
			}
		}
		return "status is not allowed for mapped source type; record excluded"
	}
	return ""
}

func configValuesForType(rules map[string][]string, sourceType string) []string {
	for key, values := range rules {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(sourceType)) {
			return values
		}
	}
	return nil
}

func frontmatterFieldPresent(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case []string:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case nil:
		return false
	default:
		return true
	}
}

func invalidMarkdownFieldReason(m map[string]any) string {
	stringFields := []string{"type", "id", "name", "title", "status", "hostname", "os", "location", "category", "model", "scope", "trigger", "deploy_path", "deployment_path", "purpose", "form", "remote_repository", "repository", "upstream", "maintenance_state", "recommended_version", "successor_id", "common_configuration_reference", "runbook_reference", "role", "authoritative_reference", "superseded_by", "steps_reference", "stop_condition_reference", "rollback_reference", "verification_method", "executable_status", "kind", "rationale_reference", "decision_evidence_reference", "gateway_reference", "vpn_reference", "account", "credential_type", "access_reference"}
	for _, key := range stringFields {
		if value, exists := m[key]; exists {
			if _, ok := value.(string); !ok {
				return "invalid frontmatter field type; record excluded"
			}
		}
	}
	listFields := []string{"ip", "ips", "constraints", "allowed_operations", "forbidden_operations", "owned", "domains", "domain", "parent", "host", "instances", "related", "languages", "preconditions", "cidrs", "access_locations"}
	for _, key := range listFields {
		if value, exists := m[key]; exists && !isStringOrStringList(value) {
			return "invalid frontmatter field type; record excluded"
		}
	}
	for _, raw := range stringList(m["cidrs"]) {
		if _, _, err := net.ParseCIDR(raw); err != nil {
			return "invalid network CIDR; record excluded"
		}
	}
	if value, exists := m["ports"]; exists {
		if _, ok := portList(value); !ok {
			return "invalid service ports; expected integers from 1 through 65535; record excluded"
		}
	}
	for _, key := range []string{"effective", "approval_required"} {
		if value, exists := m[key]; exists {
			if _, ok := value.(bool); !ok {
				return "invalid frontmatter field type; record excluded"
			}
		}
	}
	return ""
}

func isStringOrStringList(value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	if values, ok := value.([]any); ok {
		for _, value := range values {
			if _, ok := value.(string); !ok {
				return false
			}
		}
		return true
	}
	_, ok := value.([]string)
	return ok
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
func boolValue(v any) bool             { value, _ := v.(bool); return value }
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
func markdownRelationReferences(value any) ([]markdownRelationReference, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var refs []markdownRelationReference
	if err := json.Unmarshal(raw, &refs); err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if strings.TrimSpace(string(ref.Predicate)) == "" || strings.TrimSpace(ref.Target) == "" || len(ref.ID) > 512 || strings.ContainsAny(ref.ID, "\r\n") {
			return nil, errors.New("invalid relation reference")
		}
	}
	return refs, nil
}

func decodeMarkdownModelDetails(fields map[string]any, node *graph.Node) error {
	var target any
	switch node.Type {
	case graph.Hardware:
		target = node.Hardware
	case graph.Machine:
		target = node.Machine
	case graph.SoftwareProject:
		target = node.SoftwareProject
	case graph.SoftwareInstance:
		target = node.SoftwareInstance
	case graph.AccountCredential:
		target = node.AccountCredential
	case graph.Product:
		target = node.Product
	case graph.Document:
		target = node.Document
	case graph.Runbook:
		target = node.Runbook
	case graph.WorkItem:
		target = node.WorkItem
	case graph.Network:
		target = node.Network
	default:
		return nil
	}
	camel := make(map[string]any, len(fields))
	for key, value := range fields {
		camel[snakeToCamel(key)] = value
	}
	data, err := json.Marshal(camel)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func snakeToCamel(value string) string {
	var out strings.Builder
	upper := false
	for _, r := range value {
		if r == '_' || r == '-' {
			upper = true
			continue
		}
		if upper {
			if r >= 'a' && r <= 'z' {
				r -= 32
			}
			upper = false
		}
		out.WriteRune(r)
	}
	return out.String()
}

func machineDetails(m map[string]any, kindField, configuredKind string) (*graph.MachineDetails, string) {
	d := &graph.MachineDetails{Hostname: firstString(m, "hostname"), OS: firstString(m, "os"), Status: firstString(m, "status"), MachineKind: configuredKind, Owned: stringList(m["owned"])}
	if v, ok := m[kindField]; ok {
		d.MachineKind = strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
	}
	invalidKind := false
	if d.MachineKind != "" && d.MachineKind != "physical" && d.MachineKind != "virtual" && d.MachineKind != "shared" && d.MachineKind != "rig" {
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
func instanceDetails(m map[string]any) (*graph.SoftwareInstanceDetails, string) {
	domains := m["domains"]
	if _, hasPlural := m["domains"]; !hasPlural {
		domains = m["domain"]
	}
	d := &graph.SoftwareInstanceDetails{DeploymentPath: firstString(m, "deploy_path", "deployment_path"), Domains: stringList(domains), OperationalState: firstString(m, "status")}
	if rawPorts, exists := m["ports"]; exists {
		ports, ok := portList(rawPorts)
		if !ok {
			return d, "invalid service ports excluded; expected integer ports from 1 through 65535"
		}
		d.Ports = ports
	}
	return d, ""
}

func portList(value any) ([]int, bool) {
	values, ok := value.([]any)
	if !ok {
		values = []any{value}
	}
	ports := make([]int, 0, len(values))
	for _, value := range values {
		port, valid := portNumber(value)
		if !valid || port < 1 || port > 65535 {
			return nil, false
		}
		ports = append(ports, port)
	}
	return ports, len(ports) > 0
}

func portNumber(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), int64(int(v)) == v
	case uint64:
		return int(v), uint64(int(v)) == v
	case float64:
		return int(v), float64(int(v)) == v
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		return n, err == nil
	default:
		return 0, false
	}
}

type markdownMappedField struct {
	path  string
	value any
}

func mappedFields(n graph.Node, m map[string]any) []markdownMappedField {
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
	addString("lifecycle", "lifecycle")
	if firstString(m, "lifecycle") == "" {
		addString("status", "lifecycle")
	}
	if d := n.Hardware; d != nil {
		addString("category", "hardware.category")
		addString("model", "hardware.model")
		addString("location", "hardware.location")
		addString("status", "hardware.status")
	}
	if d := n.Machine; d != nil {
		addString("hostname", "machine.hostname")
		addString("os", "machine.os")
		addString("status", "machine.status")
		if d.MachineKind != "" {
			add("machine.machineKind", d.MachineKind)
		}
		if len(d.IPs) > 0 {
			add("machine.ips", append([]string{}, d.IPs...))
		}
		if len(d.Owned) > 0 {
			add("machine.owned", append([]string{}, d.Owned...))
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
	if d := n.SoftwareProject; d != nil {
		addString("purpose", "softwareProject.purpose")
		addString("form", "softwareProject.form")
		addString("remote_repository", "softwareProject.remoteRepository")
		if len(d.Languages) > 0 {
			add("softwareProject.languages", append([]string{}, d.Languages...))
		}
		addString("upstream", "softwareProject.upstream")
		addString("maintenance_state", "softwareProject.maintenanceState")
		addString("recommended_version", "softwareProject.recommendedVersion")
		addString("successor_id", "softwareProject.successorId")
	}
	if d := n.Product; d != nil {
		addString("purpose", "product.purpose")
		addString("common_configuration_reference", "product.commonConfigurationReference")
		addString("runbook_reference", "product.runbookReference")
	}
	if d := n.Document; d != nil {
		addString("role", "document.role")
		addString("authoritative_reference", "document.authoritativeReference")
		addString("superseded_by", "document.supersededBy")
		if _, exists := m["effective"]; exists {
			add("document.effective", d.Effective)
		}
	}
	if d := n.Runbook; d != nil {
		if d.Scope != "" {
			add("runbook.scope", d.Scope)
		}
		if d.Trigger != "" {
			add("runbook.trigger", d.Trigger)
		}
		if len(d.Preconditions) > 0 {
			add("runbook.preconditions", append([]string{}, d.Preconditions...))
		}
		addString("steps_reference", "runbook.stepsReference")
		addString("stop_condition_reference", "runbook.stopConditionReference")
		addString("rollback_reference", "runbook.rollbackReference")
		addString("verification_method", "runbook.verificationMethod")
		addString("executable_status", "runbook.executableStatus")
		if _, exists := m["approval_required"]; exists {
			add("runbook.approvalRequired", d.ApprovalRequired)
		}
	}
	if d := n.WorkItem; d != nil {
		addString("kind", "workItem.kind")
		addString("status", "workItem.status")
		addString("rationale_reference", "workItem.rationaleReference")
		addString("decision_evidence_reference", "workItem.decisionEvidenceReference")
	}
	if d := n.Network; d != nil {
		addString("kind", "network.kind")
		addString("gateway_reference", "network.gatewayReference")
		addString("vpn_reference", "network.vpnReference")
		addString("status", "network.status")
		if len(d.CIDRs) > 0 {
			add("network.cidrs", append([]string{}, d.CIDRs...))
		}
		if len(d.AccessLocations) > 0 {
			add("network.accessLocations", append([]string{}, d.AccessLocations...))
		}
	}
	if d := n.AccountCredential; d != nil {
		addString("account", "accountCredential.account")
		addString("credential_type", "accountCredential.credentialType")
		addString("purpose", "accountCredential.purpose")
		addString("scope", "accountCredential.scope")
		addString("access_reference", "accountCredential.accessReference")
		addString("status", "accountCredential.status")
	}
	if d := n.SoftwareInstance; d != nil {
		if d.DeploymentPath != "" {
			add("softwareInstance.deploymentPath", d.DeploymentPath)
		}
		if len(d.Domains) > 0 {
			add("softwareInstance.domains", append([]string{}, d.Domains...))
		}
		if len(d.Ports) > 0 {
			add("softwareInstance.ports", append([]int{}, d.Ports...))
		}
		if d.OperationalState != "" {
			add("softwareInstance.operationalState", d.OperationalState)
		}
	}
	addAllPresentTypedFields(&out, n, m)
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
	return (a == graph.Hardware && b == graph.Machine) || (a == graph.Machine && (b == graph.Machine || b == graph.SoftwareInstance))
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
func relationID(from string, predicate graph.Predicate, to string) string {
	sum := sha256.Sum256([]byte(from + "\x00" + string(predicate) + "\x00" + to))
	return "md-rel-" + hex.EncodeToString(sum[:16])
}
