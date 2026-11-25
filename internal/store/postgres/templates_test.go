package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

func TestProjectViewTemplatesCRUD(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := openTestStore(t)
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	tmpl1 := graph.ProjectViewTemplate{
		ID:          "custom-xasc-cards",
		ProjectID:   "projects/xasc",
		Name:        "XASC 运维卡片",
		Description: "专供 XASC 运维值班使用",
		LayoutType:  "cards",
		Config:      json.RawMessage(`{"density":"compact","highlightSecret":true}`),
		IsDefault:   true,
	}

	if err := store.SaveProjectViewTemplate(ctx, tmpl1); err != nil {
		t.Fatalf("save tmpl1 failed: %v", err)
	}

	tmplGlobal := graph.ProjectViewTemplate{
		ID:          "global-matrix",
		ProjectID:   "*",
		Name:        "全局资源矩阵",
		Description: "通用资源矩阵",
		LayoutType:  "table",
		Config:      json.RawMessage(`{"showPort":true}`),
		IsDefault:   false,
	}

	if err := store.SaveProjectViewTemplate(ctx, tmplGlobal); err != nil {
		t.Fatalf("save tmplGlobal failed: %v", err)
	}

	// List for projects/xasc should include both tmpl1 and tmplGlobal
	list, err := store.ListProjectViewTemplates(ctx, "projects/xasc")
	if err != nil {
		t.Fatalf("list templates for xasc failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 templates for xasc, got %d", len(list))
	}
	// Default should come first
	if list[0].ID != "custom-xasc-cards" {
		t.Errorf("expected custom-xasc-cards first, got %s", list[0].ID)
	}

	// List for another project should only include tmplGlobal
	otherList, err := store.ListProjectViewTemplates(ctx, "projects/other")
	if err != nil {
		t.Fatalf("list templates for other failed: %v", err)
	}
	if len(otherList) != 1 || otherList[0].ID != "global-matrix" {
		t.Fatalf("expected 1 global template for other, got %+v", otherList)
	}

	// Get single template
	got, err := store.GetProjectViewTemplate(ctx, "custom-xasc-cards")
	if err != nil {
		t.Fatalf("get template failed: %v", err)
	}
	if got == nil || got.Name != "XASC 运维卡片" {
		t.Fatalf("unexpected template: %+v", got)
	}

	// Update template
	tmpl1.Name = "XASC 运维卡片 v2"
	if err := store.SaveProjectViewTemplate(ctx, tmpl1); err != nil {
		t.Fatalf("update tmpl1 failed: %v", err)
	}
	updated, err := store.GetProjectViewTemplate(ctx, "custom-xasc-cards")
	if err != nil || updated == nil || updated.Name != "XASC 运维卡片 v2" {
		t.Fatalf("expected updated name, got %+v", updated)
	}

	// Delete template
	if err := store.DeleteProjectViewTemplate(ctx, "custom-xasc-cards"); err != nil {
		t.Fatalf("delete tmpl1 failed: %v", err)
	}
	deleted, err := store.GetProjectViewTemplate(ctx, "custom-xasc-cards")
	if err != nil {
		t.Fatalf("get deleted template error: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected nil after delete, got %+v", deleted)
	}
}
