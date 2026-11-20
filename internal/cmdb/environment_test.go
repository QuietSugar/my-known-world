package cmdb

import (
	"context"
	"testing"
)

type memoryEnvironments map[string]Environment

func (m memoryEnvironments) List(context.Context) ([]Environment, error) {
	out := []Environment{}
	for _, e := range m {
		out = append(out, e)
	}
	return out, nil
}
func (m memoryEnvironments) Put(_ context.Context, e Environment) error { m[e.ID] = e; return nil }
func (m memoryEnvironments) Delete(_ context.Context, id string) error  { delete(m, id); return nil }
func TestCatalogSavesGenericGroupsAndFiltersWithoutCoreTypeAssumptions(t *testing.T) {
	repo := memoryEnvironments{}
	catalog := Catalog{Repository: repo}
	ctx := context.Background()
	if err := catalog.Save(ctx, Environment{ID: "env-test", Name: "Synthetic lab", Description: "staging services", Tags: []string{"Lab", "staging", "Lab"}, ServiceIDs: []string{"svc-a", "svc-a"}, AccountIDs: []string{"acct-a"}}); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.Search(ctx, EnvironmentFilter{Query: "synthetic", Tag: "lab", ServiceID: "svc-a"})
	if err != nil || len(got) != 1 {
		t.Fatalf("filter result=%v err=%v", got, err)
	}
	if len(got[0].Tags) != 2 || len(got[0].ServiceIDs) != 1 {
		t.Fatalf("membership was not normalized: %+v", got[0])
	}
	if err := catalog.Remove(ctx, "env-test"); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.Search(ctx, EnvironmentFilter{})
	if err != nil || len(got) != 0 {
		t.Fatalf("removed environment remains: %v %v", got, err)
	}
}
