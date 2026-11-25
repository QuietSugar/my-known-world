package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// TemplateStore manages persistent storage for project view templates.
type TemplateStore interface {
	ListProjectViewTemplates(ctx context.Context, projectID string) ([]graph.ProjectViewTemplate, error)
	GetProjectViewTemplate(ctx context.Context, id string) (*graph.ProjectViewTemplate, error)
	SaveProjectViewTemplate(ctx context.Context, tmpl graph.ProjectViewTemplate) error
	DeleteProjectViewTemplate(ctx context.Context, id string) error
}

// MemoryTemplateStore provides an in-memory fallback for TemplateStore.
type MemoryTemplateStore struct {
	mu        sync.RWMutex
	templates map[string]graph.ProjectViewTemplate
}

func NewMemoryTemplateStore() *MemoryTemplateStore {
	return &MemoryTemplateStore{
		templates: make(map[string]graph.ProjectViewTemplate),
	}
}

func (m *MemoryTemplateStore) ListProjectViewTemplates(_ context.Context, projectID string) ([]graph.ProjectViewTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []graph.ProjectViewTemplate
	for _, t := range m.templates {
		if t.ProjectID == "*" || t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *MemoryTemplateStore) GetProjectViewTemplate(_ context.Context, id string) (*graph.ProjectViewTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	if !ok {
		return nil, nil
	}
	return &t, nil
}

func (m *MemoryTemplateStore) SaveProjectViewTemplate(_ context.Context, tmpl graph.ProjectViewTemplate) error {
	if tmpl.ID == "" {
		return errors.New("template id cannot be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if tmpl.CreatedAt.IsZero() {
		tmpl.CreatedAt = now
	}
	tmpl.UpdatedAt = now
	m.templates[tmpl.ID] = tmpl
	return nil
}

func (m *MemoryTemplateStore) DeleteProjectViewTemplate(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.templates, id)
	return nil
}

var defaultMemoryTemplateStore = NewMemoryTemplateStore()

// BuiltinTemplates returns the system-provided templates.
func BuiltinTemplates() []graph.ProjectViewTemplate {
	return []graph.ProjectViewTemplate{
		{
			ID:          "preset-secret-cards",
			ProjectID:   "*",
			Name:        "密码速查流 (Quick Secret Cards)",
			Description: "按生产/测试环境分 Tab 呈现高密度密码速查卡片，点击一键极速复制",
			LayoutType:  "cards",
			Config:      json.RawMessage(`{"groupBy":"environment","highlightSecret":true,"showCredentialDetails":true,"density":"comfortable"}`),
			IsDefault:   true,
			Builtin:     true,
		},
		{
			ID:          "preset-env-board",
			ProjectID:   "*",
			Name:        "多列环境看板 (Environment Columns)",
			Description: "按开发、测试、生产环境分列，一屏看清各环境实例、机器及专属口令",
			LayoutType:  "board",
			Config:      json.RawMessage(`{"groupBy":"environment","showInstances":true,"showMachines":true,"showCredentials":true}`),
			IsDefault:   false,
			Builtin:     true,
		},
		{
			ID:          "preset-compact-matrix",
			ProjectID:   "*",
			Name:        "紧凑资源矩阵 (Compact Matrix)",
			Description: "高信息密度表格，适合大屏排查、全局快速检索与密码速查",
			LayoutType:  "table",
			Config:      json.RawMessage(`{"density":"compact","showPort":true,"showHost":true,"showAccount":true}`),
			IsDefault:   false,
			Builtin:     true,
		},
	}
}

func generateTemplateID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("custom-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

// projectTemplatesRoute handles /api/workspace/projects/{id}/templates[/{templateId}]
func (h *Handler) projectTemplatesRoute(w http.ResponseWriter, r *http.Request, subPath string) {
	idx := strings.Index(subPath, "/templates")
	if idx == -1 {
		writeError(w, http.StatusNotFound, "not_found", "invalid templates path")
		return
	}

	projectID, err := url.PathUnescape(subPath[:idx])
	if err != nil || projectID == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "project id is invalid")
		return
	}

	templateSub := strings.TrimPrefix(subPath[idx+len("/templates"):], "/")
	templateID, _ := url.PathUnescape(templateSub)

	store := h.TemplateStore
	if store == nil {
		store = defaultMemoryTemplateStore
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		if templateID != "" {
			for _, b := range BuiltinTemplates() {
				if b.ID == templateID {
					writeJSON(w, http.StatusOK, b)
					return
				}
			}
			tmpl, err := store.GetProjectViewTemplate(ctx, templateID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "store_error", err.Error())
				return
			}
			if tmpl == nil {
				writeError(w, http.StatusNotFound, "not_found", "template not found")
				return
			}
			writeJSON(w, http.StatusOK, tmpl)
			return
		}

		builtins := BuiltinTemplates()
		customs, err := store.ListProjectViewTemplates(ctx, projectID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}

		all := make([]graph.ProjectViewTemplate, 0, len(builtins)+len(customs))
		all = append(all, builtins...)
		all = append(all, customs...)
		writeJSON(w, http.StatusOK, all)

	case http.MethodPost:
		if !h.isWriteAuthorized(w, r) {
			return
		}
		var input graph.ProjectViewTemplate
		if err := decodeBody(w, r, createBodyLimit, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if input.Name == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "template name cannot be empty")
			return
		}
		if input.ID == "" {
			input.ID = generateTemplateID()
		}
		if input.ProjectID == "" {
			input.ProjectID = projectID
		}
		if input.LayoutType == "" {
			input.LayoutType = "cards"
		}
		input.Builtin = false
		if err := store.SaveProjectViewTemplate(ctx, input); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, input)

	case http.MethodPut:
		if !h.isWriteAuthorized(w, r) {
			return
		}
		if templateID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "template id required for PUT")
			return
		}
		for _, b := range BuiltinTemplates() {
			if b.ID == templateID {
				writeError(w, http.StatusForbidden, "forbidden", "system preset templates cannot be modified")
				return
			}
		}
		var input graph.ProjectViewTemplate
		if err := decodeBody(w, r, createBodyLimit, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		input.ID = templateID
		if input.ProjectID == "" {
			input.ProjectID = projectID
		}
		input.Builtin = false
		if err := store.SaveProjectViewTemplate(ctx, input); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, input)

	case http.MethodDelete:
		if !h.isWriteAuthorized(w, r) {
			return
		}
		if templateID == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "template id required for DELETE")
			return
		}
		for _, b := range BuiltinTemplates() {
			if b.ID == templateID {
				writeError(w, http.StatusForbidden, "forbidden", "system preset templates cannot be deleted")
				return
			}
		}
		if err := store.DeleteProjectViewTemplate(ctx, templateID); err != nil {
			writeError(w, http.StatusInternalServerError, "store_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": templateID})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported method")
	}
}

func (h *Handler) isWriteAuthorized(w http.ResponseWriter, r *http.Request) bool {
	if len(h.WriteToken) == 0 {
		return true
	}
	return h.authorized(w, r)
}
