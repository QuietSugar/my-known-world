CREATE TABLE IF NOT EXISTS project_view_templates (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL DEFAULT '*',
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    layout_type TEXT NOT NULL DEFAULT 'cards',
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_view_templates_project ON project_view_templates(project_id);
