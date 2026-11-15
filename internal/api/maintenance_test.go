package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

type editRepo struct {
	memoryRepository
	created, updated, added, deleted string
}

func (r *editRepo) CreateNode(_ context.Context, n graph.Node, _ graph.Relation) error {
	r.created = n.ID
	return nil
}
func (r *editRepo) UpdateNode(_ context.Context, id string, _ graph.Node) error {
	r.updated = id
	return nil
}
func (r *editRepo) AddRelation(_ context.Context, v graph.Relation) error { r.added = v.ID; return nil }
func (r *editRepo) DeleteRelation(_ context.Context, id string) error     { r.deleted = id; return nil }
func TestLegacyDirectMaintenanceRoutesRequirePreview(t *testing.T) {
	token := strings.Repeat("e", 32)
	repo := &editRepo{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	cases := []struct{ method, path, body string }{{http.MethodPost, "/api/nodes", `{"node":{"id":"new-node","type":"Machine"},"relation":{}}`}, {http.MethodPut, "/api/nodes/machine", `{"id":"machine","type":"Machine","name":"Changed"}`}, {http.MethodPost, "/api/relations", `{"id":"new-rel","from":"self","predicate":"uses","to":"machine"}`}, {http.MethodDelete, "/api/relations/r1", ""}, {http.MethodDelete, "/api/nodes/machine", ""}}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusPreconditionRequired {
			t.Errorf("%s %s: got %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	if repo.created != "" || repo.updated != "" || repo.added != "" || repo.deleted != "" {
		t.Fatal("legacy direct write route reached repository")
	}
}
