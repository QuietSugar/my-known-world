package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func setupTestGraph() *graph.Graph {
	g := graph.New()
	now := time.Now()
	recent := now.Add(-10 * 24 * time.Hour)
	stale := now.Add(-100 * 24 * time.Hour)

	// 1. Self node
	_ = g.AddNode(graph.Node{
		ID:           "self",
		Type:         graph.Self,
		Name:         "Self",
		Source:       "system",
		EvidenceKind: graph.EvidenceManual,
		Certainty:    graph.CertaintyConfirmed,
		ModelVersion: 1,
	})

	// 2. Machine connected to Self with business relation to service
	_ = g.Publish(graph.Node{
		ID:            "machine-prod-01",
		Type:          graph.Machine,
		Name:          "Production Server",
		Lifecycle:     "active",
		Source:        "ansible",
		EvidenceKind:  graph.EvidenceDocument,
		Certainty:     graph.CertaintyConfirmed,
		ModelVersion:  1,
		LastConfirmed: &recent,
		Machine: &graph.MachineDetails{
			Hostname: "prod-01.internal.net",
			OS:       "Debian 12",
			IPs:      []string{"10.0.1.10", "192.168.1.50"},
		},
	}, graph.Relation{
		ID:        "self-owns-machine",
		Predicate: graph.Owns,
		To:        "machine-prod-01",
		Source:    "system",
	})

	// 3. SoftwareInstance hosted on machine-prod-01 (business relation Hosts)
	_ = g.AddNode(graph.Node{
		ID:            "instance-web",
		Type:          graph.SoftwareInstance,
		Name:          "Web Application",
		Lifecycle:     "running",
		Source:        "deploy",
		EvidenceKind:  graph.EvidenceDocument,
		Certainty:     graph.CertaintyConfirmed,
		ModelVersion:  1,
		LastConfirmed: &recent,
		SoftwareInstance: &graph.SoftwareInstanceDetails{
			ProductName:    "EnterpriseApp",
			Domains:        []string{"app.example.com", "portal.example.com"},
			Ports:          []int{8080, 8443},
			DeploymentPath: "/opt/app",
		},
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "machine-hosts-instance",
		From:      "machine-prod-01",
		Predicate: graph.Hosts,
		To:        "instance-web",
		Source:    "deploy",
	})

	// 4. SoftwareProject
	_ = g.Publish(graph.Node{
		ID:            "proj-frontend",
		Type:          graph.SoftwareProject,
		Name:          "Frontend UI Portal",
		Lifecycle:     "maintained",
		Source:        "gitlab",
		EvidenceKind:  graph.EvidenceDocument,
		Certainty:     graph.CertaintyConfirmed,
		ModelVersion:  1,
		LastConfirmed: &recent,
		SoftwareProject: &graph.SoftwareProjectDetails{
			RemoteRepository: "github.com/company/frontend-portal",
			Purpose:          "User authentication and dashboard interface",
		},
	}, graph.Relation{
		ID:        "self-maintains-proj",
		Predicate: graph.Maintains,
		To:        "proj-frontend",
		Source:    "system",
	})
	// Connect instance to project via business relation
	_ = g.AddRelation(graph.Relation{
		ID:        "instance-implemented-from-proj",
		From:      "instance-web",
		Predicate: graph.ImplementedFrom,
		To:        "proj-frontend",
		Source:    "deploy",
	})

	// 5. Hardware (Model & Category) - isolated leaf (only Self anchor)
	_ = g.Publish(graph.Node{
		ID:            "hw-switch-01",
		Type:          graph.Hardware,
		Name:          "Core Switch",
		Lifecycle:     "active",
		Source:        "inventory",
		EvidenceKind:  graph.EvidenceDocument,
		Certainty:     graph.CertaintyConfirmed,
		ModelVersion:  1,
		LastConfirmed: &stale, // > 90 days ago -> staleConfirmation
		Hardware: &graph.HardwareDetails{
			Model:    "Nexus-9300",
			Category: "switch",
			Location: "Rack-A1",
		},
	}, graph.Relation{
		ID:        "self-owns-hw",
		Predicate: graph.Owns,
		To:        "hw-switch-01",
		Source:    "inventory",
	})

	// 6. ProvisionalObject with reviewStatus == "pending"
	_ = g.Publish(graph.Node{
		ID:              "provisional-host-candidate",
		Type:            graph.ProvisionalObject,
		Name:            "Suspected Staging VM",
		Source:          "scanner",
		EvidenceKind:    graph.EvidenceDocument,
		Certainty:       graph.CertaintyProvisional,
		ModelVersion:    1,
		CandidateType:   graph.Machine,
		ReviewStatus:    "pending",
		ReviewQuestions: []string{"Which team owns this VM?"},
		// LastConfirmed is nil -> staleConfirmation
	}, graph.Relation{
		ID:        "self-knows-provisional",
		Predicate: graph.KnowsAbout,
		To:        "provisional-host-candidate",
		Source:    "scanner",
	})

	// 7. Verified Credential (healthy)
	_ = g.Publish(graph.Node{
		ID:            "cred-vault-root",
		Type:          graph.AccountCredential,
		Name:          "Vault Root Token",
		Lifecycle:     "active",
		Source:        "vault",
		EvidenceKind:  graph.EvidenceManual,
		Certainty:     graph.CertaintyConfirmed,
		ModelVersion:  1,
		LastConfirmed: &recent,
		AccountCredential: &graph.AccountCredentialDetails{
			Account: "root",
			Status:  "verified",
			Purpose: "Secret management access",
		},
	}, graph.Relation{
		ID:        "self-uses-cred-vault",
		Predicate: graph.Uses,
		To:        "cred-vault-root",
		Source:    "vault",
	})
	_ = g.AddRelation(graph.Relation{
		ID:        "cred-applies-to-machine",
		From:      "cred-vault-root",
		Predicate: graph.AppliesTo,
		To:        "machine-prod-01",
		Source:    "vault",
	})

	// 8. Unverified Credential (status not verified, nil LastConfirmed)
	_ = g.Publish(graph.Node{
		ID:           "cred-legacy-db",
		Type:         graph.AccountCredential,
		Name:         "Legacy DB Password",
		Lifecycle:    "deprecated",
		Source:       "manual:notes",
		EvidenceKind: graph.EvidenceManual,
		Certainty:    graph.CertaintyProvisional,
		ModelVersion: 1,
		// LastConfirmed is nil
		AccountCredential: &graph.AccountCredentialDetails{
			Account: "admin_db",
			Status:  "unverified",
			Purpose: "Legacy MySQL root",
		},
	}, graph.Relation{
		ID:        "self-knows-legacy-db",
		Predicate: graph.KnowsAbout,
		To:        "cred-legacy-db",
		Source:    "manual:notes",
	})

	return g
}

