package graph

import "time"

type MutationOperation struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	RecordedAt time.Time `json:"recordedAt"`
	Reversible bool      `json:"reversible"`
}
