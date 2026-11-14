package graph

import "time"

// AssertionView is a value-free projection safe for routine API reads. It
// omits ValueHash and any original field value.
type AssertionView struct {
	ID           string       `json:"id"`
	NodeID       string       `json:"nodeId"`
	FieldPath    string       `json:"fieldPath"`
	Reference    string       `json:"reference,omitempty"`
	Source       string       `json:"source"`
	EvidenceKind EvidenceKind `json:"evidenceKind"`
	Certainty    Certainty    `json:"certainty"`
	AssertedAt   time.Time    `json:"assertedAt"`
	Resolved     bool         `json:"resolved"`
	Selected     bool         `json:"selected"`
	ResolutionID *int64       `json:"resolutionId,omitempty"`
}

// BuildAssertionViews applies the newest explicit decision per field. Resolutions
// should be newest-first; field values and hashes are never included.
func BuildAssertionViews(assertions []FieldAssertion, resolutions []FieldResolution) []AssertionView {
	latest := map[string]FieldResolution{}
	for _, r := range resolutions {
		if _, ok := latest[r.FieldPath]; !ok {
			latest[r.FieldPath] = r
		}
	}
	out := make([]AssertionView, 0, len(assertions))
	for _, a := range assertions {
		view := AssertionView{ID: a.ID, NodeID: a.NodeID, FieldPath: a.FieldPath, Reference: a.Reference, Source: a.Source, EvidenceKind: a.EvidenceKind, Certainty: a.Certainty, AssertedAt: a.AssertedAt}
		if decision, ok := latest[a.FieldPath]; ok {
			view.Resolved = true
			view.ResolutionID = &decision.ID
			for _, id := range decision.SelectedAssertionIDs {
				if id == a.ID {
					view.Selected = true
					break
				}
			}
		}
		out = append(out, view)
	}
	return out
}
