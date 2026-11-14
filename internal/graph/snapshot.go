package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Snapshot struct {
	Nodes      []Node           `json:"nodes"`
	Relations  []Relation       `json:"relations"`
	Assertions []FieldAssertion `json:"assertions"`
}

func (g *Graph) Snapshot() Snapshot {
	return Snapshot{Nodes: g.Nodes(), Relations: g.Relations(), Assertions: g.Assertions()}
}
func (g *Graph) Fingerprint() (string, error) {
	raw, err := json.Marshal(g.Snapshot())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func RestoreSnapshot(snapshot Snapshot) (*Graph, error) {
	g := New()
	if len(snapshot.Nodes) == 0 {
		if len(snapshot.Relations) != 0 || len(snapshot.Assertions) != 0 {
			return nil, fmt.Errorf("empty snapshot cannot contain relations or assertions")
		}
		return g, nil
	}
	for _, n := range snapshot.Nodes {
		if err := g.AddNode(n); err != nil {
			return nil, err
		}
	}
	for _, r := range snapshot.Relations {
		if err := g.AddRelation(r); err != nil {
			return nil, err
		}
	}
	for _, a := range snapshot.Assertions {
		if err := g.AddAssertion(a); err != nil {
			return nil, err
		}
	}
	if len(g.Nodes()) == 0 {
		return nil, fmt.Errorf("snapshot is empty")
	}
	for _, n := range g.Nodes() {
		if _, ok := g.PathFromSelf(n.ID); !ok {
			return nil, fmt.Errorf("snapshot contains disconnected node %q", n.ID)
		}
	}
	return g, nil
}

// CanonicalizeTimes aligns timestamps to PostgreSQL's microsecond precision so
// snapshots and fingerprints remain stable across persistence round trips.
func (g *Graph) CanonicalizeTimes() *Graph {
	out := cloneGraph(g)
	cut := func(v *time.Time) {
		if v != nil {
			*v = v.UTC().Truncate(time.Microsecond)
		}
	}
	for id, n := range out.nodes {
		cut(n.LastConfirmed)
		cut(n.StartedAt)
		cut(n.EndedAt)
		cut(n.VerificationAt)
		if n.Machine != nil {
			cut(n.Machine.LastVerified)
		}
		if n.SoftwareInstance != nil {
			cut(n.SoftwareInstance.VerifiedAt)
		}
		if n.AccountCredential != nil {
			cut(n.AccountCredential.RotationAt)
			cut(n.AccountCredential.LastVerified)
		}
		if n.Document != nil {
			cut(n.Document.LastVerified)
		}
		if n.Runbook != nil {
			cut(n.Runbook.LastVerified)
		}
		out.nodes[id] = n
	}
	for id, r := range out.relations {
		r.RecordedAt = r.RecordedAt.UTC().Truncate(time.Microsecond)
		out.relations[id] = r
	}
	for id, a := range out.assertions {
		a.AssertedAt = a.AssertedAt.UTC().Truncate(time.Microsecond)
		out.assertions[id] = a
	}
	return out
}
