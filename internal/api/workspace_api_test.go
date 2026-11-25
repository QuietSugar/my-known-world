package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestProjectWorkspaceAPI_NotFound(t *testing.T) {
	h := Handler{Graph: fixture()}
	w := request(h, http.MethodGet, "/api/workspace/projects/non-existent-proj")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "project not found") {
		t.Fatalf("expected 'project not found', got %s", w.Body.String())
	}
}

func TestProjectWorkspaceAPI_InvalidNodeType(t *testing.T) {
	g := fixture() // contains "machine" which is of type Machine
	h := Handler{Graph: g}
	w := request(h, http.MethodGet, "/api/workspace/projects/machine")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "node is not a SoftwareProject") {
		t.Fatalf("expected 'node is not a SoftwareProject', got %s", w.Body.String())
	}
}

func TestProjectWorkspaceAPI_Success(t *testing.T) {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})

	// SoftwareProject
	_ = g.Publish(graph.Node{
		ID:           "xasc",
		Type:         graph.SoftwareProject,
		Name:         "XASC Platform",
		ModelVersion: 1,
		SoftwareProject: &graph.SoftwareProjectDetails{
			Purpose:          "Core security aggregation platform",
			Languages:        []string{"Go", "TypeScript"},
			Form:             "service",
			RemoteRepository: "git@codeup.aliyun.com:org/xasc.git",
			Upstream:         "internal",
			MaintenanceState: "active",
		},
	}, graph.Relation{ID: "r-self-xasc", To: "xasc", Predicate: graph.KnowsAbout})

	// SoftwareInstance
	_ = g.AddNode(graph.Node{
		ID:           "xasc-prod",
		Type:         graph.SoftwareInstance,
		Name:         "xasc-prod-01",
		ModelVersion: 1,
		SoftwareInstance: &graph.SoftwareInstanceDetails{
			ProductName:      "xasc",
			Domains:          []string{"xasc.prod.internal"},
			Ports:            []int{8443},
			DeploymentPath:   "/opt/services/xasc",
			OperationalState: "running",
			DeploymentState:  "deployed",
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "r-xasc-inst",
		From:      "xasc-prod",
		Predicate: graph.ImplementedFrom,
		To:        "xasc",
	})

	// Machine
	_ = g.AddNode(graph.Node{
		ID:           "machine-srv1",
		Type:         graph.Machine,
		Name:         "srv-prod-01",
		ModelVersion: 1,
		Machine: &graph.MachineDetails{
			Hostname: "srv-prod-01",
			IPs:      []string{"10.20.30.40"},
			Network:  "prod-vpc",
			Status:   "healthy",
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "r-host-xasc",
		From:      "machine-srv1",
		Predicate: graph.Hosts,
		To:        "xasc-prod",
	})

	// Credential attached to instance
	_ = g.AddNode(graph.Node{
		ID:           "cred-xasc-db",
		Type:         graph.AccountCredential,
		Name:         "xasc-db-cred",
		ModelVersion: 1,
		AccountCredential: &graph.AccountCredentialDetails{
			Account:         "xasc_user",
			CredentialType:  "database_password",
			Purpose:         "Production Postgres DB access",
			AccessReference: "vault://secret/prod/xasc/db",
		},
	})
	if err := g.AddRelation(graph.Relation{
		ID:        "r-cred-xasc",
		From:      "xasc-prod",
		Predicate: graph.HasCredential,
		To:        "cred-xasc-db",
	}); err != nil {
		t.Fatalf("failed to add credential relation: %v", err)
	}

	// Document
	_ = g.AddNode(graph.Node{
		ID:           "doc-arch",
		Type:         graph.Document,
		Name:         "XASC Architecture Design",
		ModelVersion: 1,
		Document: &graph.DocumentDetails{
			Role:                   "architecture",
			AuthoritativeReference: "https://wiki.corp/xasc/arch",
			Effective:              true,
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "r-doc-xasc",
		From:      "doc-arch",
		Predicate: graph.Documents,
		To:        "xasc",
	})

	// Runbook
	_ = g.AddNode(graph.Node{
		ID:           "rb-restart",
		Type:         graph.Runbook,
		Name:         "XASC Restart Procedure",
		ModelVersion: 1,
		Runbook: &graph.RunbookDetails{
			Trigger:        "service degradation",
			Preconditions:  []string{"check database health"},
			StepsReference: "wiki://runbooks/xasc/restart.md",
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "r-rb-inst",
		From:      "rb-restart",
		Predicate: graph.AppliesTo,
		To:        "xasc-prod",
	})

	// WorkItem
	_ = g.AddNode(graph.Node{
		ID:           "wi-upgrade",
		Type:         graph.WorkItem,
		Name:         "Upgrade to Go 1.24",
		ModelVersion: 1,
		WorkItem: &graph.WorkItemDetails{
			Kind:   "maintenance",
			Status: "in_progress",
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "r-wi-xasc",
		From:      "wi-upgrade",
		Predicate: graph.Targets,
		To:        "xasc",
	})

	h := Handler{Graph: g}
	w := request(h, http.MethodGet, "/api/workspace/projects/xasc")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var data ProjectWorkspaceData
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if data.Project.ID != "xasc" {
		t.Errorf("expected project ID xasc, got %s", data.Project.ID)
	}
	if data.Project.SoftwareProject == nil || data.Project.SoftwareProject.RemoteRepository != "git@codeup.aliyun.com:org/xasc.git" {
		t.Errorf("softwareProject details not preserved: %+v", data.Project.SoftwareProject)
	}
	if len(data.Instances) != 1 || data.Instances[0].ID != "xasc-prod" {
		t.Errorf("expected 1 instance xasc-prod, got: %+v", data.Instances)
	}
	if len(data.Machines) != 1 || data.Machines[0].ID != "machine-srv1" {
		t.Errorf("expected 1 machine machine-srv1, got: %+v", data.Machines)
	}
	if len(data.Credentials) != 1 || data.Credentials[0].ID != "cred-xasc-db" {
		t.Errorf("expected 1 credential cred-xasc-db, got: %+v", data.Credentials)
	}
	if len(data.Documents) != 2 {
		t.Errorf("expected 2 documents (doc-arch, rb-restart), got %d: %+v", len(data.Documents), data.Documents)
	}
	if len(data.WorkItems) != 1 || data.WorkItems[0].ID != "wi-upgrade" {
		t.Errorf("expected 1 work item wi-upgrade, got: %+v", data.WorkItems)
	}
	if len(data.Relations) != 6 {
		t.Errorf("expected 6 active relations, got %d: %+v", len(data.Relations), data.Relations)
	}
}

func TestProjectWorkspaceAPI_EmptyArraysSerializeCleanly(t *testing.T) {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{
		ID:           "empty-proj",
		Type:         graph.SoftwareProject,
		Name:         "Empty Project",
		ModelVersion: 1,
	}, graph.Relation{ID: "r-empty", To: "empty-proj"})

	h := Handler{Graph: g}
	w := request(h, http.MethodGet, "/api/workspace/projects/empty-proj")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, expected := range []string{`"instances":[]`, `"machines":[]`, `"credentials":[]`, `"documents":[]`, `"workItems":[]`, `"relations":[]`} {
		if !strings.Contains(body, expected) {
			t.Errorf("expected body to contain %s, got: %s", expected, body)
		}
	}
}

func TestProjectWorkspaceAPI_SmartIDMatching(t *testing.T) {
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{
		ID:           "projects/xasc",
		Type:         graph.SoftwareProject,
		Name:         "XASC Platform",
		ModelVersion: 1,
	}, graph.Relation{ID: "r-xasc", To: "projects/xasc"})

	h := Handler{Graph: g}

	// 1. Match by trimmed ID prefix ("xasc" -> "projects/xasc")
	w1 := request(h, http.MethodGet, "/api/workspace/projects/xasc")
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 for 'xasc', got %d: %s", w1.Code, w1.Body.String())
	}

	// 2. Match by exact full ID
	w2 := request(h, http.MethodGet, "/api/workspace/projects/projects/xasc")
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for 'projects/xasc', got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Match by Project Name
	w3 := request(h, http.MethodGet, "/api/workspace/projects/XASC%20Platform")
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for name match, got %d: %s", w3.Code, w3.Body.String())
	}
}