func TestSearchAPI_ValidationAndMatching(t *testing.T) {
	g := setupTestGraph()
	handler := &Handler{Graph: g}

	// 1. Missing parameter 'q'
	req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing q, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Empty parameter 'q'
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=%20%20", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty q, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Invalid limit
	for _, invalidLimit := range []string{"0", "-5", "51", "abc"} {
		req = httptest.NewRequest(http.MethodGet, "/api/search?q=prod&limit="+invalidLimit, nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for limit=%s, got %d: %s", invalidLimit, w.Code, w.Body.String())
		}
	}

	// 4. Test matching across different fields
	cases := []struct {
		name       string
		query      string
		expectedID string
	}{
		{"by Node ID", "machine-prod-01", "machine-prod-01"},
		{"by Node Name", "Production Server", "machine-prod-01"},
		{"by Node Lifecycle", "active", "machine-prod-01"}, // also matches hw-switch-01, cred-vault-root
		{"by Node Source", "ansible", "machine-prod-01"},
		{"by Machine Hostname", "prod-01.internal.net", "machine-prod-01"},
		{"by Machine IP", "192.168.1.50", "machine-prod-01"},
		{"by Hardware Model", "Nexus-9300", "hw-switch-01"},
		{"by Hardware Category", "switch", "hw-switch-01"},
		{"by SoftwareProject RemoteRepository", "frontend-portal", "proj-frontend"},
		{"by SoftwareProject Purpose", "dashboard interface", "proj-frontend"},
		{"by SoftwareInstance Domain", "portal.example.com", "instance-web"},
		{"by SoftwareInstance Port", "8443", "instance-web"},
		{"by AccountCredential Account", "admin_db", "cred-legacy-db"},
		{"case insensitive matching", "PROD-01.INTERNAL", "machine-prod-01"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/search?q="+url.QueryEscape(tc.query), nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}
			var resp SearchResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			if resp.Count == 0 || len(resp.Items) == 0 {
				t.Fatalf("expected results for %q, got 0", tc.query)
			}
			if resp.Query != tc.query {
				t.Fatalf("expected query %q, got %q", tc.query, resp.Query)
			}
			found := false
			for _, item := range resp.Items {
				if item.ID == tc.expectedID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected item %q in search results for %q, got: %+v", tc.expectedID, tc.query, resp.Items)
			}
		})
	}

	// 5. Test limit parameter defaults and truncation
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=a&limit=2", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var limitResp SearchResponse
	_ = json.Unmarshal(w.Body.Bytes(), &limitResp)
	if len(limitResp.Items) > 2 || limitResp.Count > 2 {
		t.Fatalf("expected at most 2 items with limit=2, got %d", len(limitResp.Items))
	}

	// 6. Test no results
	req = httptest.NewRequest(http.MethodGet, "/api/search?q=nonexistent_string_12345", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var noMatchResp SearchResponse
	_ = json.Unmarshal(w.Body.Bytes(), &noMatchResp)
	if noMatchResp.Count != 0 || len(noMatchResp.Items) != 0 {
		t.Fatalf("expected 0 results, got %d", noMatchResp.Count)
	}

	// 7. Verify zero-resident-memory: search does not accumulate state across 100 iterations
	for i := 0; i < 100; i++ {
		req = httptest.NewRequest(http.MethodGet, "/api/search?q=prod", nil)
		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("iteration %d failed", i)
		}
	}
}

