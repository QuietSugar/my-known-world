# 检查与验证手册

> 本文件把「改完数据后怎么证明没坏」固化成可复用的检查项。每次改动
> my-known-world 的数据或 hub 文档后，按下面对应的小节执行并留证。

## 一、自动化测试（首选，可回归）

```bash
cd ~/git-repo/github.com/QuietSugar/my-known-world
go test ./...            # 单元测试，无需数据库
```

需要数据库的集成测试（UUID 稳定性等）必须提供 DSN：

```bash
# 需要一个可建 schema 的库；测试自行 CREATE SCHEMA 并在结束时 DROP
POSTGRES_TEST_DSN="postgres://<user>:<pw>@127.0.0.1:<port>/mkw_integration_test?sslmode=disable" \
  go test ./internal/store/postgres/ -count=1
```

### UUID 相关（`uuid_stability_test.go`）

| 测试 | 守住的契约 |
|---|---|
| `TestUUIDsSurviveSnapshotRewrite` | 快照保存是 DELETE + INSERT，UUID 必须不变 |
| `TestUUIDsAreDistinctAndPersisted` | 每个实体有 UUID 且不重复；关系无悬空端点 |
| `TestLoadedNodesCarryUUID` | 读取路径必须把 UUID 带出来 |

已知的既有失败（与 UUID 改动无关，2026-11-26 用改动前代码复现确认）：
`TestApplyCreateBatchIsAtomicRecordedAndRevisionChecked`、`TestDirectPatchAndDeleteMutations`、
`TestMigrationRepeatKeepsMarkdownRelationPredicates` 等 6 项。

## 二、数据完整性（线上库）

```bash
ssh svc "docker exec hall-pgsql psql -U myknownworld -d myknownworld -tAc \"
SELECT 'nodes', count(*) FROM nodes
UNION ALL SELECT 'relations', count(*) FROM relations
UNION ALL SELECT 'assertions', count(*) FROM field_assertions
UNION ALL SELECT 'secrets', count(*) FROM cmdb_local_secrets
UNION ALL SELECT 'uuid_form_ids', count(*) FROM nodes WHERE id ~ '^[0-9a-f]{8}-'
UNION ALL SELECT 'id_eq_uuid', count(*) FROM nodes WHERE id = uuid::text
UNION ALL SELECT 'dangling_rel', count(*) FROM relations r
  WHERE NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=r.from_id)
     OR NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=r.to_id)
UNION ALL SELECT 'dangling_assert', count(*) FROM field_assertions a
  WHERE NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=a.node_id)
UNION ALL SELECT 'dup_rel', count(*) FROM (
  SELECT from_id,predicate,to_id FROM relations GROUP BY 1,2,3 HAVING count(*)>1) x
\"" 
```

期望：数量与改动前一致；`uuid_form_ids = nodes`；`id_eq_uuid = nodes`；
三个 dangling/dup 计数均为 0。

## 三、UUID 稳定性（人工，改动注册表后必做）

同步会整体重写节点。取改动前后的全库指纹对比：

```bash
ssh svc "docker exec hall-pgsql psql -U myknownworld -d myknownworld -tAc \
  \"SELECT md5(string_agg(id,',' ORDER BY id)) FROM nodes\""
```

清理注册表后必须再跑一次 `sync-wiki.sh`（`/home/dev/deploy/my-known-world/sync-wiki.sh`），
再取一次指纹——两次都相同才算安全。

## 四、密码可读性（改动 `cmdb_local_secrets` 后必做）

```bash
ssh svc "docker exec hall-pgsql psql -U myknownworld -d myknownworld -tAc \
  \"SELECT ref FROM cmdb_local_secrets ORDER BY ref\"" > /tmp/refs.txt
while read -r r; do
  [ -z "$r" ] && continue
  curl -s -k "https://mkw.svc.hp.hall/api/credentials/secret?ref=$r" \
    | python3 -c "import sys,json; print(json.load(sys.stdin).get('password',''))"
done < /tmp/refs.txt
```

期望：每条都能取到非空值。删除重复前缀副本后，用旧前缀（如 `vaultwarden:xasc/...`）
也要能读取——`postgres_secret_provider.go` 会把两种前缀互为候选键。

## 五、hub 文档事实完整性

`scripts/verify-migration.py` 以基线为准，列出从 hub 原文消失的事实：

```bash
python3 scripts/verify-migration.py compare     # 应输出「校验通过」
```

结构发生较大变化、或按设计把路径替换为 UUID 后，重建基线前**必须先做交叉验证**：
把 hub 中引用的 UUID 拿到库里解析，确认全部命中。基线变更原因记入
`~/backups/hub/baseline-change-reason.json`。

```bash
# 交叉验证：hub 里引用的每个 UUID 都必须能在库中解析
grep -rhoE '\`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\`' \
  --include='*.md' <hall-wiki>/ | tr -d '\`' | sort -u
```

## 六、迁移安全性（改 migration 后必做）

两个硬性要求：

1. **migration 不得自带 `BEGIN`/`COMMIT`**——`Migrate` 已在外层开启事务，
   内层事务会报 `pq: unexpected transaction status idle` 并导致 502。
2. **migration 必须幂等**——每次启动都会重放全部 migration。

验证方式：在 `pg_restore` 出的独立副本上连续执行两次，比对节点 ID 指纹。

```bash
ssh svc "docker exec hall-pgsql psql -U postgres -d postgres -c 'CREATE DATABASE mkw_check OWNER myknownworld'"
ssh svc "docker exec -i hall-pgsql pg_restore -U postgres -d mkw_check --no-owner" < <dump>
ssh svc "docker exec -i hall-pgsql psql -U postgres -d mkw_check -v ON_ERROR_STOP=1" < <migration.sql>
# 执行两次，比对 SELECT md5(string_agg(id,',' ORDER BY id)) FROM nodes
```

migration 若引用某张表，必须能自己建表——`cmdb_local_secrets` 由应用运行时创建，
不在 migration 链里，020 已补上 `CREATE TABLE IF NOT EXISTS`。

## 七、备份

```bash
bash scripts/backup-hub.sh <批次标签>          # hub 所有改动文件
ssh svc "docker exec hall-pgsql pg_dump -U myknownworld -d myknownworld \
  --format=custom --no-owner" > ~/backups/my-known-world/<名称>.dump
# 验证可读
ssh svc "docker exec -i hall-pgsql pg_restore --list" < <dump> | grep -c "TABLE DATA"
```
