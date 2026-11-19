// Package materialize exports a read-only graph snapshot as deterministic Markdown copies.
// It never invokes Git, contacts PostgreSQL, or attempts to reconstruct document bodies.
package materialize

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/QuietSugar/my-known-world/internal/graph"
	yaml "github.com/goccy/go-yaml"
)

type Manifest struct {
	Version          int                `yaml:"version" json:"version"`
	Nodes            []ManifestNode     `yaml:"nodes" json:"nodes"`
	IdentityMappings []IdentityMapping  `yaml:"identityMappings" json:"identityMappings"`
	Relations        []ManifestRelation `yaml:"relations" json:"relations"`
}
type IdentityMapping struct {
	SourceID string `yaml:"sourceId" json:"sourceId"`
	NodeID   string `yaml:"nodeId" json:"nodeId"`
}
type ManifestNode struct {
	ID        string          `yaml:"id" json:"id"`
	Type      graph.NodeType  `yaml:"type" json:"type"`
	Certainty graph.Certainty `yaml:"certainty" json:"certainty"`
	Path      string          `yaml:"path" json:"path"`
}
type ManifestRelation struct {
	ID        string          `yaml:"id" json:"id"`
	From      string          `yaml:"from" json:"from"`
	Predicate graph.Predicate `yaml:"predicate" json:"predicate"`
	To        string          `yaml:"to" json:"to"`
}
type Report struct {
	Unsupported []string `yaml:"unsupported" json:"unsupported"`
}
type Options struct {
	Overwrite bool
	Source    string
	Types     map[graph.NodeType]bool
}

