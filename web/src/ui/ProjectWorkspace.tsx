import { useEffect, useState } from 'react'
import {
  ArrowLeft,
  GitBranch,
  Layers3,
  Server,
  Lock,
  LifeBuoy,
  FileText,
  CircleHelp,
  Edit3,
  ShieldCheck,
  RefreshCw,
  Tag
} from 'lucide-react'
import {
  api,
  type Node,
  type ProjectWorkspaceData,
  type RunbookDetails,
  type DocumentDetails
} from './api'
import './workspace.css'

interface ProjectWorkspaceProps {
  projectId: string
  projects: Node[]
  onSelectProject: (id: string) => void
  onBackToGraph: () => void
  onOpenDetailsEditor: (node: Node) => void
}

type TabKey = 'overview' | 'dcup' | 'did' | 'docs'

export function ProjectWorkspace({
  projectId,
  projects,
  onSelectProject,
  onBackToGraph,
  onOpenDetailsEditor
}: ProjectWorkspaceProps) {
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const [data, setData] = useState<ProjectWorkspaceData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const loadWorkspace = async (id: string) => {
    setLoading(true)
    setError('')
    try {
      const result = await api.projectWorkspace(id)
      setData(result)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '加载项目工作区失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (projectId) {
      void loadWorkspace(projectId)
    }
  }, [projectId])

  if (loading) {
    return (
      <div className="project-workspace">
        <div className="workspace-header">
          <div className="loading-state">
            <span className="spinner" /> 正在聚合项目工作区上下文…
          </div>
        </div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="project-workspace">
        <div className="workspace-header">
          <div className="workspace-nav-bar">
            <button className="back-btn" onClick={onBackToGraph}>
              <ArrowLeft size={14} /> 返回图谱
            </button>
          </div>
          <div className="alert">
            <CircleHelp size={16} />
            <span>{error || '未找到项目信息'}</span>
          </div>
        </div>
      </div>
    )
  }

  const { project, instances, machines, credentials, documents, workItems, relations } = data
  const projectDetails = project.softwareProject ?? {}

  // Filter runbooks and general documents
  const runbooks = documents.filter(d => d.type === 'Runbook')
  const generalDocs = documents.filter(d => d.type === 'Document')

  // Find node by id in aggregated data
  const findNode = (id: string): Node | undefined => {
    if (project.id === id) return project
    return instances.find(n => n.id === id) ||
      machines.find(n => n.id === id) ||
      credentials.find(n => n.id === id) ||
      documents.find(n => n.id === id) ||
      workItems.find(n => n.id === id)
  }

  return (
    <div className="project-workspace">
      {/* Top Header & Switcher */}
      <header className="workspace-header">
        <div className="workspace-nav-bar">
          <div className="workspace-nav-left">
            <button className="back-btn" onClick={onBackToGraph}>
              <ArrowLeft size={14} /> 返回图谱
            </button>
            <div className="project-selector">
              <span>切换项目:</span>
              <select
                value={projectId}
                onChange={e => onSelectProject(e.target.value)}
                aria-label="选择项目"
              >
                {projects.map(p => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.id})
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="workspace-nav-right">
            <button
              className="back-btn"
              onClick={() => void loadWorkspace(projectId)}
              title="刷新工作区数据"
            >
              <RefreshCw size={13} /> 刷新
            </button>
            <button
              className="action-annotate-btn"
              onClick={() => onOpenDetailsEditor(project)}
            >
              <Edit3 size={13} /> 人工标注此项目
            </button>
          </div>
        </div>

        {/* Project Hero Card */}
        <div className="project-hero">
          <div className="project-hero-main">
            <div className="project-hero-title-group">
              <div className="project-icon-box">
                <GitBranch size={24} />
              </div>
              <div>
                <h1>{project.name}</h1>
                <div className="project-badges">
                  <span className="badge badge-mono">ID: {project.id}</span>
                  <span className="badge badge-amber">{projectDetails.form || '服务工程'}</span>
                  <span className="badge badge-green">
                    维护状态: {projectDetails.maintenanceState || project.lifecycle || 'active'}
                  </span>
                  <span className="badge badge-teal">确定性: {project.certainty}</span>
                  <span className="badge">来源: {project.source}</span>
                </div>
              </div>
            </div>
          </div>

          <div className="project-facts-grid">
            <div className="fact-item">
              <span className="fact-label">业务用途 / 业务定位</span>
              <span className="fact-value">{projectDetails.purpose || '（暂无业务说明，建议人工补充）'}</span>
            </div>
            <div className="fact-item">
              <span className="fact-label">主要开发语言</span>
              <span className="fact-value">
                {projectDetails.languages && projectDetails.languages.length > 0
                  ? projectDetails.languages.join(', ')
                  : '未登记'}
              </span>
            </div>
            <div className="fact-item">
              <span className="fact-label">远程仓库代码库</span>
              <span className="fact-value">
                {projectDetails.remoteRepository ? (
                  <code className="code-ref">{projectDetails.remoteRepository}</code>
                ) : (
                  '未登记'
                )}
              </span>
            </div>
            <div className="fact-item">
              <span className="fact-label">上游依赖 / 继任状态</span>
              <span className="fact-value">
                {projectDetails.upstream || '无上游'}
                {projectDetails.successorId && ` (继任者: ${projectDetails.successorId})`}
              </span>
            </div>
          </div>
        </div>

        {/* Workspace Tab Navigation */}
        <nav className="workspace-tabs" role="tablist">
          <button
            role="tab"
            aria-selected={activeTab === 'overview'}
            className={`workspace-tab-btn ${activeTab === 'overview' ? 'active' : ''}`}
            onClick={() => setActiveTab('overview')}
          >
            <GitBranch size={15} /> 概览与依赖
            <span className="tab-badge">{relations.length} 关系</span>
          </button>
          <button
            role="tab"
            aria-selected={activeTab === 'dcup'}
            className={`workspace-tab-btn ${activeTab === 'dcup' ? 'active' : ''}`}
            onClick={() => setActiveTab('dcup')}
          >
            <Server size={15} /> 部署与运行运维 (dcup 视图)
            <span className="tab-badge">{instances.length} 实例 · {machines.length} 机器</span>
          </button>
          <button
            role="tab"
            aria-selected={activeTab === 'did'}
            className={`workspace-tab-btn ${activeTab === 'did' ? 'active' : ''}`}
            onClick={() => setActiveTab('did')}
          >
            <Lock size={15} /> 身份与凭据联动 (did / CMDB 视图)
            <span className="tab-badge">{credentials.length} 凭据元信息</span>
          </button>
          <button
            role="tab"
            aria-selected={activeTab === 'docs'}
            className={`workspace-tab-btn ${activeTab === 'docs' ? 'active' : ''}`}
            onClick={() => setActiveTab('docs')}
          >
            <LifeBuoy size={15} /> 运维手册与事项 (Runbooks & Docs)
            <span className="tab-badge">{documents.length} 文档/手册 · {workItems.length} 事项</span>
          </button>
        </nav>
      </header>

      {/* Tab Panels */}
      <div className="tab-content">
        {/* Tab 1: 概览与依赖 */}
        {activeTab === 'overview' && (
          <>
            <div className="annotation-banner">
              <div className="annotation-banner-text">
                <Edit3 size={16} />
                <span>
                  <strong>人工认知协同：</strong> 偏向录入或校正 Agent 无法自动推断的业务上下文、架构权衡与设计动机。
                </span>
              </div>
              <button
                className="annotation-banner-btn"
                onClick={() => onOpenDetailsEditor(project)}
              >
                补充当前项目人工标注
              </button>
            </div>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <Tag size={16} /> 关联依赖与拓扑关系
                  </div>
                  <div className="section-subtitle">
                    直接或间接与当前项目关联的拓扑边（{relations.length} 条已确认关系）
                  </div>
                </div>
              </div>

              {relations.length === 0 ? (
                <div className="workspace-empty">当前项目暂未发现其他关联依赖拓扑。</div>
              ) : (
                <div className="relation-chip-list">
                  {relations.map(r => {
                    const fromNode = findNode(r.from)
                    const toNode = findNode(r.to)
                    return (
                      <div className="relation-chip-row" key={r.id}>
                        <div className="relation-chip-left">
                          <strong>{fromNode?.name || r.from}</strong>
                          <span className="relation-pred">{r.predicate}</span>
                          <strong>{toNode?.name || r.to}</strong>
                        </div>
                        <div className="project-badges">
                          <span className="badge badge-mono">{r.id}</span>
                          <span className="badge">{r.source}</span>
                          <span className="badge badge-green">{r.certainty}</span>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>

            {workItems.length > 0 && (
              <section className="workspace-section">
                <div className="section-head">
                  <div>
                    <div className="section-title">
                      <CircleHelp size={16} /> 关联工作事项与决策跟踪
                    </div>
                    <div className="section-subtitle">
                      与此项目相关的升级、迁移、缺陷及重构事项
                    </div>
                  </div>
                </div>
                <div className="cards-grid">
                  {workItems.map(item => (
                    <div className="entity-card" key={item.id}>
                      <div className="entity-card-head">
                        <div>
                          <div className="entity-card-title">{item.name}</div>
                          <div className="entity-card-id">{item.id}</div>
                        </div>
                        <span className="badge badge-amber">{item.workItem?.status || '进行中'}</span>
                      </div>
                      <div className="entity-grid-facts">
                        <div className="fact-item">
                          <span className="fact-label">事项类别</span>
                          <span className="fact-value">{item.workItem?.kind || '工作事项'}</span>
                        </div>
                        <div className="fact-item">
                          <span className="fact-label">决策理由引用</span>
                          <span className="fact-value">
                            {item.workItem?.rationaleReference ? (
                              <code className="code-ref">{item.workItem.rationaleReference}</code>
                            ) : (
                              '未记录'
                            )}
                          </span>
                        </div>
                      </div>
                      <div className="entity-card-actions">
                        <button
                          className="card-action-btn"
                          onClick={() => onOpenDetailsEditor(item)}
                        >
                          <Edit3 size={12} /> 编辑事项标注
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            )}
          </>
        )}

        {/* Tab 2: 部署与运行运维 (dcup 视图) */}
        {activeTab === 'dcup' && (
          <>
            <div className="annotation-banner">
              <div className="annotation-banner-text">
                <Server size={16} />
                <span>
                  <strong>dcup 视图：</strong> 汇聚当前项目的部署拓扑、宿主机器、监听端口、域名绑定及运行时环境。
                </span>
              </div>
            </div>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <Layers3 size={16} /> 运行实例列表 (Software Instances)
                  </div>
                  <div className="section-subtitle">
                    基于代码工程部署出来的在线/离线实例
                  </div>
                </div>
              </div>

              {instances.length === 0 ? (
                <div className="workspace-empty">
                  当前项目尚未在图谱中登记部署实例（SoftwareInstance）。可通过图谱添加实例节点并关联 implementedFrom。
                </div>
              ) : (
                <div className="cards-grid">
                  {instances.map(inst => {
                    const instDetails = inst.softwareInstance ?? {}
                    return (
                      <div className="entity-card" key={inst.id}>
                        <div className="entity-card-head">
                          <div>
                            <div className="entity-card-title">{inst.name}</div>
                            <div className="entity-card-id">{inst.id}</div>
                          </div>
                          <div className="project-badges">
                            <span className="badge badge-green">
                              {instDetails.operationalState || 'running'}
                            </span>
                            <span className="badge badge-teal">
                              {instDetails.deploymentState || 'deployed'}
                            </span>
                          </div>
                        </div>

                        <div className="entity-grid-facts">
                          <div className="fact-item">
                            <span className="fact-label">服务标识</span>
                            <span className="fact-value">{instDetails.productName || inst.name}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">部署方式</span>
                            <span className="fact-value">{instDetails.deploymentMethod || 'Docker / 容器化'}</span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">部署路径</span>
                            <span className="fact-value">
                              {instDetails.deploymentPath ? (
                                <code className="code-ref">{instDetails.deploymentPath}</code>
                              ) : (
                                '未登记路径'
                              )}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">绑定域名</span>
                            <span className="fact-value">
                              {instDetails.domains && instDetails.domains.length > 0 ? (
                                instDetails.domains.map((d: string) => (
                                  <code key={d} className="code-ref" style={{ marginRight: 4 }}>
                                    {d}
                                  </code>
                                ))
                              ) : (
                                '未绑定独立域名'
                              )}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">监听端口</span>
                            <span className="fact-value">
                              {instDetails.ports && instDetails.ports.length > 0 ? (
                                instDetails.ports.map((p: number) => (
                                  <span key={p} className="badge badge-mono" style={{ marginRight: 4 }}>
                                    :{p}
                                  </span>
                                ))
                              ) : (
                                '—'
                              )}
                            </span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">运行身份 (非敏感)</span>
                            <span className="fact-value">
                              {instDetails.runtimeIdentity || '非特权服务账号'}
                            </span>
                          </div>
                          {instDetails.verifiedAt && (
                            <div className="fact-item entity-grid-full">
                              <span className="fact-label">最近验证</span>
                              <span className="fact-value">
                                {new Date(instDetails.verifiedAt).toLocaleString('zh-CN')} · 结果:{' '}
                                {instDetails.verificationResult || '正常'}
                              </span>
                            </div>
                          )}
                        </div>

                        <div className="entity-card-actions">
                          <button
                            className="card-action-btn"
                            onClick={() => onOpenDetailsEditor(inst)}
                          >
                            <Edit3 size={12} /> 编辑实例元数据
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <Server size={16} /> 宿主机环境 (Host Machines)
                  </div>
                  <div className="section-subtitle">
                    承载上述运行实例的服务器与虚拟化计算节点
                  </div>
                </div>
              </div>

              {machines.length === 0 ? (
                <div className="workspace-empty">
                  未发现关联的宿主机信息（可能直接运行在 Serverless 或未建立 hosts 关联）。
                </div>
              ) : (
                <div className="cards-grid">
                  {machines.map(m => {
                    const mDetails = m.machine ?? {}
                    return (
                      <div className="entity-card" key={m.id}>
                        <div className="entity-card-head">
                          <div>
                            <div className="entity-card-title">{m.name}</div>
                            <div className="entity-card-id">{m.id}</div>
                          </div>
                          <span className="badge badge-blue">
                            {mDetails.machineKind || 'virtual'}
                          </span>
                        </div>

                        <div className="entity-grid-facts">
                          <div className="fact-item">
                            <span className="fact-label">主机名 (Hostname)</span>
                            <span className="fact-value">{mDetails.hostname || '—'}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">操作系统</span>
                            <span className="fact-value">{mDetails.os || 'Linux'}</span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">IP 地址</span>
                            <span className="fact-value">
                              {mDetails.ips && mDetails.ips.length > 0 ? (
                                mDetails.ips.map((ip: string) => (
                                  <code key={ip} className="code-ref" style={{ marginRight: 4 }}>
                                    {ip}
                                  </code>
                                ))
                              ) : (
                                '未分配公网/专网静态 IP'
                              )}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">所属网络</span>
                            <span className="fact-value">{mDetails.network || '默认 VPC'}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">节点状态</span>
                            <span className="fact-value">{mDetails.status || 'healthy'}</span>
                          </div>
                          {mDetails.responsibilityBoundary && (
                            <div className="fact-item entity-grid-full">
                              <span className="fact-label">责任边界 / 运维约束</span>
                              <span className="fact-value">{mDetails.responsibilityBoundary}</span>
                            </div>
                          )}
                        </div>

                        <div className="entity-card-actions">
                          <button
                            className="card-action-btn"
                            onClick={() => onOpenDetailsEditor(m)}
                          >
                            <Edit3 size={12} /> 编辑机器运维配置
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>
          </>
        )}

        {/* Tab 3: 身份与凭据联动 (did / CMDB 视图) */}
        {activeTab === 'did' && (
          <>
            <div className="security-callout">
              <div className="security-callout-icon">
                <ShieldCheck size={24} />
              </div>
              <div className="security-callout-content">
                <h4>零明文秘密原则 (Zero-Secret Principle)</h4>
                <p>
                  此视图用于将项目的身份定义（did）与外部受控权限中心（CMDB / KMS / Vault）建立安全指针映射。
                  工作区<strong>绝对不记录、不传输、不展示</strong>任何明文密码、证书私钥或长期 Token。
                  所有凭据均通过 <code className="code-ref">accessReference</code> 指针定向索引，安全职责严格划分。
                </p>
              </div>
            </div>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <Lock size={16} /> 关联凭据元信息与 CMDB 访问指针
                  </div>
                  <div className="section-subtitle">
                    项目、实例或宿主机已绑定的访问凭据元信息与鉴权指引
                  </div>
                </div>
              </div>

              {credentials.length === 0 ? (
                <div className="workspace-empty">
                  当前项目及其部署环境尚未关联凭据元信息。若需登记 CMDB/Vault 访问指针，可在此人工补充。
                </div>
              ) : (
                <div className="cards-grid">
                  {credentials.map(c => {
                    const cDetails = c.accountCredential ?? {}
                    return (
                      <div className="entity-card" key={c.id}>
                        <div className="entity-card-head">
                          <div>
                            <div className="entity-card-title">{c.name}</div>
                            <div className="entity-card-id">{c.id}</div>
                          </div>
                          <span className="badge badge-rose">
                            {cDetails.credentialType || '账号凭据'}
                          </span>
                        </div>

                        <div className="entity-grid-facts">
                          <div className="fact-item">
                            <span className="fact-label">账号标识 (非敏感)</span>
                            <span className="fact-value">{cDetails.account || '默认服务账号'}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">权限范围 (Scope)</span>
                            <span className="fact-value">{cDetails.scope || '项目专有'}</span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">用途说明</span>
                            <span className="fact-value">{cDetails.purpose || '未填写用途说明'}</span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">CMDB / 安全访问指针 (Access Reference)</span>
                            <span className="fact-value">
                              {cDetails.accessReference ? (
                                <code className="code-ref">{cDetails.accessReference}</code>
                              ) : (
                                '未配置指针引用'
                              )}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">轮换周期 / 上次轮换</span>
                            <span className="fact-value">
                              {cDetails.rotationAt
                                ? new Date(cDetails.rotationAt).toLocaleDateString('zh-CN')
                                : '未设定'}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">核验状态</span>
                            <span className="fact-value">{c.certainty}</span>
                          </div>
                        </div>

                        <div className="entity-card-actions">
                          <button
                            className="card-action-btn"
                            onClick={() => onOpenDetailsEditor(c)}
                          >
                            <Edit3 size={12} /> 维护凭据元信息/指针
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>
          </>
        )}

        {/* Tab 4: 运维手册与事项 (Runbooks & Docs) */}
        {activeTab === 'docs' && (
          <>
            <div className="annotation-banner">
              <div className="annotation-banner-text">
                <LifeBuoy size={16} />
                <span>
                  <strong>运维标准化：</strong> 集中管理部署、重启、巡检 Runbook 及设计交付文档。
                </span>
              </div>
            </div>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <LifeBuoy size={16} /> 运维 Runbook 操作手册
                  </div>
                  <div className="section-subtitle">
                    执行触发条件、前置安全校验、步骤引用与回退指引
                  </div>
                </div>
              </div>

              {runbooks.length === 0 ? (
                <div className="workspace-empty">当前项目暂未登记 Runbook 操作手册。</div>
              ) : (
                <div className="cards-grid">
                  {runbooks.map(rb => {
                    const rbDetails: Partial<RunbookDetails> = rb.runbook ?? {}
                    return (
                      <div className="entity-card" key={rb.id}>
                        <div className="entity-card-head">
                          <div>
                            <div className="entity-card-title">{rb.name}</div>
                            <div className="entity-card-id">{rb.id}</div>
                          </div>
                          <span className="badge badge-amber">
                            {rbDetails.approvalRequired ? '需人工审批' : '日常可执行'}
                          </span>
                        </div>

                        <div className="entity-grid-facts">
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">触发时机 / 适用场景</span>
                            <span className="fact-value">{rbDetails.trigger || '常规运维'}</span>
                          </div>
                          {rbDetails.preconditions && rbDetails.preconditions.length > 0 && (
                            <div className="fact-item entity-grid-full">
                              <span className="fact-label">前置确认清单</span>
                              <ul style={{ margin: '4px 0 0 16px', padding: 0 }}>
                                {rbDetails.preconditions.map((p, idx) => (
                                  <li key={idx}>{p}</li>
                                ))}
                              </ul>
                            </div>
                          )}
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">操作步骤引用 (Steps Reference)</span>
                            <span className="fact-value">
                              {rbDetails.stepsReference ? (
                                <code className="code-ref">{rbDetails.stepsReference}</code>
                              ) : (
                                '未登记步骤引用'
                              )}
                            </span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">停止条件</span>
                            <span className="fact-value">{rbDetails.stopConditionReference || '—'}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">回退方案引用</span>
                            <span className="fact-value">
                              {rbDetails.rollbackReference ? (
                                <code className="code-ref">{rbDetails.rollbackReference}</code>
                              ) : (
                                '—'
                              )}
                            </span>
                          </div>
                        </div>

                        <div className="entity-card-actions">
                          <button
                            className="card-action-btn"
                            onClick={() => onOpenDetailsEditor(rb)}
                          >
                            <Edit3 size={12} /> 编辑手册配置
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>

            <section className="workspace-section">
              <div className="section-head">
                <div>
                  <div className="section-title">
                    <FileText size={16} /> 关联设计与交付文档
                  </div>
                  <div className="section-subtitle">
                    架构方案、API 说明、环境交付规范及权威文档
                  </div>
                </div>
              </div>

              {generalDocs.length === 0 ? (
                <div className="workspace-empty">当前项目暂未关联通用技术文档。</div>
              ) : (
                <div className="cards-grid">
                  {generalDocs.map(doc => {
                    const docDetails: Partial<DocumentDetails> = doc.document ?? {}
                    return (
                      <div className="entity-card" key={doc.id}>
                        <div className="entity-card-head">
                          <div>
                            <div className="entity-card-title">{doc.name}</div>
                            <div className="entity-card-id">{doc.id}</div>
                          </div>
                          <span className="badge badge-blue">
                            {docDetails.effective ? '当前有效' : '已废弃'}
                          </span>
                        </div>

                        <div className="entity-grid-facts">
                          <div className="fact-item">
                            <span className="fact-label">文档角色</span>
                            <span className="fact-value">{docDetails.role || '技术文档'}</span>
                          </div>
                          <div className="fact-item">
                            <span className="fact-label">核验时间</span>
                            <span className="fact-value">
                              {docDetails.lastVerified
                                ? new Date(docDetails.lastVerified).toLocaleDateString('zh-CN')
                                : '—'}
                            </span>
                          </div>
                          <div className="fact-item entity-grid-full">
                            <span className="fact-label">权威文档引用 (Authoritative Reference)</span>
                            <span className="fact-value">
                              {docDetails.authoritativeReference ? (
                                <code className="code-ref">{docDetails.authoritativeReference}</code>
                              ) : (
                                '未登记外部引用'
                              )}
                            </span>
                          </div>
                        </div>

                        <div className="entity-card-actions">
                          <button
                            className="card-action-btn"
                            onClick={() => onOpenDetailsEditor(doc)}
                          >
                            <Edit3 size={12} /> 编辑文档指引
                          </button>
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </section>
          </>
        )}
      </div>
    </div>
  )
}