func TestPatrolAPI_FindingsAndSummary(t *testing.T) {
	g := setupTestGraph()
	handler := &Handler{Graph: g}

	req := httptest.NewRequest(http.MethodGet, "/api/patrol", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp PatrolResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal patrol response failed: %v", err)
	}

	// Check stale confirmation
	staleIDs := make(map[string]bool)
	for _, item := range resp.Findings.StaleConfirmation {
		staleIDs[item.ID] = true
	}
	// hw-switch-01 (>90d), provisional-host-candidate (nil), cred-legacy-db (nil) must be in staleConfirmation
	if !staleIDs["hw-switch-01"] {
		t.Error("hw-switch-01 (>90d) should be in staleConfirmation")
	}
	if !staleIDs["provisional-host-candidate"] {
		t.Error("provisional-host-candidate (nil lastConfirmed) should be in staleConfirmation")
	}
	if !staleIDs["cred-legacy-db"] {
		t.Error("cred-legacy-db (nil lastConfirmed) should be in staleConfirmation")
	}
	// machine-prod-01, instance-web, proj-frontend, cred-vault-root are recently confirmed (<90d)
	if staleIDs["machine-prod-01"] || staleIDs["instance-web"] || staleIDs["proj-frontend"] || staleIDs["cred-vault-root"] {
		t.Error("recently confirmed nodes must not be in staleConfirmation")
	}
	// Self must not be in staleConfirmation
	if staleIDs["self"] {
		t.Error("Self node must not be in staleConfirmation")
	}

	// Check pending provisionals
	pendingIDs := make(map[string]bool)
	for _, item := range resp.Findings.PendingProvisionals {
		pendingIDs[item.ID] = true
	}
	if !pendingIDs["provisional-host-candidate"] {
		t.Error("provisional-host-candidate should be in pendingProvisionals")
	}
	if pendingIDs["machine-prod-01"] || pendingIDs["cred-vault-root"] {
		t.Error("non-pending nodes must not be in pendingProvisionals")
	}

	// Check isolated leaves
	isolatedIDs := make(map[string]bool)
	for _, item := range resp.Findings.IsolatedLeaves {
		isolatedIDs[item.ID] = true
	}
	// hw-switch-01 has only self-owns-hw relation, zero business relations -> isolated
	if !isolatedIDs["hw-switch-01"] {
		t.Error("hw-switch-01 has zero business relations, should be in isolatedLeaves")
	}
	// provisional-host-candidate has only self-knows-provisional -> isolated
	if !isolatedIDs["provisional-host-candidate"] {
		t.Error("provisional-host-candidate should be in isolatedLeaves")
	}
	// machine-prod-01 has Hosts and AppliesTo relations -> NOT isolated
	if isolatedIDs["machine-prod-01"] {
		t.Error("machine-prod-01 has business relations, must not be in isolatedLeaves")
	}
	// instance-web has Hosts and ImplementedFrom -> NOT isolated
	if isolatedIDs["instance-web"] {
		t.Error("instance-web has business relations, must not be in isolatedLeaves")
	}
	// proj-frontend has ImplementedFrom -> NOT isolated
	if isolatedIDs["proj-frontend"] {
		t.Error("proj-frontend has business relations, must not be in isolatedLeaves")
	}

	// Check unverified credentials
	unverifiedIDs := make(map[string]bool)
	for _, item := range resp.Findings.UnverifiedCredentials {
		unverifiedIDs[item.ID] = true
	}
	if !unverifiedIDs["cred-legacy-db"] {
		t.Error("cred-legacy-db (status unverified, nil lastConfirmed) should be in unverifiedCredentials")
	}
	if unverifiedIDs["cred-vault-root"] {
		t.Error("cred-vault-root (status verified, fresh lastConfirmed) must not be in unverifiedCredentials")
	}

	// Verify Summary
	if resp.Summary.TotalNodes != len(g.Nodes()) {
		t.Errorf("expected totalNodes %d, got %d", len(g.Nodes()), resp.Summary.TotalNodes)
	}
	expectedTotal := resp.Summary.StaleConfirmationCount + resp.Summary.PendingProvisionalsCount +
		resp.Summary.IsolatedLeavesCount + resp.Summary.UnverifiedCredentialsCount
	if resp.Summary.TotalFindings != expectedTotal {
		t.Errorf("expected totalFindings %d, got %d", expectedTotal, resp.Summary.TotalFindings)
	}
	if resp.Summary.Grade == "" || resp.Summary.Status == "" {
		t.Error("grade and status should be non-empty")
	}

	// Test Healthy Graph Patrol
	t.Run("HealthyGraph", func(t *testing.T) {
		healthyG := graph.New()
		now := time.Now()
		_ = healthyG.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
		_ = healthyG.Publish(graph.Node{
			ID:            "m1",
			Type:          graph.Machine,
			Name:          "M1",
			ModelVersion:  1,
			LastConfirmed: &now,
		}, graph.Relation{ID: "r1", Predicate: graph.Owns, To: "m1"})
		_ = healthyG.Publish(graph.Node{
			ID:            "m2",
			Type:          graph.Machine,
			Name:          "M2",
			ModelVersion:  1,
			LastConfirmed: &now,
		}, graph.Relation{ID: "r2", Predicate: graph.Owns, To: "m2"})
		// Add business relation so neither is isolated
		if err := healthyG.AddRelation(graph.Relation{
			ID:        "m1-depends-on-m2",
			From:      "m1",
			Predicate: graph.DependsOn,
			To:        "m2",
		}); err != nil {
			t.Fatalf("failed to add business relation: %v", err)
		}
		hHandler := &Handler{Graph: healthyG}
		hReq := httptest.NewRequest(http.MethodGet, "/api/patrol", nil)
		hW := httptest.NewRecorder()
		hHandler.ServeHTTP(hW, hReq)
		if hW.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", hW.Code)
		}
		var hResp PatrolResponse
		_ = json.Unmarshal(hW.Body.Bytes(), &hResp)
		if hResp.Summary.TotalFindings != 0 {
			t.Errorf("expected 0 findings for healthy graph, got %d", hResp.Summary.TotalFindings)
		}
		if hResp.Summary.Grade != "A" || hResp.Summary.Status != "healthy" {
			t.Errorf("expected grade A and healthy, got %s / %s", hResp.Summary.Grade, hResp.Summary.Status)
		}
	})
}

