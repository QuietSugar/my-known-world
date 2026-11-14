CREATE TABLE IF NOT EXISTS field_assertions (
    id text PRIMARY KEY,
    node_id text NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    field_path text NOT NULL,
    value_hash char(64) NOT NULL CHECK (value_hash ~ '^[0-9a-f]{64}$'),
    reference text NOT NULL DEFAULT '',
    source text NOT NULL,
    evidence_kind text NOT NULL,
    certainty text NOT NULL,
    asserted_at timestamptz NOT NULL,
    UNIQUE(id,node_id)
);
CREATE INDEX IF NOT EXISTS field_assertions_node_field_idx ON field_assertions(node_id,field_path,asserted_at DESC);
