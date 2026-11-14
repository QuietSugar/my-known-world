DO $$ BEGIN
 ALTER TABLE relations DROP CONSTRAINT IF EXISTS relations_predicate_check;
 ALTER TABLE relations ADD CONSTRAINT relations_predicate_check CHECK (predicate IN ('owns','maintains','knowsAbout','uses','canOperate','routesThrough','providesAccessTo','hosts','implementedFrom','hasRunningPeriod','hasCredential','documents','appliesTo','targets','supersedes','dependsOn'));
END $$;
