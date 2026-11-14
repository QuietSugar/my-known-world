CREATE TABLE IF NOT EXISTS sync_identity_mappings (
    source text NOT NULL,
    source_id text NOT NULL,
    node_id text NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    PRIMARY KEY (source, source_id)
);
