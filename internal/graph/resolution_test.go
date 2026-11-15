package graph

import "testing"

func TestFieldResolutionValidation(t *testing.T) {
	base := FieldResolution{NodeID: "m", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"a1"}, RationaleReference: "ticket:42"}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	base.SelectedAssertionIDs = []string{"a1", "a1"}
	if err := base.Validate(); err == nil {
		t.Fatal("duplicate selected assertions accepted")
	}
	base.SelectedAssertionIDs = []string{"a1"}
	base.RationaleReference = "secret\ntext"
	if err := base.Validate(); err == nil {
		t.Fatal("unsafe rationale reference accepted")
	}
	base.RationaleReference = "ticket:42"
	base.ID = 4
	if err := base.Validate(); err == nil {
		t.Fatal("client supplied server-assigned resolution ID accepted")
	}
}
