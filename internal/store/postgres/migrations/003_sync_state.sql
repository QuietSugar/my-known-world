CREATE TABLE IF NOT EXISTS sync_sources (
    source text PRIMARY KEY,
    source_hash text NOT NULL,
    parser_version text NOT NULL DEFAULT '',
    applied_at timestamptz NOT NULL DEFAULT now()
);
