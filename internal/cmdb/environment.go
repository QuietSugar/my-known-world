package cmdb

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// Environment is application-owned grouping metadata; it is not a new core graph type.
type Environment struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ServiceIDs  []string `json:"serviceIds,omitempty"`
	AccountIDs  []string `json:"accountIds,omitempty"`
}
type EnvironmentFilter struct {
	Query     string
	Tag       string
	ServiceID string
	AccountID string
}
type EnvironmentRepository interface {
	List(context.Context) ([]Environment, error)
	Put(context.Context, Environment) error
	Delete(context.Context, string) error
}
type Catalog struct{ Repository EnvironmentRepository }

func (c Catalog) Save(ctx context.Context, e Environment) error {
	if c.Repository == nil {
		return errors.New("environment repository is not configured")
	}
	e.ID = strings.TrimSpace(e.ID)
	e.Name = strings.TrimSpace(e.Name)
	if e.ID == "" || e.Name == "" || len(e.ID) > 256 || len(e.Name) > 256 || strings.ContainsAny(e.ID+e.Name, "\r\n") {
		return errors.New("environment ID and name are required and must be safe")
	}
	e.Description = strings.TrimSpace(e.Description)
	e.Tags = cleanStrings(e.Tags)
	e.ServiceIDs = cleanStrings(e.ServiceIDs)
	e.AccountIDs = cleanStrings(e.AccountIDs)
	if len(e.Tags) > 100 || len(e.ServiceIDs) > 1000 || len(e.AccountIDs) > 1000 {
		return errors.New("environment membership limit exceeded")
	}
	return c.Repository.Put(ctx, e)
}
func (c Catalog) Search(ctx context.Context, f EnvironmentFilter) ([]Environment, error) {
	if c.Repository == nil {
		return nil, errors.New("environment repository is not configured")
	}
	items, err := c.Repository.List(ctx)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(strings.TrimSpace(f.Query))
	tag := strings.ToLower(strings.TrimSpace(f.Tag))
	out := []Environment{}
	for _, e := range items {
		if q != "" && !strings.Contains(strings.ToLower(e.Name+" "+e.Description+" "+strings.Join(e.Tags, " ")), q) {
			continue
		}
		if tag != "" && !hasFold(e.Tags, tag) {
			continue
		}
		if f.ServiceID != "" && !contains(e.ServiceIDs, f.ServiceID) {
			continue
		}
		if f.AccountID != "" && !contains(e.AccountIDs, f.AccountID) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (c Catalog) Remove(ctx context.Context, id string) error {
	if c.Repository == nil {
		return errors.New("environment repository is not configured")
	}
	if strings.TrimSpace(id) == "" {
		return errors.New("environment ID is required")
	}
	return c.Repository.Delete(ctx, id)
}
func cleanStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
func hasFold(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
