CREATE TABLE IF NOT EXISTS sync_source_policies (
 source text PRIMARY KEY CHECK (length(source) BETWEEN 1 AND 200),
 priority integer NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 100),
 allowed_node_types jsonb NOT NULL CHECK (jsonb_typeof(allowed_node_types)='array'),
 allowed_predicates jsonb NOT NULL CHECK (jsonb_typeof(allowed_predicates)='array'),
 allowed_field_paths jsonb NOT NULL CHECK (jsonb_typeof(allowed_field_paths)='array'),
 updated_at timestamptz NOT NULL DEFAULT now()
);
