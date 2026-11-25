# 🤖 大模型只读 MCP 协议接入

让 Cursor、Claude Desktop 或 Antigravity AI 编程助手直接读取你的主观图谱。

---

## 什么是 MCP 协议？

MyKnownWorld 原生内置了只读 **Model Context Protocol (MCP)** 端点。AI 助手挂载后，可随时执行语义搜索、查询机器配置与聚合项目架构。

```json
// 在 Cursor / Claude Desktop / Antigravity 的 mcp 配置中添加：
{
  "mcpServers": {
    "myknownworld": {
      "url": "https://<your-myknownworld-host>/mcp"
    }
  }
}
```

---

## 安全保障：MCP 天然受只读沙箱保护

通过 MCP 协议暴露的接口仅提供节点检索、关系查询与拓扑探索功能，**不暴露任何写操作，也不提供敏感凭据正文**，保证外部大模型调用时的绝对只读安全性。
