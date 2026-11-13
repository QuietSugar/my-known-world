package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

var testSchemaSequence atomic.Uint64

// openTestStore creates an isolated schema and drops only that schema at test end.
// It never truncates or alters tables in the database supplied by the caller.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("mkw_test_%d_%d", os.Getpid(), testSchemaSequence.Add(1))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err = admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	store, err := Open(ctx, u.String())
	if err != nil {
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = admin.Close()
	})
	return store
}

func TestSnapshotRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migration should be repeatable: %v", err)
	}

	original := graph.New()
	if err := original.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := original.Publish(graph.Node{ID: "machine", Type: graph.Machine, Name: "Test machine", Lifecycle: "active", Source: "inventory fixture", EvidenceKind: graph.EvidenceObserved, Certainty: graph.CertaintyConfirmed, ModelVersion: 1}, graph.Relation{ID: "r1", Predicate: graph.Maintains, To: "machine", Source: "integration fixture", EvidenceKind: graph.EvidenceObserved, Certainty: graph.CertaintyConfirmed}); err != nil {
		t.Fatal(err)
	}
	confirmed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := original.AddNode(graph.Node{ID: "project", Type: graph.SoftwareProject, Name: "Project", LastConfirmed: &confirmed, ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := original.AddRelation(graph.Relation{ID: "r2", From: "self", Predicate: graph.KnowsAbout, To: "project", Source: "test", RecordedAt: confirmed}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, original); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Nodes()) != 3 || len(loaded.Relations()) != 2 {
		t.Fatalf("unexpected restored graph: %d nodes, %d relations", len(loaded.Nodes()), len(loaded.Relations()))
	}
	if _, ok := loaded.PathFromSelf("machine"); !ok {
		t.Fatal("restored machine is disconnected from Self")
	}
	n, ok := loaded.Node("project")
	if !ok || n.LastConfirmed == nil || !n.LastConfirmed.Equal(confirmed) {
		t.Fatalf("last confirmation was not preserved: %+v", n)
	}
	machine, ok := loaded.Node("machine")
	if !ok || machine.Source != "inventory fixture" || machine.EvidenceKind != graph.EvidenceObserved || machine.Certainty != graph.CertaintyConfirmed {
		t.Fatalf("node provenance was not preserved: %+v", machine)
	}
	if got := loaded.Relations()[0]; got.Source != "integration fixture" || got.EvidenceKind != graph.EvidenceObserved || got.Certainty != graph.CertaintyConfirmed {
		t.Fatalf("relation provenance was not preserved: %+v", got)
	}
}

func TestSnapshotRejectsDisconnectedGraphWithoutChangingStoredState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	initial := graph.New()
	if err := initial.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSnapshot(ctx, initial); err != nil {
		t.Fatal(err)
	}
	before, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bad := graph.New()
	_ = bad.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Me", ModelVersion: 1})
	_ = bad.AddNode(graph.Node{ID: "orphan", Type: graph.Product, Name: "Orphan", ModelVersion: 1})
	if err = store.SaveSnapshot(ctx, bad); err == nil {
		t.Fatal("expected disconnected snapshot rejection")
	}
	after, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Nodes()) != len(after.Nodes()) {
		t.Fatal("rejected snapshot changed stored state")
	}
}
