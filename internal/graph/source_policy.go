package graph

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// SourcePolicy is an allow-list for source-owned data, not an automatic trust
// or truth ranking. Priority is advisory and never resolves conflicting facts.
type SourcePolicy struct {
	Source            string      `json:"source"`
	Priority          int         `json:"priority"`
	AllowedNodeTypes  []NodeType  `json:"allowedNodeTypes"`
	AllowedPredicates []Predicate `json:"allowedPredicates"`
	AllowedFieldPaths []string    `json:"allowedFieldPaths"`
}

func (p SourcePolicy) Validate() error {
	if strings.TrimSpace(p.Source) == "" || len(p.Source) > 200 {
		return errors.New("source is required and must be at most 200 characters")
	}
	if p.AllowedNodeTypes == nil || p.AllowedPredicates == nil || p.AllowedFieldPaths == nil {
		return errors.New("all allow-lists must be explicitly provided (empty arrays are allowed)")
	}
	if p.Priority < 0 || p.Priority > 100 {
		return errors.New("priority must be between 0 and 100")
	}
	types := map[NodeType]bool{}
	for _, v := range p.AllowedNodeTypes {
		if !nodeTypes[v] {
			return fmt.Errorf("unknown allowed node type %q", v)
		}
		if types[v] {
			return fmt.Errorf("duplicate allowed node type %q", v)
		}
		types[v] = true
	}
	predicates := map[Predicate]bool{}
	validPredicates := map[Predicate]bool{Owns: true, Maintains: true, KnowsAbout: true, Uses: true, CanOperate: true, RoutesThrough: true, ProvidesAccessTo: true, Hosts: true, ImplementedFrom: true, HasRunningPeriod: true, HasCredential: true, Documents: true, AppliesTo: true, Targets: true, Supersedes: true, DependsOn: true}
	for _, v := range p.AllowedPredicates {
		if !validPredicates[v] {
			return fmt.Errorf("unknown allowed predicate %q", v)
		}
		if predicates[v] {
			return fmt.Errorf("duplicate allowed predicate %q", v)
		}
		predicates[v] = true
	}
	fields := map[string]bool{}
	for _, v := range p.AllowedFieldPaths {
		if strings.TrimSpace(v) == "" || len(v) > 200 {
			return errors.New("field paths must be non-empty and at most 200 characters")
		}
		if fields[v] {
			return fmt.Errorf("duplicate allowed field path %q", v)
		}
		fields[v] = true
	}
	return nil
}
func (p SourcePolicy) ValidateBatch(batch SourceBatch) []string {
	if err := p.Validate(); err != nil {
		return []string{err.Error()}
	}
	violations := []string{}
	types := map[NodeType]bool{}
	for _, v := range p.AllowedNodeTypes {
		types[v] = true
	}
	predicates := map[Predicate]bool{}
	for _, v := range p.AllowedPredicates {
		predicates[v] = true
	}
	fields := map[string]bool{}
	for _, v := range p.AllowedFieldPaths {
		fields[v] = true
	}
	for _, n := range batch.Nodes {
		if !types[n.Type] {
			violations = append(violations, fmt.Sprintf("source policy %q disallows node type %q for %q", p.Source, n.Type, n.ID))
		}
	}
	for _, r := range batch.Relations {
		if !predicates[r.Predicate] {
			violations = append(violations, fmt.Sprintf("source policy %q disallows relation predicate %q for %q", p.Source, r.Predicate, r.ID))
		}
	}
	for _, a := range batch.Assertions {
		if !fields[a.FieldPath] {
			violations = append(violations, fmt.Sprintf("source policy %q disallows field path %q for assertion %q", p.Source, a.FieldPath, a.ID))
		}
	}
	sort.Strings(violations)
	return violations
}
