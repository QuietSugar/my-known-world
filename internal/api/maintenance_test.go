package api

import (
	"context"
	"github.com/QuietSugar/my-known-world/internal/graph"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type editRepo struct {
	memoryRepository
	created string
	updated string
	added   string
	deleted string
}

func (r *editRepo) CreateNode(_ context.Context, n graph.Node, relation graph.Relation) error {
	r.created = n.ID
	return nil
}
func (r *editRepo) UpdateNode(_ context.Context, id string, n graph.Node) error {
	r.updated = id
	return nil
}
func (r *editRepo) AddRelation(_ context.Context, v graph.Relation) error { r.added = v.ID; return nil }
func (r *editRepo) DeleteRelation(_ context.Context, id string) error     { r.deleted = id; return nil }
func TestAuthenticatedMaintenanceRoutes(t *testing.T) {
	token := strings.Repeat("e", 32)
	repo := &editRepo{memoryRepository: memoryRepository{g: fixture()}}
	h := Handler{Graph: repo.g, Repository: repo, WriteToken: token}
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	create := `{"node":{"id":"new-node","type":"Network","name":"Home net","modelVersion":1,"network":{"kind":"subnet","cidrs":["192.0.2.0/24"]}},"relation":{"id":"onboard-new","from":"self","predicate":"knowsAbout","to":"new-node"}}`
	if w := call(http.MethodPost, "/api/nodes", create); w.Code != http.StatusCreated {
		t.Fatalf("node create: %d %s", w.Code, w.Body.String())
	}
	if repo.created != "new-node" {
		t.Fatal("node create not dispatched")
	}
	node := `{"id":"server","type":"Machine","name":"Server","modelVersion":1}`
	if w := call(http.MethodPut, "/api/nodes/server", node); w.Code != http.StatusOK {
		t.Fatalf("node update: %d %s", w.Code, w.Body.String())
	}
	if repo.updated != "server" {
		t.Fatal("node update not dispatched")
	}
	rel := `{"id":"uses-server","from":"self","predicate":"uses","to":"server"}`
	if w := call(http.MethodPost, "/api/relations", rel); w.Code != http.StatusCreated {
		t.Fatalf("relation create: %d %s", w.Code, w.Body.String())
	}
	if repo.added != "uses-server" {
		t.Fatal("relation create not dispatched")
	}
	if w := call(http.MethodDelete, "/api/relations/uses-server", ""); w.Code != http.StatusOK {
		t.Fatalf("relation delete: %d %s", w.Code, w.Body.String())
	}
	if repo.deleted != "uses-server" {
		t.Fatal("relation delete not dispatched")
	}
}
