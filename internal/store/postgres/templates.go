package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/QuietSugar/my-known-world/internal/graph"
)

// ListProjectViewTemplates returns all templates associated with the given project, plus global '*' templates.
func (s *Store) ListProjectViewTemplates(ctx context.Context, projectID string) ([]graph.ProjectViewTemplate, error) {
	query := `
		SELECT id, project_id, name, description, layout_type, config, is_default, created_at, updated_at
		FROM project_view_templates
		WHERE project_id = $1 OR project_id = '*'
		ORDER BY is_default DESC, created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project view templates: %w", err)
	}
	defer rows.Close()

	var templates []graph.ProjectViewTemplate
	for rows.Next() {
		var tmpl graph.ProjectViewTemplate
		var rawConfig []byte
		if err := rows.Scan(
			&tmpl.ID,
			&tmpl.ProjectID,
			&tmpl.Name,
			&tmpl.Description,
			&tmpl.LayoutType,
			&rawConfig,
			&tmpl.IsDefault,
			&tmpl.CreatedAt,
			&tmpl.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan project view template: %w", err)
		}
		if len(rawConfig) > 0 {
			tmpl.Config = json.RawMessage(rawConfig)
		} else {
			tmpl.Config = json.RawMessage("{}")
		}
		templates = append(templates, tmpl)
	}
	return templates, rows.Err()
}

// GetProjectViewTemplate retrieves a template by ID.
func (s *Store) GetProjectViewTemplate(ctx context.Context, id string) (*graph.ProjectViewTemplate, error) {
	query := `
		SELECT id, project_id, name, description, layout_type, config, is_default, created_at, updated_at
		FROM project_view_templates
		WHERE id = $1
	`
	var tmpl graph.ProjectViewTemplate
	var rawConfig []byte
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&tmpl.ID,
		&tmpl.ProjectID,
		&tmpl.Name,
		&tmpl.Description,
		&tmpl.LayoutType,
		&rawConfig,
		&tmpl.IsDefault,
		&tmpl.CreatedAt,
		&tmpl.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get project view template: %w", err)
	}
	if len(rawConfig) > 0 {
		tmpl.Config = json.RawMessage(rawConfig)
	} else {
		tmpl.Config = json.RawMessage("{}")
	}
	return &tmpl, nil
}

// SaveProjectViewTemplate inserts or updates a project view template.
func (s *Store) SaveProjectViewTemplate(ctx context.Context, tmpl graph.ProjectViewTemplate) error {
	if tmpl.ID == "" {
		return errors.New("template id cannot be empty")
	}
	if tmpl.Name == "" {
		return errors.New("template name cannot be empty")
	}
	if tmpl.ProjectID == "" {
		tmpl.ProjectID = "*"
	}
	if tmpl.LayoutType == "" {
		tmpl.LayoutType = "cards"
	}
	cfgBytes := []byte(tmpl.Config)
	if len(cfgBytes) == 0 {
		cfgBytes = []byte("{}")
	}
	now := time.Now()
	query := `
		INSERT INTO project_view_templates (id, project_id, name, description, layout_type, config, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			layout_type = EXCLUDED.layout_type,
			config = EXCLUDED.config,
			is_default = EXCLUDED.is_default,
			updated_at = EXCLUDED.updated_at
	`
	_, err := s.db.ExecContext(ctx, query, tmpl.ID, tmpl.ProjectID, tmpl.Name, tmpl.Description, tmpl.LayoutType, cfgBytes, tmpl.IsDefault, now)
	if err != nil {
		return fmt.Errorf("save project view template: %w", err)
	}
	return nil
}

// DeleteProjectViewTemplate deletes a template by ID.
func (s *Store) DeleteProjectViewTemplate(ctx context.Context, id string) error {
	query := `DELETE FROM project_view_templates WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete project view template: %w", err)
	}
	return nil
}
