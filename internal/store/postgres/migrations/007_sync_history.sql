CREATE TABLE IF NOT EXISTS sync_source_runs (
    run_id bigserial PRIMARY KEY,
    source text NOT NULL,
    source_hash text NOT NULL,
    parser_version text NOT NULL DEFAULT '',
    applied_at timestamptz NOT NULL DEFAULT now(),
    preview jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS sync_source_runs_source_time_idx ON sync_source_runs(source, applied_at DESC, run_id DESC);
