package graph

import (
	"encoding/json"
	"time"
)

// ChangeEvent is an immutable audit event. Snapshot is the safe modeled state
// after a change; deletion events have no snapshot.
type ChangeEvent struct {
	ID         int64           `json:"id"`
	RunID      *int64          `json:"runId,omitempty"`
	Source     string          `json:"source"`
	EntityType string          `json:"entityType"`
	EntityID   string          `json:"entityId"`
	Action     string          `json:"action"`
	RecordedAt time.Time       `json:"recordedAt"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
}
