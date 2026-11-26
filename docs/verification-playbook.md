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
| `TestIdentitySurvivesSnapshotRewrite` | 快照保存是 DELETE + INSERT，身份必须不变 |
| `TestUUIDColumnIsGone` | 冗余 uuid 列与注册表不得复活（身份只在 id 一列） |
| `TestLoadedNodesCarryUUID` | 读取路径必须把标识带出来 |

身份模型（2026-11-26 起）：`nodes.id` / `relations.id` / `field_assertions.id` 是唯一
标识，曾经并存的 `uuid` 列及其 `node_uuid` / `from_uuid` / `to_uuid` 副本列已全部删除。
仅保留 `cmdb_local_secrets.uuid`（密码条目身份）与 `graph_change_history.entity_uuid`
（审计快照）。数据库不再自动生成标识，由写入方提供。

已知的既有失败（与身份改动无关）：`TestApplyCreateBatchIsAtomicRecordedAndRevisionChecked`、
`TestDirectPatchAndDeleteMutations`、`TestManualOperationsCanOnlyUndoInSafeReverseOrder`、
`TestSyncBatchUndoRestoresGraphSourceStateAndIdentityMappings` 共 4 项，都是撤销/乐观锁
语义问题。

## 二、数据完整性（线上库）

```bash
ssh svc "docker exec hall-pgsql psql -U myknownworld -d myknownworld -tAc \"
SELECT 'nodes', count(*) FROM nodes
UNION ALL SELECT 'relations', count(*) FROM relations
UNION ALL SELECT 'assertions', count(*) FROM field_assertions
UNION ALL SELECT 'secrets', count(*) FROM cmdb_local_secrets
UNION ALL SELECT 'uuid_form_ids', count(*) FROM nodes WHERE id ~ '^[0-9a-f]{8}-'
UNION ALL SELECT 'dangling_rel', count(*) FROM relations r
  WHERE NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=r.from_id)
     OR NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=r.to_id)
UNION ALL SELECT 'dangling_assert', count(*) FROM field_assertions a
  WHERE NOT EXISTS(SELECT 1 FROM nodes n WHERE n.id=a.node_id)
UNION ALL SELECT 'dup_rel', count(*) FROM (
  SELECT from_id,predicate,to_id FROM relations GROUP BY 1,2,3 HAVING count(*)>1) x
\"" 
```

期望：数量与改动前一致；`uuid_form_ids = nodes`（全部为 UUID 形态）；
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

现在只有**一个**结构迁移文件 `migrations/001_schema.sql`，它一次建成完整结构。
2026-11-26 之前的 24 步增量迁移链已删除——只有开发环境和 svc 在用，两者都已是
最终结构，保留历史步骤反而出过故障：`Migrate` 曾无条件重放全部文件，而 021 会
`UPDATE nodes SET id = uuid::text`，uuid 列删除后这句直接报错，容器进入重启循环。

三个硬性要求：

1. **migration 不得自带 `BEGIN`/`COMMIT`**——`Migrate` 已在外层开启事务，
   内层事务会报 `pq: unexpected transaction status idle` 并导致 502。
2. **不要写死 `public.` 前缀**——集成测试用独立 schema（`search_path`）隔离，
   写死公共 schema 会让所有测试抢同一套表。
3. **`schema_migrations` 由 `Migrate` 自己建**，不要放进结构文件——版本要先能读，
   才能判断该不该建表，否则新库第一次启动就卡在鸡生蛋问题上。

`Migrate` 依据 `schema_migrations` 跳过已应用版本，所以**改已发布的 001 不会重跑**。
结构需要变更时，加 `002_xxx.sql` 并把 `schemaVersion` 改成 2。

验证方式：在 `pg_restore` 出的独立副本上跑一次空库迁移，比对节点数与结构。

```bash
# 空库全新建结构
ssh svc "docker exec hall-pgsql psql -U postgres -d postgres -c 'CREATE DATABASE mkw_check OWNER myknownworld'"
# 用 cmd 里的迁移入口或直接启动应用，随后核对
ssh svc "docker exec -i hall-pgsql psql -U myknownworld -d mkw_check -tAc \
  \"SELECT count(*) FROM pg_tables WHERE schemaname='public'\"   # 期望 25
ssh svc "docker exec -i hall-pgsql psql -U myknownworld -d mkw_check -tAc \
  \"SELECT sum(version) FROM schema_migrations\"               # 期望 1
```

## 七、备份

```bash
bash scripts/backup-hub.sh <批次标签>          # hub 所有改动文件
ssh svc "docker exec hall-pgsql pg_dump -U myknownworld -d myknownworld \
  --format=custom --no-owner" > ~/backups/my-known-world/<名称>.dump
# 验证可读
ssh svc "docker exec -i hall-pgsql pg_restore --list" < <dump> | grep -c "TABLE DATA"
```
