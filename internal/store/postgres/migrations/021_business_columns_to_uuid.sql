-- 021: 把业务关联列切换为 UUID。
--
-- 做法是把 id 列的值换成 uuid 文本，旧路径保存到 legacy_id，因此应用代码
-- 无需改动（它把 id 当作不透明字符串）。导入侧配合 identityMappings 把文件
-- 路径映射到同一批 UUID，避免产生重复节点。
--
-- 幂等：重复执行时 legacy_id 已有值、id 已等于 uuid，UPDATE 影响 0 行而校验仍通过。
BEGIN;

-- 0. 记录旧路径，供 identityMappings 生成使用
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS legacy_id text;
UPDATE nodes SET legacy_id = id WHERE legacy_id IS NULL;

-- 1. 节点：id 换成 UUID 文本
UPDATE nodes SET id = uuid::text;
ALTER TABLE nodes ALTER COLUMN id TYPE text;
CREATE UNIQUE INDEX IF NOT EXISTS nodes_legacy_id_key ON nodes (legacy_id);

-- 2. 关系：from_id/to_id 换成对应节点 UUID
UPDATE relations r SET from_id = n.id FROM nodes n WHERE n.uuid::text <> r.from_id AND n.legacy_id = r.from_id;
UPDATE relations r SET to_id   = n.id FROM nodes n WHERE n.uuid::text <> r.to_id   AND n.legacy_id = r.to_id;

-- 3. 断言：node_id 换成节点 UUID
UPDATE field_assertions a SET node_id = n.id FROM nodes n WHERE n.uuid::text <> a.node_id AND n.legacy_id = a.node_id;

-- 4. 各 details 表同样处理
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['account_credential_details','document_details','hardware_details',
                           'machine_details','network_details','product_details','runbook_details',
                           'running_periods','software_instance_details','software_project_details',
                           'work_item_details'] LOOP
    EXECUTE format('UPDATE %I d SET node_id = n.id FROM nodes n WHERE n.uuid::text <> d.node_id AND n.legacy_id = d.node_id', t);
  END LOOP;
END $$;

-- 5. 同步映射表同步更新（迁移后其 node_id 必须指向 UUID）
UPDATE sync_identity_mappings m SET node_id = n.id
  FROM nodes n WHERE n.legacy_id = m.node_id AND m.node_id <> n.id;

-- 6. 校验：不允许存在指向不存在节点的引用
DO $$
DECLARE bad bigint;
BEGIN
  SELECT count(*) INTO bad FROM relations r
   WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = r.from_id)
      OR NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = r.to_id);
  IF bad > 0 THEN RAISE EXCEPTION 'relations with dangling endpoints: %', bad; END IF;

  SELECT count(*) INTO bad FROM field_assertions a
   WHERE NOT EXISTS (SELECT 1 FROM nodes n WHERE n.id = a.node_id);
  IF bad > 0 THEN RAISE EXCEPTION 'assertions with dangling node_id: %', bad; END IF;
END $$;

COMMIT;
