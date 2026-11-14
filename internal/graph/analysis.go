package graph

import "sort"

// ImpactPath contains an object reached through confirmed impact-propagating relations.
type ImpactPath struct {
	Node         Node       `json:"node"`
	RelationPath []Relation `json:"relationPath"`
}

// ImpactFromSelf follows only confirmed operational/deployment relations. knowsAbout,
// documents and appliesTo are deliberately excluded from impact propagation.
func (g *Graph) ImpactFromSelf() []ImpactPath {
	self := ""
	for id, node := range g.nodes {
		if node.Type == Self {
			self = id
			break
		}
	}
	if self == "" {
		return nil
	}
	allowed := map[Predicate]bool{Owns: true, Maintains: true, Uses: true, Hosts: true, ImplementedFrom: true, HasRunningPeriod: true, HasCredential: true, DependsOn: true, RoutesThrough: true, ProvidesAccessTo: true}
	paths := map[string][]Relation{self: {}}
	queue := []string{self}
	relations := g.Relations()
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, relation := range relations {
			if !allowed[relation.Predicate] || relation.From != current {
				continue
			}
			if _, seen := paths[relation.To]; seen {
				continue
			}
			path := append(append([]Relation{}, paths[current]...), relation)
			paths[relation.To] = path
			queue = append(queue, relation.To)
		}
	}
	ids := make([]string, 0, len(paths))
	for id := range paths {
		if id != self {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	result := make([]ImpactPath, 0, len(ids))
	for _, id := range ids {
		result = append(result, ImpactPath{Node: g.nodes[id], RelationPath: paths[id]})
	}
	return result
}
