CREATE TABLE IF NOT EXISTS schema_migrations (
    version integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
    id text PRIMARY KEY,
    type text NOT NULL,
    name text NOT NULL,
    lifecycle text NOT NULL DEFAULT '',
    last_confirmed timestamptz,
    model_version integer NOT NULL CHECK (model_version > 0),
    CHECK (type IN ('Self','Hardware','Machine','SoftwareProject','SoftwareInstance','RunningPeriod','AccountCredential','Product','Document','Runbook','WorkItem','ProvisionalObject'))
);
CREATE UNIQUE INDEX IF NOT EXISTS nodes_single_self ON nodes ((type)) WHERE type = 'Self';

CREATE TABLE IF NOT EXISTS relations (
    id text PRIMARY KEY,
    from_id text NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    predicate text NOT NULL,
    to_id text NOT NULL REFERENCES nodes(id) ON DELETE RESTRICT,
    source text NOT NULL DEFAULT '',
    recorded_at timestamptz NOT NULL,
    CHECK (from_id <> to_id),
    CHECK (predicate IN ('owns','maintains','knowsAbout','hosts','implementedFrom','hasRunningPeriod','hasCredential','documents','appliesTo','targets','supersedes','dependsOn'))
);
CREATE INDEX IF NOT EXISTS relations_from_idx ON relations (from_id, predicate);
CREATE INDEX IF NOT EXISTS relations_to_idx ON relations (to_id, predicate);
