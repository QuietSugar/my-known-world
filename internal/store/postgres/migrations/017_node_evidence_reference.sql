ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS evidence_reference text NOT NULL DEFAULT '';
