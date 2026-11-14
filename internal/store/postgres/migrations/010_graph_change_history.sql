CREATE TABLE IF NOT EXISTS graph_change_history (
    history_id bigserial PRIMARY KEY,
    run_id bigint REFERENCES sync_source_runs(run_id) ON DELETE SET NULL,
    source text NOT NULL,
    entity_type text NOT NULL CHECK (entity_type IN ('node','relation','assertion')),
    entity_id text NOT NULL,
    action text NOT NULL CHECK (action IN ('created','updated','stale','deleted')),
    recorded_at timestamptz NOT NULL DEFAULT now(),
    snapshot jsonb
);
CREATE INDEX IF NOT EXISTS graph_change_history_entity_time_idx ON graph_change_history(entity_type,entity_id,recorded_at DESC,history_id DESC);
