package graph

import "testing"

func TestTypedModelDetailsRejectMismatchesAndInvalidEndpoints(t *testing.T) {
	tests := []Node{
		{ID: "wrong-type", Type: Machine, Name: "Wrong", ModelVersion: 1, Hardware: &HardwareDetails{Model: "x"}},
		{ID: "bad-ip", Type: Machine, Name: "Bad IP", ModelVersion: 1, Machine: &MachineDetails{IPs: []string{"not-an-ip"}}},
		{ID: "bad-port", Type: SoftwareInstance, Name: "Bad endpoint", ModelVersion: 1, SoftwareInstance: &SoftwareInstanceDetails{Endpoints: []Endpoint{{Kind: "http", Address: "example.invalid", Port: 65536}}}},
		{ID: "bad-claim", Type: Machine, Name: "Bad claim", ModelVersion: 1, Machine: &MachineDetails{Resources: []ResourceClaim{{Kind: "port", Name: "x", Port: -1}}}},
	}
	for _, n := range tests {
		if err := New().AddNode(n); err == nil {
			t.Errorf("invalid typed model accepted: %+v", n)
		}
	}
}
