CREATE TABLE IF NOT EXISTS graph_mutation_operations (
 operation_id bigserial PRIMARY KEY,
 action text NOT NULL,
 before_state jsonb NOT NULL CHECK (jsonb_typeof(before_state)='object'),
 after_state jsonb NOT NULL CHECK (jsonb_typeof(after_state)='object'),
 after_hash char(64) NOT NULL CHECK (after_hash ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS graph_operation_undos (
 undo_id bigserial PRIMARY KEY,
 operation_id bigint NOT NULL UNIQUE REFERENCES graph_mutation_operations(operation_id),
 before_hash char(64) NOT NULL CHECK (before_hash ~ '^[0-9a-f]{64}$'),
 recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS graph_mutation_operations_time_idx ON graph_mutation_operations(recorded_at DESC,operation_id DESC);
