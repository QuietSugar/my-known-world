package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type PatrolSummary struct {
	Grade                      string `json:"grade"`
	Status                     string `json:"status"`
	TotalNodes                 int    `json:"totalNodes"`
	TotalFindings              int    `json:"totalFindings"`
	StaleConfirmationCount     int    `json:"staleConfirmationCount"`
	PendingProvisionalsCount   int    `json:"pendingProvisionalsCount"`
	IsolatedLeavesCount        int    `json:"isolatedLeavesCount"`
	UnverifiedCredentialsCount int    `json:"unverifiedCredentialsCount"`
}

type PatrolFindings struct {
	StaleConfirmation     []graph.NodeSummary `json:"staleConfirmation"`
	PendingProvisionals   []graph.NodeSummary `json:"pendingProvisionals"`
	IsolatedLeaves        []graph.NodeSummary `json:"isolatedLeaves"`
	UnverifiedCredentials []graph.NodeSummary `json:"unverifiedCredentials"`
}

type PatrolResponse struct {
	Summary  PatrolSummary  `json:"summary"`
	Findings PatrolFindings `json:"findings"`
}

func (h *Handler) patrolAPI(w http.ResponseWriter, r *http.Request, g *graph.Graph) {
	now := time.Now()
	threshold := now.Add(-90 * 24 * time.Hour)

	findings := PatrolFindings{
		StaleConfirmation:     make([]graph.NodeSummary, 0),
		PendingProvisionals:   make([]graph.NodeSummary, 0),
		IsolatedLeaves:        make([]graph.NodeSummary, 0),
		UnverifiedCredentials: make([]graph.NodeSummary, 0),
	}

	nodes := g.Nodes()
	relations := g.Relations()

	selfID := ""
	for _, n := range nodes {
		if n.Type == graph.Self {
			selfID = n.ID
			break
		}
	}

	// Calculate business relations for each node.
	// A business relation connects a node to a non-Self entity.
	businessRelCount := make(map[string]int, len(nodes))
	for _, rel := range relations {
		if rel.From != selfID && rel.To != selfID {
			businessRelCount[rel.From]++
			businessRelCount[rel.To]++
		}
	}

	for _, n := range nodes {
		summary := n.Summary()

		// 1. staleConfirmation: Nodes where lastConfirmed is nil or older than 90 days.
		// (Self node represents the identity root, not an external entity to confirm)
		if n.Type != graph.Self {
			if n.LastConfirmed == nil || n.LastConfirmed.Before(threshold) {
				findings.StaleConfirmation = append(findings.StaleConfirmation, summary)
			}
		}

		// 2. pendingProvisionals: Nodes where reviewStatus == "pending".
		if n.ReviewStatus == "pending" || (n.Type == graph.ProvisionalObject && (n.ReviewStatus == "" || n.ReviewStatus == "pending")) {
			findings.PendingProvisionals = append(findings.PendingProvisionals, summary)
		}

		// 3. isolatedLeaves: Nodes that only have an automatic Self anchor and zero business relations.
		if n.Type != graph.Self {
			if businessRelCount[n.ID] == 0 {
				findings.IsolatedLeaves = append(findings.IsolatedLeaves, summary)
			}
		}

		// 4. unverifiedCredentials: AccountCredential nodes with status not "verified" or lastConfirmed nil.
		if n.Type == graph.AccountCredential {
			unverified := false
			if n.AccountCredential == nil || !strings.EqualFold(n.AccountCredential.Status, "verified") || n.LastConfirmed == nil {
				unverified = true
			}
			if unverified {
				findings.UnverifiedCredentials = append(findings.UnverifiedCredentials, summary)
			}
		}
	}

	totalFindings := len(findings.StaleConfirmation) + len(findings.PendingProvisionals) +
		len(findings.IsolatedLeaves) + len(findings.UnverifiedCredentials)

	grade := "A"
	status := "healthy"
	switch {
	case totalFindings == 0:
		grade = "A"
		status = "healthy"
	case totalFindings <= 2:
		grade = "B"
		status = "warning"
	case totalFindings <= 5:
		grade = "C"
		status = "warning"
	default:
		grade = "D"
		status = "attention_required"
	}

	summary := PatrolSummary{
		Grade:                      grade,
		Status:                     status,
		TotalNodes:                 len(nodes),
		TotalFindings:              totalFindings,
		StaleConfirmationCount:     len(findings.StaleConfirmation),
		PendingProvisionalsCount:   len(findings.PendingProvisionals),
		IsolatedLeavesCount:        len(findings.IsolatedLeaves),
		UnverifiedCredentialsCount: len(findings.UnverifiedCredentials),
	}

	writeJSON(w, http.StatusOK, PatrolResponse{
		Summary:  summary,
		Findings: findings,
	})
}
