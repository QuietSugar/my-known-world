import { useEffect, useMemo, useState } from 'react'
import {
  Activity,
  ArrowDownRight,
  ArrowLeft,
  ArrowRight,
  Box,
  ChevronDown,
  CircleHelp,
  Cloud,
  Command,
  Database,
  ExternalLink,
  FileText,
  FolderTree,
  GitBranch,
  HardDrive,
  Layers3,
  LifeBuoy,
  Maximize2,
  Network,
  Plus,
  Settings2,
  ShieldCheck,
  Sparkles,
  TerminalSquare
} from 'lucide-react'
import {
  api,
  type ChangeEvent,
  type FieldResolution,
  type FieldAssertionView,
  type Impact,
  type Node,
  type NodeType,
  type Relation,
  type SyncPreview,
  type SyncRun,
  type SyncSource,
  type SourcePolicy,
  type ManualMutation,
  type MutationPlan
} from './api'
import { GraphExplorer } from './GraphExplorer'
import { FolderBrowser } from './FolderBrowser'
import { DetailsEditor } from './DetailsEditor'
import { ProjectWorkspace } from './ProjectWorkspace'
import { ProjectReadOnlyView } from './ProjectReadOnlyView'
import { DocumentationView } from './DocumentationView'
import {
  App as AntdApp,
  Button,
  Card,
  Descriptions,
  Drawer,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Timeline,
  Tooltip,
  Alert,
  type TableColumnsType
} from 'antd'
import { WriteTokenModal } from './WriteTokenModal'
import { EditNameModal } from './EditNameModal'
import { antdColorForType } from './theme'

type View = 'overview' | 'all' | 'browse' | 'impact' | 'provisional' | 'sources' | 'sync' | 'explore' | 'workspace' | 'docs' | 'project-focus' | 'project-readonly'

const labels: Record<NodeType, string> = {
  Self: '我自己',
  Hardware: '硬件',
  Machine: '机器',
  SoftwareProject: '软件项目',
  SoftwareInstance: '部署实例',
  RunningPeriod: '运行周期',
  AccountCredential: '凭据元信息',
  Product: '产品',
  Document: '文档',
  Runbook: 'Runbook',
  WorkItem: '工作事项',
  Network: '网络',
  ProvisionalObject: '待分类'
}

const navTypes: NodeType[] = ['Hardware', 'Machine', 'SoftwareProject', 'SoftwareInstance', 'AccountCredential', 'Document', 'WorkItem', 'Network']
const stableTypes: NodeType[] = ['Hardware', 'Machine', 'SoftwareProject', 'SoftwareInstance', 'RunningPeriod', 'AccountCredential', 'Product', 'Document', 'Runbook', 'WorkItem', 'Network']

const iconFor: Record<NodeType, typeof Box> = {
  Self: Sparkles,
  Hardware: HardDrive,
  Machine: TerminalSquare,
  SoftwareProject: GitBranch,
  SoftwareInstance: Layers3,
  RunningPeriod: Activity,
  AccountCredential: ShieldCheck,
  Product: Box,
  Document: FileText,
  Runbook: LifeBuoy,
  WorkItem: CircleHelp,
  Network: Network,
  ProvisionalObject: CircleHelp
}

const typeColor: Record<NodeType, string> = {
  Self: 'violet',
  Hardware: 'blue',
  Machine: 'green',
  SoftwareProject: 'amber',
  SoftwareInstance: 'teal',
  RunningPeriod: 'teal',
  AccountCredential: 'rose',
  Product: 'blue',
  Document: 'slate',
  Runbook: 'amber',
  WorkItem: 'violet',
  Network: 'teal',
  ProvisionalObject: 'rose'
}

const evidenceLabel = { unknown: '未知', manual: '人工确认', document: '文档声明', observed: '现场核验', derived: '系统派生', imported: '导入' }
const certaintyLabel = { unknown: '未知', provisional: '暂定', confirmed: '已确认', disputed: '有冲突', stale: '已过期' }

async function previewAndConfirmMutation(mutation: ManualMutation): Promise<MutationPlan | null> {
  const plan = await api.previewMutation(mutation)
  const d = plan.delta
  const changes = [
    ['新增对象', d.addedNodes],
    ['更新对象', d.updatedNodes],
    ['删除对象', d.deletedNodes],
    ['新增关系', d.addedRelations],
    ['更新关系', d.updatedRelations],
    ['删除关系', d.deletedRelations],
    ['新增断言', d.addedAssertions],
    ['删除断言', d.deletedAssertions]
  ].filter(([, ids]) => (ids as string[]).length > 0).map(([label, ids]) => `${label}：${(ids as string[]).join('、')}`).join('\n')

  if (!changes) {
    window.alert('预览结果没有实际变更。')
    return null
  }
  return window.confirm(
    `服务端预览的变更：\n${changes}\n\n本次提交内容：\n${JSON.stringify(mutation, null, 2)}\n\n图谱版本：${plan.expectedRevision.slice(0, 12)}…\n仅确认以上确切内容后应用；不要写入秘密或正文。`
  ) ? plan : null
}

const getInitialProjectReadOnlyId = (): string | null => {
  const path = window.location.pathname
  if (path.startsWith('/view/projects/')) {
    const raw = path.replace(/^\/view\/projects\/?/, '').replace(/\/$/, '')
    return raw ? decodeURIComponent(raw) : null
  }
  if (path.startsWith('/view/project/')) {
    const raw = path.replace(/^\/view\/project\/?/, '').replace(/\/$/, '')
    return raw ? decodeURIComponent(raw) : null
  }
  const params = new URLSearchParams(window.location.search)
  if (params.get('mode') === 'readonly' && params.get('project')) {
    return params.get('project')
  }
  return null
}

const getInitialProjectFocusId = (): string | null => {
  const path = window.location.pathname
  if (path.startsWith('/project/')) {
    const raw = path.replace(/^\/project\/?/, '').replace(/\/$/, '')
    return raw ? decodeURIComponent(raw) : null
  }
  const params = new URLSearchParams(window.location.search)
  const p = params.get('project')
  if (p) return p
  return null
}

const getInitialView = (): View => {
  if (getInitialProjectReadOnlyId()) return 'project-readonly'
  if (getInitialProjectFocusId()) return 'project-focus'
  if (window.location.pathname.startsWith('/docs')) return 'docs'
  const params = new URLSearchParams(window.location.search)
  const v = params.get('view') as View
  const validViews: View[] = [
    'overview', 'all', 'browse', 'impact', 'provisional', 'sources',
    'sync', 'explore', 'workspace', 'docs', 'project-focus', 'project-readonly'
  ]
  if (validViews.includes(v)) return v
  if (window.location.hash === '#docs') return 'docs'
  return 'overview'
}

