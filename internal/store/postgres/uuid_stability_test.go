package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// uuidFingerprint collects every persisted entity UUID so tests can assert that
// identity survives a snapshot rewrite. The snapshot save path is DELETE + INSERT,
// so without the registry triggers every UUID would be regenerated.
func uuidFingerprints(t *testing.T, store *Store) (map[string]string, map[string]string) {
	t.Helper()
	ctx := context.Background()

	nodes := map[string]string{}
	rows, err := store.db.QueryContext(ctx, `SELECT id, uuid::text FROM nodes`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, uuid string
		if err := rows.Scan(&id, &uuid); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		nodes[id] = uuid
	}
	rows.Close()

	relations := map[string]string{}
	relRows, err := store.db.QueryContext(ctx, `SELECT id, uuid::text FROM relations`)
	if err != nil {
		t.Fatal(err)
	}
	for relRows.Next() {
		var id, uuid string
		if err := relRows.Scan(&id, &uuid); err != nil {
			relRows.Close()
			t.Fatal(err)
		}
		relations[id] = uuid
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

// TestUUIDsSurviveSnapshotRewrite is the regression guard for identity stability.
// It failed in practice on 2026-11-26 when a snapshot save regenerated every UUID,
// which silently broke external references.
func TestUUIDsSurviveSnapshotRewrite(t *testing.T) {
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
	firstNodes, firstRels := uuidFingerprints(t, store)
	if len(firstNodes) == 0 || len(firstRels) == 0 {
		t.Fatal("expected seeded entities to be persisted")
	}

	// Saving the same snapshot again is what every import does.
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	secondNodes, secondRels := uuidFingerprints(t, store)

	for id, want := range firstNodes {
		if got := secondNodes[id]; got != want {
			t.Errorf("node %q changed UUID across snapshot save: %s -> %s", id, want, got)
		}
	}
	for id, want := range firstRels {
		if got := secondRels[id]; got != want {
			t.Errorf("relation %q changed UUID across snapshot save: %s -> %s", id, want, got)
		}
	}
}

// TestNewNodeUUIDEqualsID guards migration 023: a brand-new node must get its own
// id as UUID. Before that fix the trigger called gen_random_uuid(), so freshly
// imported nodes ended up with id != uuid while migrated nodes had id == uuid,
// leaving callers unsure which value to reference.
func TestNewNodeUUIDEqualsID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Node IDs must be UUIDs for this rule to apply, which is what the importer
	// produces once identityMappings are in place.
	const vmID = "14d9f61c-368c-4dd4-ab0c-68daf4998dc8"
	const svcID = "c860c2d6-c296-426d-88a5-1c6605555fa0"
	g := graph.New()
	if err := g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []graph.Node{
		{ID: vmID, Type: graph.Machine, Name: "vm128", Lifecycle: "active", Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1,
			Machine: &graph.MachineDetails{Hostname: "vm128.test"}},
		{ID: svcID, Type: graph.SoftwareInstance, Name: "samba", Lifecycle: "active", Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed, ModelVersion: 1,
			SoftwareInstance: &graph.SoftwareInstanceDetails{DeploymentPath: "/srv/samba"}},
	} {
		if err := g.Publish(n, graph.Relation{ID: "anchor-" + n.ID, Predicate: graph.KnowsAbout, To: n.ID, Source: "wiki", EvidenceKind: graph.EvidenceDocument, Certainty: graph.CertaintyConfirmed}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}

	var got string
	if err := store.db.QueryRowContext(ctx, `SELECT uuid::text FROM nodes WHERE id = $1`, vmID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != vmID {
		t.Fatalf("new node %s got UUID %s; want its own id", vmID, got)
	}
}

// TestUUIDsAreDistinctAndPersisted guards the basic contract: every entity has a
// UUID, and no two entities share one.
func TestUUIDsAreDistinctAndPersisted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, seedUUIDGraph(t)); err != nil {
		t.Fatal(err)
	}

	nodes, rels := uuidFingerprints(t, store)
	seen := map[string]string{}
	for id, uuid := range nodes {
		if uuid == "" {
			t.Errorf("node %q has no UUID", id)
		}
		if prev, dup := seen[uuid]; dup {
			t.Errorf("nodes %q and %q share UUID %s", prev, id, uuid)
		}
		seen[uuid] = id
	}
	for id, uuid := range rels {
		if uuid == "" {
			t.Errorf("relation %q has no UUID", id)
		}
		if prev, dup := seen[uuid]; dup {
			t.Errorf("relation %q reuses UUID %s of %q", id, uuid, prev)
		}
		seen[uuid] = id
	}

	// Every relation endpoint must point at a node that has a UUID.
	var dangling int
	if err := store.db.QueryRowContext(ctx, `SELECT count(*) FROM relations r
		WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = r.from_id)
		   OR NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = r.to_id)`).Scan(&dangling); err != nil {
		t.Fatal(err)
	}
	if dangling != 0 {
		t.Fatalf("relations with dangling endpoints: %d", dangling)
	}
}

// TestLoadedNodesCarryUUID ensures the read path surfaces the UUID, otherwise
// clients cannot use the stable identifier.
func TestLoadedNodesCarryUUID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, seedUUIDGraph(t)); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range loaded.Nodes() {
		if n.UUID == "" {
			t.Errorf("node %q loaded without UUID", n.ID)
		}
	}
}