func TestMetricsAPI_PrometheusTextFormat(t *testing.T) {
	g := setupTestGraph()
	handler := &Handler{Graph: g}

	for _, endpoint := range []string{"/metrics", "/api/metrics"} {
		t.Run(endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, endpoint, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.HasPrefix(contentType, "text/plain; version=0.0.4") {
				t.Fatalf("expected Prometheus 0.0.4 content-type, got %q", contentType)
			}

			body := w.Body.String()

			// Check mkw_nodes_total gauges
			if !strings.Contains(body, "# HELP mkw_nodes_total") || !strings.Contains(body, "# TYPE mkw_nodes_total gauge") {
				t.Error("missing mkw_nodes_total metadata")
			}
			if !strings.Contains(body, `mkw_nodes_total{type="Machine"}`) {
				t.Error("missing mkw_nodes_total for Machine")
			}
			if !strings.Contains(body, `mkw_nodes_total{type="Self"}`) {
				t.Error("missing mkw_nodes_total for Self")
			}

			// Check mkw_relations_total gauges
			if !strings.Contains(body, "# HELP mkw_relations_total") || !strings.Contains(body, "# TYPE mkw_relations_total gauge") {
				t.Error("missing mkw_relations_total metadata")
			}
			if !strings.Contains(body, `mkw_relations_total{predicate="hosts"}`) {
				t.Error("missing mkw_relations_total for hosts")
			}

			// Check mkw_provisional_nodes_total
			if !strings.Contains(body, "# HELP mkw_provisional_nodes_total") || !strings.Contains(body, "# TYPE mkw_provisional_nodes_total gauge") {
				t.Error("missing mkw_provisional_nodes_total metadata")
			}
			if !strings.Contains(body, "mkw_provisional_nodes_total 1") {
				t.Errorf("expected mkw_provisional_nodes_total 1, body: %s", body)
			}

			// Check mkw_graph_revision
			if !strings.Contains(body, "# HELP mkw_graph_revision") || !strings.Contains(body, "# TYPE mkw_graph_revision gauge") {
				t.Error("missing mkw_graph_revision metadata")
			}
			fp, _ := g.Fingerprint()
			if !strings.Contains(body, `mkw_graph_revision{revision="`+fp+`"} 1`) {
				t.Errorf("missing mkw_graph_revision with fingerprint %s", fp)
			}

			// Check standard Go runtime stats
			runtimeMetrics := []string{
				"go_goroutines",
				"go_memstats_alloc_bytes",
				"go_memstats_alloc_bytes_total",
				"go_memstats_sys_bytes",
				"go_memstats_heap_alloc_bytes",
				"go_memstats_heap_objects",
			}
			for _, m := range runtimeMetrics {
				if !strings.Contains(body, m) {
					t.Errorf("missing Go runtime metric %s", m)
				}
			}
		})
	}
}
