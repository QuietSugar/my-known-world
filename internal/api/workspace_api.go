package api

import (
	"errors"
	"net/http"
	"sort"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidNodeType = errors.New("node is not a SoftwareProject")
)

type ProjectWorkspaceData struct {
	Project     graph.Node       `json:"project"`
	Instances   []graph.Node     `json:"instances"`
	Machines    []graph.Node     `json:"machines"`
	Credentials []graph.Node     `json:"credentials"`
	Documents   []graph.Node     `json:"documents"`
	WorkItems   []graph.Node     `json:"workItems"`
	Relations   []graph.Relation `json:"relations"`
}

// AggregateProjectWorkspace collects all entities and relations relevant to a SoftwareProject.
func AggregateProjectWorkspace(g *graph.Graph, projectID string) (ProjectWorkspaceData, error) {
	if g == nil {
		return ProjectWorkspaceData{}, errors.New("graph is nil")
	}
	projNode, ok := g.Node(projectID)
	if !ok {
		return ProjectWorkspaceData{}, ErrProjectNotFound
	}
	if projNode.Type != graph.SoftwareProject {
		return ProjectWorkspaceData{}, ErrInvalidNodeType
	}

	allRelations := g.Relations()

	// 1. Discover all SoftwareInstance nodes connected to the project.
	instanceIDs := make(map[string]bool)
	for _, r := range allRelations {
		// instance -> implementedFrom -> project
		if r.To == projectID && r.Predicate == graph.ImplementedFrom {
			if n, exists := g.Node(r.From); exists && n.Type == graph.SoftwareInstance {
				instanceIDs[r.From] = true
			}
		}
		// project -> hasInstance -> instance
		if r.From == projectID && r.Predicate == graph.HasInstance {
			if n, exists := g.Node(r.To); exists && n.Type == graph.SoftwareInstance {
				instanceIDs[r.To] = true
			}
		}
		// Any direct relation between project and an instance
		if r.From == projectID {
			if n, exists := g.Node(r.To); exists && n.Type == graph.SoftwareInstance {
				instanceIDs[r.To] = true
			}
		}
		if r.To == projectID {
			if n, exists := g.Node(r.From); exists && n.Type == graph.SoftwareInstance {
				instanceIDs[r.From] = true
			}
		}
	}

	// 1b. Transitive instance dependencies (e.g. instance -> dependsOn/hasInstance -> instance)
	changed := true
	for changed {
		changed = false
		for _, r := range allRelations {
			if r.Predicate == graph.DependsOn || r.Predicate == graph.HasInstance {
				if instanceIDs[r.From] {
					if n, exists := g.Node(r.To); exists && n.Type == graph.SoftwareInstance && !instanceIDs[r.To] {
						instanceIDs[r.To] = true
						changed = true
					}
				}
				if instanceIDs[r.To] {
					if n, exists := g.Node(r.From); exists && n.Type == graph.SoftwareInstance && !instanceIDs[r.From] {
						instanceIDs[r.From] = true
						changed = true
					}
				}
			}
		}
	}

	// 2. Discover host Machine nodes hosting the instances (via hosts or direct relations)
	machineIDs := make(map[string]bool)
	for _, r := range allRelations {
		// machine -> hosts -> instance
		if r.Predicate == graph.Hosts {
			if instanceIDs[r.To] {
				if n, exists := g.Node(r.From); exists && n.Type == graph.Machine {
					machineIDs[r.From] = true
				}
			}
			if instanceIDs[r.From] {
				if n, exists := g.Node(r.To); exists && n.Type == graph.Machine {
					machineIDs[r.To] = true
				}
			}
		}
		// instance -> dependsOn -> machine
		if r.Predicate == graph.DependsOn {
			if instanceIDs[r.From] {
				if n, exists := g.Node(r.To); exists && n.Type == graph.Machine {
					machineIDs[r.To] = true
				}
			}
		}
		// Direct relation to project
		if r.From == projectID {
			if n, exists := g.Node(r.To); exists && n.Type == graph.Machine {
				machineIDs[r.To] = true
			}
		}
		if r.To == projectID {
			if n, exists := g.Node(r.From); exists && n.Type == graph.Machine {
				machineIDs[r.From] = true
			}
		}
	}

	isBaseEntity := func(id string) bool {
		return id == projectID || instanceIDs[id] || machineIDs[id]
	}

	// 3. Discover AccountCredential nodes attached to project, instances, or host machines
	credentialIDs := make(map[string]bool)
	for _, r := range allRelations {
		if isBaseEntity(r.From) {
			if n, exists := g.Node(r.To); exists && n.Type == graph.AccountCredential {
				credentialIDs[r.To] = true
			}
		}
		if isBaseEntity(r.To) {
			if n, exists := g.Node(r.From); exists && n.Type == graph.AccountCredential {
				credentialIDs[r.From] = true
			}
		}
	}

	// 4. Discover Document and Runbook nodes (documents, appliesTo, relatedTo)
	documentIDs := make(map[string]bool)
	for _, r := range allRelations {
		if isBaseEntity(r.From) || credentialIDs[r.From] {
			if n, exists := g.Node(r.To); exists && (n.Type == graph.Document || n.Type == graph.Runbook) {
				documentIDs[r.To] = true
			}
		}
		if isBaseEntity(r.To) || credentialIDs[r.To] {
			if n, exists := g.Node(r.From); exists && (n.Type == graph.Document || n.Type == graph.Runbook) {
				documentIDs[r.From] = true
			}
		}
	}

	// 5. Discover WorkItem nodes (targets, relatedTo)
	workItemIDs := make(map[string]bool)
	for _, r := range allRelations {
		if isBaseEntity(r.From) || documentIDs[r.From] {
			if n, exists := g.Node(r.To); exists && n.Type == graph.WorkItem {
				workItemIDs[r.To] = true
			}
		}
		if isBaseEntity(r.To) || documentIDs[r.To] {
			if n, exists := g.Node(r.From); exists && n.Type == graph.WorkItem {
				workItemIDs[r.From] = true
			}
		}
	}

	// 6. Active relations between all identified entities
	allEntityIDs := make(map[string]bool)
	allEntityIDs[projectID] = true
	for id := range instanceIDs {
		allEntityIDs[id] = true
	}
	for id := range machineIDs {
		allEntityIDs[id] = true
	}
	for id := range credentialIDs {
		allEntityIDs[id] = true
	}
	for id := range documentIDs {
		allEntityIDs[id] = true
	}
	for id := range workItemIDs {
		allEntityIDs[id] = true
	}

	activeRelations := make([]graph.Relation, 0)
	for _, r := range allRelations {
		if allEntityIDs[r.From] && allEntityIDs[r.To] {
			activeRelations = append(activeRelations, r)
		}
	}

	collectSorted := func(idMap map[string]bool) []graph.Node {
		ids := make([]string, 0, len(idMap))
		for id := range idMap {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		nodes := make([]graph.Node, 0, len(ids))
		for _, id := range ids {
			if n, exists := g.Node(id); exists {
				nodes = append(nodes, n)
			}
		}
		return nodes
	}

	sort.Slice(activeRelations, func(i, j int) bool {
		return activeRelations[i].ID < activeRelations[j].ID
	})

	return ProjectWorkspaceData{
		Project:     projNode,
		Instances:   collectSorted(instanceIDs),
		Machines:    collectSorted(machineIDs),
		Credentials: collectSorted(credentialIDs),
		Documents:   collectSorted(documentIDs),
		WorkItems:   collectSorted(workItemIDs),
		Relations:   activeRelations,
	}, nil
}

func (h Handler) projectWorkspaceAPI(w http.ResponseWriter, r *http.Request, g *graph.Graph, id string) {
	data, err := AggregateProjectWorkspace(g, id)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "project not found")
			return
		}
		if errors.Is(err, ErrInvalidNodeType) {
			writeError(w, http.StatusBadRequest, "invalid_node_type", "node is not a SoftwareProject")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}
