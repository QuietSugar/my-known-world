-- 020: 为所有实体附加不可变 UUID 标识。
--
-- 目的：每个节点、关系、断言、密码都持有一个永不改变的 UUID；
-- 名称与路径只用于展示与导入映射，不参与对外关联。
--
-- 与既有 migration 兼容：不改任何现有列名，只新增 uuid 列与配套对象，
-- 因此可以在已有库上重复执行。
--
-- UUID 的稳定性由注册表 + BEFORE INSERT 触发器保证：快照保存会先 DELETE
-- 再 INSERT，触发器按 id 从注册表取回原 UUID，避免标识漂移。

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS uuid uuid DEFAULT gen_random_uuid();
UPDATE nodes SET uuid = gen_random_uuid() WHERE uuid IS NULL;
ALTER TABLE nodes ALTER COLUMN uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS nodes_uuid_key ON nodes (uuid);

ALTER TABLE relations ADD COLUMN IF NOT EXISTS uuid uuid DEFAULT gen_random_uuid();
UPDATE relations SET uuid = gen_random_uuid() WHERE uuid IS NULL;
ALTER TABLE relations ALTER COLUMN uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS relations_uuid_key ON relations (uuid);

ALTER TABLE field_assertions ADD COLUMN IF NOT EXISTS uuid uuid DEFAULT gen_random_uuid();
UPDATE field_assertions SET uuid = gen_random_uuid() WHERE uuid IS NULL;
ALTER TABLE field_assertions ALTER COLUMN uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS field_assertions_uuid_key ON field_assertions (uuid);