// Export writes only explicitly modeled, non-secret entity fields. Existing paths
// are rejected unless Overwrite was explicitly requested; unrelated files remain untouched.
func Export(root string, g *graph.Graph, options Options) (Manifest, Report, error) {
	if g == nil {
		return Manifest{}, Report{}, errors.New("graph snapshot is required")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, Report{}, err
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		return Manifest{}, Report{}, errors.New("output path must be a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return Manifest{}, Report{}, err
	}
	manifest := Manifest{Version: 1, Nodes: []ManifestNode{}, IdentityMappings: []IdentityMapping{}, Relations: []ManifestRelation{}}
	fileOutputs := map[string][]byte{}
	report := Report{Unsupported: []string{"Original Markdown bodies were not stored and cannot be recovered.", "History, assertion hashes, and evidence values are not materialized.", "Not every typed model field is currently mapped by the Markdown importer; this export is not yet a complete round trip (see layered-adapters documentation)."}}
	nodes := g.Nodes()
	nodePaths := map[string]string{}
	for _, n := range nodes {
		if options.Source != "" && n.Source != options.Source {
			continue
		}
		if len(options.Types) > 0 && !options.Types[n.Type] {
			continue
		}
		path := nodePath(n.ID, n.Type)
		nodePaths[n.ID] = path
	}
	for _, n := range nodes {
		path, selected := nodePaths[n.ID]
		if !selected {
			continue
		}
		relations := g.Relations()
		content, unsupported, err := renderNode(n, path, nodePaths, relations)
		if err != nil {
			return Manifest{}, Report{}, fmt.Errorf("render %s: %w", n.ID, err)
		}
		for _, item := range unsupported {
			report.Unsupported = append(report.Unsupported, n.ID+": "+item)
		}
		if !markdownTypeSupported(n.Type) {
			report.Unsupported = append(report.Unsupported, n.ID+": node type "+string(n.Type)+" has no importer type mapping")
		}
		if n.Certainty != graph.CertaintyConfirmed {
			report.Unsupported = append(report.Unsupported, n.ID+": certainty "+string(n.Certainty)+" is descriptive only; current Markdown importer cannot restore it")
		}
		fileOutputs[path] = content
		manifest.Nodes = append(manifest.Nodes, ManifestNode{ID: n.ID, Type: n.Type, Certainty: n.Certainty, Path: path})
		manifest.IdentityMappings = append(manifest.IdentityMappings, IdentityMapping{SourceID: path, NodeID: n.ID})
	}
	for _, r := range g.Relations() {
		if _, from := nodePaths[r.From]; !from {
			continue
		}
		if _, to := nodePaths[r.To]; !to {
			continue
		}
		manifest.Relations = append(manifest.Relations, ManifestRelation{ID: r.ID, From: r.From, Predicate: r.Predicate, To: r.To})
	}
	sort.Slice(manifest.Nodes, func(i, j int) bool { return manifest.Nodes[i].ID < manifest.Nodes[j].ID })
	sort.Slice(manifest.IdentityMappings, func(i, j int) bool {
		return manifest.IdentityMappings[i].SourceID < manifest.IdentityMappings[j].SourceID
	})
	sort.Slice(manifest.Relations, func(i, j int) bool { return manifest.Relations[i].ID < manifest.Relations[j].ID })
	sort.Strings(report.Unsupported)
	report.Unsupported = unique(report.Unsupported)
	for name, value := range map[string]any{"manifest.yaml": manifest, "unsupported.yaml": report, "config.yaml": importConfig(manifest)} {
		b, err := yaml.Marshal(value)
		if err != nil {
			return Manifest{}, Report{}, err
		}
		fileOutputs[name] = b
	}
	pathsToWrite := make([]string, 0, len(fileOutputs))
	for name := range fileOutputs {
		pathsToWrite = append(pathsToWrite, name)
	}
	sort.Strings(pathsToWrite)
	if !options.Overwrite {
		for _, name := range pathsToWrite {
			target := filepath.Join(root, filepath.FromSlash(name))
			if _, err := os.Lstat(target); err == nil {
				return Manifest{}, Report{}, fmt.Errorf("refusing to overwrite existing file %q; pass explicit overwrite option", name)
			} else if !os.IsNotExist(err) {
				return Manifest{}, Report{}, err
			}
		}
	}
	for _, name := range pathsToWrite {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := safeMkdirAll(root, filepath.Dir(target)); err != nil {
			return Manifest{}, Report{}, err
		}
		if err := writeFile(target, fileOutputs[name], options.Overwrite); err != nil {
			return Manifest{}, Report{}, err
		}
	}
	return manifest, report, nil
}

func safeMkdirAll(root, dir string) error {
	if err := os.MkdirAll(root, 0750); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("output path escapes selected directory")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("refusing unsafe output directory %q", current)
			}
			continue
		}
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.Mkdir(current, 0750); err != nil {
			return err
		}
	}
	return nil
}
func writeFile(path string, data []byte, overwrite bool) error {
	if overwrite {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0640)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil {
		_ = os.Remove(path)
		return werr
	}
	return cerr
}
func nodePath(id string, typ graph.NodeType) string {
	sum := sha256.Sum256([]byte(id))
	return filepath.ToSlash(filepath.Join("entities", string(typ), hex.EncodeToString(sum[:12])+".md"))
}
func unique(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
func renderNode(n graph.Node, currentPath string, paths map[string]string, relations []graph.Relation) ([]byte, []string, error) {
	if looksLikeSecretValue(n.Name) || looksLikeSecretValue(n.Lifecycle) {
		return nil, nil, fmt.Errorf("node %s has secret-like identity or lifecycle content; refusing materialization", n.ID)
	}
	fields := map[string]any{"id": n.ID, "type": sourceType(n), "name": n.Name}
	if n.Lifecycle != "" {
		fields["status"] = n.Lifecycle
	}
	unsupported := []string{}
	// Marshal only modeled detail structs; node provenance/evidence references and hashes are not copied.
	if raw, err := n.MarshalModelDetails(); err != nil {
		return nil, nil, err
	} else if string(raw) != "{}" {
		var detail map[string]any
		if err := json.Unmarshal(raw, &detail); err != nil {
			return nil, nil, err
		}
		for k, v := range detail {
			clean := sanitizeValue(v, k, &unsupported)
			if clean == omittedValue {
				continue
			}
			fields[snakeCase(k)] = clean
		}
	}
	for _, r := range relations {
		key, target := "", ""
		switch {
		case r.Predicate == graph.Hosts && r.To == n.ID:
			key, target = "host", r.From
		case r.Predicate == graph.HasInstance && r.From == n.ID && n.Type == graph.Product:
			key, target = "instances", r.To
		case r.Predicate == graph.RelatedTo && r.From == n.ID && n.Type == graph.WorkItem:
			key, target = "related", r.To
		case r.From == n.ID || r.To == n.ID:
			unsupported = append(unsupported, "relation requires an unsupported Markdown reference mapping: "+string(r.Predicate)+" (see manifest.yaml)")
		}
		if key != "" {
			if targetPath, ok := paths[target]; ok {
				rel, err := filepath.Rel(filepath.Dir(currentPath), targetPath)
				if err != nil {
					return nil, nil, err
				}
				fields[key] = appendString(fields[key], filepath.ToSlash(rel))
			}
		}
	}
	if n.Type == graph.AccountCredential {
		unsupported = append(unsupported, "credential-provider values are never materialized; only non-secret metadata is eligible")
	}
	body, err := yaml.Marshal(fields)
	if err != nil {
		return nil, nil, err
	}
	return append(append([]byte("---\n"), body...), []byte("---\n")...), unsupported, nil
}
func markdownTypeSupported(t graph.NodeType) bool {
	switch t {
	case graph.Hardware, graph.Machine, graph.SoftwareProject, graph.SoftwareInstance, graph.AccountCredential, graph.Product, graph.Document, graph.Runbook, graph.WorkItem, graph.Network:
		return true
	default:
		return false
	}
}
func importConfig(manifest Manifest) map[string]any {
	return map[string]any{
		"source":           "materialized-export",
		"typeMappings":     map[string]graph.NodeType{"hardware": graph.Hardware, "machine": graph.Machine, "rig": graph.Machine, "shared": graph.Machine, "software_project": graph.SoftwareProject, "service": graph.SoftwareInstance, "credential": graph.AccountCredential, "product": graph.Product, "design": graph.Document, "runbook": graph.Runbook, "task": graph.WorkItem, "network": graph.Network},
		"machineKindField": "machine_kind",
		"identityMappings": manifest.IdentityMappings,
	}
}
func sourceType(n graph.Node) string {
	t := n.Type
	switch t {
	case graph.SoftwareInstance:
		return "service"
	case graph.SoftwareProject:
		return "software_project"
	case graph.WorkItem:
		return "task"
	case graph.Document:
		return "design"
	case graph.Machine:
		if n.Machine != nil && (n.Machine.MachineKind == "rig" || n.Machine.MachineKind == "shared") {
			return n.Machine.MachineKind
		}
		return "machine"
	case graph.AccountCredential:
		return "credential"
	default:
		return strings.ToLower(string(t))
	}
}
func appendString(existing any, value string) []string {
	switch values := existing.(type) {
	case []string:
		return append(values, value)
	case string:
		if values != "" {
			return []string{values, value}
		}
	}
	return []string{value}
}
func snakeCase(s string) string {
	var out strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(r + ('a' - 'A'))
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}

var omittedValue = &struct{}{}

func sanitizeValue(value any, path string, notes *[]string) any {
	if isSecretKey(path) {
		*notes = append(*notes, "secret-like field omitted: "+path)
		return omittedValue
	}
	switch typed := value.(type) {
	case string:
		if looksLikeSecretValue(typed) {
			*notes = append(*notes, "secret-like value omitted: "+path)
			return omittedValue
		}
		return typed
	case map[string]any:
		clean := make(map[string]any, len(typed))
		for key, item := range typed {
			sanitized := sanitizeValue(item, path+"."+key, notes)
			if sanitized != omittedValue {
				clean[key] = sanitized
			}
		}
		return clean
	case []any:
		clean := make([]any, 0, len(typed))
		for index, item := range typed {
			sanitized := sanitizeValue(item, fmt.Sprintf("%s[%d]", path, index), notes)
			if sanitized != omittedValue {
				clean = append(clean, sanitized)
			}
		}
		return clean
	default:
		return value
	}
}

var secretValuePattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key)\s*[:=]\s*[^\s,;]+|bearer\s+[A-Za-z0-9._~+/-]{12,}|-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`)

func looksLikeSecretValue(value string) bool { return secretValuePattern.MatchString(value) }
func isSecretKey(k string) bool {
	k = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(k, "_", ""), "-", ""))
	for _, x := range []string{"password", "secret", "token", "privatekey", "apikey"} {
		if strings.Contains(k, x) {
			return true
		}
	}
	return k == "credential" || k == "credentialvalue"
}
