# 实体与凭据 UUID 化迁移方案

> 状态：方案待执行｜2026-11-25
> 目标：所有实体、关系、密码都持有**永不改变**的 UUID；名称只用于展示，可重复，
> 不参与任何关联。

## 一、为什么改

当前节点 ID 是**文件相对路径**（`services/instances/vm128/samba.md`），带来三个问题：

1. 重命名文件 = 换 ID，历史与关系全部断链；
2. 名称无法重复（两个 `nginx` 必须靠路径区分）；
3. 密码密文的 `opaque_id` 与节点 ID 是两套体系，无法稳定互指。

## 二、目标形态

| 对象 | 标识 | 说明 |
|---|---|---|
| 节点 | `uuid`（RFC 4122 v4） | 主键，永不变 |
| 关系 | `uuid` | 主键，永不变 |
| 断言 | `uuid` | 主键，永不变 |
| 密码 | `uuid` | 主键，永不变 |
| 名称 | `name` | 可重复，仅展示 |

关联一律用 UUID：

- 关系：`from_uuid` / `to_uuid`
- 断言：`node_uuid`
- 服务与凭据：`hasCredential` 关系边（`from_uuid`=服务，`to_uuid`=凭据）

## 三、关键约束：导入的稳定性

导入器现在用 `f.rel`（文件路径）当节点 ID。改成 UUID 后必须有稳定映射，
否则每次导入都生成新 UUID，数据指数膨胀。

**解法**：用已有的 `identityMappings`（`sourceId` → `nodeId`）承载
「文件路径 → 节点 UUID」，作为导入配置的一部分固化下来。

```yaml
identityMappings:
  - sourceId: services/instances/vm128/samba.md
    nodeId: 3f9a1c2e-...
```

`sync_identity_mappings` 表已存在，可用于持久化这份映射。

## 四、现状规模（迁移前实测）

| 表 | 行数 |
|---|---|
| nodes | 123 |
| relations | 264 |
| field_assertions | 318 |
| cmdb_local_secrets | 39 |
| graph_change_history | 3882 |
| field_resolutions | 0 |
| sync_identity_mappings | 0 |

## 五、受影响的引用点

| 表 | 列 |
|---|---|
| relations | `from_id`、`to_id` |
| field_assertions | `node_id` |
| 各 `*_details` 表 | `node_id` |
| graph_change_history | `entity_id` |
| cmdb_local_secrets | `ref`（当前等于节点 ID） |
| sync_identity_mappings | `node_id` |

## 六、执行步骤（每步可回滚）

1. **全量备份**
   `pg_dump` 到 `~/backups/my-known-world/uuid-migration-<日期>.dump`，
   `pg_restore --list` 验证可读。
2. **建映射**
   为 123 个节点、264 个关系、318 个断言生成 UUID，落库到
   `uuid_migration_map` 临时表（旧 ID → 新 UUID）。同一事务内完成，失败即回滚。
3. **改主键**
   新增 `uuid` 列并回填，保留旧 ID 列作为 `legacy_id` 一段时间，不立即删除。
4. **改引用**
   按映射批量更新 `relations`、`field_assertions`、各 details 表、
   `graph_change_history`、`cmdb_local_secrets`。
5. **密码 UUID 化**
   为 39 条密码生成独立 UUID；建立 `hasCredential` 关系边连接服务与凭据；
   `cmdb_local_secrets` 改用密码 UUID 作为主键，保留 `legacy_ref`。
6. **导入器改造**
   ID 由 `identityMappings` 决定；缺失映射时拒绝导入而非自动生成，
   避免重复。
7. **校验**
   - 节点/关系/断言数量前后一致
   - 每条关系的两端都能在 `nodes` 找到
   - 每个密码 UUID 都能被某条 `hasCredential` 引用
   - 原有全部字段值逐条比对无变化
8. **兼容期**
   API/MCP 同时接受 UUID 与旧路径 ID，两者都能解析；观察一段时间后再移除旧 ID。

## 七、回滚

任一步失败：
- 数据库：从步骤 1 的 dump 恢复
- 代码：`git revert` 对应提交
- 旧 ID 列在兼容期内一直保留，因此可直接回退到旧查询方式

## 八、风险

| 风险 | 缓解 |
|---|---|
| 3882 条历史记录需重写 | 在单事务内完成，先备份 |
| 导入器生成新 UUID 导致重复 | 缺失映射时拒绝导入 |
| 外部调用方仍用旧 ID | 兼容期双解析 |
| `softwareInstance.domains` 等业务字段误判为 ID | 只改 `*_id` 列，业务字段不动 |

## 九、暂不纳入

- `cmdb_local_secrets` 中 19 条 `vaultwarden:` 前缀的镜像副本：迁移时合并或标记，
  不引入新的 UUID 语义。
- hub 文档中的明文密码：迁入密码库后删除，与 UUID 无关。
