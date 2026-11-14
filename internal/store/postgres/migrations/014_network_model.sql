DO $$ BEGIN
 ALTER TABLE nodes DROP CONSTRAINT IF EXISTS nodes_type_check;
 ALTER TABLE nodes ADD CONSTRAINT nodes_type_check CHECK (type IN ('Self','Hardware','Machine','SoftwareProject','SoftwareInstance','RunningPeriod','AccountCredential','Product','Document','Runbook','WorkItem','Network','ProvisionalObject'));
END $$;
CREATE TABLE IF NOT EXISTS network_details (
 node_id text PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
 details jsonb NOT NULL CHECK (jsonb_typeof(details)='object')
);
