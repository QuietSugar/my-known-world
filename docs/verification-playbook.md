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

集成测试目前全部通过。历史上长期失败的撤销与乐观锁四项已解决——根因是快照
读写指纹不一致（读路径多填 uuid 字段、写路径未做微秒时间规范化），撤销因此恒
不可用。撤销功能随后按设计下线，见下节。

## 一·五、撤销已下线，改用备份 + 逻辑删除

撤销原先的实现是「每次写存两份整图快照」，快照体积随图规模线性增长，
历史表迅速膨胀，且一次改 IP 就要重写整张图。所以移除：

- 数据库不再存操作快照，`graph_mutation_operations` 只留 `action`/`recorded_at` 作审计
- API 与前端不再提供撤销入口
- **危险操作前跑 `scripts/backup-db.sh <标签>`**，它会在成功后校验表数量，坏备份直接报错
- 业务删除改为逻辑删除：`DeleteNode` 现在设 `lifecycle=retired`，节点与关系保留，
  身份不变。物理清除仍是 `PurgeNode`，但不经 API/MCP 暴露
- MCP 只留 `graph_retire_node`，`graph_delete_node` 已合并进去

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

清理注册表后应**重启应用**（`docker compose up -d app`）再取一次指纹——两次都相同才算安全。

> `sync-wiki.sh` 已于 2026-11-26 删除并禁止恢复：它的实现是全量 DELETE + INSERT，
> 误跑会覆盖库中所有直接编辑。数据只在库中维护，见 hub 的 `AGENTS.md`。

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

`scripts/verify-migration.py` 以基线为准，找出从 hub 原文消失的事实，并**逐条查库确认**：

```bash
python3 scripts/verify-migration.py compare
```

输出分两类：`已确认在库中可查` 与 `未能在库中确认`。后者返回非零，需要人工判断
（可能确实丢失，也可能是不该入库的叙述性内容，如历史日期、内网地址）。

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

## 七之二、验证写入时的坑

回读验证是写完数据后唯一的确认手段，但它本身会产生假象。以下是实际踩过的。

### 写入后必须等缓存失效

服务端有快照缓存（`internal/api/cache.go`）。**2026-11-26 之前，`applyManualMutation`
写入成功后没有调用 `invalidateCache`**，导致写完立刻回读会拿到写入前的数据。

现象：apply 返回 200，回读却发现字段没变。当时误判为「`go run` 用了缓存二进制」，
重装重跑了几轮，真实原因是服务端缓存没失效。

已修复（`createNode` / `updateNode` 同样缺这一步，一并补上），并有
`TestApplyManualMutationInvalidatesCache` 守住。**若再次遇到「写完读不到」，先查
`invalidateCache`，不要先怀疑构建缓存。**

### 本地库结构可能落后于生产

本地开发库若曾用旧代码建过，`schema_migrations` 会记着旧版本号。而 `Migrate` 依据
版本号跳过已应用的迁移——旧结构的表不会因为代码更新而自动修正。

2026-11-26 的表现是 apply 报 `null value in column "before_state"`：撤销功能下线时
删除了该列，本地库仍带着它且 NOT NULL。

核对方式：

```bash
# 本地与生产的版本、关键表结构应当一致
docker exec hall-mkw-test-db psql -U myknownworld -d myknownworld -tAc \
  "SELECT string_agg(version::text, ',' ORDER BY version) FROM schema_migrations"
ssh svc "docker exec -i hall-pgsql psql -U myknownworld -d myknownworld -tAc \
  \"SELECT string_agg(version::text, ',' ORDER BY version) FROM schema_migrations\""
```

### 测试要能反向验证

写完测试后把修复去掉，确认它会失败。否则测的可能是「它本来就能过」。

本轮两个测试都做过这一步：`TestPatchMergesNestedDetails`（去掉合并 → 字段丢失）、
`TestApplyManualMutationInvalidatesCache`（去掉失效 → 读到旧值）。

### 两条写入路径都要测

Web 走 `graph.ApplyManualMutation`，MCP 走 `store.PatchNode`。2026-11-26 只修了后者
就以为完成，实际 Web 路径仍会丢字段。**改了写入逻辑，两条都要验证。**

## 八、待办：校验体系需要收敛（2026-11-26 记录）

现状能用，但**不优雅**。分散在两个仓库、四个脚本，没有统一入口，也存在覆盖盲区。
以下是待解决的问题，不是已完成的检查项。

### 现状