export function App() {
  const { message } = AntdApp.useApp()
  const [nodes, setNodes] = useState<Node[]>([])
  const [relations, setRelations] = useState<Relation[]>([])
  const [impacts, setImpacts] = useState<Impact[]>([])
  const [provisionals, setProvisionals] = useState<Node[]>([])
  const [selected, setSelected] = useState<Node | null>(null)
  const [detailsEditorNode, setDetailsEditorNode] = useState<Node | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [tokenModalOpen, setTokenModalOpen] = useState(false)
  const [editNameNode, setEditNameNode] = useState<Node | null>(null)
  const [path, setPath] = useState<{ nodes: Node[]; relations: Relation[] } | null>(null)
  const [assertions, setAssertions] = useState<FieldAssertionView[]>([])
  const [entityHistory, setEntityHistory] = useState<ChangeEvent[]>([])
  const [fieldResolutions, setFieldResolutions] = useState<FieldResolution[]>([])
  const [view, setView] = useState<View>(getInitialView)
  const [selectedProjectId, setSelectedProjectId] = useState('')
  const [focusedProjectId, setFocusedProjectId] = useState<string | null>(getInitialProjectFocusId)
  const [readOnlyProjectId, setReadOnlyProjectId] = useState<string | null>(getInitialProjectReadOnlyId)
  const softwareProjects = useMemo(() => nodes.filter(item => item.type === 'SoftwareProject'), [nodes])
  const [filter, setFilter] = useState<NodeType | 'all'>('all')
  const [query, setQuery] = useState('')
  const [online, setOnline] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [reviewTarget, setReviewTarget] = useState<NodeType>('Machine')
  const [syncSources, setSyncSources] = useState<SyncSource[]>([])
  const [syncHistory, setSyncHistory] = useState<SyncRun[]>([])
  const [selectedSource, setSelectedSource] = useState('')

  const refresh = async () => {
    const [nodeResult, relationResult, impactResult, provisionalResult] = await Promise.all([
      api.nodes(),
      api.relations(),
      api.impact(),
      api.provisionals()
    ])
    setNodes(nodeResult.items)
    setRelations(relationResult.items)
    setImpacts(impactResult.items)
    setProvisionals(provisionalResult.items)
    setSelected(current => (current ? nodeResult.items.find(node => node.id === current.id) ?? null : null))
    setOnline(true)
    setError('')
  }

  useEffect(() => {
    const handlePopState = () => {
      const pid = getInitialProjectFocusId()
      if (pid) {
        setFocusedProjectId(pid)
        setView('project-focus')
      } else {
        const v = getInitialView()
        setView(v)
      }
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    let active = true
    Promise.all([api.nodes(), api.relations(), api.impact(), api.provisionals(), api.health()])
      .then(([nodeResult, relationResult, impactResult, provisionalResult]) => {
        if (active) {
          setNodes(nodeResult.items)
          setRelations(relationResult.items)
          setImpacts(impactResult.items)
          setProvisionals(provisionalResult.items)
          setOnline(true)
          setError('')
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setOnline(false)
          setError(cause instanceof Error ? cause.message : '无法连接 API')
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [])

  useEffect(() => {
    const handlePopState = () => {
      setView(getInitialView())
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    if (selected?.candidateType) setReviewTarget(selected.candidateType)
  }, [selected])

  useEffect(() => {
    if (view !== 'sources') return
    let active = true
    api.syncSources()
      .then(result => {
        if (active) {
          setSyncSources(result.items)
          if (result.items.length && !selectedSource) setSelectedSource(result.items[0].source)
        }
      })
      .catch(cause => {
        if (active) setError(cause instanceof Error ? cause.message : '来源历史读取失败')
      })
    return () => {
      active = false
    }
  }, [view, selectedSource])

  useEffect(() => {
    if (view !== 'sources' || !selectedSource) {
      setSyncHistory([])
      return
    }
    let active = true
    api.syncHistory(selectedSource)
      .then(result => {
        if (active) setSyncHistory(result.items)
      })
      .catch(cause => {
        if (active) setError(cause instanceof Error ? cause.message : '同步历史读取失败')
      })
    return () => {
      active = false
    }
  }, [view, selectedSource])

  const saveTypeDetails = async (updated: Node) => {
    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      setTokenModalOpen(true)
      return
    }
    try {
      const mutation: ManualMutation = { kind: 'updateNode', id: updated.id, node: updated }
      const plan = await previewAndConfirmMutation(mutation)
      if (!plan) return
      await api.applyMutation(mutation, plan, token)
      setDetailsEditorNode(null)
      setSelected(updated)
      await refresh()
      message.success('类型详情已成功保存')
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      throw cause
    }
  }

  const resolveFieldAssertion = async (assertion: FieldAssertionView) => {
    const rationaleReference = window.prompt(`为 ${assertion.fieldPath} 选择此断言。请输入安全的决策/依据引用（不要输入秘密或原始敏感值）：`)
    if (!rationaleReference?.trim()) return
    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      setTokenModalOpen(true)
      return
    }
    try {
      const decision = { nodeId: assertion.nodeId, fieldPath: assertion.fieldPath, selectedAssertionIds: [assertion.id], rationaleReference }
      const preview = await api.previewFieldResolution(decision)
      if (
        !window.confirm(
          `服务端预览：为 ${assertion.nodeId} · ${assertion.fieldPath} 追加裁决，选择断言 ${assertion.id}（来源：${assertion.source}，证据：${evidenceLabel[assertion.evidenceKind]}，确定性：${certaintyLabel[assertion.certainty]}）。原始断言不会修改。版本 ${preview.plan.expectedRevision.slice(0, 12)}…\n继续应用？`
        )
      ) return
      const resolution = await api.createFieldResolution(decision, preview.plan, token)
      setFieldResolutions(old => [resolution, ...old.filter(item => item.id !== resolution.id)])
      setAssertions(old =>
        old.map(item =>
          item.fieldPath === assertion.fieldPath
            ? { ...item, resolved: true, selected: item.id === assertion.id, resolutionId: resolution.id }
            : item
        )
      )
      message.success('字段裁决已追加；原始断言仍完整保留。')
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      setError(cause instanceof Error ? cause.message : '字段裁决失败')
    }
  }

  const reviewAction = async (node: Node, action: 'classify' | 'reject') => {
    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      setTokenModalOpen(true)
      return
    }
    try {
      if (action === 'classify') {
        const certainty = window.prompt('确定性：unknown / provisional / confirmed / disputed / stale', node.certainty || 'confirmed')
        if (certainty === null) return
        const source = window.prompt('证据来源（可留空以保留原来源）', '')
        if (source === null) return
        const reference = window.prompt('证据引用（可留空；仅填安全引用，不填秘密或正文）', '')
        if (reference === null) return
        const mutation: ManualMutation = {
          kind: 'reviewProvisional',
          id: node.id,
          targetType: reviewTarget,
          reviewEvidence: {
            certainty: certainty as import('./api').Certainty,
            source: source.trim() || undefined,
            reference: reference.trim() || undefined
          }
        }
        const plan = await previewAndConfirmMutation(mutation)
        if (!plan) return
        await api.applyMutation(mutation, plan, token)
        message.success('已转为明确对象')
      } else {
        const mutation: ManualMutation = { kind: 'rejectProvisional', id: node.id }
        const plan = await previewAndConfirmMutation(mutation)
        if (!plan) return
        await api.applyMutation(mutation, plan, token)
        message.info('已保留并拒绝分类')
      }
      await refresh()
      setSelected(null)
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      setError(cause instanceof Error ? cause.message : '审查操作失败')
    }
  }

  useEffect(() => {
    if (!selected) {
      setPath(null)
      setAssertions([])
      setEntityHistory([])
      setFieldResolutions([])
      return
    }
    let active = true
    api.path(selected.id)
      .then(result => { if (active) setPath(result) })
      .catch(() => { if (active) setPath(null) })
    api.assertions(selected.id)
      .then(result => { if (active) setAssertions(result.items) })
      .catch(() => { if (active) setAssertions([]) })
    api.entityHistory('node', selected.id)
      .then(result => { if (active) setEntityHistory(result.items) })
      .catch(() => { if (active) setEntityHistory([]) })
    api.fieldResolutions(selected.id)
      .then(result => { if (active) setFieldResolutions(result.items) })
      .catch(() => { if (active) setFieldResolutions([]) })
    return () => {
      active = false
    }
  }, [selected])

  const self = nodes.find(node => node.type === 'Self')
  const provisionalIds = useMemo(() => new Set(provisionals.map(node => node.id)), [provisionals])
  const visibleNodes = useMemo(() => nodes.filter(node => {
    const matchesFilter = filter === 'all' || node.type === filter
    const matchesQuery = `${node.name} ${node.type} ${node.id}`.toLowerCase().includes(query.toLowerCase())
    const matchesView = view !== 'provisional' || provisionalIds.has(node.id)
    return node.type !== 'Self' && matchesFilter && matchesQuery && matchesView
  }), [nodes, filter, query, view, provisionalIds])

  const recent = [...nodes].filter(node => node.lastConfirmed).sort((a, b) => (b.lastConfirmed ?? '').localeCompare(a.lastConfirmed ?? '')).slice(0, 5)
  const counts = new Map<NodeType, number>()
  nodes.forEach(node => counts.set(node.type, (counts.get(node.type) ?? 0) + 1))
  const impactIds = new Set(impacts.map(item => item.node.id))
  const focus = (node: Node) => { setSelected(node) }

  if (view === 'docs') {
    const slug = window.location.pathname.startsWith('/docs')
      ? window.location.pathname.replace(/^\/docs\/?/, '').split('/')[0]
      : undefined
    return (
      <DocumentationView
        initialSlug={slug}
        onBackToGraph={() => {
          if (window.opener && window.history.length <= 1) {
            window.close()
          } else {
            window.history.pushState({}, '', '/')
            setView('overview')
          }
        }}
      />
    )
  }

  if (view === 'project-readonly' && readOnlyProjectId) {
    return (
      <ProjectReadOnlyView
        projectId={readOnlyProjectId}
        projects={softwareProjects}
        onBackToGraph={() => {
          if (window.opener && window.history.length <= 1) {
            window.close()
          } else {
            window.history.pushState({}, '', '/')
            setReadOnlyProjectId(null)
            setView('overview')
          }
        }}
        onSwitchToWorkspace={() => {
          window.history.pushState({}, '', `/project/${encodeURIComponent(readOnlyProjectId)}`)
          setFocusedProjectId(readOnlyProjectId)
          setReadOnlyProjectId(null)
          setView('project-focus')
        }}
      />
    )
  }

  if (view === 'project-focus' && focusedProjectId) {
    const projName = softwareProjects.find(p => p.id === focusedProjectId)?.name || focusedProjectId
    return (
      <div className="standalone-project-container" style={{ minHeight: '100vh', background: '#f8faf7' }}>
        <header style={{
          height: 54,
          background: '#fff',
          borderBottom: '1px solid #e2e8e0',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          position: 'sticky',
          top: 0,
          zIndex: 100,
          boxShadow: '0 1px 3px rgba(0,0,0,0.02)'
        }}>
          <Space size="middle">
            <Button
              icon={<ArrowLeft size={14} />}
              onClick={() => {
                if (window.opener && window.history.length <= 1) {
                  window.close()
                } else {
                  window.history.pushState({}, '', '/')
                  setFocusedProjectId(null)
                  setView('overview')
                }
              }}
            >
              返回图谱全景
            </Button>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{
                width: 30,
                height: 30,
                borderRadius: 8,
                background: '#eaf0e7',
                color: '#42633c',
                display: 'grid',
                placeItems: 'center',
                flexShrink: 0
              }}>
                <GitBranch size={17} />
              </div>
              <strong style={{ fontSize: 16, color: '#1f241d' }}>{projName}</strong>
              <Tag color="green">独立项目模式 (无全局菜单)</Tag>
            </div>
          </Space>
          <Space size="middle">
            <Button
              style={{ borderColor: '#42633c', color: '#42633c', fontWeight: 600 }}
              icon={<ShieldCheck size={14} />}
              onClick={() => {
                window.history.pushState({}, '', `/view/projects/${encodeURIComponent(focusedProjectId)}`)
                setReadOnlyProjectId(focusedProjectId)
                setFocusedProjectId(null)
                setView('project-readonly')
              }}
            >
              👁️ 只读信息速查
            </Button>
            <div className={`connection ${online ? 'connected' : ''}`}>
              <span />{online ? 'API 已连接' : 'API 离线'}
            </div>
            <Button
              icon={<CircleHelp size={14} />}
              onClick={() => window.open('/docs/', '_blank')}
            >
              帮助与文档
            </Button>
          </Space>
        </header>

        <main style={{ maxWidth: 1400, margin: '0 auto', padding: '24px 24px 60px' }}>
          <ProjectWorkspace
            projectId={focusedProjectId}
            projects={softwareProjects}
            standalone={true}
            onBackToGraph={() => {
              if (window.opener && window.history.length <= 1) {
                window.close()
              } else {
                window.history.pushState({}, '', '/')
                setFocusedProjectId(null)
                setView('overview')
              }
            }}
            onOpenDetailsEditor={node => setDetailsEditorNode(node)}
          />
        </main>

        {detailsEditorNode && (
          <DetailsEditor
            node={detailsEditorNode}
            onClose={() => setDetailsEditorNode(null)}
            onSave={saveTypeDetails}
          />
        )}
      </div>
    )
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-mark"><Network size={19} strokeWidth={2.3} /></div>
          <span>my<span className="brand-light">known</span>world</span>
          <button className="workspace-switch" aria-label="切换工作区"><ChevronDown size={14} /></button>
        </div>
        <div className="workspace">
          <div className="workspace-avatar">M</div>
          <div><strong>我的世界</strong><small>个人认知图谱</small></div>
          <span className={`status-dot ${online ? 'on' : ''}`} title={online ? 'API 在线' : 'API 离线'} />
        </div>
        <div className="side-label">工作区</div>
        <button className={`nav-item ${view === 'overview' ? 'active' : ''}`} onClick={() => { setView('overview'); setFilter('all') }}>
          <Layers3 size={17} /><span>总览</span><span className="nav-kbd">⌘ 1</span>
        </button>
        <button className={`nav-item ${view === 'all' ? 'active' : ''}`} onClick={() => { setView('all'); setFilter('all') }}>
          <Database size={17} /><span>全部对象</span><span className="nav-count">{Math.max(0, nodes.length - 1)}</span>
        </button>
        <button className={`nav-item ${view === 'browse' ? 'active' : ''}`} onClick={() => setView('browse')}>
          <FolderTree size={17} /><span>目录浏览</span>
        </button>
        <button className={`nav-item ${view === 'impact' ? 'active' : ''}`} onClick={() => { setView('impact'); setFilter('all') }}>
          <Activity size={17} /><span>影响范围</span>
        </button>
        <button className={`nav-item ${view === 'explore' ? 'active' : ''}`} onClick={() => setView('explore')}>
          <Network size={17} /><span>关系图探索</span>
        </button>
        <button className={`nav-item ${view === 'provisional' ? 'active' : ''}`} onClick={() => { setView('provisional'); setFilter('all') }}>
          <CircleHelp size={17} /><span>待审查队列</span><span className="nav-count">{provisionals.length}</span>
        </button>
        <button className={`nav-item ${view === 'sync' ? 'active' : ''}`} onClick={() => setView('sync')}>
          <ArrowDownRight size={17} /><span>批量同步</span>
        </button>
        <button className={`nav-item ${view === 'sources' ? 'active' : ''}`} onClick={() => setView('sources')}>
          <Database size={17} /><span>来源与同步历史</span>
        </button>
        <button className={`nav-item ${view === 'workspace' ? 'active' : ''}`} onClick={() => {
          if (!selectedProjectId && softwareProjects.length > 0) {
            setSelectedProjectId(softwareProjects[0].id)
          }
          setView('workspace')
        }}>
          <GitBranch size={17} /><span>项目工作区</span><span className="nav-count">{softwareProjects.length}</span>
        </button>
        <div className="side-label category-label">分类 <button aria-label="分类设置"><Settings2 size={14} /></button></div>
        {navTypes.map(type => (
          <button key={type} className={`nav-item sub-item ${filter === type ? 'active' : ''}`} onClick={() => { setFilter(type); setView('all') }}>
            <span className={`type-dot ${typeColor[type]}`} />
            <span>{labels[type]}</span>
            <span className="nav-count">{counts.get(type) ?? 0}</span>
          </button>
        ))}
        <div className="sidebar-bottom">
          <div className="storage-card">
            <div className="storage-top"><span className="storage-icon"><Cloud size={15} /></span><span>{online ? '数据连接正常' : '等待 API 连接'}</span></div>
            <p>{online ? '图谱连接正常' : '检查本地 API 服务'}</p>
            <div className="storage-track"><span style={{ width: online ? '72%' : '8%' }} /></div>
            <small>{online ? 'PostgreSQL / API' : '离线模式'}</small>
          </div>
          <a
            href="/docs/"
            target="_blank"
            rel="noopener noreferrer"
            className="nav-item"
            style={{ textDecoration: 'none' }}
            title="在新标签页中打开帮助与文档中心 (/docs/)"
          >
            <CircleHelp size={17} />
            <span>帮助与文档</span>
            <ExternalLink size={13} style={{ marginLeft: 'auto', opacity: 0.6 }} />
          </a>
          <div className="profile-row">
            <div className="profile-avatar">M</div>
            <div><strong>Me</strong><small>本地空间</small></div>
            <button aria-label="写操作令牌管理" onClick={() => setTokenModalOpen(true)} title="管理写操作令牌">
              <Settings2 size={16} />
            </button>
          </div>
        </div>
      </aside>

      <main className="main-area">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>我的世界</span>
            <span className="crumb-slash">/</span>
            <strong>
              {view === 'overview' ? '总览' : view === 'impact' ? '影响范围' : view === 'provisional' ? '待分类对象' : view === 'sources' ? '来源与同步历史' : view === 'sync' ? '批量同步' : view === 'explore' ? '关系图探索' : view === 'workspace' ? `项目工作区 · ${softwareProjects.find(p => p.id === selectedProjectId)?.name || selectedProjectId || '选择项目'}` : filter === 'all' ? '全部对象' : labels[filter]}
            </strong>
          </div>
          <div className="top-actions">
            <div className={`connection ${online ? 'connected' : ''}`}><span />{online ? '已连接' : '未连接'}</div>
            <div className="top-divider" />
            <button className="icon-button" aria-label="搜索" onClick={() => { setView('all') }}><Command size={16} /></button>
            <div className="top-avatar">M</div>
          </div>
        </header>

        <div className="content">
          {view !== 'workspace' && (
            <section className="welcome-row">
              <div>
                <div className="eyebrow"><span className="eyebrow-line" />个人认知图谱 <span className="eyebrow-dot">·</span> <span>只记录我已知的世界</span></div>
                <h1>
                  {view === 'overview' ? <>你的世界，<span>一目了然。</span></> : view === 'impact' ? <>关系与<span>影响路径。</span></> : view === 'provisional' ? <>先收集，再<span>慢慢确认。</span></> : view === 'sources' ? <>每次同步，<span>都有记录。</span></> : view === 'sync' ? <>批量导入，<span>先预览再应用。</span></> : view === 'explore' ? <>从任意对象，<span>追踪关系。</span></> : <>所有对象，<span>清晰连接。</span></>}
                </h1>
                <p className="subtitle">从对象、关系与来源出发，理解哪些事物与你有关。</p>
              </div>
              <Button type="primary" icon={<Plus size={16} />} size="large" onClick={() => setCreateOpen(true)} style={{ height: 40, fontWeight: 600 }}>
                新建对象
              </Button>
            </section>
          )}

          {error && <Alert message={error} type="error" closable onClose={() => setError('')} showIcon style={{ marginBottom: 16 }} />}

          {loading && view !== 'workspace' ? (
            <div className="loading-state"><span className="spinner" /> 正在读取你的图谱…</div>
          ) : (
            <>
              {view === 'overview' && (
                <>
                  <section className="summary-grid">
                    <SummaryCard label="已知对象" value={Math.max(0, nodes.length - 1)} hint="不含 Self 锚点" icon={<Box size={17} />} tone="lilac" />
                    <SummaryCard label="影响范围" value={impacts.length} hint="排除仅知晓关系" icon={<Activity size={17} />} tone="mint" />
                    <SummaryCard label="待分类" value={provisionals.length} hint="等待后续审查" icon={<CircleHelp size={17} />} tone="peach" />
                    <SummaryCard label="已维护对象" value={impacts.filter(item => item.relationPath.some(relation => relation.predicate === 'maintains')).length} hint="从 Self 出发可追溯" icon={<ShieldCheck size={17} />} tone="bluewash" />
                  </section>
                  <section className="panel self-relations">
                    <div className="panel-head">
                      <div>
                        <div className="panel-title">Self 关系视角</div>
                        <div className="panel-sub">owns、maintains 与 knowsAbout 各自独立，不相互推导</div>
                      </div>
                    </div>
                    <div className="self-relation-groups">
                      {([
                        { predicate: 'owns', label: '我拥有 / 控制', tone: 'blue' },
                        { predicate: 'maintains', label: '我负责维护', tone: 'green' },
                        { predicate: 'uses', label: '我使用', tone: 'teal' },
                        { predicate: 'canOperate', label: '我可受限操作', tone: 'rose' },
                        { predicate: 'knowsAbout', label: '我仅知晓', tone: 'amber' }
                      ] as const).map(group => {
                        const targets = relations.filter(relation => relation.from === self?.id && relation.predicate === group.predicate)
                          .map(relation => nodes.find(node => node.id === relation.to))
                          .filter((node): node is Node => Boolean(node))
                        return (
                          <div className="self-relation-group" key={group.predicate}>
                            <div className={`relation-group-title ${group.tone}`}>
                              <span />{group.label}<small>{targets.length}</small>
                            </div>
                            {targets.length ? (
                              targets.map(node => (
                                <button className="relation-target" key={node.id} onClick={() => focus(node)}>
                                  <TypeIcon type={node.type} /><span>{node.name}</span><ArrowRight size={13} />
                                </button>
                              ))
                            ) : (
                              <div className="relation-empty">暂无对象</div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </section>
                  <section className="overview-grid">
                    <div className="panel graph-panel">
                      <div className="panel-head">
                        <div>
                          <div className="panel-title">关系图谱</div>
                          <div className="panel-sub">从 Self 出发的已确认影响关系</div>
                        </div>
                        <button className="text-action" onClick={() => setView('impact')}>查看影响 <ArrowRight size={14} /></button>
                      </div>
                      <GraphPreview self={self} impacts={impacts} onSelect={focus} />
                    </div>
                    <div className="panel pulse-panel">
                      <div className="panel-head">
                        <div>
                          <div className="panel-title">图谱状态</div>
                          <div className="panel-sub">当前认知概况</div>
                        </div>
                        <span className="tiny-live"><span /> 已查询</span>
                      </div>
                      <div className="pulse-score">
                        <div className="score-number">{nodes.length ? '—' : '0'}<span> / 100</span></div>
                        <div className="score-label">图谱完整度暂不评分</div>
                      </div>
                      <div className="pulse-divider" />
                      <div className="pulse-row"><span className="pulse-bullet green" />所有对象可追溯至 Self<span className="pulse-check">✓</span></div>
                      <div className="pulse-row"><span className="pulse-bullet blue" />影响分析排除 knowsAbout<span className="pulse-check">✓</span></div>
                      <div className="pulse-row"><span className="pulse-bullet amber" />来源与核验时间持续完善<span className="pulse-muted">进行中</span></div>
                    </div>
                  </section>
                  <section className="panel objects-panel">
                    <div className="panel-head object-head">
                      <div>
                        <div className="panel-title">最近确认的对象</div>
                        <div className="panel-sub">按最近核验时间排序</div>
                      </div>
                      <button className="text-action" onClick={() => { setView('all'); setFilter('all') }}>全部对象 <ArrowRight size={14} /></button>
                    </div>
                    <ObjectTable nodes={recent} impacts={impactIds} selected={selected} onSelect={focus} />
                  </section>
                </>
              )}
              {view === 'sources' && <SyncHistoryPanel sources={syncSources} history={syncHistory} selectedSource={selectedSource} onSelect={setSelectedSource} />}
              {view === 'sync' && (
                <>
                  <SourcePolicyPanel />
                  <SyncBatchPanel onApplied={refresh} />
                </>
              )}
              {view === 'browse' && <FolderBrowser nodes={nodes} relations={relations} selectedId={selected?.id} onOpen={focus} />}
              {view === 'explore' && <GraphExplorer nodes={nodes} onSelect={focus} />}
              {view === 'workspace' && (
                <ProjectWorkspace
                  projectId={selectedProjectId || softwareProjects[0]?.id || ''}
                  projects={softwareProjects}
                  onSelectProject={id => setSelectedProjectId(id)}
                  onBackToGraph={() => setView('overview')}
                  onOpenDetailsEditor={node => setDetailsEditorNode(node)}
                />
              )}
              {view !== 'overview' && view !== 'browse' && view !== 'sources' && view !== 'sync' && view !== 'explore' && view !== 'workspace' && (
                <section className="panel objects-panel listing-panel">
                  <div className="list-toolbar">
                    <div>
                      <div className="panel-title">{view === 'impact' ? '影响传播路径' : view === 'provisional' ? '待确认对象' : filter === 'all' ? '对象目录' : labels[filter]}</div>
                      <div className="panel-sub">{view === 'impact' ? '仅沿明确的业务与运行关系传播' : '对象关系与状态可独立追溯'}</div>
                    </div>
                    <Input.Search
                      placeholder="搜索对象名称、类型或标识…"
                      value={query}
                      onChange={event => setQuery(event.target.value)}
                      style={{ width: 280 }}
                      allowClear
                    />
                  </div>
                  {view === 'impact' ? (
                    <ImpactList impacts={impacts.filter(item => `${item.node.name} ${labels[item.node.type]}`.toLowerCase().includes(query.toLowerCase()))} onSelect={focus} />
                  ) : (
                    <ObjectTable nodes={visibleNodes} impacts={impactIds} selected={selected} onSelect={focus} />
                  )}
                </section>
              )}
            </>
          )}
          <footer className="page-footer">
            <span>MyKnownWorld</span>
            <span>准确表达已知，不臆测未知。</span>
            <span>模型版本 1.0</span>
          </footer>
        </div>
      </main>

      {detailsEditorNode && <DetailsEditor node={detailsEditorNode} onClose={() => setDetailsEditorNode(null)} onSave={saveTypeDetails} />}
      {createOpen && <NodeCreateDialog nodes={nodes} onClose={() => setCreateOpen(false)} onCreated={async () => { setCreateOpen(false); await refresh() }} />}
      <WriteTokenModal open={tokenModalOpen} onClose={() => setTokenModalOpen(false)} />
      <EditNameModal node={editNameNode} open={Boolean(editNameNode)} onClose={() => setEditNameNode(null)} onNeedToken={() => setTokenModalOpen(true)} onSuccess={updated => { setSelected(updated); void refresh() }} />

      <Drawer
        open={Boolean(selected)}
        onClose={() => setSelected(null)}
        width={580}
        title={
          selected ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <div className={`type-icon ${typeColor[selected.type]}`} style={{ width: 34, height: 34, borderRadius: 8, display: 'grid', placeItems: 'center', flexShrink: 0 }}>
                <TypeIcon type={selected.type} />
              </div>
              <div>
                <div style={{ fontSize: 16, fontWeight: 700, color: '#222' }}>{selected.name}</div>
                <div style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>{selected.id}</div>
              </div>
            </div>
          ) : null
        }
        extra={
          selected ? (
            <Space>
              <Button size="small" onClick={() => setEditNameNode(selected)}>改名</Button>
              {selected.type !== 'Self' && selected.type !== 'ProvisionalObject' && (
                <Button size="small" type="primary" onClick={() => setDetailsEditorNode(selected)}>编辑类型详情</Button>
              )}
            </Space>
          ) : null
        }
      >
        {selected && (
          <div>
            <Space wrap style={{ marginBottom: 16 }}>
              <Tag color={antdColorForType(selected.type)}>{labels[selected.type]}</Tag>
              {selected.lifecycle && <Tag>生命周期: {selected.lifecycle}</Tag>}
              <Tag color="cyan">确定性: {certaintyLabel[selected.certainty]}</Tag>
              <Tag>来源: {selected.source}</Tag>
              <Tag>模型 v{selected.modelVersion}</Tag>
            </Space>

            {selected.type === 'SoftwareProject' && (
              <Space direction="vertical" style={{ width: '100%', marginBottom: 16 }}>
                <Button
                  type="primary"
                  icon={<ExternalLink size={15} />}
                  style={{ width: '100%', height: 40, fontWeight: 600, background: '#42633c', borderColor: '#42633c' }}
                  onClick={() => {
                    window.open(`/project/${encodeURIComponent(selected.id)}`, '_blank')
                  }}
                >
                  在新标签页打开「{selected.name}」独立项目页 ↗
                </Button>
                <Button
                  icon={<Maximize2 size={14} />}
                  style={{ width: '100%', height: 34 }}
                  onClick={() => {
                    window.history.pushState({}, '', `/project/${encodeURIComponent(selected.id)}`)
                    setFocusedProjectId(selected.id)
                    setView('project-focus')
                    setSelected(null)
                  }}
                >
                  在当前窗口进入独立项目页 (无全局菜单)
                </Button>
              </Space>
            )}

            {selected.type === 'ProvisionalObject' && selected.reviewStatus !== 'rejected' && (
              <Card size="small" style={{ marginBottom: 16, background: '#fffbe6', borderColor: '#ffe58f' }} title="待审查暂存对象">
                <p style={{ margin: '0 0 8px 0', fontSize: 12 }}>候选类型：{selected.candidateType ? labels[selected.candidateType] : '尚未建议'}</p>
                {selected.reviewQuestions?.map(question => (
                  <div key={question} style={{ fontSize: 11, color: '#888', marginBottom: 4 }}>• {question}</div>
                ))}
                <div style={{ marginTop: 10 }}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <Select
                      style={{ width: '100%' }}
                      value={reviewTarget}
                      onChange={v => setReviewTarget(v as NodeType)}
                      options={stableTypes.map(t => ({ label: `${labels[t]} (${t})`, value: t }))}
                    />
                    <Space>
                      <Button type="primary" size="small" onClick={() => void reviewAction(selected, 'classify')}>转为明确对象</Button>
                      <Button danger size="small" onClick={() => void reviewAction(selected, 'reject')}>保留并拒绝分类</Button>
                    </Space>
                  </Space>
                </div>
              </Card>
            )}

            <Card size="small" style={{ marginBottom: 16 }} title={`与 Self 的关系路径 (${path?.relations.length ?? 0} 条关系)`}>
              {path ? (
                path.relations.length ? (
                  <Timeline
                    style={{ marginTop: 12 }}
                    items={path.nodes.map((node, index) => ({
                      children: (
                        <div>
                          <strong>{node.name}</strong> <Tag color={antdColorForType(node.type)}>{labels[node.type]}</Tag>
                          {index < path.nodes.length - 1 && (
                            <div style={{ color: '#42633c', fontSize: 11, margin: '4px 0', fontWeight: 600 }}>
                              ↓ [{path.relations[index]?.predicate}]
                            </div>
                          )}
                        </div>
                      )
                    }))}
                  />
                ) : (
                  <div style={{ color: '#888', fontSize: 12 }}>Self 节点本身是图谱锚点。</div>
                )
              ) : (
                <div style={{ color: '#888', fontSize: 12 }}>正在查询关系路径…</div>
              )}
            </Card>

            <Descriptions size="small" bordered column={1} title="基本已知信息" style={{ marginBottom: 16 }}>
              <Descriptions.Item label="生命周期">{selected.lifecycle || '未记录'}</Descriptions.Item>
              <Descriptions.Item label="信息来源">{selected.source === 'unspecified' ? '未说明' : selected.source}</Descriptions.Item>
              <Descriptions.Item label="证据类型 / 确定性">{evidenceLabel[selected.evidenceKind]} / {certaintyLabel[selected.certainty]}</Descriptions.Item>
              <Descriptions.Item label="最近核验">{selected.lastConfirmed ? new Date(selected.lastConfirmed).toLocaleDateString('zh-CN') : '尚未核验'}</Descriptions.Item>
              <Descriptions.Item label="节点标识"><code style={{ fontFamily: 'monospace' }}>{selected.id}</code></Descriptions.Item>
            </Descriptions>

            <ModelDetailsPanel node={selected} />

            <RelationMaintenancePanel node={selected} nodes={nodes} relations={relations} onChanged={refresh} onNeedToken={() => setTokenModalOpen(true)} />

            {assertions.length > 0 && (
              <Card size="small" title={`字段级来源断言 (${assertions.length} 条)`} style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={assertions}
                  renderItem={assertion => (
                    <List.Item
                      actions={
                        !assertion.selected
                          ? [<Button type="link" size="small" key="resolve" onClick={() => void resolveFieldAssertion(assertion)}>选择此断言</Button>]
                          : undefined
                      }
                    >
                      <List.Item.Meta
                        title={
                          <span>
                            {assertion.fieldPath}
                            <Tag color={assertion.selected ? 'green' : assertion.resolved ? 'default' : 'orange'} style={{ marginLeft: 8 }}>
                              {assertion.selected ? '当前已选证据' : assertion.resolved ? '与当前裁决不同' : '未裁决'}
                            </Tag>
                          </span>
                        }
                        description={
                          <div style={{ fontSize: 11 }}>
                            <span>{assertion.source} · {evidenceLabel[assertion.evidenceKind]} · {certaintyLabel[assertion.certainty]}</span>
                            <div style={{ color: '#888' }}>{assertion.reference || '无外部引用'} · {new Date(assertion.assertedAt).toLocaleString('zh-CN')}</div>
                          </div>
                        }
                      />
                    </List.Item>
                  )}
                />
              </Card>
            )}

            {selected.type === 'RunningPeriod' && (
              <Descriptions size="small" bordered column={1} title="运行周期记录" style={{ marginBottom: 16 }}>
                <Descriptions.Item label="开始 / 结束">
                  {selected.startedAt ? new Date(selected.startedAt).toLocaleString('zh-CN') : '未知'} / {selected.endedAt ? new Date(selected.endedAt).toLocaleString('zh-CN') : '仍在运行或未知'}
                </Descriptions.Item>
                <Descriptions.Item label="部署版本">{selected.deployedVersion || '未知'}</Descriptions.Item>
                <Descriptions.Item label="最近验证">{selected.verificationAt ? new Date(selected.verificationAt).toLocaleString('zh-CN') : '未验证'}</Descriptions.Item>
                <Descriptions.Item label="验证结果">{selected.verificationResult || '未知'}</Descriptions.Item>
                <Descriptions.Item label="证据引用">{selected.evidenceRef || '未记录'}</Descriptions.Item>
              </Descriptions>
            )}

            {entityHistory.length > 0 && (
              <Card size="small" style={{ marginBottom: 16 }}>
                <details className="history-disclosure">
                  <summary>
                    <span>对象变更历史</span>
                    <span className="history-count">{entityHistory.length} 条</span>
                  </summary>
                <Timeline
                  style={{ marginTop: 12 }}
                  items={entityHistory.map(event => ({
                    children: (
                      <div>
                        <strong>{({ created: '建立', updated: '更新', stale: '来源缺失/过期', deleted: '显式删除' } as Record<string, string>)[event.action] ?? event.action}</strong>
                        <span style={{ marginLeft: 8, fontSize: 11, color: '#888' }}>{event.source} · {new Date(event.recordedAt).toLocaleString('zh-CN')}</span>
                        <div style={{ fontSize: 11, color: '#aaa' }}>{event.runId ? `同步批次 #${event.runId}` : '人工操作'}</div>
                      </div>
                    )
                  }))}
                />
                </details>
              </Card>
            )}

            {fieldResolutions.length > 0 && (
              <Card size="small" title={`字段分歧裁决记录 (${fieldResolutions.length} 条)`} style={{ marginBottom: 16 }}>
                <List
                  size="small"
                  dataSource={fieldResolutions}
                  renderItem={item => (
                    <List.Item>
                      <List.Item.Meta
                        title={item.fieldPath}
                        description={
                          <div>
                            <div>选定断言：{item.selectedAssertionIds.join('、')}</div>
                            <small>{item.rationaleReference} · {new Date(item.recordedAt).toLocaleString('zh-CN')}</small>
                          </div>
                        }
                      />
                    </List.Item>
                  )}
                />
              </Card>
            )}

            <Alert
              message="路径展示已记录的关系；可达不代表当前运行或状态正常。"
              type="info"
              showIcon
              icon={<ShieldCheck size={16} />}
              style={{ marginTop: 16 }}
            />
          </div>
        )}
      </Drawer>
    </div>
  )
}

function NodeCreateDialog({
  nodes,
  onClose,
  onCreated
}: {
  nodes: Node[]
  onClose: () => void
  onCreated: () => Promise<void>
}) {
  const { message: antdMsg } = AntdApp.useApp()
  const [form] = Form.useForm()
  const types = stableTypes
  const [busy, setBusy] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  const detailKey: Partial<Record<NodeType, string>> = {
    Hardware: 'hardware',
    Machine: 'machine',
    SoftwareProject: 'softwareProject',
    SoftwareInstance: 'softwareInstance',
    AccountCredential: 'accountCredential',
    Product: 'product',
    Document: 'document',
    Runbook: 'runbook',
    WorkItem: 'workItem',
    Network: 'network'
  }

  const handleFinish = async (values: {
    type: NodeType
    predicate: string
    id: string
    name: string
    lifecycle?: string
    details?: string
  }) => {
    setErrorMsg('')
    const { type, predicate, id, name, lifecycle, details } = values
    let detail: unknown
    try {
      detail = JSON.parse(details || '{}')
    } catch {
      setErrorMsg('类型详情必须是有效的 JSON 格式')
      return
    }

    const self = nodes.find(item => item.type === 'Self')
    if (!self) {
      setErrorMsg('当前图谱缺少 Self 锚点')
      return
    }

    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      token = window.prompt('请输入 MYKNOWNWORLD_WRITE_TOKEN（仅保存在当前浏览器会话）')
      if (!token) return
      sessionStorage.setItem('mkw-write-token', token)
    }

    const node = {
      id: id.trim(),
      type,
      name: name.trim(),
      lifecycle: lifecycle?.trim() || undefined,
      modelVersion: 1,
      source: 'manual:ui',
      evidenceKind: 'manual',
      certainty: 'confirmed',
      ...(detailKey[type] ? { [detailKey[type]!]: detail } : {})
    } as Node

    const relation: Relation = {
      id: `onboard-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
      from: self.id,
      predicate,
      to: node.id,
      source: 'manual:ui',
      evidenceKind: 'manual',
      certainty: 'confirmed',
      recordedAt: new Date().toISOString()
    }

    setBusy(true)
    try {
      const mutation: ManualMutation = { kind: 'createNode', node, relation }
      const plan = await previewAndConfirmMutation(mutation)
      if (!plan) return
      await api.applyMutation(mutation, plan, token)
      antdMsg.success(`已成功创建对象「${name}」`)
      await onCreated()
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      setErrorMsg(cause instanceof Error ? cause.message : '创建失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open
      title="创建明确对象"
      onCancel={onClose}
      onOk={() => form.submit()}
      confirmLoading={busy}
      okText="创建并发布"
      cancelText="取消"
      width={600}
    >
      <p style={{ color: '#777', fontSize: 12, marginTop: -4, marginBottom: 16 }}>
        发布对象时须显式指定与 Self 的关系；不自动推断拥有或维护。
      </p>

      {errorMsg && <Alert type="error" message={errorMsg} showIcon style={{ marginBottom: 16 }} />}

      <Form
        form={form}
        layout="vertical"
        initialValues={{ type: 'Machine', predicate: 'knowsAbout', details: '{}' }}
        onFinish={handleFinish}
      >
        <Space direction="horizontal" size={16} style={{ display: 'flex' }}>
          <Form.Item label="对象类型" name="type" style={{ flex: 1 }} rules={[{ required: true }]}>
            <Select
              options={types.map(t => ({ label: `${labels[t]} (${t})`, value: t }))}
              onChange={() => form.setFieldsValue({ details: '{}' })}
            />
          </Form.Item>
          <Form.Item label="与 Self 关系" name="predicate" style={{ flex: 1 }} rules={[{ required: true }]}>
            <Select options={['knowsAbout', 'owns', 'maintains', 'uses', 'canOperate'].map(v => ({ label: v, value: v }))} />
          </Form.Item>
        </Space>

        <Space direction="horizontal" size={16} style={{ display: 'flex' }}>
          <Form.Item label="稳定唯一 ID" name="id" style={{ flex: 1 }} rules={[{ required: true, message: 'ID 必填' }]}>
            <Input placeholder="例如 machine-lab" />
          </Form.Item>
          <Form.Item label="对象名称" name="name" style={{ flex: 1 }} rules={[{ required: true, message: '名称必填' }]}>
            <Input placeholder="例如 实验室主机" />
          </Form.Item>
        </Space>

        <Form.Item label="生命周期" name="lifecycle">
          <Input placeholder="例如 active / production / retired（可选）" />
        </Form.Item>

        <Form.Item
          label="类型专属详情 (JSON)"
          name="details"
          extra="不要在类型详情中填写密码、私钥、Token 或低熵敏感凭据。缺少详情不会阻止录入。"
        >
          <Input.TextArea rows={4} style={{ fontFamily: 'monospace', fontSize: 12 }} />
        </Form.Item>
      </Form>
    </Modal>
  )
}

function RelationMaintenancePanel({
  node,
  nodes,
  relations,
  onChanged,
  onNeedToken
}: {
  node: Node
  nodes: Node[]
  relations: Relation[]
  onChanged: () => Promise<void>
  onNeedToken: () => void
}) {
  const { message } = AntdApp.useApp()
  const [predicate, setPredicate] = useState('uses')
  const [target, setTarget] = useState('')
  const touching = relations.filter(item => item.from === node.id || item.to === node.id)
  const candidates = nodes.filter(item => item.id !== node.id)

  const getToken = () => {
    const value = sessionStorage.getItem('mkw-write-token')
    if (!value) {
      onNeedToken()
      return ''
    }
    return value
  }

  const add = async () => {
    const auth = getToken()
    if (!auth || !target) return
    const relation: Relation = {
      id: `manual-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      from: node.id,
      predicate,
      to: target,
      source: 'manual:ui',
      evidenceKind: 'manual',
      certainty: 'confirmed',
      recordedAt: new Date().toISOString()
    }
    try {
      const mutation: ManualMutation = { kind: 'addRelation', relation }
      const plan = await previewAndConfirmMutation(mutation)
      if (!plan) return
      await api.applyMutation(mutation, plan, auth)
      message.success('关系已添加')
      setTarget('')
      await onChanged()
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : '关系添加失败')
    }
  }

  const editEvidence = async (item: Relation) => {
    const evidenceKind = window.prompt('证据类型：unknown / manual / document / observed / derived / imported', item.evidenceKind)
    if (evidenceKind === null) return
    if (!['unknown', 'manual', 'document', 'observed', 'derived', 'imported'].includes(evidenceKind)) {
      message.warning('证据类型无效')
      return
    }
    const certainty = window.prompt('确定性：unknown / provisional / confirmed / disputed / stale', item.certainty)
    if (certainty === null) return
    if (!['unknown', 'provisional', 'confirmed', 'disputed', 'stale'].includes(certainty)) {
      message.warning('确定性无效')
      return
    }
    if (!window.confirm(`只更新关系 ${item.id} 的证据类型与确定性。端点、谓词、来源标识保持不变。继续？`)) return
    const auth = getToken()
    if (!auth) return
    try {
      const relation = {
        ...item,
        evidenceKind: evidenceKind as Relation['evidenceKind'],
        certainty: certainty as Relation['certainty'],
        recordedAt: new Date().toISOString()
      }
      const mutation: ManualMutation = { kind: 'updateRelation', id: item.id, relation }
      const plan = await previewAndConfirmMutation(mutation)
      if (!plan) return
      await api.applyMutation(mutation, plan, auth)
      message.success('关系证据已更新')
      await onChanged()
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : '关系证据更新失败')
    }
  }

  const remove = async (id: string) => {
    const auth = getToken()
    if (!auth) return
    try {
      const mutation: ManualMutation = { kind: 'deleteRelation', id }
      const plan = await previewAndConfirmMutation(mutation)
      if (!plan) return
      await api.applyMutation(mutation, plan, auth)
      message.success('关系已删除')
      await onChanged()
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : '关系删除失败')
    }
  }

  const predicateOptions = [
    'owns', 'maintains', 'uses', 'canOperate', 'knowsAbout',
    'hosts', 'routesThrough', 'providesAccessTo', 'implementedFrom',
    'hasRunningPeriod', 'hasCredential', 'documents', 'appliesTo',
    'targets', 'supersedes', 'dependsOn', 'hasInstance', 'relatedTo'
  ].map(p => ({ label: p, value: p }))

  return (
    <Card size="small" title={`关系维护 (${touching.length} 条)`} style={{ marginBottom: 16 }}>
      {touching.length > 0 ? (
        <List
          size="small"
          dataSource={touching}
          renderItem={item => (
            <List.Item
              actions={[
                <Button type="link" size="small" key="edit" onClick={() => void editEvidence(item)}>编辑证据</Button>,
                <Popconfirm
                  key="delete"
                  title="确认删除关系"
                  description="若删除后造成对象与 Self 断连，服务器将拒绝。"
                  okText="删除"
                  cancelText="取消"
                  okButtonProps={{ danger: true }}
                  onConfirm={() => void remove(item.id)}
                >
                  <Button type="link" danger size="small">删除</Button>
                </Popconfirm>
              ]}
            >
              <List.Item.Meta
                title={
                  <span>
                    <strong>{item.from === node.id ? '→' : '←'} {item.predicate}</strong> · {item.from === node.id ? item.to : item.from}
                  </span>
                }
                description={
                  <div style={{ fontSize: 11 }}>
                    {item.source} · {evidenceLabel[item.evidenceKind]} · {certaintyLabel[item.certainty]}
                  </div>
                }
              />
            </List.Item>
          )}
        />
      ) : (
        <div style={{ color: '#888', fontSize: 12, padding: '8px 0' }}>暂无关联关系</div>
      )}

      <div style={{ marginTop: 12, borderTop: '1px solid #f0f0f0', paddingTop: 12 }}>
        <div style={{ fontSize: 12, fontWeight: 600, marginBottom: 8 }}>添加新关系 (从当前对象出发):</div>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space.Compact style={{ width: '100%' }}>
            <Select
              style={{ width: '40%' }}
              value={predicate}
              onChange={setPredicate}
              options={predicateOptions}
            />
            <Select
              style={{ width: '60%' }}
              showSearch
              filterOption={(input, option) =>
                ((option?.label as string) ?? '').toLowerCase().includes(input.toLowerCase())
              }
              placeholder="选择目标对象"
              value={target || undefined}
              onChange={setTarget}
              options={candidates.map(c => ({
                label: `${c.name} (${labels[c.type]})`,
                value: c.id
              }))}
            />
          </Space.Compact>
          <Button type="primary" size="small" disabled={!target} onClick={() => void add()}>
            确认添加出向关系
          </Button>
        </Space>
      </div>
    </Card>
  )
}

function ModelDetailsPanel({ node }: { node: Node }) {
  const list = (values?: string[]) => values?.join(', ') || '—'
  const rows: [string, string][] = []

  if (node.hardware) {
    const d = node.hardware
    rows.push(
      ['设备类别 / 型号', [d.category, d.model].filter(Boolean).join(' / ') || '—'],
      ['位置 / 状态', [d.location, d.status].filter(Boolean).join(' / ') || '—'],
      ['硬件规格', Object.entries(d.specifications ?? {}).map(([k, v]) => `${k}: ${v}`).join(' · ') || '—'],
      ['访问说明引用', d.accessReference || '—'],
      ['维护说明引用', d.maintenanceReference || '—']
    )
  }
  if (node.machine) {
    const d = node.machine
    rows.push(
      ['Hostname / OS', [d.hostname, d.os].filter(Boolean).join(' / ') || '—'],
      ['规格', d.spec || '—'],
      ['角色', list(d.roles)],
      ['IP / 网络', [list(d.ips), d.network].filter(x => x !== '—').join(' / ') || '—'],
      ['机器状态 / 责任边界', [d.status, d.responsibilityBoundary].filter(Boolean).join(' / ') || '—'],
      ['访问方式 / 前提', [d.accessMethod, list(d.accessPrerequisites)].filter(x => x && x !== '—').join(' / ') || '—'],
      ['允许操作', list(d.allowedOperations)],
      ['禁止操作', list(d.forbiddenOperations)],
      ['自管资源', (d.resources ?? []).map(r => `${r.kind}: ${r.name}${r.port ? ` :${r.port}` : ''}`).join(' · ') || '—'],
      ['最近核验', d.lastVerified ? new Date(d.lastVerified).toLocaleString('zh-CN') : '—']
    )
  }
  if (node.softwareProject) {
    const d = node.softwareProject
    rows.push(
      ['用途 / 形态', [d.purpose, d.form].filter(Boolean).join(' / ') || '—'],
      ['语言', list(d.languages)],
      ['远程仓库 / 上游', [d.remoteRepository, d.upstream].filter(Boolean).join(' · ') || '—'],
      ['维护 / 推荐版本', [d.maintenanceState, d.recommendedVersion].filter(Boolean).join(' / ') || '—'],
      ['继任项目', d.successorId || '—']
    )
  }
  if (node.softwareInstance) {
    const d = node.softwareInstance
    rows.push(
      ['产品 / 部署方式', [d.productName, d.deploymentMethod].filter(Boolean).join(' / ') || '—'],
      ['部署路径 / 身份', [d.deploymentPath, d.runtimeIdentity].filter(Boolean).join(' / ') || '—'],
      ['端点', (d.endpoints ?? []).map(e => `${e.kind} ${e.address}${e.port ? `:${e.port}` : ''}`).join(' · ') || '—'],
      ['配置引用', list(d.configurationReferences)],
      ['部署 / 运行 / 维护', [d.deploymentState, d.operationalState, d.maintenanceState].filter(Boolean).join(' / ') || '—'],
      ['运维要点', list(d.notes)],
      ['最近验证', [d.verificationResult, d.verifiedAt].filter(Boolean).join(' · ') || '—']
    )
  }
  if (node.accountCredential) {
    const d = node.accountCredential
    rows.push(
      ['账号 / 类型', [d.account, d.credentialType].filter(Boolean).join(' / ') || '—'],
      ['用途 / 范围', [d.purpose, d.scope].filter(Boolean).join(' / ') || '—'],
      ['保管位置引用', d.accessReference || '—'],
      ['状态 / 轮换', [d.status, d.rotationAt].filter(Boolean).join(' / ') || '—']
    )
  }
  if (node.product) {
    const d = node.product
    rows.push(
      ['用途', d.purpose || '—'],
      ['共性配置 / Runbook', [d.commonConfigurationReference, d.runbookReference].filter(Boolean).join(' · ') || '—']
    )
  }
  if (node.document) {
    const d = node.document
    rows.push(
      ['职责 / 权威位置', [d.role, d.authoritativeReference].filter(Boolean).join(' / ') || '—'],
      ['当前有效', d.effective ? '是' : '未知'],
      ['最近核验 / 替代', [d.lastVerified, d.supersededBy].filter(Boolean).join(' / ') || '—']
    )
  }
  if (node.runbook) {
    const d = node.runbook
    rows.push(
      ['触发 / 前置条件', [d.trigger, list(d.preconditions)].filter(x => x && x !== '—').join(' / ') || '—'],
      ['步骤 / 停止 / 回退引用', [d.stepsReference, d.stopConditionReference, d.rollbackReference].filter(Boolean).join(' · ') || '—'],
      ['验证方式 / 状态', [d.verificationMethod, d.executableStatus].filter(Boolean).join(' / ') || '—'],
      ['需要人工批准', d.approvalRequired ? '是' : '否']
    )
  }
  if (node.workItem) {
    const d = node.workItem
    rows.push(
      ['事项类型 / 状态', [d.kind, d.status].filter(Boolean).join(' / ') || '—'],
      ['理由 / 决策证据引用', [d.rationaleReference, d.decisionEvidenceReference].filter(Boolean).join(' / ') || '—']
    )
  }
  if (node.network) {
    const d = node.network
    rows.push(
      ['网络类型 / 状态', [d.kind, d.status].filter(Boolean).join(' / ') || '—'],
      ['CIDR', d.cidrs?.join('、') || '—'],
      ['网关 / VPN 引用', [d.gatewayReference, d.vpnReference].filter(Boolean).join(' · ') || '—'],
      ['访问位置', d.accessLocations?.join('、') || '—']
    )
  }

  if (!rows.length) return null

  return (
    <Descriptions size="small" bordered column={1} title="类型专属信息" style={{ marginBottom: 16 }}>
      {rows.map(([label, value]) => (
        <Descriptions.Item key={label} label={label}>
          {value}
        </Descriptions.Item>
      ))}
    </Descriptions>
  )
}


function SourcePolicyPanel() {
  const [items, setItems] = useState<SourcePolicy[]>([])
  const [text, setText] = useState('{\n  "source": "inventory",\n  "priority": 50,\n  "allowedNodeTypes": ["Machine"],\n  "allowedPredicates": ["knowsAbout", "hosts"],\n  "allowedFieldPaths": ["machine.hostname", "machine.os", "machine.ips"]\n}')
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = () => api.sourcePolicies().then(result => setItems(result.items)).catch(cause => setMessage(cause instanceof Error ? cause.message : '来源策略不可用'))
  useEffect(() => { void refresh() }, [])

  const save = async () => {
    let policy: SourcePolicy
    try {
      policy = JSON.parse(text) as SourcePolicy
    } catch {
      setMessage('JSON 格式无效')
      return
    }
    if (!policy.source?.trim()) {
      setMessage('来源标识必填')
      return
    }
    const token = sessionStorage.getItem('mkw-write-token') || window.prompt('请输入 MYKNOWNWORLD_WRITE_TOKEN')
    if (!token) return
    sessionStorage.setItem('mkw-write-token', token)
    setBusy(true)
    try {
      const preview = await api.previewSourcePolicy(policy)
      const before = preview.previousPolicy ? JSON.stringify(preview.previousPolicy, null, 2) : '（新来源策略）'
      const after = JSON.stringify(preview.policy, null, 2)
      if (
        !window.confirm(
          `服务端预览来源策略变更：\n${before}\n→\n${after}\n优先级仅为提示，不自动裁决事实。策略版本 ${preview.plan.expectedRevision.slice(0, 12)}…\n确认应用？`
        )
      ) return
      await api.saveSourcePolicy(policy, preview.plan, token)
      setMessage(`已保存来源策略：${policy.source}`)
      await refresh()
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      setMessage(cause instanceof Error ? cause.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel sync-batch-panel">
      <div className="panel-head">
        <div>
          <div className="panel-title">来源范围与优先级</div>
          <div className="panel-sub">精确 allow-list；优先级仅作提示，不自动裁决事实。未配置的来源保持旧行为。</div>
        </div>
      </div>
      <div className="sync-run-tags" style={{ margin: '12px 0' }}>
        {items.map(item => (
          <Button key={item.source} size="small" onClick={() => setText(JSON.stringify(item, null, 2))}>
            {item.source} · P{item.priority} · {item.allowedNodeTypes.length} 类对象
          </Button>
        ))}
      </div>
      <Input.TextArea
        value={text}
        onChange={event => setText(event.target.value)}
        spellCheck={false}
        rows={8}
        style={{ fontFamily: 'monospace', fontSize: 12 }}
        aria-label="来源策略 JSON"
      />
      <div style={{ marginTop: 12, display: 'flex', alignItems: 'center', gap: 10 }}>
        <Button type="primary" onClick={() => void save()} loading={busy}>
          保存/更新策略
        </Button>
        <span>{busy ? '保存中…' : message}</span>
      </div>
      <small style={{ display: 'block', marginTop: 10, color: '#888' }}>
        策略适用于批次内全部节点类型、关系谓词及字段断言。被拒绝的批次不会部分写入。建议按来源职责配置最小范围。
      </small>
    </section>
  )
}

function SyncBatchPanel({ onApplied }: { onApplied: () => Promise<void> }) {
  const [text, setText] = useState('')
  const [preview, setPreview] = useState<SyncPreview | null>(null)
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  const previewBatch = async () => {
    setBusy(true)
    setMessage('')
    try {
      const batch = JSON.parse(text) as { source?: string }
      const result = await api.previewBatch(batch)
      setPreview(result)
      setMessage(`预览完成：${result.source}`)
    } catch (cause) {
      setPreview(null)
      setMessage(cause instanceof Error ? cause.message : '预览失败')
    } finally {
      setBusy(false)
    }
  }

  const applyBatch = async () => {
    if (!preview || preview.conflicts.length || !preview.source) return
    let batch: unknown
    try {
      batch = JSON.parse(text)
    } catch {
      setMessage('JSON 格式无效')
      return
    }
    if (!window.confirm(`将批次应用到数据源「${preview.source}」。缺失记录只会标记 stale，不会删除。继续？`)) return
    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      token = window.prompt('请输入 MYKNOWNWORLD_WRITE_TOKEN（仅保存在当前浏览器会话）')
      if (!token) return
      sessionStorage.setItem('mkw-write-token', token)
    }
    setBusy(true)
    setMessage('')
    try {
      if (!preview.expectedRevision || !preview.planHash) {
        setMessage('当前服务器没有提供受保护的批次计划，不能应用')
        return
      }
      const result = await api.applyBatch(batch, preview, token)
      setPreview(result.preview)
      setMessage('批次已原子应用')
      await onApplied()
    } catch (cause) {
      sessionStorage.removeItem('mkw-write-token')
      setMessage(cause instanceof Error ? cause.message : '批次应用失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="panel sync-batch-panel">
      <div className="panel-head">
        <div>
          <div className="panel-title">导入来源批次</div>
          <div className="panel-sub">提交一个完整来源快照；先预览差异，再以确认令牌原子应用。最大 8 MiB。</div>
        </div>
      </div>
      <Input.TextArea
        value={text}
        onChange={event => {
          setText(event.target.value)
          setPreview(null)
        }}
        placeholder={'粘贴 SourceBatch JSON，例如 {"source":"my-source","nodes":[],"relations":[]}'}
        spellCheck={false}
        rows={9}
        style={{ fontFamily: 'monospace', fontSize: 12, marginTop: 12 }}
      />
      <div style={{ marginTop: 12, display: 'flex', alignItems: 'center', gap: 10 }}>
        <Button onClick={() => void previewBatch()} loading={busy} disabled={!text.trim()}>
          预览差异
        </Button>
        <Button
          type="primary"
          onClick={() => void applyBatch()}
          disabled={busy || !preview || !preview.expectedRevision || !preview.planHash || preview.conflicts.length > 0}
        >
          确认并应用
        </Button>
        <span>{busy ? '处理中…' : message}</span>
      </div>
      {preview && (
        <div className="sync-preview-result">
          <strong>{preview.source} · {preview.sourceHash}</strong>
          <div className="sync-run-tags">
            <span>新增对象 {preview.addedNodes.length}</span>
            <span>更新对象 {preview.updatedNodes.length}</span>
            <span>新缺失对象 {preview.newlyMissingNodes.length}</span>
            <span>仍缺失对象 {preview.alreadyStaleNodes.length}</span>
            <span>新缺失关系 {preview.newlyMissingRelations.length}</span>
            <span>仍缺失关系 {preview.alreadyStaleRelations.length}</span>
            <span>新缺失字段 {preview.newlyMissingAssertions.length}</span>
            <span>仍缺失字段 {preview.alreadyStaleAssertions.length}</span>
            <span>新增关系 {preview.addedRelations.length}</span>
            <span>更新关系 {preview.updatedRelations.length}</span>
            <span>字段断言 {preview.addedAssertions.length + preview.updatedAssertions.length}</span>
            <span>冲突 {preview.conflicts.length}</span>
          </div>
          {preview.conflicts.map(conflict => (
            <p className="sync-conflict" key={conflict}>{conflict}</p>
          ))}
          {preview.valueConflicts?.map(conflict => (
            <p className="sync-conflict" key={`${conflict.nodeId}:${conflict.fieldPath}`}>
              字段值有跨来源分歧：{conflict.nodeId} · {conflict.fieldPath}（来源：{conflict.sources.join('、')}；断言：{conflict.assertionIds.join('、')}）。未显示摘要或原值，不自动裁决。
            </p>
          ))}
          <small>不会自动删除缺失数据。重命名需要显式或已保存的身份映射。</small>
        </div>
      )}
    </section>
  )
}

function SyncHistoryPanel({
  sources,
  history,
  selectedSource,
  onSelect
}: {
  sources: SyncSource[]
  history: SyncRun[]
  selectedSource: string
  onSelect: (source: string) => void
}) {
  if (!sources.length) {
    return (
      <section className="panel">
        <div className="empty-state">
          <div className="empty-art"><Database size={22} /></div>
          <strong>还没有已应用的数据源</strong>
          <span>批次预览不会写入历史；应用批次后，这里会显示来源、哈希和变更报告。</span>
        </div>
      </section>
    )
  }

  return (
    <section className="panel sync-history-panel">
      <div className="panel-head">
        <div>
          <div className="panel-title">来源目录与同步历史</div>
          <div className="panel-sub">只记录来源标识、哈希、解析器版本和变更 ID；不保存导入正文。</div>
        </div>
        <Select
          style={{ width: 220 }}
          value={selectedSource}
          onChange={onSelect}
          options={sources.map(source => ({ label: source.source, value: source.source }))}
        />
      </div>
      <div className="source-summary">
        {sources.filter(source => source.source === selectedSource).map(source => (
          <div key={source.source}>
            <span>最近应用</span><strong>{new Date(source.appliedAt).toLocaleString('zh-CN')}</strong>
            <span>解析器</span><strong>{source.parserVersion || '未提供'}</strong>
            <span>SHA-256</span><code>{source.sourceHash}</code>
          </div>
        ))}
      </div>
      <div className="sync-run-list">
        {history.map(run => (
          <article className="sync-run" key={run.id}>
            <div className="sync-run-head">
              <strong>运行 #{run.id}{run.undone ? ' · 已撤回' : ''}</strong>
              <time>{new Date(run.appliedAt).toLocaleString('zh-CN')}</time>
            </div>
            <div className="sync-run-tags">
              <span>新增对象 {run.preview.addedNodes?.length ?? 0}</span>
              <span>更新对象 {run.preview.updatedNodes?.length ?? 0}</span>
              <span>缺失标记 {run.preview.missingNodes?.length ?? 0}</span>
              <span>新增关系 {run.preview.addedRelations?.length ?? 0}</span>
              <span>冲突 {run.preview.conflicts?.length ?? 0}</span>
            </div>
            <code>{run.sourceHash}</code>
          </article>
        ))}
      </div>
    </section>
  )
}

function SummaryCard({ label, value, hint, icon, tone }: { label: string; value: number; hint: string; icon: React.ReactNode; tone: string }) {
  return (
    <div className="summary-card">
      <div className={`summary-icon ${tone}`}>{icon}</div>
      <div className="summary-label">{label}</div>
      <div className="summary-value">{value}<span className="summary-unit"> 个</span></div>
      <div className="summary-hint">{hint}</div>
    </div>
  )
}

function TypeIcon({ type }: { type: NodeType }) {
  const Icon = iconFor[type]
  return <Icon size={17} strokeWidth={1.8} />
}

function ObjectTable({
  nodes,
  impacts,
  selected,
  onSelect
}: {
  nodes: Node[]
  impacts: Set<string>
  selected: Node | null
  onSelect: (node: Node) => void
}) {
  const columns: TableColumnsType<Node> = [
    {
      title: '对象名称 / 标识',
      dataIndex: 'name',
      key: 'name',
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (_, record) => (
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div className={`type-icon ${typeColor[record.type]}`} style={{ width: 28, height: 28, borderRadius: 6, display: 'grid', placeItems: 'center', flexShrink: 0 }}>
            <TypeIcon type={record.type} />
          </div>
          <div>
            <div style={{ fontWeight: 600, fontSize: 13, color: '#242821' }}>{record.name}</div>
            <div style={{ fontFamily: 'ui-monospace,SFMono-Regular,Menlo,monospace', fontSize: 11, color: '#888' }}>{record.id}</div>
          </div>
        </div>
      )
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      filters: navTypes.map(t => ({ text: labels[t], value: t })),
      onFilter: (value, record) => record.type === value,
      render: (type: NodeType) => (
        <Tag color={antdColorForType(type)}>{labels[type]}</Tag>
      )
    },
    {
      title: '生命周期',
      dataIndex: 'lifecycle',
      key: 'lifecycle',
      render: (lifecycle?: string) => (
        lifecycle ? <Tag color="default">{lifecycle}</Tag> : <span style={{ color: '#bbb', fontSize: 12 }}>未记录</span>
      )
    },
    {
      title: '确定性',
      dataIndex: 'certainty',
      key: 'certainty',
      render: (certainty: keyof typeof certaintyLabel) => {
        const color = certainty === 'confirmed' ? 'success' : certainty === 'provisional' ? 'warning' : certainty === 'disputed' ? 'error' : certainty === 'stale' ? 'purple' : 'default'
        return <Tag color={color}>{certaintyLabel[certainty] || certainty}</Tag>
      }
    },
    {
      title: '影响范围',
      key: 'impact',
      render: (_, record) => (
        impacts.has(record.id) ? (
          <Tag color="cyan" icon={<Activity size={12} style={{ verticalAlign: -1, marginRight: 3 }} />}>可传播</Tag>
        ) : (
          <span style={{ color: '#bbb', fontSize: 12 }}>—</span>
        )
      )
    },
    {
      title: '最近核验',
      dataIndex: 'lastConfirmed',
      key: 'lastConfirmed',
      sorter: (a, b) => (a.lastConfirmed ?? '').localeCompare(b.lastConfirmed ?? ''),
      render: (val?: string) => (
        val ? <span style={{ fontSize: 12 }}>{new Date(val).toLocaleDateString('zh-CN')}</span> : <span style={{ color: '#bbb', fontSize: 12 }}>未核验</span>
      )
    },
    {
      title: '操作',
      key: 'action',
      width: 140,
      render: (_, record) => (
        <Space size="small">
          <Button type="link" size="small" onClick={(e) => { e.stopPropagation(); onSelect(record) }}>
            详情 <ArrowRight size={13} style={{ verticalAlign: -2 }} />
          </Button>
          {record.type === 'SoftwareProject' && (
            <Tooltip title={`在新标签页打开「${record.name}」独立项目工作台 (无全局菜单)`}>
              <Button
                type="link"
                size="small"
                icon={<ExternalLink size={12} />}
                onClick={(e) => {
                  e.stopPropagation()
                  window.open(`/project/${encodeURIComponent(record.id)}`, '_blank')
                }}
                style={{ padding: '0 4px', color: '#42633c' }}
              >
                项目页
              </Button>
            </Tooltip>
          )}
        </Space>
      )
    }
  ]

  return (
    <div style={{ padding: '0 18px 18px' }}>
      <Table<Node>
        rowKey="id"
        columns={columns}
        dataSource={nodes}
        size="middle"
        pagination={{
          defaultPageSize: 10,
          showSizeChanger: true,
          pageSizeOptions: ['10', '20', '50'],
          showTotal: (total) => `共 ${total} 个对象`,
          style: { marginTop: 14 }
        }}
        onRow={(record) => ({
          onClick: () => onSelect(record),
          style: { cursor: 'pointer' },
          className: selected?.id === record.id ? 'ant-table-row-selected' : ''
        })}
      />
    </div>
  )
}

function ImpactList({ impacts, onSelect }: { impacts: Impact[]; onSelect: (node: Node) => void }) {
  if (!impacts.length) {
    return (
      <div className="empty-state">
        <div className="empty-art"><Activity size={22} /></div>
        <strong>暂无可传播的影响路径</strong>
        <span>仅知晓关系不会进入影响分析。维护、拥有和明确运行关系才会参与传播。</span>
      </div>
    )
  }

  return (
    <div className="impact-list">
      {impacts.map(item => (
        <button className="impact-row" key={item.node.id} onClick={() => onSelect(item.node)}>
          <div className={`type-icon ${typeColor[item.node.type]}`}>
            <TypeIcon type={item.node.type} />
          </div>
          <div className="impact-main">
            <strong>{item.node.name}</strong>
            <span>{item.relationPath.map(relation => relation.predicate).join('  →  ')}</span>
          </div>
          <Tag color={antdColorForType(item.node.type)}>{labels[item.node.type]}</Tag>
          <ArrowRight size={15} className="row-arrow" />
        </button>
      ))}
    </div>
  )
}

function GraphPreview({ self, impacts, onSelect }: { self?: Node; impacts: Impact[]; onSelect: (node: Node) => void }) {
  const items = impacts.slice(0, 4)
  if (!self || !items.length) {
    return (
      <div className="graph-empty">
        <div className="graph-empty-circle"><Network size={23} /></div>
        <strong>关系图谱正在等待明确的业务关系</strong>
        <span>仅知晓（knowsAbout）关系不会进入影响传播。</span>
        <button
          className="text-action"
          onClick={() => onSelect(self ?? { id: 'self', name: 'Self', type: 'Self', source: 'unspecified', evidenceKind: 'unknown', certainty: 'unknown', modelVersion: 1 })}
        >
          查看 Self 锚点 <ArrowRight size={14} />
        </button>
      </div>
    )
  }
  return (
    <div className="graph-canvas">
      <div className="graph-grid" />
      <svg className="graph-lines" viewBox="0 0 600 244" preserveAspectRatio="none" aria-hidden="true">
        {items.map((_, index) => (
          <path key={index} d={`M 300 120 C ${200 + index * 15} ${120 - 45 + index * 22}, 145 ${46 + index * 47}, 100 ${45 + index * 47}`} />
        ))}
      </svg>
      <button className="graph-node self-node" onClick={() => onSelect(self)}>
        <span className="graph-node-mark"><Sparkles size={18} /></span>
        <strong>Self</strong><small>认知锚点</small>
      </button>
      {items.map((item, index) => (
        <button key={item.node.id} className={`graph-node connected-node node-${index}`} onClick={() => onSelect(item.node)}>
          <span className={`graph-node-mark ${typeColor[item.node.type]}`}><TypeIcon type={item.node.type} /></span>
          <strong>{item.node.name}</strong><small>{item.relationPath.at(-1)?.predicate ?? labels[item.node.type]}</small>
        </button>
      ))}
      <div className="graph-legend">
        <span><i className="legend-dot self" />Self</span>
        <span><i className="legend-dot relation" />明确关系</span>
      </div>
    </div>
  )
}
