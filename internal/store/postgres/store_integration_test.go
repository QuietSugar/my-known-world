package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	fixturedata "github.com/QuietSugar/my-known-world/internal/fixtures"
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

func TestSyntheticWorldBatchAppliesAndRoundTrips(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	batch, err := fixturedata.SyntheticWorld()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := store.ApplySync(ctx, batch)
	if err != nil {
		t.Fatalf("apply synthetic batch: %v", err)
	}
	if len(preview.AddedNodes) != len(batch.Nodes) || len(preview.AddedRelations) < len(batch.Relations) {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	network, ok := loaded.Node("network-home")
	if !ok || network.Network == nil || network.Network.CIDRs[0] != "192.0.2.0/24" {
		t.Fatalf("network model did not round-trip: %+v", network)
	}
}

func TestEntityHistoryTracksCreateUpdateStaleAndDelete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	initial := graph.New()
	_ = initial.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	if err := store.SaveSnapshot(ctx, initial); err != nil {
		t.Fatal(err)
	}
	batch := graph.SourceBatch{Source: "history-test", Nodes: []graph.Node{{ID: "server", Type: graph.Machine, Name: "Server v1", ModelVersion: 1}}}
	if _, err := store.ApplySync(ctx, batch); err != nil {
		t.Fatal(err)
	}
	batch.Nodes[0].Name = "Server v2"
	if _, err := store.ApplySync(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplySync(ctx, graph.SourceBatch{Source: "history-test"}); err != nil {
		t.Fatal(err)
	}
	history, err := store.EntityHistory(ctx, "node", "server", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 || history[0].Action != "stale" || history[1].Action != "updated" || history[2].Action != "created" {
		t.Fatalf("unexpected history: %+v", history)
	}
	if err = store.DeleteNode(ctx, "server"); err != nil {
		t.Fatal(err)
	}
	history, err = store.EntityHistory(ctx, "node", "server", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 || history[0].Action != "deleted" {
		t.Fatalf("delete event missing: %+v", history)
	}
}

func TestFieldResolutionsAppendAndValidateSelectedAssertions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "m", Type: graph.Machine, Name: "Machine", ModelVersion: 1}, graph.Relation{ID: "knows-m", To: "m"})
	for _, a := range []graph.FieldAssertion{{ID: "a", NodeID: "m", FieldPath: "machine.hostname", ValueHash: graph.HashFieldValue("host-a"), Source: "source-a", EvidenceKind: graph.EvidenceObserved, Certainty: graph.CertaintyConfirmed}, {ID: "b", NodeID: "m", FieldPath: "machine.hostname", ValueHash: graph.HashFieldValue("host-b"), Source: "source-b", EvidenceKind: graph.EvidenceObserved, Certainty: graph.CertaintyConfirmed}} {
		if err := g.AddAssertion(a); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	decision, err := store.ResolveField(ctx, graph.FieldResolution{NodeID: "m", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"b"}, RationaleReference: "ticket:42"})
	if err != nil || decision.ID == 0 {
		t.Fatalf("resolution failed: %+v %v", decision, err)
	}
	if _, err = store.ResolveField(ctx, graph.FieldResolution{NodeID: "m", FieldPath: "machine.hostname", SelectedAssertionIDs: []string{"a"}, RationaleReference: "ticket:43"}); err != nil {
		t.Fatal(err)
	}
	items, err := store.FieldResolutions(ctx, "m")
	if err != nil || len(items) != 2 {
		t.Fatalf("resolution history failed: %+v %v", items, err)
	}
	views, err := store.AssertionViews(ctx, "m")
	if err != nil || len(views) != 2 {
		t.Fatalf("effective assertion view failed: %+v %v", views, err)
	}
	selected := 0
	for _, v := range views {
		if v.Selected {
			selected++
			if v.ID != "a" {
				t.Fatalf("latest decision selected wrong assertion: %+v", v)
			}
		}
	}
	if selected != 1 {
		t.Fatalf("expected exactly one selected assertion: %+v", views)
	}
	if _, err = store.ResolveField(ctx, graph.FieldResolution{NodeID: "m", FieldPath: "machine.os", SelectedAssertionIDs: []string{"a"}, RationaleReference: "ticket:44"}); err == nil {
		t.Fatal("cross-field assertion selected")
	}
}

