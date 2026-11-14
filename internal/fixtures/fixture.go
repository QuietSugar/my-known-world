// Package testdata provides maintained, fully synthetic fixtures for exercising
// the graph model and sync pipeline. Never put personal or production data here.
package testdata

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

//go:embed synthetic-world.json
var files embed.FS

// SyntheticWorld returns a fresh fixture batch on every call.
func SyntheticWorld() (graph.SourceBatch, error) {
	raw, err := files.ReadFile("synthetic-world.json")
	if err != nil {
		return graph.SourceBatch{}, err
	}
	var batch graph.SourceBatch
	if err = json.Unmarshal(raw, &batch); err != nil {
		return graph.SourceBatch{}, fmt.Errorf("decode synthetic world fixture: %w", err)
	}
	return batch, nil
}