-- cmdb_local_secrets 由应用在运行时创建（postgres_secret_provider.EnsureSchema），
-- 不在这条 migration 链里；这里先建表，使全新部署也能跑通 migration。
CREATE TABLE IF NOT EXISTS cmdb_local_secrets (
    ref        text PRIMARY KEY,
    opaque_id  text NOT NULL,
    ciphertext bytea NOT NULL,
    nonce      bytea NOT NULL,
    metadata   jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE cmdb_local_secrets ADD COLUMN IF NOT EXISTS uuid uuid DEFAULT gen_random_uuid();
UPDATE cmdb_local_secrets SET uuid = gen_random_uuid() WHERE uuid IS NULL;
ALTER TABLE cmdb_local_secrets ALTER COLUMN uuid SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS cmdb_local_secrets_uuid_key ON cmdb_local_secrets (uuid);

-- 关系与断言的 UUID 视图列（node_uuid/from_uuid/to_uuid）用于按 UUID 关联查询；
-- 保持可空，由触发器在插入时按旧 id 解析填充，不影响既有写入路径。
ALTER TABLE relations ADD COLUMN IF NOT EXISTS from_uuid uuid;
ALTER TABLE relations ADD COLUMN IF NOT EXISTS to_uuid uuid;
ALTER TABLE field_assertions ADD COLUMN IF NOT EXISTS node_uuid uuid;
ALTER TABLE graph_change_history ADD COLUMN IF NOT EXISTS entity_uuid uuid;

CREATE INDEX IF NOT EXISTS relations_from_uuid_idx ON relations (from_uuid);
CREATE INDEX IF NOT EXISTS relations_to_uuid_idx ON relations (to_uuid);
CREATE INDEX IF NOT EXISTS assertions_node_uuid_idx ON field_assertions (node_uuid);

-- 注册表：记住 id → uuid，使标识在快照重写后保持不变。
CREATE TABLE IF NOT EXISTS node_uuid_registry (
    id   text PRIMARY KEY,
    uuid uuid NOT NULL
);
CREATE TABLE IF NOT EXISTS relation_uuid_registry (
    id   text PRIMARY KEY,
    uuid uuid NOT NULL
);
CREATE TABLE IF NOT EXISTS assertion_uuid_registry (
    id   text PRIMARY KEY,
    uuid uuid NOT NULL
);

INSERT INTO node_uuid_registry (id, uuid)
SELECT id, uuid FROM nodes
ON CONFLICT (id) DO NOTHING;
INSERT INTO relation_uuid_registry (id, uuid)
SELECT id, uuid FROM relations
ON CONFLICT (id) DO NOTHING;
INSERT INTO assertion_uuid_registry (id, uuid)
SELECT id, uuid FROM field_assertions
ON CONFLICT (id) DO NOTHING;

CREATE OR REPLACE FUNCTION reuse_node_uuid() RETURNS trigger AS $$
DECLARE existing uuid;
BEGIN
  SELECT uuid INTO existing FROM node_uuid_registry WHERE id = NEW.id;
  IF existing IS NOT NULL THEN
    NEW.uuid := existing;
  ELSIF NEW.uuid IS NULL THEN
    NEW.uuid := gen_random_uuid();
  END IF;
  INSERT INTO node_uuid_registry (id, uuid) VALUES (NEW.id, NEW.uuid)
    ON CONFLICT (id) DO UPDATE SET uuid = node_uuid_registry.uuid;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reuse_relation_uuid() RETURNS trigger AS $$
DECLARE existing uuid;
BEGIN
  SELECT uuid INTO existing FROM relation_uuid_registry WHERE id = NEW.id;
  IF existing IS NOT NULL THEN
    NEW.uuid := existing;
  ELSIF NEW.uuid IS NULL THEN
    NEW.uuid := gen_random_uuid();
  END IF;
  INSERT INTO relation_uuid_registry (id, uuid) VALUES (NEW.id, NEW.uuid)
    ON CONFLICT (id) DO UPDATE SET uuid = relation_uuid_registry.uuid;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reuse_assertion_uuid() RETURNS trigger AS $$
DECLARE existing uuid;
BEGIN
  SELECT uuid INTO existing FROM assertion_uuid_registry WHERE id = NEW.id;
  IF existing IS NOT NULL THEN
    NEW.uuid := existing;
  ELSIF NEW.uuid IS NULL THEN
    NEW.uuid := gen_random_uuid();
  END IF;
  INSERT INTO assertion_uuid_registry (id, uuid) VALUES (NEW.id, NEW.uuid)
    ON CONFLICT (id) DO UPDATE SET uuid = assertion_uuid_registry.uuid;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fill_relation_uuid() RETURNS trigger AS $$
BEGIN
  IF NEW.from_uuid IS NULL THEN SELECT uuid INTO NEW.from_uuid FROM nodes WHERE id = NEW.from_id; END IF;
  IF NEW.to_uuid IS NULL THEN SELECT uuid INTO NEW.to_uuid FROM nodes WHERE id = NEW.to_id; END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fill_assertion_uuid() RETURNS trigger AS $$
BEGIN
  IF NEW.node_uuid IS NULL THEN SELECT uuid INTO NEW.node_uuid FROM nodes WHERE id = NEW.node_id; END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION fill_detail_node_uuid() RETURNS trigger AS $$
BEGIN
  IF NEW.node_uuid IS NULL THEN SELECT uuid INTO NEW.node_uuid FROM nodes WHERE id = NEW.node_id; END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS nodes_reuse_uuid ON nodes;
CREATE TRIGGER nodes_reuse_uuid BEFORE INSERT ON nodes
FOR EACH ROW EXECUTE FUNCTION reuse_node_uuid();

DROP TRIGGER IF EXISTS relations_reuse_uuid ON relations;
CREATE TRIGGER relations_reuse_uuid BEFORE INSERT ON relations
FOR EACH ROW EXECUTE FUNCTION reuse_relation_uuid();

DROP TRIGGER IF EXISTS assertions_reuse_uuid ON field_assertions;
CREATE TRIGGER assertions_reuse_uuid BEFORE INSERT ON field_assertions
FOR EACH ROW EXECUTE FUNCTION reuse_assertion_uuid();

DROP TRIGGER IF EXISTS relations_fill_uuid ON relations;
CREATE TRIGGER relations_fill_uuid BEFORE INSERT ON relations
FOR EACH ROW EXECUTE FUNCTION fill_relation_uuid();

DROP TRIGGER IF EXISTS assertions_fill_uuid ON field_assertions;
CREATE TRIGGER assertions_fill_uuid BEFORE INSERT ON field_assertions
FOR EACH ROW EXECUTE FUNCTION fill_assertion_uuid();

DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['account_credential_details','document_details','hardware_details',
                           'machine_details','network_details','product_details','runbook_details',
                           'running_periods','software_instance_details','software_project_details',
                           'work_item_details'] LOOP
    EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS node_uuid uuid', t);
    EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', t || '_fill_uuid', t);
    EXECUTE format('CREATE TRIGGER %I BEFORE INSERT ON %I FOR EACH ROW EXECUTE FUNCTION fill_detail_node_uuid()', t || '_fill_uuid', t);
  END LOOP;
END $$;