func TestSourcePolicyConstrainsPreviewAndAtomicApply(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	base := graph.New()
	_ = base.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	if err := store.SaveSnapshot(ctx, base); err != nil {
		t.Fatal(err)
	}
	policy := graph.SourcePolicy{Source: "inventory", Priority: 40, AllowedNodeTypes: []graph.NodeType{graph.Machine}, AllowedPredicates: []graph.Predicate{graph.KnowsAbout}, AllowedFieldPaths: []string{"machine.hostname"}}
	if err := store.SetSourcePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	batch := graph.SourceBatch{Source: "inventory", Nodes: []graph.Node{{ID: "product", Type: graph.Product, Name: "Out of scope", ModelVersion: 1}}}
	preview, err := store.PreviewSync(ctx, batch)
	if err != nil || len(preview.Conflicts) == 0 {
		t.Fatalf("out-of-scope preview not reported: %+v %v", preview, err)
	}
	if _, err = store.ApplySync(ctx, batch); err == nil {
		t.Fatal("out-of-scope batch applied")
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Node("product"); ok {
		t.Fatal("rejected batch partially applied")
	}
	policies, err := store.SourcePolicies(ctx)
	if err != nil || len(policies) != 1 {
		t.Fatalf("policy persistence failed: %+v %v", policies, err)
	}
}

func TestPersistentManualNodeAndRelationMaintenance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	base := graph.New()
	_ = base.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	if err := store.SaveSnapshot(ctx, base); err != nil {
		t.Fatal(err)
	}
	node := graph.Node{ID: "manual-net", Type: graph.Network, Name: "Home network", ModelVersion: 1, Source: "manual:test", Network: &graph.NetworkDetails{Kind: "subnet", CIDRs: []string{"192.0.2.0/24"}}}
	relation := graph.Relation{ID: "manual-knows", From: "self", Predicate: graph.KnowsAbout, To: "manual-net", Source: "manual:test"}
	if err := store.CreateNode(ctx, node, relation); err != nil {
		t.Fatal(err)
	}
	node.Name = "Renamed network"
	node.Source = "forged"
	if err := store.UpdateNode(ctx, node.ID, node); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := loaded.Node(node.ID)
	if saved.Name != "Renamed network" || saved.Source != "manual:test" || saved.Network == nil {
		t.Fatalf("edit did not persist: %+v", saved)
	}
	if err := store.DeleteRelation(ctx, relation.ID); err == nil {
		t.Fatal("orphaning relation deletion accepted")
	}
	extra := graph.Relation{ID: "manual-owns", From: "self", Predicate: graph.Owns, To: "manual-net", Source: "manual:test"}
	if err := store.AddRelation(ctx, extra); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRelation(ctx, relation.ID); err != nil {
		t.Fatal(err)
	}
}

func TestManualOperationsCanOnlyUndoInSafeReverseOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	base := graph.New()
	_ = base.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	if err := store.SaveSnapshot(ctx, base); err != nil {
		t.Fatal(err)
	}
	n := graph.Node{ID: "created", Type: graph.Machine, Name: "Before", ModelVersion: 1, Source: "manual:test"}
	r := graph.Relation{ID: "onboard", From: "self", Predicate: graph.KnowsAbout, To: "created", Source: "manual:test"}
	if err := store.CreateNode(ctx, n, r); err != nil {
		t.Fatal(err)
	}
	n.Name = "After"
	if err := store.UpdateNode(ctx, n.ID, n); err != nil {
		t.Fatal(err)
	}
	ops, err := store.MutationOperations(ctx, 20)
	if err != nil || len(ops) != 2 || !ops[0].Reversible {
		t.Fatalf("latest op should be reversible: %+v %v", ops, err)
	}
	if err = store.UndoOperation(ctx, ops[0].ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.Node("created")
	if restored.Name != "Before" {
		t.Fatalf("wrong restored state: %+v", restored)
	}
	ops, err = store.MutationOperations(ctx, 20)
	if err != nil || len(ops) != 2 || ops[0].Reversible || !ops[1].Reversible {
		t.Fatalf("undo stack should expose only unchanged predecessor: %+v %v", ops, err)
	}
	if err = store.UndoOperation(ctx, ops[1].ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := loaded.Node("created"); ok {
		t.Fatalf("creation undo did not remove node: %+v", loaded.Nodes())
	}
}

func TestUndoSnapshotsAreBoundedAndExpiredRecordsRemainAuditOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "m", Type: graph.Machine, Name: "0", ModelVersion: 1}, graph.Relation{ID: "knows", To: "m"})
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	node, _ := g.Node("m")
	for i := 1; i <= 51; i++ {
		node.Name = fmt.Sprintf("%d", i)
		if err := store.UpdateNode(ctx, node.ID, node); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}
	if err := store.UndoOperation(ctx, 1); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("old undo payload should expire, got %v", err)
	}
	ops, err := store.MutationOperations(ctx, 100)
	if err != nil || len(ops) != 51 {
		t.Fatalf("audit summaries should remain after snapshot pruning: count=%d err=%v", len(ops), err)
	}
}

