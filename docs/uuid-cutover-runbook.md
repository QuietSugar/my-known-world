# 把业务关联列切换为 UUID：上线步骤

> 状态：已演练通过（2026-11-26），待执行上线
> 前置：`020_entity_uuids.sql` 已部署，所有实体已有 `uuid` 列

## 一、为什么这样切

不能只改导入配置——那样 UUID 节点与路径节点并存，实测会让节点从 125 涨到 177。
也不能只改数据库列——应用启动会重放 `001_init.sql`，列名对不上会 502。

**正确做法是同时改两侧**：把 `nodes.id` 的值本身换成 UUID 文本，旧路径移入 `legacy_id`。
这样应用代码零改动（它只把 `id` 当作不透明字符串），导入则靠 `identityMappings`
把文件路径映射到同一批 UUID，因此不会产生重复。

## 二、演练结果（证据）

在 `pg_restore` 出的独立副本上执行 `rehearsal-switch-to-uuid.sql`：

| 校验项 | 结果 |
|---|---|
| 节点 / 关系 / 断言 / 密码 | 125 / 268 / 318 / 41（与迁移前一致） |
| 重复 ID | 0 |
| 关系悬空端点 | 0 |
| 断言悬空节点 | 0 |
| `nodes.id` 等于自身 `uuid` | 125 / 125 |
| `legacy_id` 保留 | 125 |
| 重复执行脚本后 ID 指纹 | 完全相同（幂等） |
| 用生成的 52 条映射重新导入 | 52 / 52 命中已有节点，零新增 |

## 三、上线步骤

1. **备份**（必须，且验证可读）
   ```bash
   ssh svc "docker exec hall-pgsql pg_dump -U myknownworld -d myknownworld --format=custom" \
     > ~/backups/my-known-world/pre-uuid-switch-<日期>.dump
   ssh svc "docker exec -i hall-pgsql pg_restore --list" < <上面的 dump> | grep -c "TABLE DATA"
   ```

2. **停写**：确认无 agent 正在同步（否则并发写入会被旧 ID 引用）。

3. **执行迁移**（单事务，失败自动回滚）
   ```bash
   ssh svc "docker exec -i hall-pgsql psql -U myknownworld -d myknownworld -v ON_ERROR_STOP=1" \
     < ~/backups/my-known-world/rehearsal-switch-to-uuid.sql
   ```

4. **生成并写入 identityMappings**
   ```sql
   SELECT legacy_id || '|' || id FROM nodes WHERE source = 'hall-wiki-private';
   ```
   按 `sourceId: <路径> / nodeId: <UUID>` 写入 `wiki-source.yaml` 的 `identityMappings`，
   跳过已从 wiki 删除的文件（导入器会因映射指向不存在的文件而拒绝运行）。

5. **重启应用并验证**
   ```bash
   curl -fsS -k https://mkw.svc.hp.hall/healthz
   curl -s -k "https://mkw.svc.hp.hall/api/nodes?limit=1"   # id 应为 UUID
   ```

6. **同步一次并确认零重复**
   ```bash
   /home/dev/deploy/my-known-world/sync-wiki.sh
   # 节点数必须仍是切换前的值
   ```

## 四、失败回滚

```bash
ssh svc "docker exec -i hall-pgsql pg_restore -U myknownworld -d myknownworld \
  --clean --if-exists --no-owner --no-privileges" < ~/backups/my-known-world/pre-uuid-switch-<日期>.dump
ssh svc "docker compose ... restart app"
```

同时撤回 `wiki-source.yaml` 中的 `identityMappings`，否则下次同步会造出重复节点。

## 五、仍需处理

- **历史表**：`graph_change_history` 中 4197 条记录指向已删除实体，没有 UUID 可映射。
  这些是历史事实，不伪造 UUID，迁移时保持 `entity_uuid` 为空。
- **`sync_identity_mappings`**：迁移后其 `node_id` 指向旧路径，需要在步骤 4 之后
  一并更新为 UUID，否则同步的映射表与图谱不一致。
