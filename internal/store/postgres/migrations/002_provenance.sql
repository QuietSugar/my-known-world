ALTER TABLE nodes
    ADD COLUMN IF NOT EXISTS source text NOT NULL DEFAULT 'unspecified',
    ADD COLUMN IF NOT EXISTS evidence_kind text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS certainty text NOT NULL DEFAULT 'unknown';

ALTER TABLE relations
    ADD COLUMN IF NOT EXISTS evidence_kind text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS certainty text NOT NULL DEFAULT 'unknown';

UPDATE relations SET source = 'unspecified' WHERE btrim(source) = '';
ALTER TABLE relations ALTER COLUMN source SET DEFAULT 'unspecified';

DO $$ BEGIN
    ALTER TABLE nodes ADD CONSTRAINT nodes_evidence_kind_check
        CHECK (evidence_kind IN ('unknown','manual','document','observed','derived','imported'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE nodes ADD CONSTRAINT nodes_certainty_check
        CHECK (certainty IN ('unknown','provisional','confirmed','disputed','stale'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE relations ADD CONSTRAINT relations_evidence_kind_check
        CHECK (evidence_kind IN ('unknown','manual','document','observed','derived','imported'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$ BEGIN
    ALTER TABLE relations ADD CONSTRAINT relations_certainty_check
        CHECK (certainty IN ('unknown','provisional','confirmed','disputed','stale'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
