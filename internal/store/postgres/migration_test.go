package postgres

import (
	"context"
	"testing"
	"time"
)

func TestMigrationRepeatKeepsMarkdownRelationPredicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// ids come from the caller; there is no separate uuid column any more
	// (migration 024 merged identity into id).
	if _, err := store.db.ExecContext(ctx, `INSERT INTO nodes(id,type,name,model_version) VALUES
		('11111111-1111-4111-8111-111111111111','Self','Me',1),
		('22222222-2222-4222-8222-222222222222','Product','Product',1),
		('33333333-3333-4333-8333-333333333333','SoftwareInstance','Service',1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `INSERT INTO relations(id,from_id,predicate,to_id,source,recorded_at) VALUES
		('44444444-4444-4444-8444-444444444444','22222222-2222-4222-8222-222222222222','hasInstance','33333333-3333-4333-8333-333333333333','',now()),
		('55555555-5555-4555-8555-555555555555','11111111-1111-4111-8111-111111111111','relatedTo','33333333-3333-4333-8333-333333333333','',now())`); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("restart with Markdown relations: %v", err)
	}
}
