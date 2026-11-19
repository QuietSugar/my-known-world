package cmdb

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestPostgresEnvironmentRepositoryCRUD(t *testing.T) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := PostgresEnvironmentRepository{DB: db}
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	id := "test-cmdb-env-" + time.Now().UTC().Format("20060102150405.000000000")
	defer repo.Delete(context.Background(), id)
	catalog := Catalog{Repository: repo}
	want := Environment{ID: id, Name: "Synthetic integration", Description: "temporary", Tags: []string{"lab"}, ServiceIDs: []string{"svc-test"}, AccountIDs: []string{"acct-test"}}
	if err := catalog.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	items, err := catalog.Search(ctx, EnvironmentFilter{Tag: "lab", ServiceID: "svc-test"})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, item := range items {
		if item.ID == id {
			found = true
			if len(item.AccountIDs) != 1 || item.AccountIDs[0] != "acct-test" {
				t.Fatalf("stored membership changed: %+v", item)
			}
		}
	}
	if !found {
		t.Fatal("inserted application environment not found")
	}
	want.Name = "Synthetic updated"
	if err := catalog.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	items, err = catalog.Search(ctx, EnvironmentFilter{Query: "updated"})
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range items {
		if item.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("environment upsert did not update search fields")
	}
	resources := ResourceCatalog{Repository: repo}
	service := Resource{ID: id + "-svc", Kind: ServiceResource, Name: "Synthetic service", EnvironmentIDs: []string{id}, CredentialRef: "opaque-provider-ref"}
	if err := resources.Save(ctx, service); err != nil {
		t.Fatal(err)
	}
	defer resources.Remove(context.Background(), ServiceResource, service.ID)
	accounts, err := resources.List(ctx, ServiceResource)
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, item := range accounts {
		if item.ID == service.ID {
			matched = true
			if item.CredentialRef != "opaque-provider-ref" {
				t.Fatalf("credential reference changed: %+v", item)
			}
		}
	}
	if !matched {
		t.Fatal("service catalog insert missing")
	}
	if err := catalog.Remove(ctx, id); err != nil {
		t.Fatal(err)
	}
}
