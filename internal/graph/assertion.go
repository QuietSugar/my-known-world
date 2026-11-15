package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// FieldAssertion records provenance for one modeled field without storing its
// value. ValueHash is a SHA-256 digest; Reference is a safe external citation.
type FieldAssertion struct {
	ID           string       `json:"id"`
	NodeID       string       `json:"nodeId"`
	FieldPath    string       `json:"fieldPath"`
	ValueHash    string       `json:"valueHash"`
	Reference    string       `json:"reference,omitempty"`
	Source       string       `json:"source"`
	EvidenceKind EvidenceKind `json:"evidenceKind"`
	Certainty    Certainty    `json:"certainty"`
	AssertedAt   time.Time    `json:"assertedAt"`
}

// HashFieldValue returns a digest for a field value. Callers must not send or
// persist the original value if it may contain a secret or document body.
func HashFieldValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func validFieldPath(n Node, path string) bool {
	common := map[string]bool{"name": true, "lifecycle": true, "lastConfirmed": true}
	if common[path] {
		return true
	}
	fields := map[NodeType]map[string]bool{
		Hardware:          {"hardware.category": true, "hardware.model": true, "hardware.location": true, "hardware.specifications": true, "hardware.status": true, "hardware.accessReference": true, "hardware.maintenanceReference": true},
		Machine:           {"machine.machineKind": true, "machine.hostname": true, "machine.os": true, "machine.ips": true, "machine.network": true, "machine.status": true, "machine.accessMethod": true, "machine.accessPrerequisites": true, "machine.allowedOperations": true, "machine.operationConstraints": true, "machine.forbiddenOperations": true, "machine.responsibilityBoundary": true, "machine.resources": true, "machine.lastVerified": true},
		SoftwareProject:   {"softwareProject.purpose": true, "softwareProject.languages": true, "softwareProject.form": true, "softwareProject.remoteRepository": true, "softwareProject.upstream": true, "softwareProject.maintenanceState": true, "softwareProject.recommendedVersion": true, "softwareProject.successorId": true},
		SoftwareInstance:  {"softwareInstance.productName": true, "softwareInstance.domains": true, "softwareInstance.deploymentPath": true, "softwareInstance.deploymentMethod": true, "softwareInstance.runtimeIdentity": true, "softwareInstance.endpoints": true, "softwareInstance.configurationReferences": true, "softwareInstance.deploymentState": true, "softwareInstance.operationalState": true, "softwareInstance.maintenanceState": true, "softwareInstance.verificationResult": true, "softwareInstance.verifiedAt": true},
		AccountCredential: {"accountCredential.account": true, "accountCredential.credentialType": true, "accountCredential.purpose": true, "accountCredential.scope": true, "accountCredential.accessReference": true, "accountCredential.rotationAt": true, "accountCredential.status": true, "accountCredential.lastVerified": true},
		Product:           {"product.purpose": true, "product.commonConfigurationReference": true, "product.runbookReference": true},
		Document:          {"document.role": true, "document.authoritativeReference": true, "document.effective": true, "document.lastVerified": true, "document.supersededBy": true},
		Runbook:           {"runbook.trigger": true, "runbook.preconditions": true, "runbook.stepsReference": true, "runbook.stopConditionReference": true, "runbook.rollbackReference": true, "runbook.verificationMethod": true, "runbook.lastVerified": true, "runbook.executableStatus": true, "runbook.approvalRequired": true},
		WorkItem:          {"workItem.kind": true, "workItem.status": true, "workItem.rationaleReference": true, "workItem.decisionEvidenceReference": true},
		Network:           {"network.kind": true, "network.cidrs": true, "network.gatewayReference": true, "network.accessLocations": true, "network.vpnReference": true, "network.status": true},
		RunningPeriod:     {"startedAt": true, "endedAt": true, "deployedVersion": true, "verificationAt": true, "verificationResult": true, "evidenceRef": true},
	}
	return fields[n.Type][path]
}

func (g *Graph) AddAssertion(a FieldAssertion) error {
	if a.ID == "" || a.NodeID == "" || a.FieldPath == "" {
		return errors.New("assertion id, node id, and field path are required")
	}
	n, ok := g.nodes[a.NodeID]
	if !ok {
		return fmt.Errorf("assertion node %q not found", a.NodeID)
	}
	if !validFieldPath(n, a.FieldPath) {
		return fmt.Errorf("field %q is not modeled for %s", a.FieldPath, n.Type)
	}
	if len(a.ValueHash) != 64 {
		return errors.New("valueHash must be a SHA-256 hex digest")
	}
	if _, err := hex.DecodeString(a.ValueHash); err != nil {
		return errors.New("valueHash must be a SHA-256 hex digest")
	}
	if len(a.Reference) > 1024 || strings.ContainsAny(a.Reference, "\r\n") {
		return errors.New("assertion reference is invalid or too long")
	}
	var err error
	a.Source, a.EvidenceKind, a.Certainty, err = normalizeEvidence(a.Source, a.EvidenceKind, a.Certainty)
	if err != nil {
		return err
	}
	if a.AssertedAt.IsZero() {
		a.AssertedAt = time.Now().UTC()
	}
	if old, exists := g.assertions[a.ID]; exists {
		if old.NodeID != a.NodeID || old.FieldPath != a.FieldPath || old.Source != a.Source {
			return fmt.Errorf("assertion identity conflict %q", a.ID)
		}
	}
	g.assertions[a.ID] = a
	return nil
}

func (g *Graph) Assertions() []FieldAssertion {
	out := make([]FieldAssertion, 0, len(g.assertions))
	for _, a := range g.assertions {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
func (g *Graph) AssertionsForNode(id string) []FieldAssertion {
	out := []FieldAssertion{}
	for _, a := range g.Assertions() {
		if a.NodeID == id {
			out = append(out, a)
		}
	}
	return out
}
