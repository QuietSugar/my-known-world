package graph

import (
	"errors"
	"strings"
	"time"
)

// FieldResolution is an append-only decision selecting one or more existing
// assertions as accepted for a modeled field. It never edits or deletes evidence.
type FieldResolution struct {
	ID                   int64     `json:"id"`
	NodeID               string    `json:"nodeId"`
	FieldPath            string    `json:"fieldPath"`
	SelectedAssertionIDs []string  `json:"selectedAssertionIds"`
	RationaleReference   string    `json:"rationaleReference"`
	RecordedAt           time.Time `json:"recordedAt"`
}

func (r FieldResolution) Validate() error {
	if r.NodeID == "" || r.FieldPath == "" || len(r.SelectedAssertionIDs) == 0 {
		return errors.New("nodeId, fieldPath, and selectedAssertionIds are required")
	}
	if strings.TrimSpace(r.RationaleReference) == "" || len(r.RationaleReference) > 1024 || strings.ContainsAny(r.RationaleReference, "\r\n") {
		return errors.New("rationaleReference must be a safe non-empty reference")
	}
	seen := map[string]bool{}
	for _, id := range r.SelectedAssertionIDs {
		if strings.TrimSpace(id) == "" || seen[id] {
			return errors.New("selected assertion IDs must be non-empty and unique")
		}
		seen[id] = true
	}
	return nil
}
