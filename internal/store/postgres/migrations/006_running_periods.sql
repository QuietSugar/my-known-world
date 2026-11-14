CREATE TABLE IF NOT EXISTS running_periods (
    node_id text PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    started_at timestamptz,
    ended_at timestamptz,
    deployed_version text NOT NULL DEFAULT '',
    verification_at timestamptz,
    verification_result text NOT NULL DEFAULT '',
    evidence_ref text NOT NULL DEFAULT '',
    CHECK (started_at IS NULL OR ended_at IS NULL OR ended_at >= started_at),
    CHECK ((verification_at IS NULL) = (verification_result = ''))
);
