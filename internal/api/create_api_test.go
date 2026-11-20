package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

const createTestToken = "0123456789abcdef0123456789abcdef"

type createRepository struct {
	memoryRepository
	applied  []graph.CreateBatch
	conflict error
}

func (r *createRepository) ApplyCreateBatch(_ context.Context, batch graph.CreateBatch, expected string) (graph.CreateResult, string, error) {
	if r.conflict != nil {
		return graph.CreateResult{}, "", r.conflict
	}
	next, result, err := r.g.ApplyCreateBatch(batch)
	if err != nil {
		return graph.CreateResult{}, "", err
	}
	if expected != "" && expected != "test-revision" {
		return graph.CreateResult{}, "", errors.New("stale revision")
	}
	r.g = next
	r.applied = append(r.applied, batch)
	return result, "test-revision", nil
}

func createHandler() (*Handler, *createRepository) {
	repo := &createRepository{memoryRepository: memoryRepository{g: fixture()}}
	return &Handler{Graph: repo.g, Repository: repo, WriteToken: createTestToken}, repo
}

func createRequest(h *Handler, path, body string, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	return response
}

func TestCreateNodeAPIPublishesTheSelfAnchor(t *testing.T) {
	h, repo := createHandler()
	if response := createRequest(h, "/api/write/nodes", `{"node":{"id":"machine-new","type":"Machine","name":"New host","modelVersion":1}}`, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: %d %s", response.Code, response.Body.String())
	}
	response := createRequest(h, "/api/write/nodes", `{"node":{"id":"machine-new","type":"Machine","name":"New host","modelVersion":1},"anchor":{"predicate":"owns"}}`, createTestToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("create node: %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"nodeIds":["machine-new"]`) || !strings.Contains(response.Body.String(), `"relationIds":["anchor-machine-new"]`) {
		t.Fatalf("response does not report the created objects: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"revision":"test-revision"`) {
		t.Fatalf("response does not report the revision: %s", response.Body.String())
	}
	if len(repo.applied) != 1 {
		t.Fatalf("repository was not called once: %d", len(repo.applied))
	}
	node, ok := repo.g.Node("machine-new")
	if !ok || node.Name != "New host" {
		t.Fatalf("node missing: %+v", node)
	}
	var anchored bool
	for _, relation := range repo.g.Relations() {
		if relation.ID == "anchor-machine-new" && relation.From == "self" && relation.To == "machine-new" && relation.Predicate == graph.Owns {
			anchored = true
		}
	}
	if !anchored {
		t.Fatal("Self anchor relation missing or wrong")
	}
}

func TestCreateRelationAPIRejectsUnknownEndpoints(t *testing.T) {
	h, repo := createHandler()
	response := createRequest(h, "/api/write/relations", `{"relation":{"id":"r-new","from":"self","predicate":"owns","to":"ghost"}}`, createTestToken)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "create_rejected") {
		t.Fatalf("unknown endpoint: %d %s", response.Code, response.Body.String())
	}
	if len(repo.applied) != 0 {
		t.Fatal("rejected relation reached the repository")
	}
	response = createRequest(h, "/api/write/relations", `{"relation":{"id":"r-new","from":"self","predicate":"owns","to":"machine"}}`, createTestToken)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"relationIds":["r-new"]`) {
		t.Fatalf("create relation: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateBatchAPICreatesNodesAndRelationsTogether(t *testing.T) {
	src := fixture()
	g := graph.New()
	for _, node := range src.Nodes() {
		if err := g.AddNode(node); err != nil {
			t.Fatal(err)
		}
	}
	for _, relation := range src.Relations() {
		if err := g.AddRelation(relation); err != nil {
			t.Fatal(err)
		}
	}
	repo := &createRepository{memoryRepository: memoryRepository{g: g}}
	h := &Handler{Graph: repo.g, Repository: repo, WriteToken: createTestToken}
	body := `{"nodes":[` +
		`{"node":{"id":"machine-a","type":"Machine","name":"A","modelVersion":1}},` +
		`{"node":{"id":"machine-b","type":"Machine","name":"B","modelVersion":1}}],` +
		`"relations":[{"id":"depends","from":"machine-a","predicate":"dependsOn","to":"machine-b"}]}`
	response := createRequest(h, "/api/write/batch", body, createTestToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("create batch: %d %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"nodeIds":["machine-a","machine-b"]`) || !strings.Contains(response.Body.String(), `"relationIds":["anchor-machine-a","anchor-machine-b","depends"]`) {
		t.Fatalf("batch response: %s", response.Body.String())
	}
	if _, ok := repo.g.Node("machine-b"); !ok {
		t.Fatal("batch did not create the second node")
	}
}

func TestCreateAPIValidatesPayloadsBeforeTouchingTheGraph(t *testing.T) {
	h, repo := createHandler()
	cases := []struct{ name, path, body string }{
		{name: "unknown field", path: "/api/write/nodes", body: `{"node":{"id":"x","type":"Machine","name":"x","modelVersion":1},"secret":"value"}`},
		{name: "trailing value", path: "/api/write/nodes", body: `{"node":{}} {}`},
		{name: "oversized body", path: "/api/write/batch", body: `{"nodes":[{"node":{"id":"` + strings.Repeat("x", createBatchLimit) + `"}}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := createRequest(h, tc.path, tc.body, createTestToken)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("got %d %s", response.Code, response.Body.String())
			}
			if len(repo.applied) != 0 {
				t.Fatal("invalid payload reached the repository")
			}
		})
	}
}

func TestCreateAPIRejectsDuplicatesAndStaleRevisions(t *testing.T) {
	h, repo := createHandler()
	response := createRequest(h, "/api/write/nodes", `{"node":{"id":"machine","type":"Machine","name":"dup","modelVersion":1}}`, createTestToken)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate node: %d %s", response.Code, response.Body.String())
	}
	repo.conflict = errors.New("graph changed since this batch was prepared")
	response = createRequest(h, "/api/write/nodes", `{"node":{"id":"machine-new","type":"Machine","name":"x","modelVersion":1},"expectedRevision":"old"}`, createTestToken)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "graph changed") {
		t.Fatalf("stale revision: %d %s", response.Code, response.Body.String())
	}
}

func TestCreateAPIFailsClosedWithoutWriteSupport(t *testing.T) {
	h := &Handler{Graph: fixture(), Repository: &memoryRepository{g: fixture()}, WriteToken: createTestToken}
	response := createRequest(h, "/api/write/nodes", `{"node":{"id":"n","type":"Machine","name":"x","modelVersion":1}}`, createTestToken)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unsupported repository: %d %s", response.Code, response.Body.String())
	}
	disabled := &Handler{Graph: fixture(), Repository: &memoryRepository{g: fixture()}}
	response = createRequest(disabled, "/api/write/nodes", `{"node":{"id":"n","type":"Machine","name":"x","modelVersion":1}}`, createTestToken)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "writes_disabled") {
		t.Fatalf("writes disabled: %d %s", response.Code, response.Body.String())
	}
}

func TestGuardedLegacyRoutesStillRequirePreview(t *testing.T) {
	h, repo := createHandler()
	response := createRequest(h, "/api/nodes", `{"node":{"id":"machine-legacy","type":"Machine","name":"x"},"relation":{}}`, createTestToken)
	if response.Code != http.StatusPreconditionRequired || len(repo.applied) != 0 {
		t.Fatalf("legacy route must stay guarded: %d %s", response.Code, response.Body.String())
	}
}
