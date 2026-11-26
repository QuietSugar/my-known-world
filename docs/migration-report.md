# 搬迁完成报告

> 2026-11-26。权威数据源是 my-known-world；hub 是投影层。
>
> **本文记录的是处置结论，不记录统计数字。** 数量类信息随每次同步变化，
> 需要时直接跑「四、可复核方式」里的命令，不要引用任何文档里的数字。

## 一、结构化事实：逐值校验通过

frontmatter 的每一条结构化事实都与库中值相等：缺失、不等、未覆盖三类均为零。
范围与当前值请跑 `parity-report.py --strict` 现查（退出码 0 即通过）。

`scripts/parity-report.py` 从 `internal/graph/models.go` 的 JSON tag 反推可接受字段
（不靠手写清单），逐字段比对 hub frontmatter 与库中实际值。只容忍等价形式：标量 vs
单元素列表、布尔文本 vs 布尔、端口字符串 vs 整数。

## 二、正文处置

正文是叙述性内容（排错、历史、决策、命令），库里没有对应字段。每个文件的处置结果：

- **已声明以库为准**：结构化事实改为节点 UUID 引用，正文只留库外知识
- **叙述保留**：通篇是过程性知识，无需引用改写

判定结果会随编辑变化，当前分布跑 `body-disposition.py` 查看。以下是各文件的处置方向：

| 文件 | 叙述特征 | 判定 |
|---|---|---|
| services/instances/svc/tokentracker/README.md | 排错/历史/操作 | 已声明以库为准 |
| services/instances/svc/nexus3/README.md | 排错/历史/操作 | 已声明以库为准 |
| services/instances/vm247/vaultwarden.md | 排错/历史/决策/操作 | 已声明以库为准 |
| services/instances/vm247/vaultwarden-backup.md | 排错/历史/操作 | 叙述保留 |
| machine/hp/vm128.md | 排错/历史 | 已声明以库为准 |
| services/instances/vm128/mihomo.md | 历史/操作 | 已声明以库为准 |
| machine/pve/vm247.md | 排错/历史 | 已声明以库为准 |
| services/instances/svc/my-known-world/README.md | 排错/历史/操作 | 已声明以库为准 |
| machine/mint.md | 排错/历史/操作 | 已声明以库为准 |
| machine/hp/svc.md | 历史/操作 | 已声明以库为准 |
| machine/openwrt.md | 历史 | 已声明以库为准 |
| machine/pve.md | 排错/历史/操作 | 已声明以库为准 |
| services/instances/svc/beszel/README.md | 历史 | 叙述保留 |
| services/instances/svc/miraihub/README.md | 历史/操作 | 叙述保留 |
| hardware/hp.md | 排错/历史/操作 | 叙述保留 |
| machine/vps-aliyun.md | 排错/历史/操作 | 已声明以库为准 |
| machine/hp/vm121.md | 排错/历史 | 已声明以库为准 |
| services/instances/svc/home-route/README.md | 历史/操作 | 叙述保留 |
| services/instances/svc/postgresql/README.md | 历史 | 叙述保留 |
| services/instances/vm128/nginx.md | 排错/历史 | 叙述保留 |
| services/instances/svc/reservoir-lite/README.md | 历史/操作 | 叙述保留 |
| machine/hp/istoreos.md | 历史 | 叙述保留 |
| services/instances/svc/nginx.md | 操作 | 已声明以库为准 |
| services/instances/svc/wireguard/README.md | 历史 | 叙述保留 |
| hardware/deskmini.md | 排错/历史 | 叙述保留 |
| machine/hp/vm120.md | 操作 | 已声明以库为准 |
| services/instances/vm247/nexus3.md | 排错/历史 | 已声明以库为准 |
| services/instances/svc/gitea/README.md | 历史 | 已声明以库为准 |
| services/instances/vm128/samba.md | 排错/历史 | 已声明以库为准 |
| services/instances/svc/mihomo/README.md | 历史 | 叙述保留 |
| services/instances/mint/dsh.md | 排错/历史 | 已声明以库为准 |
| services/instances/vm128/hall-mcp.md | 历史 | 已声明以库为准 |
| services/instances/svc/doc-service/README.md | 历史 | 已声明以库为准 |
| machine/hp/vm131.md | 历史 | 叙述保留 |
| services/instances/vm247/nginx.md | 排错/历史 | 叙述保留 |
| services/instances/svc/portainer/README.md | 历史 | 已声明以库为准 |
| machine/h618.md | 历史 | 已声明以库为准 |
| machine/hallmini.md | 历史 | 已声明以库为准 |
| services/instances/svc/it-tools.md | 决策 | 已声明以库为准 |
| services/instances/svc/homer.md | - | 已声明以库为准 |
| services/instances/vm128/repo-cache.md | 历史 | 叙述保留 |
| services/instances/vm247/gitea.md | - | 已声明以库为准 |
| services/instances/vm247/homer.md | 历史 | 已声明以库为准 |
| hardware/eq59.md | 历史 | 叙述保留 |
| machine/pve/fnos.md | 历史/操作 | 已声明以库为准 |
| services/instances/vm247/postgresql.md | - | 已声明以库为准 |
| services/instances/vm128/hall-ops.md | 历史 | 已声明以库为准 |
| services/instances/vm247/atrust-autologin.md | 历史 | 已声明以库为准 |
| services/instances/hallmini/syncthing.md | - | 已声明以库为准 |
| hardware/macmini.md | - | 已声明以库为准 |
| hardware/h618.md | - | 已声明以库为准 |
| services/instances/vm128/my-known-world-test/README.md | - | 已声明以库为准 |
| services/instances/vm128/dsh.md | 排错 | 已声明以库为准 |
| hardware/cudy.md | - | 已声明以库为准 |
| services/backup-plan.md | 操作 | 叙述保留 |
| services/instances/vm128/syncthing.md | - | 已声明以库为准 |
| services/instances/vps/wg-easy.md | - | 已声明以库为准 |
| services/instances/openwrt/openclash.md | 排错 | 已声明以库为准 |



## 三、其他校验

| 项 | 结果 |
|---|---|
| hub 引用 UUID 可解析 | 全部命中 |
| 关系端点 | 无悬空 |
| 断言引用 | 无悬空 |
| 每个文件自称 UUID 与 identityMappings | 全部一致 |
| 单元测试 / 集成测试 | 通过 |

## 四、可复核方式

以下命令在 my-known-world 仓库根目录执行（`scripts/` 在该仓库，hub 内没有）：
`cd ~/git-repo/github.com/QuietSugar/my-known-world`

```bash
python3 scripts/parity-report.py --strict    # 有缺口返回非零
python3 scripts/body-disposition.py           # 逐文件正文去向
python3 scripts/verify-migration.py compare   # 消失事实是否在库中可查
```

hub 内部的链接与结构检查则在 hall-wiki 目录执行：

```bash
cd <hub>/hall-wiki && python3 tooling/check-docs.py --all
```
