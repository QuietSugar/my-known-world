ALTER TABLE graph_mutation_operations ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata)='object');
ALTER TABLE graph_mutation_operations ADD COLUMN IF NOT EXISTS run_id bigint REFERENCES sync_source_runs(run_id) ON DELETE SET NULL;
CREATE UNIQUE INDEX IF NOT EXISTS graph_mutation_operations_run_idx ON graph_mutation_operations(run_id) WHERE run_id IS NOT NULL;
