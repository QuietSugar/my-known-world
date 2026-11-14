package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

type HardwareDetails struct {
	Category             string            `json:"category,omitempty"`
	Model                string            `json:"model,omitempty"`
	Location             string            `json:"location,omitempty"`
	Specifications       map[string]string `json:"specifications,omitempty"`
	Status               string            `json:"status,omitempty"`
	AccessReference      string            `json:"accessReference,omitempty"`
	MaintenanceReference string            `json:"maintenanceReference,omitempty"`
}
type ResourceClaim struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Scope string `json:"scope,omitempty"`
	Port  int    `json:"port,omitempty"`
}
type MachineDetails struct {
	Hostname               string          `json:"hostname,omitempty"`
	OS                     string          `json:"os,omitempty"`
	IPs                    []string        `json:"ips,omitempty"`
	Network                string          `json:"network,omitempty"`
	Status                 string          `json:"status,omitempty"`
	AccessMethod           string          `json:"accessMethod,omitempty"`
	AccessPrerequisites    []string        `json:"accessPrerequisites,omitempty"`
	AllowedOperations      []string        `json:"allowedOperations,omitempty"`
	ForbiddenOperations    []string        `json:"forbiddenOperations,omitempty"`
	ResponsibilityBoundary string          `json:"responsibilityBoundary,omitempty"`
	Resources              []ResourceClaim `json:"resources,omitempty"`
	LastVerified           *time.Time      `json:"lastVerified,omitempty"`
}
type SoftwareProjectDetails struct {
	Purpose            string   `json:"purpose,omitempty"`
	Languages          []string `json:"languages,omitempty"`
	Form               string   `json:"form,omitempty"`
	RemoteRepository   string   `json:"remoteRepository,omitempty"`
	Upstream           string   `json:"upstream,omitempty"`
	MaintenanceState   string   `json:"maintenanceState,omitempty"`
	RecommendedVersion string   `json:"recommendedVersion,omitempty"`
	SuccessorID        string   `json:"successorId,omitempty"`
}
type Endpoint struct {
	Kind    string `json:"kind"`
	Address string `json:"address"`
	Port    int    `json:"port,omitempty"`
}
type SoftwareInstanceDetails struct {
	ProductName             string     `json:"productName,omitempty"`
	DeploymentPath          string     `json:"deploymentPath,omitempty"`
	DeploymentMethod        string     `json:"deploymentMethod,omitempty"`
	RuntimeIdentity         string     `json:"runtimeIdentity,omitempty"`
	Endpoints               []Endpoint `json:"endpoints,omitempty"`
	ConfigurationReferences []string   `json:"configurationReferences,omitempty"`
	DeploymentState         string     `json:"deploymentState,omitempty"`
	OperationalState        string     `json:"operationalState,omitempty"`
	MaintenanceState        string     `json:"maintenanceState,omitempty"`
	VerificationResult      string     `json:"verificationResult,omitempty"`
	VerifiedAt              *time.Time `json:"verifiedAt,omitempty"`
}
type AccountCredentialDetails struct {
	Account         string     `json:"account,omitempty"`
	CredentialType  string     `json:"credentialType,omitempty"`
	Purpose         string     `json:"purpose,omitempty"`
	Scope           string     `json:"scope,omitempty"`
	AccessReference string     `json:"accessReference,omitempty"`
	RotationAt      *time.Time `json:"rotationAt,omitempty"`
	Status          string     `json:"status,omitempty"`
	LastVerified    *time.Time `json:"lastVerified,omitempty"`
}
type ProductDetails struct {
	Purpose                      string `json:"purpose,omitempty"`
	CommonConfigurationReference string `json:"commonConfigurationReference,omitempty"`
	RunbookReference             string `json:"runbookReference,omitempty"`
}
type DocumentDetails struct {
	Role                   string     `json:"role,omitempty"`
	AuthoritativeReference string     `json:"authoritativeReference,omitempty"`
	Effective              bool       `json:"effective"`
	LastVerified           *time.Time `json:"lastVerified,omitempty"`
	SupersededBy           string     `json:"supersededBy,omitempty"`
}
type RunbookDetails struct {
	Trigger                string     `json:"trigger,omitempty"`
	Preconditions          []string   `json:"preconditions,omitempty"`
	StepsReference         string     `json:"stepsReference,omitempty"`
	StopConditionReference string     `json:"stopConditionReference,omitempty"`
	RollbackReference      string     `json:"rollbackReference,omitempty"`
	VerificationMethod     string     `json:"verificationMethod,omitempty"`
	LastVerified           *time.Time `json:"lastVerified,omitempty"`
	ExecutableStatus       string     `json:"executableStatus,omitempty"`
	ApprovalRequired       bool       `json:"approvalRequired"`
}
type NetworkDetails struct {
	Kind             string   `json:"kind,omitempty"`
	CIDRs            []string `json:"cidrs,omitempty"`
	GatewayReference string   `json:"gatewayReference,omitempty"`
	AccessLocations  []string `json:"accessLocations,omitempty"`
	VPNReference     string   `json:"vpnReference,omitempty"`
	Status           string   `json:"status,omitempty"`
}
type WorkItemDetails struct {
	Kind                      string `json:"kind,omitempty"`
	Status                    string `json:"status,omitempty"`
	RationaleReference        string `json:"rationaleReference,omitempty"`
	DecisionEvidenceReference string `json:"decisionEvidenceReference,omitempty"`
}