| 位置 | 资产 | 看 hub | 看库 |
|---|---|---|---|
| mkw `internal/**` | Go 测试 | ✗ | ✗ |
| hub `tooling/check-docs.py` | 链接与结构检查 | ✓ | ✗ |
| mkw `scripts/parity-report.py` | frontmatter 逐值比对 | ✓ | ✓ |
| mkw `scripts/verify-migration.py` | 消失事实查库 | ✓ | ✓ |
| mkw `scripts/body-disposition.py` | 正文化文去向判定 | ✓ | ✗ |
| mkw `scripts/find-missing-facts.py` | 正文 → frontmatter 缺口 | ✓ | ✗ |

### 问题

1. **没有统一入口**。验证一次完整一致性要跨两个仓库跑四条命令，顺序还有讲究
   （先同步、再比对）。没有 CI 串联，靠人记得。

2. **hub 侧完全不认识库**。`check-docs.py` 能验证链接指向的文件存在，但无法回答
   「我引用的这个 UUID 在库里真的存在吗」。这类检查只能由 mkw 侧补。

3. **只有单向覆盖**。四个 Python 脚本都是「以 hub 为起点去库里找」。
   没有反向检查「库里每个节点是否被 hub 覆盖」——库中存在大量来自旧 CMDB 迁移、
   不经 hall-wiki 导入的节点，它们没有验证。数量需现查，不写死。

4. **职责重叠**。`parity-report`（frontmatter）、`verify-migration`（原文事实）、
   `find-missing-facts`（正文明细）三者都在回答「这条事实入库了吗」，
   只是粒度不同，容易让人不知道该跑哪个。

5. **口径不一**。`verify-migration` 和 `parity-report` 对「已入库」的判定方式不同：
   前者做字符串包含，后者做逐值相等。同一个事实可能一个说通过、一个说不通过。

### 各脚本的方法边界（重要，不要过度解读结果）

- **`parity-report.py` 只读 frontmatter**。正文提到、但 frontmatter 没写的字段，
  它查不出来。所以「零缺口」证明的是「frontmatter 已入库」，**不能推出
  「正文剥离无损」**。2026-11-26 漏掉的 3 处内容（gitea 备份路径、
  tokentracker CLI 路径、hall-ops 部署联动表）都是这么漏的。
- **`verify-migration.py compare` 基于文本基线**。它会把「UUID 化」「跨档案搬迁」
  和「真实丢失」混在一起输出，必须逐条判断落点，不能只看条数。
- **`check-docs.py` 对 UUID 链接只做格式校验**。打错一个字符它查不出来；
  UUID 是否真实存在于图谱，要另用 `graph_entity` 核验。

### 结论（2026-11-26 决定：暂不实施）

评估后认为这两项的投入产出不划算，暂不做：

- **CI 一致性检查**：diff 本身是一行命令，但现役 `wiki-source.yaml` 在
  `/home/dev/deploy/`，不在仓库内，CI 看不到。要做需先决定部署配置放哪，
  属于设计问题而非工程问题。且目前单人维护，该检查防的是「改完忘同步」，
  收益有限。
- **统一入口**：包一层 `--all` 半小时可做，但**解决不了口径问题**。
  `parity-report` 说零缺口、`verify-migration` 说一批未确认，两者都对但结论相反
  （一个查 frontmatter 值，一个查原文消失项）。
  统一入口只是把混乱收进一个文件。

更值得做的「删除前必须证明在别处存在」的机制，本质是语义判断——要区分
「这段是重复」还是「这段是独有」，脚本做不了。目前靠人工审查 +
`verify-migration` 的消失清单兜底，实际已验证能抓到问题（见 issue #1）。

**重新评估的时机**：有第二个人维护，或校验误报开始实际造成损失时。

### 建议方向（未实施）

- 合并成一个入口 `scripts/check.py`，子命令 `parity` / `facts` / `body` / `links`，
  或直接一条 `--all` 跑全套并按严重度汇总。
- `check.py --all` 退出码作为唯一判据，接进 CI。
- 补反向覆盖检查：库里存在但不来自 hub-wiki 的节点单独列出，标记来源与可信度。
- 统一「已入库」判定为一个函数，三个脚本共用，消除口径差异。
- hub 的 `check-docs.py` 保留纯 hub 自检职责，不做跨库检查——跨库归 mkw。

### 在此之前

按现有方式运行，命令见第一节至第七节。「未确认」条目的分类构成需现跑
`verify-migration.py compare` 查看，其中多数是路径转 UUID 的合法结果，其余为历史日期、
WireGuard 内网地址等本就不入库的事实。
