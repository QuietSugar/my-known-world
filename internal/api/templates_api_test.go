package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestProjectTemplatesAPI(t *testing.T) {
	g := graph.New()
	proj := graph.Node{ID: "projects/xasc", Type: graph.SoftwareProject, Name: "XASC", ModelVersion: 1}
	if err := g.AddNode(proj); err != nil {
		t.Fatal(err)
	}

	memStore := NewMemoryTemplateStore()
	h := &Handler{
		Graph:         g,
		TemplateStore: memStore,
	}

	// 1. GET templates: initially returns the 3 builtin templates
	req := httptest.NewRequest(http.MethodGet, "/api/workspace/projects/projects%2Fxasc/templates", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var templates []graph.ProjectViewTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &templates); err != nil {
		t.Fatal(err)
	}
	if len(templates) < 3 {
		t.Fatalf("expected at least 3 builtin templates, got %d", len(templates))
	}

	// 2. POST create a custom template
	newTmpl := graph.ProjectViewTemplate{
		ID:          "custom-my-view",
		Name:        "我的专享看板",
		Description: "测试自定义模板",
		LayoutType:  "board",
		Config:      json.RawMessage(`{"groupBy":"environment","density":"compact"}`),
	}
	bodyBytes, _ := json.Marshal(newTmpl)
	req = httptest.NewRequest(http.MethodPost, "/api/workspace/projects/projects%2Fxasc/templates", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created graph.ProjectViewTemplate
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "custom-my-view" || created.ProjectID != "projects/xasc" {
		t.Fatalf("unexpected created template: %+v", created)
	}

	// 3. GET single template
	req = httptest.NewRequest(http.MethodGet, "/api/workspace/projects/projects%2Fxasc/templates/custom-my-view", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for single get, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. PUT update custom template
	updatedInput := graph.ProjectViewTemplate{
		Name:        "我的专享看板 (更新版)",
		Description: "更新后的描述",
		LayoutType:  "table",
	}
	upBytes, _ := json.Marshal(updatedInput)
	req = httptest.NewRequest(http.MethodPut, "/api/workspace/projects/projects%2Fxasc/templates/custom-my-view", bytes.NewReader(upBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify update took effect
	tUpdated, err := memStore.GetProjectViewTemplate(req.Context(), "custom-my-view")
	if err != nil || tUpdated == nil || tUpdated.Name != "我的专享看板 (更新版)" {
		t.Fatalf("expected updated template in store, got %+v", tUpdated)
	}

	// 5. PUT on builtin template should return 403 Forbidden
	req = httptest.NewRequest(http.MethodPut, "/api/workspace/projects/projects%2Fxasc/templates/preset-secret-cards", bytes.NewReader(upBytes))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when updating builtin template, got %d", rec.Code)
	}

	// 6. DELETE on builtin template should return 403 Forbidden
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/projects/projects%2Fxasc/templates/preset-secret-cards", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when deleting builtin template, got %d", rec.Code)
	}

	// 7. DELETE on custom template should succeed
	req = httptest.NewRequest(http.MethodDelete, "/api/workspace/projects/projects%2Fxasc/templates/custom-my-view", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when deleting custom template, got %d", rec.Code)
	}

	// Verify deletion
	tDeleted, _ := memStore.GetProjectViewTemplate(req.Context(), "custom-my-view")
	if tDeleted != nil {
		t.Fatalf("expected custom template to be deleted, but still found: %+v", tDeleted)
	}
}

func TestProjectTemplatesAuth(t *testing.T) {
	g := graph.New()
	memStore := NewMemoryTemplateStore()
	h := &Handler{
		Graph:         g,
		TemplateStore: memStore,
		WriteToken:    strings.Repeat("a", 32),
	}

	// POST without token should fail with 401
	newTmpl := graph.ProjectViewTemplate{Name: "未授权模板"}
	bodyBytes, _ := json.Marshal(newTmpl)
	req := httptest.NewRequest(http.MethodPost, "/api/workspace/projects/p1/templates", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rec.Code)
	}

	// POST with token should succeed with 201
	req = httptest.NewRequest(http.MethodPost, "/api/workspace/projects/p1/templates", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created, got %d: %s", rec.Code, rec.Body.String())
	}
}
