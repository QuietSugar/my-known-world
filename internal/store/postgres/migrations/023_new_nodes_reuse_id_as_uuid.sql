-- 023: 新节点的 uuid 默认取 id，保证 id = uuid。
--
-- 背景：store.SaveSnapshot 的 INSERT 不写 uuid 列，靠 reuse_node_uuid 触发器
-- 生成。020 的实现是「注册表没有就 gen_random_uuid()」，于是全新节点出生即
-- id ≠ uuid，与 021 迁移过的存量节点（id = uuid）规则不一致，外部引用无法
-- 确定该用哪个。
--
-- 关键点：020 给 uuid 列设了 DEFAULT gen_random_uuid()。DEFAULT 在 BEFORE
-- 触发器之前求值，所以触发器看到的 NEW.uuid 永远非 NULL，原来那句
-- 「ELSIF NEW.uuid IS NULL」是一段死代码。这里先去掉 DEFAULT，再让触发器
-- 按 id 赋值，这样逻辑才真正生效。
--
-- 注意：本文件不得包含 BEGIN/COMMIT，Migrate 已在外层开启事务。

ALTER TABLE nodes ALTER COLUMN uuid DROP DEFAULT;

CREATE OR REPLACE FUNCTION reuse_node_uuid() RETURNS trigger AS $$
DECLARE existing uuid;
DECLARE legacy text;
BEGIN
  SELECT uuid INTO existing FROM node_uuid_registry WHERE id = NEW.id;
  IF existing IS NOT NULL THEN
    NEW.uuid := existing;
  ELSIF NEW.uuid IS NULL THEN
    -- id 本身是 UUID 时直接复用，使新节点与存量节点规则一致。
    IF NEW.id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN
      NEW.uuid := NEW.id::uuid;
    ELSE
      NEW.uuid := gen_random_uuid();
    END IF;
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