func TestSyncBatchUndoRestoresGraphSourceStateAndIdentityMappings(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	base := graph.New()
	_ = base.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	if err := store.SaveSnapshot(ctx, base); err != nil {
		t.Fatal(err)
	}
	batch := graph.SourceBatch{Source: "inventory", ParserVersion: "v1", IdentityMappings: []graph.IdentityMapping{{SourceID: "external-1", NodeID: "server"}}, Nodes: []graph.Node{{ID: "external-1", Type: graph.Machine, Name: "Before", ModelVersion: 1}}}
	first, err := store.ApplySync(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	batch.ParserVersion = "v2"
	batch.Nodes[0].Name = "After"
	if _, err = store.ApplySync(ctx, batch); err != nil {
		t.Fatal(err)
	}
	ops, err := store.MutationOperations(ctx, 20)
	if err != nil || len(ops) != 2 || !ops[0].Reversible {
		t.Fatalf("sync operations missing: %+v %v", ops, err)
	}
	if err = store.UndoOperation(ctx, ops[0].ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := loaded.Node("server")
	if n.Name != "Before" {
		t.Fatalf("sync undo did not restore object: %+v", n)
	}
	sources, err := store.SyncSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].SourceHash != first.SourceHash || sources[0].ParserVersion != "v1" {
		t.Fatalf("prior source state not restored: %+v %v", sources, err)
	}
	history, err := store.SyncHistory(ctx, "inventory", 10)
	if err != nil || len(history) != 2 || !history[0].Undone {
		t.Fatalf("run history should record undo: %+v %v", history, err)
	}
	ops, err = store.MutationOperations(ctx, 20)
	if err != nil || !ops[1].Reversible {
		t.Fatalf("previous batch should be undoable after latest is rolled back: %+v %v", ops, err)
	}
	if err = store.UndoOperation(ctx, ops[1].ID); err != nil {
		t.Fatal(err)
	}
	sources, err = store.SyncSources(ctx)
	if err != nil || len(sources) != 0 {
		t.Fatalf("first sync should revert to no prior source state: %+v %v", sources, err)
	}
	restored, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatalf("first import undo should restore the Self-only graph: %v", err)
	}
	if len(restored.Nodes()) != 1 || restored.Nodes()[0].Type != graph.Self {
		t.Fatalf("first import undo did not restore Self-only graph: %+v", restored.Nodes())
	}
}

func TestManualMutationPreviewRequiresMatchingRevisionAndPayload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	_ = g.AddNode(graph.Node{ID: "self", Type: graph.Self, Name: "Self", ModelVersion: 1})
	_ = g.Publish(graph.Node{ID: "m", Type: graph.Machine, Name: "Before", ModelVersion: 1}, graph.Relation{ID: "knows", To: "m"})
	if err := store.SaveSnapshot(ctx, g); err != nil {
		t.Fatal(err)
	}
	updated := graph.Node{ID: "m", Type: graph.Machine, Name: "After", ModelVersion: 1}
	mutation := graph.ManualMutation{Kind: "updateNode", ID: "m", Node: &updated}
	plan, err := store.PreviewManualMutation(ctx, mutation)
	if err != nil || len(plan.Delta.UpdatedNodes) != 1 {
		t.Fatalf("preview failed: %+v %v", plan, err)
	}
	bad := updated
	bad.Name = "Tampered"
	other := graph.ManualMutation{Kind: "updateNode", ID: "m", Node: &bad}
	if _, err = store.ApplyManualMutation(ctx, other, plan.ExpectedRevision, plan.PlanHash); err == nil {
		t.Fatal("tampered plan was applied")
	}
	if _, err = store.ApplyManualMutation(ctx, mutation, "wrong-revision", plan.PlanHash); err == nil {
		t.Fatal("stale revision was applied")
	}
	if _, err = store.ApplyManualMutation(ctx, mutation, plan.ExpectedRevision, plan.PlanHash); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := loaded.Node("m")
	if saved.Name != "After" {
		t.Fatalf("previewed mutation did not apply: %+v", saved)
	}
	if _, err = store.PreviewManualMutation(ctx, graph.ManualMutation{Kind: "deleteRelation", ID: "knows"}); err == nil {
		t.Fatal("disconnecting mutation should fail during preview")
	}
}
