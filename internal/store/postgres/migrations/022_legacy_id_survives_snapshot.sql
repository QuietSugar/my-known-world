-- 022: 让 legacy_id 在快照重写后存活。
--
-- 快照保存是 DELETE + INSERT 整体替换，应用只写它认识的列，因此 021 写入的
-- legacy_id 会被清空。这里把回填逻辑并进 reuse_node_uuid 触发器（它已经负责
-- 按 uuid 从注册表取回标识），避免两个 BEFORE INSERT 触发器之间看不到对方
-- 的赋值。
--
-- 注意：本文件不得包含 BEGIN/COMMIT，Migrate 已在外层开启事务。

CREATE TABLE IF NOT EXISTS node_legacy_registry (
    id   text PRIMARY KEY,
    uuid uuid NOT NULL
);

CREATE OR REPLACE FUNCTION reuse_node_uuid() RETURNS trigger AS $$
DECLARE existing uuid;
DECLARE legacy text;
BEGIN
  SELECT uuid INTO existing FROM node_uuid_registry WHERE id = NEW.id;
  IF existing IS NOT NULL THEN
    NEW.uuid := existing;
  ELSIF NEW.uuid IS NULL THEN
    NEW.uuid := gen_random_uuid();
  END IF;
  INSERT INTO node_uuid_registry (id, uuid) VALUES (NEW.id, NEW.uuid)
    ON CONFLICT (id) DO UPDATE SET uuid = node_uuid_registry.uuid;

  -- legacy_id：保留调用方给出的值，否则按 uuid 从注册表反查。
  IF NEW.legacy_id IS NOT NULL THEN
    INSERT INTO node_legacy_registry (id, uuid) VALUES (NEW.legacy_id, NEW.uuid)
      ON CONFLICT (id) DO UPDATE SET uuid = EXCLUDED.uuid;
  ELSE
    SELECT id INTO legacy FROM node_legacy_registry WHERE uuid = NEW.uuid;
    IF legacy IS NOT NULL AND legacy <> NEW.uuid::text THEN
      NEW.legacy_id := legacy;
    END IF;
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS nodes_restore_legacy_id ON nodes;
