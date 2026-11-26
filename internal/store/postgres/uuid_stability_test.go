package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// entityIDs collects every persisted node and relation id.
//
// Identity is carried by the id column alone since migration 024 dropped the
// duplicate uuid column. The snapshot save path is DELETE + INSERT, so if an id
// were regenerated on rewrite, external references would silently break.
func entityIDs(t *testing.T, store *Store) (map[string]bool, map[string]bool) {
	t.Helper()
	ctx := context.Background()

	nodes := map[string]bool{}
	rows, err := store.db.QueryContext(ctx, `SELECT id FROM nodes`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		nodes[id] = true
	}
	rows.Close()

	relations := map[string]bool{}
	relRows, err := store.db.QueryContext(ctx, `SELECT id FROM relations`)
	if err != nil {
		t.Fatal(err)
	}
	for relRows.Next() {
		var id string
		if err := relRows.Scan(&id); err != nil {
			relRows.Close()
			t.Fatal(err)
		}
		relations[id] = true
	}
	relRows.Close()
	return nodes, relations
}

func seedUUIDGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []graph.Node{
		{ID: "machine/hp/vm128.md", Type: graph.Machine, Name: "vm128", Lifecycle: "active", Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1,
			Machine: &graph.MachineDetails{Hostname: "vm128.test", Roles: []string{"development"}}},
		{ID: "services/instances/vm128/samba.md", Type: graph.SoftwareInstance, Name: "samba", Lifecycle: "active", Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1,
			SoftwareInstance: &graph.SoftwareInstanceDetails{DeploymentPath: "/srv/samba", RuntimeIdentity: "hall-samba", OperationalState: "active"}},
	} {
		rel := graph.Relation{ID: "anchor-" + n.ID, Predicate: graph.KnowsAbout, To: n.ID, Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed}
		if err := g.Publish(n, rel); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.AddRelation(graph.Relation{ID: "rel-host", From: "machine/hp/vm128.md", Predicate: graph.Hosts, To: "services/instances/vm128/samba.md", Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed}); err != nil {
		t.Fatal(err)
	}
	return g
}

// TestIdentitySurvivesSnapshotRewrite is the regression guard for identity
// stability. It failed in practice on 2026-11-26 when a snapshot save
// regenerated every UUID, which silently broke external references.
func TestIdentitySurvivesSnapshotRewrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	g := seedUUIDGraph(t)
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	firstNodes, firstRels := entityIDs(t, store)
	if len(firstNodes) == 0 || len(firstRels) == 0 {
		t.Fatal("expected seeded entities to be persisted")
	}

	// Saving the same snapshot again is what every import does.
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	secondNodes, secondRels := entityIDs(t, store)

	for id := range firstNodes {
		if !secondNodes[id] {
			t.Errorf("node %q disappeared across snapshot save", id)
		}
	}
	for id := range firstRels {
		if !secondRels[id] {
			t.Errorf("relation %q disappeared across snapshot save", id)
		}
	}
	for id := range secondNodes {
		if !firstNodes[id] {
			t.Errorf("unexpected node %q appeared after snapshot save", id)
		}
	}
	for id := range secondRels {
		if !firstRels[id] {
			t.Errorf("unexpected relation %q appeared after snapshot save", id)
		}
	}
}

// TestUUIDColumnIsGone guards migration 024: the duplicate uuid column and its
// registries must not come back, since identity now lives in the id column.
func TestUUIDColumnIsGone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"nodes", "relations", "field_assertions"} {
		var count int
		if err := store.db.QueryRowContext(ctx,
			`SELECT count(*) FROM information_schema.columns
			  WHERE table_schema = current_schema() AND table_name=$1 AND column_name='uuid'`, table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s still has a uuid column; identity must live in id", table)
		}
	}
	for _, table := range []string{"node_uuid_registry", "relation_uuid_registry", "assertion_uuid_registry"} {
		var exists bool
		if err := store.db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Errorf("registry table %q should have been dropped", table)
		}
	}
}

// TestSnapshotRoundTripKeepsFingerprint guards read/write symmetry.
//
// Undo inspects history by comparing each operation's recorded after-hash with
// the fingerprint of the graph loaded back from the database. If loading adds
// anything the in-memory graph did not have, the two never match and every
// operation becomes un-undoable. That happened on 2026-11-26 when the read path
// started filling in a uuid field the write path never set.
func TestSnapshotRoundTripKeepsFingerprint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	written := seedUUIDGraph(t)
	if err := store.SaveSnapshot(ctx, written); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// SaveSnapshot truncates timestamps to the microsecond precision PostgreSQL
	// stores, so the expected value is the canonicalized graph.
	want, err := written.CanonicalizeTimes().Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	got, err := loaded.Fingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fingerprint changed across save/load: wrote %s, read back %s", want, got)
	}
}
