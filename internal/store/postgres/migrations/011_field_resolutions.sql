CREATE TABLE IF NOT EXISTS field_resolutions (
 resolution_id bigserial PRIMARY KEY,
 node_id text NOT NULL,
 field_path text NOT NULL,
 selected_assertion_ids jsonb NOT NULL CHECK (jsonb_typeof(selected_assertion_ids) = 'array' AND jsonb_array_length(selected_assertion_ids) > 0),
 rationale_reference text NOT NULL CHECK (length(rationale_reference) <= 1024 AND position(E'\n' in rationale_reference) = 0 AND position(E'\r' in rationale_reference) = 0),
 recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS field_resolutions_node_field_time_idx ON field_resolutions(node_id,field_path,recorded_at DESC,resolution_id DESC);
