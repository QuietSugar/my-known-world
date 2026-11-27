import { useEffect, useMemo, useState } from 'react'
import {
  ArrowLeft,
  GitBranch,
  Layers3,
  Server,
  Lock,
  LifeBuoy,
  FileText,
  CircleHelp,
  Copy,
  Edit3,
  Eye,
  EyeOff,
  RefreshCw,
  ExternalLink,
  Tag as TagIcon,
  UploadCloud,
  ShieldCheck
} from 'lucide-react'
import {
  api,
  type Node,
  type ProjectWorkspaceData,
  type RunbookDetails,
  type DocumentDetails
} from './api'
import { getPresetSecret } from './presets/xascSecrets'
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Collapse,
  Descriptions,
  Empty,
  List,
  Popconfirm,
  Segmented,
  Select,
  Skeleton,
  Space,
  Spin,
  Tabs,
  Tag,
  message
} from 'antd'
import './workspace.css'

interface ProjectWorkspaceProps {
  projectId: string
  projects?: Node[]
  onSelectProject?: (id: string) => void
  onBackToGraph: () => void
  onOpenDetailsEditor: (node: Node) => void
  standalone?: boolean
}

type TabKey = 'overview' | 'dcup' | 'did' | 'docs'

export interface EnvGroup {
  key: string
  name: string
  shortName: string
  badgeName: string
  color: string
  tagColor: string
  description: string
  instances: Node[]
  machines: Node[]
  credentials: Node[]
}

export function classifyEnvironments(
  instances: Node[],
  machines: Node[],
  credentials: Node[]
): EnvGroup[] {
  const isProd = (t: string) => t.includes('prod') || t.includes('生产')
  const isTest = (t: string) => t.includes('test') || t.includes('测试') || t.includes('一体化')
  const isC0130 = (t: string) => t.includes('c0130') || t.includes('现场')

  const envs: EnvGroup[] = [
    {
      key: 'prod',
      name: '生产环境 (Production)',
      shortName: '生产环境',
      badgeName: 'PROD',
      color: '#2e7d32',
      tagColor: 'green',
      description: '承载正式外部流通利用平台定制、DID身份对接及大屏监控的生产核心集群。',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'test',
      name: '测试环境 (Testing / 一体化)',
      shortName: '测试环境',
      badgeName: 'TEST',
      color: '#0288d1',
      tagColor: 'blue',
      description: '核心联调与集成测试平台，包含买卖双方连接器、ShenYu 网关及专属测试数据库。',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'c0130',
      name: '雄安现场专区 (c0130 专区)',
      shortName: '现场专区 (c0130)',
      badgeName: 'C0130',
      color: '#ed6c02',
      tagColor: 'orange',
      description: '雄安数城现场部署专网节点（IP段 172.16.21.x / 172.16.22.x），包含 MinIO、Nacos 及区域功能节点。',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'infra',
      name: '公共基础设施与网关 (Infra)',
      shortName: '公共基础与网关',
      badgeName: 'INFRA',
      color: '#7b1fa2',
      tagColor: 'purple',
      description: '统一访问入口、堡垒机管理门户、跨链 DID 身份认证底座及前端代码仓。',
      instances: [],
      machines: [],
      credentials: []
    }
  ]

  const matchEnv = (text: string) => {
    const t = text.toLowerCase()
    if (isProd(t)) return envs[0]
    if (isTest(t)) return envs[1]
    if (isC0130(t)) return envs[2]
    return envs[3]
  }

  instances.forEach(i => matchEnv(`${i.id} ${i.name}`).instances.push(i))
  machines.forEach(m => matchEnv(`${m.id} ${m.name}`).machines.push(m))
  credentials.forEach(c => matchEnv(`${c.id} ${c.name}`).credentials.push(c))

  return envs.filter(e => e.instances.length > 0 || e.machines.length > 0 || e.credentials.length > 0)
}

// isUUID reports whether id is already a graph node id. Node ids became UUIDs
// during the 2026-11 migration; prefixing them with projects/ produced a 404.
function isUUID(id: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)
}

// 固定页面模式已停用，注册表随之清空。若将来需要秒开的只读快照，
// 应从当前图谱重新生成，而不是沿用迁移前的静态文件。
const FIXED_PRESETS: Record<string, ProjectWorkspaceData> = {}

// 模式总控：当前优先使用“固定页面模式”（秒开、稳定、零网络延迟等待）；
// 待后续业务完全切为动态时，可无缝切回全动态拉取。
// 关闭固定页面模式：presets/xascPreset.ts 是迁移前的静态快照（1962 行），
// 其中的节点 id 还是 projects/xasc 这类路径，与库中的 UUID 已脱节，
// 打开项目工作区会看到过期内容。数据已在库中，统一走 api.projectWorkspace。
export const PREFER_FIXED_PAGE = false

