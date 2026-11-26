# 搬迁完成报告

> 2026-11-26。权威数据源是 my-known-world；hub 是投影层。

## 一、结构化事实：逐值校验通过

```
逐值比对 360 个 frontmatter 事实
  字段缺失 : 0
  值不一致 : 0
  未覆盖键 : 0
```

校验方式：`scripts/parity-report.py` 从 `internal/graph/models.go` 的 JSON tag
反推可接受字段（不靠手写清单），逐字段比对 hub frontmatter 的值与库中实际值。
允许的等价形式仅限：标量 vs 单元素列表、布尔文本 vs 布尔、端口字符串 vs 整数。

## 二、其他校验

| 项 | 结果 |
|---|---|
| hub 引用 UUID 可解析 | 全部命中，无悬空 |
| 关系端点 | 无悬空（`dangling=0`） |
| 断言引用 | 无悬空 |
| 节点指纹 | 同步前后一致 `8ca30e67...` |
| 单元测试 | 通过 |
| 集成测试 | 通过 |

## 三、正文去向逐文件清单

正文是叙述性内容（排错、历史、决策、命令），库里没有对应字段。判定规则：

- **可剥离**：正文重复了 frontmatter 已有的值
- **叙述保留**：过程性知识，库无法表达
- **已声明以库为准**：已完成剥离，正文引用节点 UUID
- **需人工判断**：既无重复也无叙述特征

