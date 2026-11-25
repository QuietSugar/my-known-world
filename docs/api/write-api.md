# ✍️ 自动化写入 API 与安全规范

提供给运维脚本与自动化工具的图谱更新接口。

---

## 认证凭据要求

所有写入接口均需在请求头部携带写令牌：
`Authorization: Bearer $MYKNOWNWORLD_WRITE_TOKEN`

---

## 核心写入端点

| 请求方法与路径 | 功能说明 | 事务与安全性保障 |
|---|---|---|
| `POST /api/write/nodes` | 创建单个明确对象并锚定至 Self | 校验唯一 ID 与类型专属约束 |
| `POST /api/write/batch` | 原子批量导入（节点与关系同时提交） | 单批次最多 500 节点/500 关系，事务全或无 |
| `PATCH /api/write/nodes/{id}` | 就地更新对象可变元数据 | 带 expectedRevision 乐观并发锁 |
| `POST /api/write/nodes/{id}/retire` | 优雅废弃节点（标记为 retired） | 校验废弃后图谱不发生孤岛断连 |
| `DELETE /api/write/nodes/{id}` | 物理硬删除实体 | 需级联解除全部依赖关系 |

---

## 会话级写入安全原则

在 Web 端修改实体名称、类型详情或新建关系时，系统将使用配置的写令牌。该令牌**绝不持久化**到 `localStorage`、`IndexedDB` 或 Cookie 中，仅临时留存于当前浏览器会话（`sessionStorage`），刷新或关闭标签页即自动销毁。
