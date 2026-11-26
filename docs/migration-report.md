# 搬迁完成报告

> 2026-11-26。权威数据源是 my-known-world；hub 是投影层。

## 一、结构化事实：逐值校验通过

```
逐值比对 370 个 frontmatter 事实
  字段缺失 : 0
  值不一致 : 0
  未覆盖键 : 0
```

`scripts/parity-report.py` 从 `internal/graph/models.go` 的 JSON tag 反推可接受字段
（不靠手写清单），逐字段比对 hub frontmatter 与库中实际值。只容忍等价形式：标量 vs
单元素列表、布尔文本 vs 布尔、端口字符串 vs 整数。

## 二、正文：全部 58 个文件已处置

正文是叙述性内容（排错、历史、决策、命令），库里没有对应字段。每个文件的处置结果：

- **已声明以库为准**：结构化事实改为节点 UUID 引用，正文只留库外知识
- **叙述保留**：通篇是过程性知识，无需引用改写

```
合计 58 个文件，正文 70227 字
   41  已声明以库为准
   17  叙述保留
```

明细：

| 文件 | 正文行 | 字数 | 叙述特征 | 重复字段 | 判定 |
|---|---|---|---|---|---|
| services/instances/svc/tokentracker/README.md | 47 | 5003 | 排错/历史/操作 | - | 已声明以库为准 |
| services/instances/svc/nexus3/README.md | 66 | 4386 | 排错/历史/操作 | - | 已声明以库为准 |
| services/instances/vm247/vaultwarden.md | 62 | 4370 | 排错/历史/决策/操作 | - | 已声明以库为准 |
| services/instances/vm247/vaultwarden-backup.md | 94 | 3457 | 排错/历史/操作 | - | 叙述保留 |
| machine/hp/vm128.md | 34 | 3245 | 排错/历史 | - | 已声明以库为准 |
| services/instances/vm128/mihomo.md | 36 | 2720 | 历史/操作 | - | 已声明以库为准 |
| machine/pve/vm247.md | 33 | 2482 | 排错/历史 | - | 已声明以库为准 |
| services/instances/svc/my-known-world/README.md | 35 | 2472 | 排错/历史/操作 | - | 已声明以库为准 |
| machine/mint.md | 35 | 2297 | 排错/历史/操作 | - | 已声明以库为准 |
| machine/hp/svc.md | 25 | 2200 | 历史/操作 | - | 已声明以库为准 |
| machine/openwrt.md | 31 | 2149 | 历史 | ip | 已声明以库为准 |
| machine/pve.md | 22 | 1768 | 排错/历史/操作 | - | 已声明以库为准 |
| services/instances/svc/beszel/README.md | 20 | 1755 | 历史 | - | 叙述保留 |
| services/instances/svc/miraihub/README.md | 18 | 1749 | 历史/操作 | - | 叙述保留 |
| hardware/hp.md | 7 | 1555 | 排错/历史/操作 | - | 叙述保留 |
| machine/vps-aliyun.md | 32 | 1489 | 排错/历史/操作 | - | 已声明以库为准 |
| machine/hp/vm121.md | 16 | 1444 | 排错/历史 | - | 已声明以库为准 |
| services/instances/svc/home-route/README.md | 11 | 1277 | 历史/操作 | - | 叙述保留 |
| services/instances/svc/postgresql/README.md | 14 | 1276 | 历史 | - | 叙述保留 |
| services/instances/vm128/nginx.md | 8 | 1116 | 排错/历史 | - | 叙述保留 |
| services/instances/svc/reservoir-lite/README.md | 16 | 1085 | 历史/操作 | - | 叙述保留 |
| machine/hp/istoreos.md | 14 | 1011 | 历史 | - | 叙述保留 |
| services/instances/svc/nginx.md | 9 | 980 | 操作 | - | 已声明以库为准 |
| services/instances/svc/wireguard/README.md | 10 | 884 | 历史 | - | 叙述保留 |
| hardware/deskmini.md | 21 | 867 | 排错/历史 | - | 叙述保留 |
| machine/hp/vm120.md | 16 | 829 | 操作 | - | 已声明以库为准 |
| services/instances/vm247/nexus3.md | 12 | 829 | 排错/历史 | - | 已声明以库为准 |
| services/instances/svc/gitea/README.md | 13 | 804 | 历史 | - | 已声明以库为准 |
| services/instances/vm128/samba.md | 12 | 781 | 排错/历史 | - | 已声明以库为准 |
| services/instances/svc/mihomo/README.md | 8 | 745 | 历史 | - | 叙述保留 |
| services/instances/mint/dsh.md | 10 | 729 | 排错/历史 | - | 已声明以库为准 |
| services/instances/vm128/hall-mcp.md | 13 | 722 | 历史 | - | 已声明以库为准 |
| services/instances/svc/doc-service/README.md | 13 | 692 | 历史 | - | 已声明以库为准 |
| machine/hp/vm131.md | 7 | 666 | 历史 | - | 叙述保留 |
| services/instances/vm247/nginx.md | 8 | 656 | 排错/历史 | - | 叙述保留 |
| services/instances/svc/portainer/README.md | 12 | 651 | 历史 | - | 已声明以库为准 |
| machine/h618.md | 8 | 602 | 历史 | - | 已声明以库为准 |
| machine/hallmini.md | 11 | 544 | 历史 | - | 已声明以库为准 |
| services/instances/svc/it-tools.md | 8 | 543 | 决策 | - | 已声明以库为准 |
| services/instances/svc/homer.md | 11 | 486 | - | - | 已声明以库为准 |
| services/instances/vm128/repo-cache.md | 10 | 481 | 历史 | - | 叙述保留 |
| services/instances/vm247/gitea.md | 9 | 437 | - | - | 已声明以库为准 |
| services/instances/vm247/homer.md | 8 | 435 | 历史 | - | 已声明以库为准 |
| hardware/eq59.md | 13 | 434 | 历史 | - | 叙述保留 |
| machine/pve/fnos.md | 8 | 434 | 历史/操作 | - | 已声明以库为准 |
| services/instances/vm247/postgresql.md | 7 | 430 | - | - | 已声明以库为准 |
| services/instances/vm128/hall-ops.md | 8 | 411 | 历史 | - | 已声明以库为准 |
| services/instances/vm247/atrust-autologin.md | 9 | 411 | 历史 | - | 已声明以库为准 |
| services/instances/hallmini/syncthing.md | 8 | 408 | - | - | 已声明以库为准 |
| hardware/macmini.md | 8 | 407 | - | - | 已声明以库为准 |
| hardware/h618.md | 9 | 367 | - | - | 已声明以库为准 |
| services/instances/vm128/my-known-world-test/README.md | 7 | 365 | - | - | 已声明以库为准 |
| services/instances/vm128/dsh.md | 6 | 361 | 排错 | - | 已声明以库为准 |
| hardware/cudy.md | 7 | 355 | - | - | 已声明以库为准 |
| services/backup-plan.md | 9 | 347 | 操作 | - | 叙述保留 |
| services/instances/vm128/syncthing.md | 5 | 284 | - | - | 已声明以库为准 |
| services/instances/vps/wg-easy.md | 6 | 280 | - | - | 已声明以库为准 |
| services/instances/openwrt/openclash.md | 7 | 264 | 排错 | - | 已声明以库为准 |



## 三、其他校验

| 项 | 结果 |
|---|---|
| hub 引用 UUID 可解析 | 59 个，全部命中 |
| 关系端点 | 无悬空 |
| 断言引用 | 无悬空 |
| 每个文件自称 UUID 与 identityMappings | 全部一致 |
| 单元测试 / 集成测试 | 通过 |

## 四、可复核方式

```bash
python3 scripts/parity-report.py --strict    # 有缺口返回非零
python3 scripts/body-disposition.py           # 逐文件正文去向
```
