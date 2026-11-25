# 🏷️ 业务拓扑连线谓词规范

不同类型实体之间建立的有向关系谓词定义及其严格约束：

---

| 谓词 | 方向与连接对象 | 语义与场景说明 |
|---|---|---|
| `hosts` | Machine → SoftwareInstance | **宿主承载**：物理机或虚拟服务器承载运行该服务实例 |
| `implementedFrom` | SoftwareInstance → SoftwareProject | **工程派生**：运行实例由指定的代码工程编译、打包构建而来 |
| `hasCredential` | Machine / Instance → AccountCredential | **凭据绑定**：实体绑定了该外部账号凭据元数据指针 |
| `documents` | Document → 任意实体 | **文档阐明**：设计方案、拓扑图文档对该实体进行权威说明 |
| `appliesTo` | Runbook → SoftwareProject / Instance | **手册适用**：标准操作手册适用于该系统运维场景 |
| `dependsOn` | Service A → Service B | **调用依赖**：服务间强调用依赖，故障沿此链路级联传播 |
| `routesThrough` | Network / Machine → Gateway | **路由穿透**：底层网络或机器通过指定网关路由 |
| `providesAccessTo` | Machine → Network / Machine | **访问提供**：堡垒机或跳板机提供进入目标的通道 |
| `supersedes` | NewItem → OldItem | **继任替代**：新版文档、代码项目或机器替代旧资产 |
| `targets` | WorkItem → 任意实体 | **事项关联**：工作事项（改造、迁移）指向的目标对象 |
