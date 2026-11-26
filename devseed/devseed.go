// Package devseed provides the synthetic dataset used to populate an empty
// database during development.
//
// The real graph holds personal infrastructure details, so it never goes into
// this repository. This dataset mirrors the *shape* of the real one instead:
// several machines behind hardware, a couple of services per machine, a handful
// of credentials, and the same relation predicates. Names, addresses and paths
// are invented — domains use .lab.test and addresses come from the RFC 5737
// documentation ranges.
//
// Regenerate with scripts/gen-synthetic-world.py; ids are derived from a fixed
// namespace, so regenerating does not change them and repeated imports stay
// idempotent.
package devseed

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

//go:embed world.json
var files embed.FS

// Batch returns a fresh copy of the development seed as a source batch.
func Batch() (graph.SourceBatch, error) {
	raw, err := files.ReadFile("world.json")
	if err != nil {
		return graph.SourceBatch{}, err
	}
	var batch graph.SourceBatch
	if err = json.Unmarshal(raw, &batch); err != nil {
		return graph.SourceBatch{}, fmt.Errorf("decode development seed: %w", err)
	}
	return batch, nil
}

// Graph loads the seed into a graph, validating that every node is reachable
// from Self the same way an applied batch would be.
func Graph() (*graph.Graph, error) {
	batch, err := Batch()
	if err != nil {
		return nil, err
	}
	g := graph.New()
	applied, _, err := g.ApplySync(batch)
	if err != nil {
		return nil, fmt.Errorf("apply development seed: %w", err)
	}
	return applied, nil
}