| 文件 | 正文行 | 字数 | 叙述特征 | 重复字段 | 判定 |
|---|---|---|---|---|---|
| services/instances/svc/tokentracker/README.md | 30 | 4850 | 排错/历史/操作 | host,deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/svc/nexus3/README.md | 63 | 4610 | 排错/历史/操作 | host,deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/vm247/vaultwarden.md | 62 | 4604 | 排错/历史/决策/操作 | deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/vm247/vaultwarden-backup.md | 94 | 3457 | 排错/历史/操作 | - | 叙述保留 |
| machine/hp/vm128.md | 31 | 3322 | 排错/历史 | hostname,ip,parent,os | 正文含已入库字段值，可剥离 |
| services/instances/svc/my-known-world/README.md | 24 | 2766 | 排错/历史/操作 | host,deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/vm128/mihomo.md | 29 | 2686 | 历史/操作 | container | 正文含已入库字段值，可剥离 |
| machine/pve/vm247.md | 32 | 2475 | 排错/历史 | ip,os,spec | 正文含已入库字段值，可剥离 |
| machine/mint.md | 34 | 2206 | 排错/历史/操作 | ip,parent,os | 正文含已入库字段值，可剥离 |
| machine/hp/svc.md | 23 | 2140 | 历史/操作 | hostname,ip,parent,access_method,spec | 正文含已入库字段值，可剥离 |
| machine/openwrt.md | 32 | 1986 | 历史 | hostname,ip,parent,os | 正文含已入库字段值，可剥离 |
| machine/pve.md | 28 | 1819 | 排错/历史/操作 | ip,parent,os | 正文含已入库字段值，可剥离 |
| machine/hp/vm121.md | 23 | 1812 | 排错/历史 | hostname,ip,parent,os,status,spec | 正文含已入库字段值，可剥离 |
| services/instances/svc/beszel/README.md | 20 | 1806 | 历史 | deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/svc/miraihub/README.md | 18 | 1717 | 历史/操作 | host,deploy_path | 正文含已入库字段值，可剥离 |
| hardware/hp.md | 7 | 1555 | 排错/历史/操作 | - | 叙述保留 |
| services/instances/vm128/hall-ops.md | 17 | 1368 | 历史/决策/操作 | deploy_path,container | 正文含已入库字段值，可剥离 |
| machine/vps-aliyun.md | 27 | 1361 | 排错/历史/操作 | hostname,ip,os,spec | 正文含已入库字段值，可剥离 |
| services/instances/svc/home-route/README.md | 11 | 1313 | 历史/操作 | - | 叙述保留 |
| services/instances/svc/postgresql/README.md | 14 | 1276 | 历史 | container | 正文含已入库字段值，可剥离 |
| services/instances/vm128/nginx.md | 8 | 1116 | 排错/历史 | deploy_path | 正文含已入库字段值，可剥离 |
| services/instances/svc/reservoir-lite/README.md | 16 | 1085 | 历史/操作 | container | 正文含已入库字段值，可剥离 |
| services/instances/vm247/nexus3.md | 14 | 1041 | 排错/历史 | host,deploy_path | 正文含已入库字段值，可剥离 |
| machine/hp/istoreos.md | 14 | 1011 | 历史 | hostname,ip,os | 正文含已入库字段值，可剥离 |
| machine/hp/vm120.md | 20 | 1003 | 排错/历史/操作 | hostname,ip,parent,os,status,spec | 正文含已入库字段值，可剥离 |
| services/instances/svc/nginx.md | 9 | 980 | 操作 | container | 已声明以库为准 |
| services/instances/svc/gitea/README.md | 15 | 947 | 历史/操作 | container | 正文含已入库字段值，可剥离 |
| machine/h618.md | 17 | 886 | 排错/历史 | ip,parent | 正文含已入库字段值，可剥离 |
| services/instances/svc/wireguard/README.md | 10 | 884 | 历史 | container | 正文含已入库字段值，可剥离 |
| hardware/deskmini.md | 21 | 867 | 排错/历史 | - | 叙述保留 |
| services/instances/mint/dsh.md | 8 | 840 | 排错/历史 | host | 正文含已入库字段值，可剥离 |
| services/instances/svc/portainer/README.md | 9 | 829 | 排错/历史 | container | 正文含已入库字段值，可剥离 |
| machine/pve/fnos.md | 19 | 783 | 历史/操作 | ip,spec | 正文含已入库字段值，可剥离 |
| services/instances/vm128/samba.md | 12 | 781 | 排错/历史 | deploy_path,container | 已声明以库为准 |
| services/instances/svc/mihomo/README.md | 8 | 730 | 历史 | deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/svc/doc-service/README.md | 9 | 723 | 历史 | host,container | 正文含已入库字段值，可剥离 |
| services/instances/openwrt/openclash.md | 11 | 675 | 排错/历史/操作 | host | 正文含已入库字段值，可剥离 |
| machine/hp/vm131.md | 7 | 666 | 历史 | hostname,ip | 正文含已入库字段值，可剥离 |
| services/instances/vm128/dsh.md | 9 | 660 | 排错/操作 | host | 正文含已入库字段值，可剥离 |
| services/instances/vm247/nginx.md | 8 | 656 | 排错/历史 | deploy_path | 正文含已入库字段值，可剥离 |
| services/instances/vps/wg-easy.md | 11 | 656 | - | - | 需人工判断 |
| services/instances/vm128/my-known-world-test/README.md | 8 | 619 | - | deploy_path,container | 正文含已入库字段值，可剥离 |
| machine/hallmini.md | 13 | 592 | 历史 | hostname,os,parent | 正文含已入库字段值，可剥离 |
| services/instances/svc/it-tools.md | 8 | 543 | 决策 | container | 已声明以库为准 |
| hardware/h618.md | 14 | 521 | - | - | 需人工判断 |
| services/instances/vm128/hall-mcp.md | 6 | 513 | 历史 | container | 正文含已入库字段值，可剥离 |
| services/instances/svc/homer.md | 8 | 498 | - | container | 正文含已入库字段值，可剥离 |
| services/instances/vm128/repo-cache.md | 10 | 481 | 历史 | deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/vm247/atrust-autologin.md | 7 | 444 | 历史 | container | 正文含已入库字段值，可剥离 |
| hardware/eq59.md | 13 | 434 | 历史 | - | 叙述保留 |
| services/instances/vm247/postgresql.md | 7 | 430 | - | container | 已声明以库为准 |
| services/instances/hallmini/syncthing.md | 8 | 408 | - | container | 已声明以库为准 |
| services/instances/vm247/homer.md | 7 | 403 | 历史 | deploy_path | 正文含已入库字段值，可剥离 |
| services/instances/vm128/syncthing.md | 6 | 382 | - | host,deploy_path,container | 正文含已入库字段值，可剥离 |
| services/instances/vm247/gitea.md | 7 | 378 | - | deploy_path | 正文含已入库字段值，可剥离 |
| hardware/macmini.md | 13 | 361 | - | - | 需人工判断 |
| services/backup-plan.md | 9 | 347 | 操作 | - | 叙述保留 |
| hardware/cudy.md | 6 | 341 | - | - | 需人工判断 |

合计 58 个文件，正文 74643 字
   43  正文含已入库字段值，可剥离
    6  叙述保留
    5  已声明以库为准
    4  需人工判断


## 四、待确认

4 个「需人工判断」的文件需要人工决定正文保留与否，见第三节表格。
