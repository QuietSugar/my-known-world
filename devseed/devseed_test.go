package devseed

import (
	"regexp"
	"testing"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TestBatchIsUsable guards the four properties the development seed has to keep
// or it stops being a stand-in for the real graph.
func TestBatchIsUsable(t *testing.T) {
	batch, err := Batch()
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Nodes) < 30 {
		t.Fatalf("seed too small to resemble production: %d nodes", len(batch.Nodes))
	}

	seen := map[string]bool{}
	for _, n := range batch.Nodes {
		if !uuidPattern.MatchString(n.ID) {
			t.Errorf("node %q id is not a UUID; the real graph uses UUIDs", n.ID)
		}
		if seen[n.ID] {
			t.Errorf("duplicate node id %q", n.ID)
		}
		seen[n.ID] = true
	}
	for _, r := range batch.Relations {
		if !seen[r.From] || !seen[r.To] {
			t.Errorf("relation %q points at an unknown node", r.ID)
		}
	}

	g, err := Graph()
	if err != nil {
		t.Fatalf("seed rejected by the graph: %v", err)
	}
	for _, n := range g.Nodes() {
		if _, ok := g.PathFromSelf(n.ID); !ok {
			t.Errorf("node %q is not reachable from Self", n.ID)
		}
	}
}

// TestSeedContainsNoProductionData is the guard that matters most: this file is
// committed, so a real hostname or address leaking in is a disclosure bug rather
// than a test failure. Real infrastructure identifiers must never appear here.
func TestSeedContainsNoProductionData(t *testing.T) {
	raw, err := Batch()
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{
		"hall", "xasc", "xactds", "vaultwarden", "samba", "shenyu", "nacos",
		"homer", "nexus", "miraihub", "tokentracker", "beszel", "portainer",
		"192.168.", "10.124.", "10.16.", "121.29.", "172.16.", "172.19.",
		"mkw.svc", "hall-wiki", "hall-cmdb",
	}
	for _, n := range raw.Nodes {
		blob := n.ID + " " + n.Name + " " + n.Source
		for _, word := range banned {
			if containsFold(blob, word) {
				t.Errorf("node %q mentions %q; the seed must stay synthetic", n.Name, word)
			}
		}
	}
}

func containsFold(haystack, needle string) bool {
	return len(needle) > 0 && regexp.MustCompile(`(?i)`+regexp.QuoteMeta(needle)).MatchString(haystack)
}
