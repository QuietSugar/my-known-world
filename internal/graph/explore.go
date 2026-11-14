package graph

import (
	"errors"
	"sort"
)

type ExploreOptions struct {
	CenterID   string
	Direction  string
	Depth      int
	Predicates []Predicate
}
type Exploration struct {
	CenterID  string     `json:"centerId"`
	Direction string     `json:"direction"`
	Depth     int        `json:"depth"`
	Nodes     []Node     `json:"nodes"`
	Relations []Relation `json:"relations"`
	Truncated bool       `json:"truncated"`
}

// Explore returns the connected subgraph around any selected center while
// respecting direction, depth, and optional predicate filters.
func (g *Graph) Explore(options ExploreOptions) (Exploration, error) {
	if _, ok := g.nodes[options.CenterID]; !ok {
		return Exploration{}, errors.New("center node not found")
	}
	if options.Direction == "" {
		options.Direction = "both"
	}
	if options.Direction != "both" && options.Direction != "out" && options.Direction != "in" {
		return Exploration{}, errors.New("direction must be both, out, or in")
	}
	if options.Depth < 1 || options.Depth > 5 {
		return Exploration{}, errors.New("depth must be between 1 and 5")
	}
	allowed := map[Predicate]bool{}
	for _, p := range options.Predicates {
		allowed[p] = true
	}
	visited := map[string]int{options.CenterID: 0}
	included := map[string]Relation{}
	queue := []string{options.CenterID}
	relations := g.Relations()
	truncated := false
	const maxNodes = 250
	for len(queue) > 0 && len(visited) < maxNodes {
		cur := queue[0]
		queue = queue[1:]
		depth := visited[cur]
		if depth >= options.Depth {
			continue
		}
		for _, r := range relations {
			if len(allowed) > 0 && !allowed[r.Predicate] {
				continue
			}
			next := ""
			if r.From == cur && options.Direction != "in" {
				next = r.To
			} else if r.To == cur && options.Direction != "out" {
				next = r.From
			}
			if next == "" {
				continue
			}
			if _, seen := visited[next]; !seen {
				if len(visited) >= maxNodes {
					truncated = true
					continue
				}
				visited[next] = depth + 1
				queue = append(queue, next)
			}
			included[r.ID] = r
		}
	}
	if len(queue) > 0 {
		truncated = true
	}
	ids := make([]string, 0, len(visited))
	for id := range visited {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := Exploration{CenterID: options.CenterID, Direction: options.Direction, Depth: options.Depth, Nodes: make([]Node, 0, len(ids)), Relations: make([]Relation, 0, len(included)), Truncated: truncated}
	for _, id := range ids {
		out.Nodes = append(out.Nodes, g.nodes[id])
	}
	for _, r := range relations {
		if _, ok := included[r.ID]; ok {
			out.Relations = append(out.Relations, r)
		}
	}
	return out, nil
}
