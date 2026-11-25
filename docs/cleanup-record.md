# 冗余与兼容数据清理记录

> 2026-11-26。清理前备份：`~/backups/my-known-world/pre-cleanup-20261126-003429.dump`（29 张表，已验证可读）。

## 已清理

### A. 注册表失效行（594 条）

UUID 稳定性依赖注册表「按 id 记住 uuid」。同步是 DELETE + INSERT 整体替换，每次都会在注册表留下已不存在实体的行，长期累积成冗余。

| 表 | 清理 | 保留 | 判定 |
|---|---|---|---|
| `node_uuid_registry` | 125 | 125 | `id` 在 `nodes` 中已不存在 |
| `relation_uuid_registry` | 122 | 268 | `id` 在 `relations` 中已不存在 |
| `assertion_uuid_registry` | 274 | 318 | `id` 在 `field_assertions` 中已不存在 |
| `node_legacy_registry` | 73 | 52 | `id = uuid::text` 的早期自映射空行 |

清理后四张表与实际数据行数完全一致（125 / 268 / 318 / 52）。

**安全性验证**：清理前后全库节点 ID 指纹均为 `4b9accb501dedf0c53b2d28a338beed6`；
再执行一次同步后指纹不变，证明 UUID 稳定性未受影响。

### B. 重复密码记录（19 条）

`cmdb_local_secrets` 中存在 `vaultwarden:xasc/*` 与 `credentials/xasc/*` 两套前缀的记录，
是 Bitwarden 迁移期留下的镜像副本，内容完全相同。

删除前确认了降级查找逻辑（`postgres_secret_provider.go:120`）会把两种前缀互为候选键，
因此删除副本不会影响任何按旧前缀发起的读取。

**安全性验证**：删除后分别用 `vaultwarden:xasc/...` 与 `credentials/xasc/...` 前缀读取，
均能取到密码；全部 22 条凭据逐一实测可读，零失败。

## 未清理（有意保留）

### C. 历史记录中 `entity_uuid` 为空的 4949 条

对应实体已被删除，无 UUID 可回填。保留空值以避免历史失真；该事实已写入列注释。

### D. `~/backups/` 与回收站

按用户规则由用户决定，agent 不执行清理。

### E. 代码兼容层

- `POST /api/credentials/secret` 的 Update 分支：仍在用（存在则轮换，避免产生重复记录）
- `go vet` 仅报 `bitwarden/crypto.go` 的 go 版本告警，与本次改动无关
- 无残留编译产物、无 `.bak` 文件、无未使用的 `sqlDB` 变量

## 清理后状态

| 项 | 值 |
|---|---|
| nodes / relations / assertions | 125 / 268 / 318 |
| secrets | 39 → 22 |
| 悬空引用、重复关系 | 0 / 0 |
| 历史记录 | 8762 条（全部保留） |
| UUID 指纹 | 未变 |
| 服务 | 健康 |
| hub 事实完整性 | 校验通过 |
