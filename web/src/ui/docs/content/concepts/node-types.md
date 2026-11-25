# 📦 12 类核心实体模型 (Node Types)

图谱中登记的认知对象类型分类及其专属属性规范：

---

### 1. 计算与基础设施
- **`Machine` (计算机器)**：承载计算的实体，包括物理机、虚拟机、开发主机。专属字段包含 `hostname`, `os`, `ips`, `network`, `status`, `accessMethod`, `responsibilityBoundary` 等。
- **`Hardware` (物理硬件)**：实体物理设备。记录 `category`, `model`, `location`, `status` 及硬件规格参数。
- **`Network` (网络规划)**：专有网络与网段。记录 `kind`, `cidrs`, `gatewayReference`, `accessLocations`, `vpnReference` 等。

### 2. 软件研发与交付
- **`SoftwareProject` (代码工程)**：软件工程源码仓库，记录 `remoteRepository`, `languages`, `form`, `maintenanceState`, `upstream`, `successorId` 等。
- **`SoftwareInstance` (部署实例)**：运行态微服务实例，记录 `productName`, `domains`, `ports`, `deploymentPath`, `deploymentMethod`, `endpoints` 等。
- **`Product` (业务产品)**：逻辑产品实体，连接共性配置与 Runbook。

### 3. 安全凭据与运维支持
- **`AccountCredential` (凭据元信息)**：账号凭据元信息，**不存明文**，仅存 `account`, `credentialType`, `purpose`, `scope` 及 `accessReference`（密码库指针）。
- **`Document` (权威文档)**：权威技术文档，记录 `role`, `authoritativeReference`, `effective`, `lastVerified`, `supersededBy` 等。
- **`Runbook` (作业手册)**：标准作业排错手册，记录 `trigger`, `preconditions`, `stepsReference`, `stopConditionReference`, `rollbackReference` 等。
- **`WorkItem` (工作事项)**：升级、迁移、缺陷及重构事项，记录 `kind`, `status`, `rationaleReference`, `decisionEvidenceReference` 等。
- **`RunningPeriod` (运行周期)**：用于审计实例的历史上线、运行、验证与下线周期。
- **`ProvisionalObject` (待审查暂存)**：外部导入或自动化收集时未完全确认的暂存对象，等待人工审查转正或拒绝。