export function ProjectWorkspace({
  projectId,
  projects = [],
  onSelectProject,
  onBackToGraph,
  onOpenDetailsEditor,
  standalone = false
}: ProjectWorkspaceProps) {
  const [viewMode, setViewMode] = useState<'by-env' | 'raw'>('by-env')
  const [activeEnvTab, setActiveEnvTab] = useState<string>('prod')
  const [activeTab, setActiveTab] = useState<TabKey>('overview')
  const initialPreset = PREFER_FIXED_PAGE ? (FIXED_PRESETS[projectId] || FIXED_PRESETS[`projects/${projectId}`] || null) : null
  const [data, setData] = useState<ProjectWorkspaceData | null>(initialPreset)
  const [loading, setLoading] = useState<boolean>(!initialPreset)
  const [isFixedMode, setIsFixedMode] = useState<boolean>(Boolean(initialPreset))
  const [error, setError] = useState('')
  const [passwords, setPasswords] = useState<Record<string, string>>({})
  const [revealed, setRevealed] = useState<Record<string, boolean>>({})
  const [copying, setCopying] = useState<Record<string, boolean>>({})
  const [vaultSyncing, setVaultSyncing] = useState(false)
  const [selectedCreds, setSelectedCreds] = useState<string[]>([])
  const [credStatus, setCredStatus] = useState<Record<string, string>>({})
  const [uploading, setUploading] = useState(false)
  const [targetUpstream, setTargetUpstream] = useState<'file' | 'bitwarden'>('file')

  const loadCredStatus = async () => {
    try {
      const res = await api.credentialStatus()
      if (res.managed) {
        setCredStatus(res.managed)
      }
    } catch (e) {
      console.warn('Load credential status notice:', e)
    }
  }

  useEffect(() => {
    void loadCredStatus()
  }, [])

  const copyTextToClipboard = async (text: string): Promise<boolean> => {
    if (!text) return false
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(text)
        return true
      } catch {
        // fallback to execCommand below
      }
    }

    try {
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.style.position = 'fixed'
      textArea.style.left = '-999999px'
      textArea.style.top = '-999999px'
      textArea.setAttribute('readonly', '')
      document.body.appendChild(textArea)
      textArea.select()
      const successful = document.execCommand('copy')
      textArea.remove()
      return Boolean(successful)
    } catch (err) {
      console.error('Fallback copy error:', err)
      return false
    }
  }

  const fetchPassword = async (node: Node): Promise<string> => {
    if (passwords[node.id]) return passwords[node.id]
    const ref = node.accountCredential?.accessReference || node.id

    // 1. 优先查本地预置口令字典（若有）
    const preset = getPresetSecret(node.id, ref)
    if (preset?.password) {
      setPasswords(prev => ({ ...prev, [node.id]: preset.password! }))
      return preset.password
    }

    // 2. 动态请求后端安全解密接口（支持本地 PostgreSQL 与上游 Bitwarden/FileVault）
    try {
      const res = await api.credentialSecret(ref)
      if (res.password) {
        setPasswords(prev => ({ ...prev, [node.id]: res.password! }))
        return res.password
      }
    } catch {
      // 容错重试 node.id
      try {
        const res2 = await api.credentialSecret(node.id)
        if (res2.password) {
          setPasswords(prev => ({ ...prev, [node.id]: res2.password! }))
          return res2.password
        }
      } catch (e) {
        console.warn('api.credentialSecret retry failed:', e)
      }
    }

    throw new Error('未检索到有效密码值')
  }

  const handleCopyPassword = async (node: Node) => {
    setCopying(prev => ({ ...prev, [node.id]: true }))
    try {
      const pw = await fetchPassword(node)
      const ok = await copyTextToClipboard(pw)
      if (ok) {
        message.success(`已成功复制「${node.name}」密码到剪贴板！`)
      } else {
        message.info(`密码明文：${pw}`, 8)
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '获取密码失败')
    } finally {
      setCopying(prev => ({ ...prev, [node.id]: false }))
    }
  }

  const handleUpload = async (refs: string[]) => {
    if (refs.length === 0) return
    setUploading(true)
    try {
      const res = await api.uploadCredentials(refs, targetUpstream)
      if (res.status === 'ok') {
        const succ = res.report?.succeeded ?? 0
        const fail = res.report?.failed ?? 0
        if (fail === 0) {
          message.success(`成功上传并迁移 ${succ} 个凭据到上游 (${targetUpstream})，本地记录已清除！`)
        } else {
          message.warning(`上传完成: 成功 ${succ} 个，失败 ${fail} 个`)
        }
        if (data && res.report?.results) {
          const resMap = new Map(res.report.results.map(r => [r.ref, r]))
          setData(prev => {
            if (!prev) return prev
            return {
              ...prev,
              credentials: (prev.credentials || []).map(c => {
                const updated = resMap.get(c.id)
                if (updated && updated.success && updated.accessReference) {
                  return {
                    ...c,
                    accountCredential: {
                      ...(c.accountCredential || {}),
                      accessReference: updated.accessReference
                    }
                  }
                }
                return c
              })
            }
          })
        }
        setSelectedCreds([])
        setPasswords({})
        await loadCredStatus()
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '上传至上游失败')
    } finally {
      setUploading(false)
    }
  }

  const handleToggleReveal = async (node: Node) => {
    if (revealed[node.id]) {
      setRevealed(prev => ({ ...prev, [node.id]: false }))
      return
    }
    try {
      await fetchPassword(node)
      setRevealed(prev => ({ ...prev, [node.id]: true }))
    } catch (err) {
      message.error(err instanceof Error ? err.message : '获取密码失败')
    }
  }

  const handleSyncVault = async () => {
    setVaultSyncing(true)
    try {
      const res = await api.syncCredentials()
      message.success(res.msg || '密码库同步成功')
      setPasswords({})
    } catch (err) {
      message.error(err instanceof Error ? err.message : '同步密码库失败')
    } finally {
      setVaultSyncing(false)
    }
  }

  const loadWorkspace = async (id: string, forceDynamic = false) => {
    let resolvedId = id
    const matched = projects.find(p => p.id === id || p.name === id || p.id === `projects/${id}`)
    if (matched) {
      resolvedId = matched.id
    } else if (!resolvedId.startsWith('projects/') && !isUUID(resolvedId)) {
      // 旧约定用 projects/<slug> 寻址；图谱迁移后 ID 是 UUID，再拼前缀会 404。
      resolvedId = `projects/${resolvedId}`
    }

    // 优先固定页面模式（零网络请求、零 loading 等待）
    if (PREFER_FIXED_PAGE && !forceDynamic) {
      const preset = FIXED_PRESETS[id] || FIXED_PRESETS[resolvedId]
      if (preset) {
        setData(preset)
        setIsFixedMode(true)
        setLoading(false)
        if (standalone) {
          document.title = `${preset.project.name} · 项目专享工作区`
        }
        return
      }
    }

    setIsFixedMode(false)
    setLoading(true)
    setError('')
    try {
      try {
        const result = await api.projectWorkspace(resolvedId)
        setData(result)
        if (standalone) {
          document.title = `${result.project.name} · 项目专享工作区 (动态模式)`
        }
      } catch (err) {
        if (resolvedId !== id) {
          const retryResult = await api.projectWorkspace(id)
          setData(retryResult)
          if (standalone) {
            document.title = `${retryResult.project.name} · 项目专享工作区 (动态模式)`
          }
        } else {
          throw err
        }
      }
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
    const matchedProject = projects.find(p => p.id === projectId || p.name === projectId || p.id === `projects/${projectId}`)
    const displayTitle = matchedProject?.name || projectId.replace(/^projects\//, '')
    return (
      <div className="project-workspace">
        <header className="workspace-header">
          <div className="workspace-nav-bar" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Space size="middle">
              {!standalone && (
                <Button icon={<ArrowLeft size={14} />} onClick={onBackToGraph}>
                  返回图谱
                </Button>
              )}
              <span style={{ fontSize: 13, color: '#666' }}>
                <Spin size="small" style={{ marginRight: 8 }} />
                正在聚合项目工作区上下文…
              </span>
            </Space>
          </div>
          <Card style={{ marginTop: 16, marginBottom: 20 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, marginBottom: 16 }}>
              <div style={{ width: 44, height: 44, borderRadius: 10, background: '#eaf0e7', color: '#42633c', display: 'grid', placeItems: 'center', flexShrink: 0 }}>
                <GitBranch size={22} />
              </div>
              <div>
                <div style={{ fontSize: 20, fontWeight: 700, color: '#242821' }}>{displayTitle}</div>
                <div style={{ fontSize: 12, color: '#888', marginTop: 4 }}>ID: {projectId.startsWith('projects/') ? projectId : `projects/${projectId}`}</div>
              </div>
            </div>
            <Skeleton active paragraph={{ rows: 2 }} />
          </Card>
          <Card>
            <Skeleton active paragraph={{ rows: 6 }} />
          </Card>
        </header>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="project-workspace">
        <header className="workspace-header">
          <div className="workspace-nav-bar">
            <Button icon={<ArrowLeft size={14} />} onClick={onBackToGraph}>
              返回图谱
            </Button>
          </div>
          <Alert
            type="error"
            message={error || '未找到项目信息'}
            showIcon
            icon={<CircleHelp size={16} />}
            style={{ marginTop: 16 }}
          />
        </header>
      </div>
    )
  }

  const { project, instances, machines, credentials, documents, workItems, relations } = data
  const projectDetails = project.softwareProject ?? {}

  const runbooks = documents.filter(d => d.type === 'Runbook')
  const generalDocs = documents.filter(d => d.type === 'Document')

  const envGroups = useMemo(
    () => classifyEnvironments(instances, machines, credentials),
    [instances, machines, credentials]
  )

  const renderDocsView = () => (
    <div>
      <Alert
        type="info"
        showIcon
        icon={<LifeBuoy size={16} />}
        message="运维标准化与权威文档"
        description="集中管理部署、重启、巡检 Runbook 及设计交付文档。"
        style={{ marginBottom: 20 }}
      />

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
          <Empty description="当前项目暂未登记 Runbook 操作手册。" />
        ) : (
          <div className="cards-grid">
            {runbooks.map(rb => {
              const rbDetails: Partial<RunbookDetails> = rb.runbook ?? {}
              return (
                <Card
                  key={rb.id}
                  size="small"
                  title={
                    <div>
                      <div style={{ fontWeight: 600, fontSize: 14 }}>{rb.name}</div>
                      <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{rb.id}</div>
                    </div>
                  }
                  extra={<Tag color={rbDetails.approvalRequired ? 'orange' : 'green'}>{rbDetails.approvalRequired ? '需人工审批' : '日常可执行'}</Tag>}
                  actions={[
                    <Button
                      type="link"
                      size="small"
                      icon={<Edit3 size={12} />}
                      key="edit"
                      onClick={() => onOpenDetailsEditor(rb)}
                    >
                      编辑手册配置
                    </Button>
                  ]}
                >
                  <Descriptions size="small" column={1} bordered>
                    <Descriptions.Item label="触发时机">
                      {rbDetails.trigger || '常规运维'}
                    </Descriptions.Item>
                    {rbDetails.preconditions && rbDetails.preconditions.length > 0 && (
                      <Descriptions.Item label="前置确认清单">
                        <ul style={{ margin: '0 0 0 16px', padding: 0 }}>
                          {rbDetails.preconditions.map((p, idx) => (
                            <li key={idx}>{p}</li>
                          ))}
                        </ul>
                      </Descriptions.Item>
                    )}
                    <Descriptions.Item label="操作步骤引用">
                      {rbDetails.stepsReference ? (
                        <code className="code-ref">{rbDetails.stepsReference}</code>
                      ) : (
                        '未登记步骤引用'
                      )}
                    </Descriptions.Item>
                    <Descriptions.Item label="停止条件">
                      {rbDetails.stopConditionReference || '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="回退方案引用">
                      {rbDetails.rollbackReference ? (
                        <code className="code-ref">{rbDetails.rollbackReference}</code>
                      ) : (
                        '—'
                      )}
                    </Descriptions.Item>
                  </Descriptions>
                </Card>
              )
            })}
          </div>
        )}
      </section>

      <section className="workspace-section" style={{ marginTop: 24 }}>
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
          <Empty description="当前项目暂未关联通用技术文档。" />
        ) : (
          <div className="cards-grid">
            {generalDocs.map(doc => {
              const docDetails: Partial<DocumentDetails> = doc.document ?? {}
              return (
                <Card
                  key={doc.id}
                  size="small"
                  title={
                    <div>
                      <div style={{ fontWeight: 600, fontSize: 14 }}>{doc.name}</div>
                      <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{doc.id}</div>
                    </div>
                  }
                  extra={<Tag color={docDetails.effective ? 'blue' : 'default'}>{docDetails.effective ? '当前有效' : '已废弃'}</Tag>}
                  actions={[
                    <Button
                      type="link"
                      size="small"
                      icon={<Edit3 size={12} />}
                      key="edit"
                      onClick={() => onOpenDetailsEditor(doc)}
                    >
                      编辑文档指引
                    </Button>
                  ]}
                >
                  <Descriptions size="small" column={1} bordered>
                    <Descriptions.Item label="文档角色">
                      {docDetails.role || '技术文档'}
                    </Descriptions.Item>
                    <Descriptions.Item label="核验时间">
                      {docDetails.lastVerified ? new Date(docDetails.lastVerified).toLocaleDateString('zh-CN') : '—'}
                    </Descriptions.Item>
                    <Descriptions.Item label="权威文档引用">
                      {docDetails.authoritativeReference ? (
                        <code className="code-ref">{docDetails.authoritativeReference}</code>
                      ) : (
                        '未登记外部引用'
                      )}
                    </Descriptions.Item>
                  </Descriptions>
                </Card>
              )
            })}
          </div>
        )}
      </section>
    </div>
  )

  const renderEnvironmentView = (env: EnvGroup) => {
    const sshCred = env.credentials.find(c => c.id.includes('ssh') || c.name.toLowerCase().includes('ssh'))
    return (
      <div className="env-view-container">
        {/* Environment Banner */}
        <div
          className="env-banner"
          style={{
            background: '#ffffff',
            border: '1px solid #e2e6df',
            borderLeft: `5px solid ${env.color}`,
            padding: '16px 20px',
            borderRadius: 8,
            marginBottom: 20
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
            <div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
                <span style={{ fontSize: 18, fontWeight: 700, color: '#1f241d' }}>{env.name}</span>
                <Tag color={env.tagColor}>{env.badgeName}</Tag>
              </div>
              <div style={{ fontSize: 13, color: '#636b5e', marginTop: 4 }}>{env.description}</div>
            </div>
            <Space size="middle">
              <Tag style={{ padding: '4px 10px', fontSize: 12 }}>{env.instances.length} 个服务实例</Tag>
              <Tag style={{ padding: '4px 10px', fontSize: 12 }}>{env.machines.length} 台计算节点</Tag>
              <Tag style={{ padding: '4px 10px', fontSize: 12 }} color="magenta">{env.credentials.length} 个专属口令</Tag>
            </Space>
          </div>
        </div>

        {/* 1. Host Machines Section */}
        {env.machines.length > 0 && (
          <section className="workspace-section">
            <div className="section-head" style={{ marginBottom: 14 }}>
              <div className="section-title">
                <Server size={16} /> 宿主计算节点与服务器 ({env.machines.length} 台)
              </div>
              <div className="section-subtitle">
                承载当前环境微服务的物理机或虚拟机资源
              </div>
            </div>
            <div className="cards-grid">
              {env.machines.map(m => {
                const mDetails = m.machine ?? {}
                return (
                  <Card
                    key={m.id}
                    size="small"
                    className="env-machine-card"
                    title={
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>{m.name}</div>
                        <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{m.id}</div>
                      </div>
                    }
                    extra={<Tag color="green">{m.lifecycle || 'active'}</Tag>}
                    actions={[
                      <Button
                        type="link"
                        size="small"
                        icon={<Copy size={12} />}
                        key="copy-ip"
                        onClick={async () => {
                          const ipStr = mDetails.ips?.join(', ') || ''
                          if (ipStr) {
                            const ok = await copyTextToClipboard(ipStr)
                            if (ok) {
                              message.success(`已复制主机 IP: ${ipStr}`)
                            } else {
                              message.info(`主机 IP: ${ipStr}`, 6)
                            }
                          }
                        }}
                      >
                        复制主机 IP
                      </Button>,
                      ...(sshCred ? [
                        <Button
                          type="link"
                          size="small"
                          icon={<Lock size={12} />}
                          key="copy-ssh"
                          loading={copying[sshCred.id]}
                          onClick={() => void handleCopyPassword(sshCred)}
                        >
                          复制 SSH 密码
                        </Button>
                      ] : []),
                      <Button
                        type="link"
                        size="small"
                        icon={<Edit3 size={12} />}
                        key="edit"
                        onClick={() => onOpenDetailsEditor(m)}
                      >
                        编辑标注
                      </Button>
                    ]}
                  >
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="IP 地址">
                        <Space wrap>
                          {mDetails.ips && mDetails.ips.length > 0 ? (
                            mDetails.ips.map(ip => (
                              <code key={ip} className="code-ref" style={{ background: '#eef2ec', color: '#2b3327' }}>{ip}</code>
                            ))
                          ) : '未记录 IP'}
                        </Space>
                      </Descriptions.Item>
                      <Descriptions.Item label="主机名">
                        {mDetails.hostname || '—'}
                      </Descriptions.Item>
                      <Descriptions.Item label="系统与配置">
                        {mDetails.os || 'Linux'} {mDetails.machineKind ? `· ${mDetails.machineKind}` : ''} {mDetails.network ? `(${mDetails.network})` : ''}
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                )
              })}
            </div>
          </section>
        )}

        {/* 2. Microservices Section */}
        <section className="workspace-section">
          <div className="section-head" style={{ marginBottom: 14 }}>
            <div className="section-title">
              <Layers3 size={16} /> 环境部署微服务与实例 ({env.instances.length} 个)
            </div>
            <div className="section-subtitle">
              当前环境在线运行的业务组件、连接器与数据节点
            </div>
          </div>
          {env.instances.length === 0 ? (
            <Empty description="该环境下暂无独立运行实例登记。" />
          ) : (
            <div className="cards-grid">
              {env.instances.map(inst => {
                const instDetails = inst.softwareInstance ?? {}
                return (
                  <Card
                    key={inst.id}
                    size="small"
                    title={
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>{inst.name}</div>
                        <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{inst.id}</div>
                      </div>
                    }
                    extra={
                      <Space>
                        <Tag color="green">{instDetails.operationalState || 'running'}</Tag>
                        <Tag color="blue">{instDetails.deploymentState || 'deployed'}</Tag>
                      </Space>
                    }
                    actions={[
                      <Button
                        type="link"
                        size="small"
                        icon={<Edit3 size={12} />}
                        key="edit"
                        onClick={() => onOpenDetailsEditor(inst)}
                      >
                        编辑实例标注
                      </Button>
                    ]}
                  >
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="组件标识">
                        {instDetails.productName || '核心组件'}
                      </Descriptions.Item>
                      <Descriptions.Item label="访问端点 / 端口">
                        {instDetails.endpoints && instDetails.endpoints.length > 0 ? (
                          <Space wrap>
                            {instDetails.endpoints.map((ep, idx) => (
                              <span key={idx} className="env-endpoint-tag">
                                {ep.kind ? `${ep.kind}://` : ''}{ep.address}:{ep.port}
                              </span>
                            ))}
                          </Space>
                        ) : instDetails.ports && instDetails.ports.length > 0 ? (
                          <Space wrap>
                            {instDetails.ports.map(port => (
                              <Tag key={port} style={{ fontFamily: 'monospace' }}>:{port}</Tag>
                            ))}
                          </Space>
                        ) : instDetails.domains && instDetails.domains.length > 0 ? (
                          <Space wrap>
                            {instDetails.domains.map(d => (
                              <Tag key={d} style={{ fontFamily: 'monospace' }}>{d}</Tag>
                            ))}
                          </Space>
                        ) : '内部直接调用'}
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                )
              })}
            </div>
          )}
        </section>

        {/* 3. Credentials Section */}
        {env.credentials.length > 0 && (
          <section className="workspace-section">
            <div className="section-head" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 14 }}>
              <div>
                <div className="section-title">
                  <Lock size={16} /> 环境专属账号与访问密码 ({env.credentials.length} 个)
                </div>
                <div className="section-subtitle">
                  该环境下的管理员控制台、数据库及服务器口令（支持单选或批量上传上游并自动换绑）
                </div>
              </div>
              <Space wrap>
                <span style={{ fontSize: 12, color: '#666' }}>上游目标:</span>
                <Select
                  size="small"
                  value={targetUpstream}
                  onChange={v => setTargetUpstream(v)}
                  options={[
                    { label: '📦 本地文件上游 (离线Mock)', value: 'file' },
                    { label: '☁️ Bitwarden (远程)', value: 'bitwarden' }
                  ]}
                  style={{ width: 195 }}
                />
                <Button
                  size="small"
                  onClick={() => {
                    const envCredIds = env.credentials.map(c => c.id)
                    const allSelected = envCredIds.every(id => selectedCreds.includes(id))
                    if (allSelected) {
                      setSelectedCreds(prev => prev.filter(id => !envCredIds.includes(id)))
                    } else {
                      setSelectedCreds(prev => Array.from(new Set([...prev, ...envCredIds])))
                    }
                  }}
                >
                  {env.credentials.every(c => selectedCreds.includes(c.id)) ? '取消全选' : '全选本环境'}
                </Button>
                {selectedCreds.length > 0 && (
                  <Popconfirm
                    title="批量上传确认"
                    description={`确定将选中的 ${selectedCreds.length} 个凭据加密上传至上游 (${targetUpstream}) 并清除本地数据库记录吗？`}
                    onConfirm={() => void handleUpload(selectedCreds)}
                    okText="确定上传"
                    cancelText="取消"
                  >
                    <Button
                      size="small"
                      type="primary"
                      icon={<UploadCloud size={13} />}
                      loading={uploading}
                      style={{ background: '#2e7d32', borderColor: '#2e7d32' }}
                    >
                      批量上传 ({selectedCreds.length})
                    </Button>
                  </Popconfirm>
                )}
                <Button
                  size="small"
                  icon={<RefreshCw size={13} />}
                  loading={vaultSyncing}
                  onClick={handleSyncVault}
                >
                  重新拉取
                </Button>
              </Space>
            </div>
            <div className="cards-grid">
              {env.credentials.map(c => {
                const cDetails = c.accountCredential ?? {}
                const isSelected = selectedCreds.includes(c.id)
                const isLocal = credStatus[c.id] === 'local' || credStatus[c.name] === 'local' || !cDetails.accessReference || cDetails.accessReference.startsWith('credentials/') || cDetails.accessReference.includes('xasc/')
                return (
                  <Card
                    key={c.id}
                    size="small"
                    style={{
                      borderColor: isSelected ? '#1890ff' : undefined,
                      background: isSelected ? '#f6faff' : undefined
                    }}
                    title={
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <Checkbox
                          checked={isSelected}
                          onChange={e => {
                            const checked = e.target.checked
                            setSelectedCreds(prev => checked ? [...prev, c.id] : prev.filter(x => x !== c.id))
                          }}
                        />
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 14 }}>{c.name}</div>
                          <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{c.id}</div>
                        </div>
                      </div>
                    }
                    extra={
                      <Space size={4}>
                        <Tag color={isLocal ? 'cyan' : 'purple'}>
                          {isLocal ? '💻 本地托管' : '☁️ 上游托管'}
                        </Tag>
                        <Tag color="magenta">{cDetails.credentialType || '账号凭据'}</Tag>
                      </Space>
                    }
                    actions={[
                      <Button
                        type="link"
                        size="small"
                        icon={<Copy size={12} />}
                        key="copy"
                        loading={copying[c.id]}
                        onClick={() => void handleCopyPassword(c)}
                      >
                        复制密码
                      </Button>,
                      <Popconfirm
                        key="upload"
                        title="上传至上游确认"
                        description={`确定将「${c.name}」上传至上游 (${targetUpstream}) 并从本地数据库中清除吗？`}
                        onConfirm={() => void handleUpload([c.id])}
                        okText="上传"
                        cancelText="取消"
                      >
                        <Button
                          type="link"
                          size="small"
                          icon={<UploadCloud size={12} />}
                          loading={uploading && selectedCreds.includes(c.id)}
                        >
                          上传上游
                        </Button>
                      </Popconfirm>,
                      <Button
                        type="link"
                        size="small"
                        icon={<Edit3 size={12} />}
                        key="edit"
                        onClick={() => onOpenDetailsEditor(c)}
                      >
                        维护凭据
                      </Button>
                    ]}
                  >
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="账号标识">
                        <span style={{ fontWeight: 600, color: '#2b3327' }}>
                          {cDetails.account || '默认服务账号'}
                        </span>
                      </Descriptions.Item>
                      <Descriptions.Item label="密码口令">
                        <Space>
                          <span style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontWeight: 600, color: revealed[c.id] ? '#1f241d' : '#888' }}>
                            {revealed[c.id] ? (passwords[c.id] || '••••••••') : '••••••••'}
                          </span>
                          <Button
                            type="text"
                            size="small"
                            icon={revealed[c.id] ? <EyeOff size={13} /> : <Eye size={13} />}
                            onClick={() => void handleToggleReveal(c)}
                            title={revealed[c.id] ? '隐藏口令' : '显示真实口令'}
                          />
                        </Space>
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                )
              })}
            </div>
          </section>
        )}
      </div>
    )
  }

  const findNode = (id: string): Node | undefined => {
    if (project.id === id) return project
    return instances.find(n => n.id === id) ||
      machines.find(n => n.id === id) ||
      credentials.find(n => n.id === id) ||
      documents.find(n => n.id === id) ||
      workItems.find(n => n.id === id)
  }

  const tabItems = [
    {
      key: 'overview',
      label: (
        <span>
          <GitBranch size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
          概览与依赖 <Tag style={{ marginLeft: 4 }}>{relations.length} 关系</Tag>
        </span>
      ),
      children: (
        <div>
          <Alert
            type="info"
            showIcon
            icon={<Edit3 size={16} />}
            message="人工认知协同"
            description="偏向录入或校正 Agent 无法自动推断的业务上下文、架构权衡与设计动机。"
            action={
              <Button size="small" type="primary" onClick={() => onOpenDetailsEditor(project)}>
                补充项目人工标注
              </Button>
            }
            style={{ marginBottom: 20 }}
          />

          <section className="workspace-section">
            <div className="section-head">
              <div>
                <div className="section-title">
                  <TagIcon size={16} /> 关联依赖与拓扑关系
                </div>
                <div className="section-subtitle">
                  直接或间接与当前项目关联的拓扑边（{relations.length} 条已确认关系）
                </div>
              </div>
            </div>

            {relations.length === 0 ? (
              <Empty description="当前项目暂未发现其他关联依赖拓扑。" />
            ) : (
              <List
                bordered
                dataSource={relations}
                renderItem={r => {
                  const fromNode = findNode(r.from)
                  const toNode = findNode(r.to)
                  return (
                    <List.Item
                      key={r.id}
                      extra={
                        <Space>
                          <Tag style={{ fontFamily: 'monospace' }}>{r.id}</Tag>
                          <Tag>{r.source}</Tag>
                          <Tag color="green">{r.certainty}</Tag>
                        </Space>
                      }
                    >
                      <Space size="middle">
                        <strong>{fromNode?.name || r.from}</strong>
                        <Tag color="blue">{r.predicate}</Tag>
                        <strong>{toNode?.name || r.to}</strong>
                      </Space>
                    </List.Item>
                  )
                }}
              />
            )}
          </section>

          {workItems.length > 0 && (
            <section className="workspace-section" style={{ marginTop: 24 }}>
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
                  <Card
                    key={item.id}
                    size="small"
                    title={
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 14 }}>{item.name}</div>
                        <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{item.id}</div>
                      </div>
                    }
                    extra={<Tag color="gold">{item.workItem?.status || '进行中'}</Tag>}
                    actions={[
                      <Button
                        type="link"
                        size="small"
                        icon={<Edit3 size={12} />}
                        key="edit"
                        onClick={() => onOpenDetailsEditor(item)}
                      >
                        编辑事项标注
                      </Button>
                    ]}
                  >
                    <Descriptions size="small" column={1} bordered>
                      <Descriptions.Item label="事项类别">
                        {item.workItem?.kind || '工作事项'}
                      </Descriptions.Item>
                      <Descriptions.Item label="决策理由引用">
                        {item.workItem?.rationaleReference ? (
                          <code className="code-ref">{item.workItem.rationaleReference}</code>
                        ) : (
                          '未记录'
                        )}
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                ))}
              </div>
            </section>
          )}
        </div>
      )
    },
    {
      key: 'dcup',
      label: (
        <span>
          <Server size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
          部署与运行运维 (dcup 视图) <Tag style={{ marginLeft: 4 }}>{instances.length} 实例 · {machines.length} 机器</Tag>
        </span>
      ),
      children: (
        <div>
          <Alert
            type="info"
            showIcon
            icon={<Server size={16} />}
            message="dcup 部署与运行视图"
            description="汇聚当前项目的部署拓扑、宿主机器、监听端口、域名绑定及运行时环境。"
            style={{ marginBottom: 20 }}
          />

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
              <Empty description="当前项目尚未在图谱中登记部署实例（SoftwareInstance）。可通过图谱添加实例节点并关联 implementedFrom。" />
            ) : (
              <div className="cards-grid">
                {instances.map(inst => {
                  const instDetails = inst.softwareInstance ?? {}
                  return (
                    <Card
                      key={inst.id}
                      size="small"
                      title={
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 14 }}>{inst.name}</div>
                          <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{inst.id}</div>
                        </div>
                      }
                      extra={
                        <Space>
                          <Tag color="green">{instDetails.operationalState || 'running'}</Tag>
                          <Tag color="cyan">{instDetails.deploymentState || 'deployed'}</Tag>
                        </Space>
                      }
                      actions={[
                        <Button
                          type="link"
                          size="small"
                          icon={<Edit3 size={12} />}
                          key="edit"
                          onClick={() => onOpenDetailsEditor(inst)}
                        >
                          编辑实例元数据
                        </Button>
                      ]}
                    >
                      <Descriptions size="small" column={1} bordered>
                        <Descriptions.Item label="服务标识">
                          {instDetails.productName || inst.name}
                        </Descriptions.Item>
                        <Descriptions.Item label="部署方式">
                          {instDetails.deploymentMethod || 'Docker / 容器化'}
                        </Descriptions.Item>
                        <Descriptions.Item label="部署路径">
                          {instDetails.deploymentPath ? (
                            <code className="code-ref">{instDetails.deploymentPath}</code>
                          ) : (
                            '未登记路径'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="绑定域名">
                          {instDetails.domains && instDetails.domains.length > 0 ? (
                            instDetails.domains.map((d: string) => (
                              <code key={d} className="code-ref" style={{ marginRight: 4 }}>
                                {d}
                              </code>
                            ))
                          ) : (
                            '未绑定独立域名'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="监听端口">
                          {instDetails.ports && instDetails.ports.length > 0 ? (
                            instDetails.ports.map((p: number) => (
                              <Tag key={p} color="blue">
                                :{p}
                              </Tag>
                            ))
                          ) : (
                            '—'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="运行身份">
                          {instDetails.runtimeIdentity || '非特权服务账号'}
                        </Descriptions.Item>
                        {instDetails.verifiedAt && (
                          <Descriptions.Item label="最近验证">
                            {new Date(instDetails.verifiedAt).toLocaleString('zh-CN')} · 结果: {instDetails.verificationResult || '正常'}
                          </Descriptions.Item>
                        )}
                      </Descriptions>
                    </Card>
                  )
                })}
              </div>
            )}
          </section>

          <section className="workspace-section" style={{ marginTop: 24 }}>
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
              <Empty description="未发现关联的宿主机信息（可能直接运行在 Serverless 或未建立 hosts 关联）。" />
            ) : (
              <div className="cards-grid">
                {machines.map(m => {
                  const mDetails = m.machine ?? {}
                  return (
                    <Card
                      key={m.id}
                      size="small"
                      title={
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 14 }}>{m.name}</div>
                          <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{m.id}</div>
                        </div>
                      }
                      extra={<Tag color="blue">{mDetails.machineKind || 'virtual'}</Tag>}
                      actions={[
                        <Button
                          type="link"
                          size="small"
                          icon={<Edit3 size={12} />}
                          key="edit"
                          onClick={() => onOpenDetailsEditor(m)}
                        >
                          编辑机器运维配置
                        </Button>
                      ]}
                    >
                      <Descriptions size="small" column={1} bordered>
                        <Descriptions.Item label="主机名">
                          {mDetails.hostname || '—'}
                        </Descriptions.Item>
                        <Descriptions.Item label="操作系统">
                          {mDetails.os || 'Linux'}
                        </Descriptions.Item>
                        <Descriptions.Item label="IP 地址">
                          {mDetails.ips && mDetails.ips.length > 0 ? (
                            mDetails.ips.map((ip: string) => (
                              <code key={ip} className="code-ref" style={{ marginRight: 4 }}>
                                {ip}
                              </code>
                            ))
                          ) : (
                            '未分配公网/专网静态 IP'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="所属网络">
                          {mDetails.network || '默认 VPC'}
                        </Descriptions.Item>
                        <Descriptions.Item label="节点状态">
                          {mDetails.status || 'healthy'}
                        </Descriptions.Item>
                        {mDetails.responsibilityBoundary && (
                          <Descriptions.Item label="责任边界">
                            {mDetails.responsibilityBoundary}
                          </Descriptions.Item>
                        )}
                      </Descriptions>
                    </Card>
                  )
                })}
              </div>
            )}
          </section>
        </div>
      )
    },
    {
      key: 'did',
      label: (
        <span>
          <Lock size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
          身份与凭据联动 (did / CMDB 视图) <Tag style={{ marginLeft: 4 }}>{credentials.length} 凭据元信息</Tag>
        </span>
      ),
      children: (
        <div>
          <section className="workspace-section">
            <div className="section-head" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <div>
                <div className="section-title">
                  <Lock size={16} /> 关联凭据元信息与密码管理器
                </div>
                <div className="section-subtitle">
                  已绑定的访问凭据与 Vaultwarden 真实口令映射（支持一键复制与显隐）
                </div>
              </div>
              <Button
                icon={<RefreshCw size={13} />}
                loading={vaultSyncing}
                onClick={handleSyncVault}
              >
                从 Vaultwarden 重新拉取
              </Button>
            </div>

            {credentials.length === 0 ? (
              <Empty description="当前项目及其部署环境尚未关联凭据元信息。若需登记 CMDB/Vault 访问指针，可在此人工补充。" />
            ) : (
              <div className="cards-grid">
                {credentials.map(c => {
                  const cDetails = c.accountCredential ?? {}
                  return (
                    <Card
                      key={c.id}
                      size="small"
                      title={
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 14 }}>{c.name}</div>
                          <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{c.id}</div>
                        </div>
                      }
                      extra={<Tag color="magenta">{cDetails.credentialType || '账号凭据'}</Tag>}
                      actions={[
                        <Button
                          type="link"
                          size="small"
                          icon={<Copy size={12} />}
                          key="copy"
                          loading={copying[c.id]}
                          onClick={() => void handleCopyPassword(c)}
                        >
                          复制密码
                        </Button>,
                        <Button
                          type="link"
                          size="small"
                          icon={<Edit3 size={12} />}
                          key="edit"
                          onClick={() => onOpenDetailsEditor(c)}
                        >
                          维护凭据
                        </Button>
                      ]}
                    >
                      <Descriptions size="small" column={1} bordered>
                        <Descriptions.Item label="账号标识">
                          {cDetails.account || '默认服务账号'}
                        </Descriptions.Item>
                        <Descriptions.Item label="密码口令">
                          <Space>
                            <span style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontWeight: 600, color: revealed[c.id] ? '#1f241d' : '#888' }}>
                              {revealed[c.id] ? (passwords[c.id] || '••••••••') : '••••••••'}
                            </span>
                            <Button
                              type="text"
                              size="small"
                              icon={revealed[c.id] ? <EyeOff size={13} /> : <Eye size={13} />}
                              onClick={() => void handleToggleReveal(c)}
                              title={revealed[c.id] ? '隐藏密码' : '显示密码明文'}
                            />
                            <Button
                              size="small"
                              type="primary"
                              icon={<Copy size={12} />}
                              loading={copying[c.id]}
                              onClick={() => void handleCopyPassword(c)}
                              style={{ background: '#42633c', borderColor: '#42633c', marginLeft: 4 }}
                            >
                              复制密码
                            </Button>
                          </Space>
                        </Descriptions.Item>
                        <Descriptions.Item label="权限范围">
                          {cDetails.scope || '项目专有'}
                        </Descriptions.Item>
                        <Descriptions.Item label="用途说明">
                          {cDetails.purpose || '未填写用途说明'}
                        </Descriptions.Item>
                        <Descriptions.Item label="安全访问指针">
                          {cDetails.accessReference ? (
                            <code className="code-ref">{cDetails.accessReference}</code>
                          ) : (
                            '未配置指针引用'
                          )}
                        </Descriptions.Item>
                        <Descriptions.Item label="轮换周期">
                          {cDetails.rotationAt ? new Date(cDetails.rotationAt).toLocaleDateString('zh-CN') : '未设定'}
                        </Descriptions.Item>
                        <Descriptions.Item label="核验状态">
                          <Tag color="cyan">{c.certainty}</Tag>
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  )
                })}
              </div>
            )}
          </section>
        </div>
      )
    },
    {
      key: 'docs',
      label: (
        <span>
          <LifeBuoy size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
          运维手册与文档 (Runbooks & Docs) <Tag style={{ marginLeft: 4 }}>{documents.length} 文档/手册 · {workItems.length} 事项</Tag>
        </span>
      ),
      children: renderDocsView()
    }
  ]

  const envTabItems = [
    ...envGroups.map(env => ({
      key: env.key,
      label: (
        <span>
          <span style={{ marginRight: 6 }}>
            {env.key === 'prod' ? '🌲' : env.key === 'test' ? '🧪' : env.key === 'c0130' ? '🏢' : '🛡️'}
          </span>
          {env.shortName}
          <Tag style={{ marginLeft: 6, fontSize: 11 }} color={env.tagColor}>
            {env.instances.length} 实例
          </Tag>
        </span>
      ),
      children: renderEnvironmentView(env)
    })),
    {
      key: 'docs',
      label: (
        <span>
          <FileText size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
          项目文档与应急手册 <Tag style={{ marginLeft: 4 }}>{documents.length} 篇</Tag>
        </span>
      ),
      children: renderDocsView()
    }
  ]

  return (
    <div className="project-workspace">
      {/* Top Header & Switcher */}
      <header className="workspace-header">
        <div className="workspace-nav-bar" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Space size="middle">
            {!standalone && (
              <Button icon={<ArrowLeft size={14} />} onClick={onBackToGraph}>
                返回图谱
              </Button>
            )}
            {!standalone && projects.length > 0 && onSelectProject && (
              <Space>
                <span style={{ fontSize: 12, color: '#666' }}>切换项目:</span>
                <Select
                  style={{ width: 260 }}
                  value={projectId}
                  onChange={onSelectProject}
                  options={projects.map(p => ({
                    label: `${p.name} (${p.id})`,
                    value: p.id
                  }))}
                />
              </Space>
            )}
          </Space>
          <Space>
            <Button
              style={{ borderColor: '#42633c', color: '#42633c', fontWeight: 600 }}
              icon={<ShieldCheck size={13} />}
              onClick={() => window.open(`/view/projects/${encodeURIComponent(projectId)}`, '_blank')}
              title="打开纯粹只读信息速查页面，专为快速复制密码与多模板展现设计"
            >
              👁️ 只读信息速查
            </Button>
            {!standalone && (
              <Button
                icon={<ExternalLink size={13} />}
                onClick={() => window.open(`/project/${encodeURIComponent(projectId)}`, '_blank')}
                title="在新标签页中打开专属项目页 (无全局菜单)"
              >
                在新页面打开
              </Button>
            )}
            <Button
              icon={<RefreshCw size={13} />}
              onClick={() => void loadWorkspace(projectId, true)}
              title="强制向后端发起动态图谱同步"
            >
              {isFixedMode ? '同步动态数据' : '刷新'}
            </Button>
            <Button
              type="primary"
              icon={<Edit3 size={13} />}
              onClick={() => onOpenDetailsEditor(project)}
            >
              人工标注此项目
            </Button>
          </Space>
        </div>

        {/* Project Hero Card */}
        <Card style={{ marginTop: 16, marginBottom: 20 }}>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, marginBottom: 16 }}>
            <div style={{ width: 44, height: 44, borderRadius: 10, background: '#eaf0e7', color: '#42633c', display: 'grid', placeItems: 'center', flexShrink: 0 }}>
              <GitBranch size={22} />
            </div>
            <div>
              <div style={{ fontSize: 20, fontWeight: 700, color: '#242821' }}>{project.name}</div>
              <Space wrap style={{ marginTop: 6 }}>
                <Tag style={{ fontFamily: 'monospace' }}>ID: {project.id}</Tag>
                {isFixedMode ? (
                  <Tag color="blue">固定页面模式</Tag>
                ) : (
                  <Tag color="green">动态图谱模式</Tag>
                )}
                <Tag color="gold">{projectDetails.form || '服务工程'}</Tag>
                <Tag color="green">维护状态: {projectDetails.maintenanceState || project.lifecycle || 'active'}</Tag>
                <Tag color="cyan">确定性: {project.certainty}</Tag>
                <Tag>来源: {project.source}</Tag>
              </Space>
            </div>
          </div>

          <Descriptions size="small" bordered column={{ xs: 1, sm: 2, md: 2 }}>
            <Descriptions.Item label="业务用途 / 业务定位">
              {projectDetails.purpose || '（暂无业务说明，建议人工补充）'}
            </Descriptions.Item>
            <Descriptions.Item label="主要开发语言">
              {projectDetails.languages && projectDetails.languages.length > 0
                ? projectDetails.languages.join(', ')
                : '未登记'}
            </Descriptions.Item>
            <Descriptions.Item label="远程仓库代码库">
              {projectDetails.remoteRepository ? (
                <code className="code-ref">{projectDetails.remoteRepository}</code>
              ) : (
                '未登记'
              )}
            </Descriptions.Item>
            <Descriptions.Item label="上游依赖 / 继任状态">
              {projectDetails.upstream || '无上游'}
              {projectDetails.successorId && ` (继任者: ${projectDetails.successorId})`}
            </Descriptions.Item>
          </Descriptions>
        </Card>

        {/* View Mode Switcher */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16, flexWrap: 'wrap', gap: 12 }}>
          <Space size="middle">
            <span style={{ fontSize: 13, fontWeight: 600, color: '#2b3327' }}>展示维度:</span>
            <Segmented
              options={[
                { label: <span>🏢 按环境组织展示 (推荐)</span>, value: 'by-env' },
                { label: <span>📑 全量平铺直铺展示</span>, value: 'raw' }
              ]}
              value={viewMode}
              onChange={v => setViewMode(v as 'by-env' | 'raw')}
            />
          </Space>
          <span style={{ fontSize: 12, color: '#636b5e' }}>
            {viewMode === 'by-env'
              ? `已为您聚合 ${envGroups.length} 个运行环境（生产、测试、现场专区、公共中枢），微服务与口令按环境独立归类`
              : `当前处于全量直铺模式，直铺展示全部 ${instances.length} 个实例、${machines.length} 台机器及 ${credentials.length} 个凭据`}
          </span>
        </div>

        {/* Main Workspace Area */}
        {viewMode === 'by-env' ? (
          <div>
            <Tabs
              activeKey={activeEnvTab}
              onChange={k => setActiveEnvTab(k)}
              items={envTabItems}
              type="card"
              size="middle"
            />
            {/* 现存全量直铺展示弱化折叠栏 (满足：现存的展示可以保留,但是可以弱化一下,例如点击才展示) */}
            <Collapse
              className="env-raw-collapse"
              ghost
              items={[{
                key: 'raw-peek',
                label: (
                  <span style={{ fontSize: 13, color: '#636b5e', cursor: 'pointer' }}>
                    <Layers3 size={14} style={{ verticalAlign: -2, marginRight: 6 }} />
                    点击展开现存全量直铺展示 (原始 {instances.length} 实例平铺 · {machines.length} 机器 · {credentials.length} 凭据 · {relations.length} 原始拓扑边)
                  </span>
                ),
                children: (
                  <div style={{ paddingTop: 10 }}>
                    <Tabs
                      activeKey={activeTab}
                      onChange={k => setActiveTab(k as TabKey)}
                      items={tabItems}
                      type="card"
                      size="middle"
                    />
                  </div>
                )
              }]}
            />
          </div>
        ) : (
          <Tabs
            activeKey={activeTab}
            onChange={k => setActiveTab(k as TabKey)}
            items={tabItems}
            type="card"
            size="middle"
          />
        )}
      </header>
    </div>
  )
}