func (n Node) modelDetailCount() int {
	count := 0
	for _, present := range []bool{n.Hardware != nil, n.Machine != nil, n.SoftwareProject != nil, n.SoftwareInstance != nil, n.AccountCredential != nil, n.Product != nil, n.Document != nil, n.Runbook != nil, n.WorkItem != nil, n.Network != nil} {
		if present {
			count++
		}
	}
	return count
}
func (n Node) MarshalModelDetails() ([]byte, error) {
	switch {
	case n.Hardware != nil:
		return json.Marshal(n.Hardware)
	case n.Machine != nil:
		return json.Marshal(n.Machine)
	case n.SoftwareProject != nil:
		return json.Marshal(n.SoftwareProject)
	case n.SoftwareInstance != nil:
		return json.Marshal(n.SoftwareInstance)
	case n.AccountCredential != nil:
		return json.Marshal(n.AccountCredential)
	case n.Product != nil:
		return json.Marshal(n.Product)
	case n.Document != nil:
		return json.Marshal(n.Document)
	case n.Runbook != nil:
		return json.Marshal(n.Runbook)
	case n.WorkItem != nil:
		return json.Marshal(n.WorkItem)
	case n.Network != nil:
		return json.Marshal(n.Network)
	default:
		return []byte(`{}`), nil
	}
}
func (n *Node) UnmarshalModelDetails(raw []byte) error {
	if len(raw) == 0 || string(raw) == `{}` || string(raw) == `null` {
		return nil
	}
	switch n.Type {
	case Hardware:
		return json.Unmarshal(raw, &n.Hardware)
	case Machine:
		return json.Unmarshal(raw, &n.Machine)
	case SoftwareProject:
		return json.Unmarshal(raw, &n.SoftwareProject)
	case SoftwareInstance:
		return json.Unmarshal(raw, &n.SoftwareInstance)
	case AccountCredential:
		return json.Unmarshal(raw, &n.AccountCredential)
	case Product:
		return json.Unmarshal(raw, &n.Product)
	case Document:
		return json.Unmarshal(raw, &n.Document)
	case Runbook:
		return json.Unmarshal(raw, &n.Runbook)
	case WorkItem:
		return json.Unmarshal(raw, &n.WorkItem)
	case Network:
		return json.Unmarshal(raw, &n.Network)
	default:
		return fmt.Errorf("model details are not supported for %s", n.Type)
	}
}
func (n Node) ValidateModelDetails() error {
	if n.modelDetailCount() > 1 {
		return errors.New("node may have only one type-specific detail model")
	}
	valid := false
	switch n.Type {
	case Hardware:
		valid = n.modelDetailCount() == 0 || (n.Hardware != nil && n.modelDetailCount() == 1)
	case Machine:
		valid = n.modelDetailCount() == 0 || (n.Machine != nil && n.modelDetailCount() == 1)
	case SoftwareProject:
		valid = n.modelDetailCount() == 0 || (n.SoftwareProject != nil && n.modelDetailCount() == 1)
	case SoftwareInstance:
		valid = n.modelDetailCount() == 0 || (n.SoftwareInstance != nil && n.modelDetailCount() == 1)
	case AccountCredential:
		valid = n.modelDetailCount() == 0 || (n.AccountCredential != nil && n.modelDetailCount() == 1)
	case Product:
		valid = n.modelDetailCount() == 0 || (n.Product != nil && n.modelDetailCount() == 1)
	case Document:
		valid = n.modelDetailCount() == 0 || (n.Document != nil && n.modelDetailCount() == 1)
	case Runbook:
		valid = n.modelDetailCount() == 0 || (n.Runbook != nil && n.modelDetailCount() == 1)
	case WorkItem:
		valid = n.modelDetailCount() == 0 || (n.WorkItem != nil && n.modelDetailCount() == 1)
	case Network:
		valid = n.modelDetailCount() == 0 || (n.Network != nil && n.modelDetailCount() == 1)
	default:
		valid = n.modelDetailCount() == 0
	}
	if !valid {
		return fmt.Errorf("type-specific details do not match node type %s", n.Type)
	}
	refs := []string{}
	if n.Hardware != nil {
		refs = append(refs, n.Hardware.AccessReference, n.Hardware.MaintenanceReference)
	}
	if n.Machine != nil {
		refs = append(refs, n.Machine.AccessMethod)
	}
	if n.SoftwareProject != nil {
		refs = append(refs, n.SoftwareProject.RemoteRepository, n.SoftwareProject.Upstream)
	}
	if n.SoftwareInstance != nil {
		refs = append(refs, n.SoftwareInstance.ConfigurationReferences...)
	}
	if n.AccountCredential != nil {
		refs = append(refs, n.AccountCredential.AccessReference)
	}
	if n.Product != nil {
		refs = append(refs, n.Product.CommonConfigurationReference, n.Product.RunbookReference)
	}
	if n.Document != nil {
		refs = append(refs, n.Document.AuthoritativeReference)
	}
	if n.Runbook != nil {
		refs = append(refs, n.Runbook.StepsReference, n.Runbook.StopConditionReference, n.Runbook.RollbackReference)
	}
	if n.WorkItem != nil {
		refs = append(refs, n.WorkItem.RationaleReference, n.WorkItem.DecisionEvidenceReference)
	}
	if n.Network != nil {
		refs = append(refs, n.Network.GatewayReference, n.Network.VPNReference)
	}
	for _, ref := range refs {
		if err := validateSafeRef(ref); err != nil {
			return err
		}
	}
	if n.Machine != nil {
		for _, ip := range n.Machine.IPs {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid machine IP %q", ip)
			}
		}
		for _, r := range n.Machine.Resources {
			if r.Kind == "" || r.Name == "" || r.Port < 0 || r.Port > 65535 {
				return errors.New("invalid machine resource claim")
			}
		}
	}
	if n.Network != nil {
		for _, cidr := range n.Network.CIDRs {
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				return fmt.Errorf("invalid network CIDR %q", cidr)
			}
		}
	}
	if n.SoftwareInstance != nil {
		for _, e := range n.SoftwareInstance.Endpoints {
			if e.Kind == "" || e.Address == "" || e.Port < 0 || e.Port > 65535 {
				return errors.New("invalid software instance endpoint")
			}
		}
	}
	return nil
}
func validateSafeRef(ref string) error {
	if len(ref) > 2048 || strings.ContainsAny(ref, "\r\n") {
		return errors.New("reference is invalid or too long")
	}
	return nil
}
