package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestMigrationAddsUnknownProvenanceToLegacyRows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	legacySQL, err := migrations.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, string(legacySQL)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, `INSERT INTO nodes(id,type,name,model_version) VALUES ('self','Self','Me',1),('machine','Machine','Legacy host',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.ExecContext(ctx, `INSERT INTO relations(id,from_id,predicate,to_id,source,recorded_at) VALUES ('r1','self','knowsAbout','machine','',now())`); err != nil {
		t.Fatal(err)
	}
	if err = store.Migrate(ctx); err != nil {
		t.Fatalf("upgrade from schema v1: %v", err)
	}
	loaded, err := store.LoadSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	machine, ok := loaded.Node("machine")
	if !ok || machine.Source != "unspecified" || machine.EvidenceKind != graph.EvidenceUnknown || machine.Certainty != graph.CertaintyUnknown {
		t.Fatalf("legacy node did not migrate as unknown: %+v", machine)
	}
	relation := loaded.Relations()[0]
	if relation.Source != "unspecified" || relation.EvidenceKind != graph.EvidenceUnknown || relation.Certainty != graph.CertaintyUnknown {
		t.Fatalf("legacy relation did not migrate as unknown: %+v", relation)
	}
}
