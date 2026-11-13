# 首版实现切片

首版先实现领域核心，再在此基础上增加持久化、API 和 Web UI。此顺序不是对最终信息范围的削减：所有需求中的明确节点类型仍属于目标模型。

## 已落地：领域核心

`internal/graph` 提供明确节点类型、节点与关系的来源、证据类型和确定性元数据、端点类型校验、唯一 Self 和弱连通路径查询。未提供元数据会显式归一为 `unknown` / `unspecified`，不会静默声称已确认。`source` 当前仅为非敏感来源标签，不得填入内部路径、令牌或来源配置。发布节点通过 `Publish` 原子地创建节点并与 Self 建立合法关系；默认关系为 `knowsAbout`。失败时不留下半成品。领域包本身使用进程内图；PostgreSQL adapter 与 schema v2 持久化节点/关系级 provenance。字段级来源断言仍未实现。

## 已落地：只读 HTTP API 与影响分析

标准库 HTTP handler 提供健康检查、节点列表/详情、Self 弱连通路径（节点与关系）及从 Self 出发的影响分析。影响分析只沿明确配置的正向关系遍历，并排除 `knowsAbout`、文档和 Runbook 关系；输出包含关系路径。路径遍历按关系 ID 排序以保证稳定结果。`cmd/myknownworld` 默认启动合成数据 API；设置 `DATABASE_URL` 后会迁移并读取 PostgreSQL 快照，空库默认失败关闭。API 仍无认证、写入接口和分页，不应直接暴露到公网。

## 已落地：PostgreSQL 快照存储与 CI

`internal/store/postgres` 提供显式版本化迁移（当前 schema v2，含 provenance 字段）、完整图快照原子替换与一致性加载校验；保存前要求恰好一个 Self 且全部节点弱连通。迁移可重复执行，数据库约束包含唯一 Self、节点/关系类型及外键。集成测试使用 `POSTGRES_TEST_DSN`，在隔离临时 schema 中验证迁移幂等、快照往返和拒绝断连快照，不会清空调用方数据库中的表。`cmd/myknownworld-migrate` 只应用 schema，不会自动导入演示或真实数据。

GitHub Actions 使用 PostgreSQL 16 服务运行 gofmt 检查、vet 和 race-enabled 全量测试。执行保存快照会整体替换表中图数据，调用方必须提供完整权威快照；它不是增量同步协议。

## 已落地：Web 浏览界面

`web/` 提供 React + TypeScript + Vite 单页应用：图谱概览、类型筛选、对象目录、Self 关系路径详情和影响范围列表。界面通过 Vite 相对路径代理 `/api` 与 `/healthz`，浏览器端不访问 localhost。当前 UI 只读；缺少录入、编辑、暂存审查和完整关系探索能力。GitHub Actions 独立执行 `npm ci` 与生产构建。Docker 多阶段构建会把静态 Web 资源和 Go 服务打进单镜像；Compose 配 PostgreSQL，使用非 root UID、只读根文件系统、drop capabilities、no-new-privileges 和 tmpfs。`scripts/smoke_test.py` 提供无需第三方 Python 包的只读 API 验收。CI 构建后运行实际容器和同一 smoke test，并导出镜像 artifact；主分支/tag 可推送 GHCR。

## 后续切片

1. 为图谱快照补充安全的增量写入/导入事务协议，避免把全量替换误用为同步。
2. 扩展来源目录、字段级断言溯源、历史与删除策略，并为分析结果增加事实/推断语义；当前来源/证据/确定性仅在节点或关系层级记录，尚无来源实体与属性级证据。
3. 实现受审查的暂存区和写入流程，再在 UI 中支持对象录入、关系编辑与来源管理。
4. 设计可重复执行的只读导入预览协议，确保冲突、缺失及排除项有报告；真实来源配置与数据始终留在私有环境。

## 当前边界

当前不接入用户真实来源，不保存秘密、文档正文或实时监控流，也不提供自动化操作能力。已实现 PostgreSQL 快照存储、只读 HTTP API 和只读 UI，但对象编辑、导入、待审查流程与多人认证均未实现。
