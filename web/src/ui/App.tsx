import { useEffect, useMemo, useState } from 'react'
import { Activity, ArrowDownRight, ArrowRight, Box, ChevronDown, CircleHelp, Cloud, Command, Database, FileText, GitBranch, HardDrive, Layers3, LifeBuoy, Network, Plus, Search, Settings2, ShieldCheck, Sparkles, TerminalSquare, X } from 'lucide-react'
import { api, type Impact, type Node, type NodeType, type Relation } from './api'

type View = 'overview' | 'all' | 'impact' | 'provisional'
const labels: Record<NodeType, string> = { Self: '我自己', Hardware: '硬件', Machine: '机器', SoftwareProject: '软件项目', SoftwareInstance: '部署实例', RunningPeriod: '运行周期', AccountCredential: '凭据元信息', Product: '产品', Document: '文档', Runbook: 'Runbook', WorkItem: '工作事项', ProvisionalObject: '待分类' }
const navTypes: NodeType[] = ['Hardware', 'Machine', 'SoftwareProject', 'SoftwareInstance', 'AccountCredential', 'Document', 'WorkItem', 'ProvisionalObject']
const iconFor: Record<NodeType, typeof Box> = { Self: Sparkles, Hardware: HardDrive, Machine: TerminalSquare, SoftwareProject: GitBranch, SoftwareInstance: Layers3, RunningPeriod: Activity, AccountCredential: ShieldCheck, Product: Box, Document: FileText, Runbook: LifeBuoy, WorkItem: CircleHelp, ProvisionalObject: CircleHelp }
const typeColor: Record<NodeType, string> = { Self: 'violet', Hardware: 'blue', Machine: 'green', SoftwareProject: 'amber', SoftwareInstance: 'teal', RunningPeriod: 'teal', AccountCredential: 'rose', Product: 'blue', Document: 'slate', Runbook: 'amber', WorkItem: 'violet', ProvisionalObject: 'rose' }
const evidenceLabel = { unknown: '未知', manual: '人工确认', document: '文档声明', observed: '现场核验', derived: '系统派生', imported: '导入' }
const certaintyLabel = { unknown: '未知', provisional: '暂定', confirmed: '已确认', disputed: '有冲突', stale: '已过期' }

export function App() {
  const [nodes, setNodes] = useState<Node[]>([])
  const [impacts, setImpacts] = useState<Impact[]>([])
  const [selected, setSelected] = useState<Node | null>(null)
  const [path, setPath] = useState<{ nodes: Node[]; relations: Relation[] } | null>(null)
  const [view, setView] = useState<View>('overview')
  const [filter, setFilter] = useState<NodeType | 'all'>('all')
  const [query, setQuery] = useState('')
  const [online, setOnline] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    Promise.all([api.nodes(), api.impact(), api.health()])
      .then(([nodeResult, impactResult]) => { if (active) { setNodes(nodeResult.items); setImpacts(impactResult.items); setOnline(true); setError('') } })
      .catch((cause: unknown) => { if (active) { setOnline(false); setError(cause instanceof Error ? cause.message : '无法连接 API') } })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [])

  useEffect(() => {
    if (!selected) { setPath(null); return }
    let active = true
    api.path(selected.id).then(result => { if (active) setPath(result) }).catch(() => { if (active) setPath(null) })
    return () => { active = false }
  }, [selected])

  const self = nodes.find(node => node.type === 'Self')
  const visibleNodes = useMemo(() => nodes.filter(node => {
    const matchesFilter = filter === 'all' || node.type === filter
    const matchesQuery = `${node.name} ${node.type} ${node.id}`.toLowerCase().includes(query.toLowerCase())
    const matchesView = view !== 'provisional' || node.type === 'ProvisionalObject'
    return node.type !== 'Self' && matchesFilter && matchesQuery && matchesView
  }), [nodes, filter, query, view])
  const recent = [...nodes].filter(node => node.lastConfirmed).sort((a, b) => (b.lastConfirmed ?? '').localeCompare(a.lastConfirmed ?? '')).slice(0, 3)
  const counts = new Map<NodeType, number>()
  nodes.forEach(node => counts.set(node.type, (counts.get(node.type) ?? 0) + 1))
  const impactIds = new Set(impacts.map(item => item.node.id))
  const focus = (node: Node) => { setSelected(node) }

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark"><Network size={19} strokeWidth={2.3} /></div><span>my<span className="brand-light">known</span>world</span><button className="workspace-switch" aria-label="切换工作区"><ChevronDown size={14} /></button></div>
      <div className="workspace"><div className="workspace-avatar">M</div><div><strong>我的世界</strong><small>个人认知图谱</small></div><span className={`status-dot ${online ? 'on' : ''}`} title={online ? 'API 在线' : 'API 离线'} /></div>
      <div className="side-label">工作区</div>
      <button className={`nav-item ${view === 'overview' ? 'active' : ''}`} onClick={() => { setView('overview'); setFilter('all') }}><Layers3 size={17} /><span>总览</span><span className="nav-kbd">⌘ 1</span></button>
      <button className={`nav-item ${view === 'all' ? 'active' : ''}`} onClick={() => { setView('all'); setFilter('all') }}><Database size={17} /><span>全部对象</span><span className="nav-count">{Math.max(0, nodes.length - 1)}</span></button>
      <button className={`nav-item ${view === 'impact' ? 'active' : ''}`} onClick={() => { setView('impact'); setFilter('all') }}><Activity size={17} /><span>影响范围</span></button>
      <div className="side-label category-label">分类 <button aria-label="分类设置"><Settings2 size={14} /></button></div>
      {navTypes.map(type => { return <button key={type} className={`nav-item sub-item ${filter === type ? 'active' : ''}`} onClick={() => { setFilter(type); setView('all') }}><span className={`type-dot ${typeColor[type]}`} /><span>{labels[type]}</span><span className="nav-count">{counts.get(type) ?? 0}</span></button> })}
      <div className="sidebar-bottom"><div className="storage-card"><div className="storage-top"><span className="storage-icon"><Cloud size={15} /></span><span>{online ? '数据连接正常' : '等待 API 连接'}</span></div><p>{online ? '只读图谱已同步' : '检查本地 API 服务'}</p><div className="storage-track"><span style={{ width: online ? '72%' : '8%' }} /></div><small>{online ? 'PostgreSQL / API' : '离线模式'}</small></div><button className="nav-item"><CircleHelp size={17} /><span>帮助与文档</span></button><div className="profile-row"><div className="profile-avatar">M</div><div><strong>Me</strong><small>本地空间</small></div><button aria-label="个人设置"><Settings2 size={16} /></button></div></div>
    </aside>

    <main className="main-area">
      <header className="topbar"><div className="breadcrumbs"><span>我的世界</span><span className="crumb-slash">/</span><strong>{view === 'overview' ? '总览' : view === 'impact' ? '影响范围' : view === 'provisional' ? '待分类对象' : filter === 'all' ? '全部对象' : labels[filter]}</strong></div><div className="top-actions"><div className={`connection ${online ? 'connected' : ''}`}><span />{online ? '已连接' : '未连接'}</div><div className="top-divider" /><button className="icon-button" aria-label="搜索"><Command size={16} /></button><div className="top-avatar">M</div></div></header>
      <div className="content">
        <section className="welcome-row"><div><div className="eyebrow"><span className="eyebrow-line" />个人认知图谱 <span className="eyebrow-dot">·</span> <span>只记录我已知的世界</span></div><h1>{view === 'overview' ? <>你的世界，<span>一目了然。</span></> : view === 'impact' ? <>关系与<span>影响路径。</span></> : view === 'provisional' ? <>先收集，再<span>慢慢确认。</span></> : <>所有对象，<span>清晰连接。</span></>}</h1><p className="subtitle">从对象、关系与来源出发，理解哪些事物与你有关。</p></div><button className="add-button" onClick={() => setError('目前 API 为只读模式；对象录入与审查流程尚未开放。')}><Plus size={17} /> 新建对象</button></section>

        {error && <div className="alert"><CircleHelp size={16} /><span>{error}</span><button onClick={() => setError('')} aria-label="关闭"><X size={15} /></button></div>}
        {loading ? <div className="loading-state"><span className="spinner" /> 正在读取你的图谱…</div> : <>
          {view === 'overview' && <>
            <section className="summary-grid">
              <SummaryCard label="已知对象" value={Math.max(0, nodes.length - 1)} hint="不含 Self 锚点" icon={<Box size={17} />} tone="lilac" />
              <SummaryCard label="影响范围" value={impacts.length} hint="排除仅知晓关系" icon={<Activity size={17} />} tone="mint" />
              <SummaryCard label="待分类" value={counts.get('ProvisionalObject') ?? 0} hint="等待后续审查" icon={<CircleHelp size={17} />} tone="peach" />
              <SummaryCard label="已维护对象" value={impacts.filter(item => item.relationPath.some(relation => relation.predicate === 'maintains')).length} hint="从 Self 出发可追溯" icon={<ShieldCheck size={17} />} tone="bluewash" />
            </section>
            <section className="overview-grid">
              <div className="panel graph-panel"><div className="panel-head"><div><div className="panel-title">关系图谱</div><div className="panel-sub">从 Self 出发的已确认影响关系</div></div><button className="text-action" onClick={() => setView('impact')}>查看影响 <ArrowRight size={14} /></button></div><GraphPreview self={self} impacts={impacts} onSelect={focus} /></div>
              <div className="panel pulse-panel"><div className="panel-head"><div><div className="panel-title">图谱状态</div><div className="panel-sub">当前认知概况</div></div><span className="tiny-live"><span /> 已查询</span></div><div className="pulse-score"><div className="score-number">{nodes.length ? '—' : '0'}<span> / 100</span></div><div className="score-label">图谱完整度暂不评分</div></div><div className="pulse-divider" /><div className="pulse-row"><span className="pulse-bullet green" />所有对象可追溯至 Self<span className="pulse-check">✓</span></div><div className="pulse-row"><span className="pulse-bullet blue" />影响分析排除 knowsAbout<span className="pulse-check">✓</span></div><div className="pulse-row"><span className="pulse-bullet amber" />来源与核验时间持续完善<span className="pulse-muted">进行中</span></div></div>
            </section>
            <section className="panel objects-panel"><div className="panel-head object-head"><div><div className="panel-title">最近确认的对象</div><div className="panel-sub">按最近核验时间排序</div></div><button className="text-action" onClick={() => { setView('all'); setFilter('all') }}>全部对象 <ArrowRight size={14} /></button></div><ObjectTable nodes={recent} impacts={impactIds} selected={selected} onSelect={focus} /></section>
          </>}
          {view !== 'overview' && <section className="panel objects-panel listing-panel"><div className="list-toolbar"><div><div className="panel-title">{view === 'impact' ? '影响传播路径' : view === 'provisional' ? '待确认对象' : filter === 'all' ? '对象目录' : labels[filter]}</div><div className="panel-sub">{view === 'impact' ? '仅沿明确的业务与运行关系传播' : '对象关系与状态可独立追溯'}</div></div><label className="search-box"><Search size={15} /><input value={query} onChange={event => setQuery(event.target.value)} placeholder="搜索对象名称或类型…" /></label></div>{view === 'impact' ? <ImpactList impacts={impacts.filter(item => `${item.node.name} ${labels[item.node.type]}`.toLowerCase().includes(query.toLowerCase()))} onSelect={focus} /> : <ObjectTable nodes={visibleNodes} impacts={impactIds} selected={selected} onSelect={focus} />}</section>}
        </>}
        <footer className="page-footer"><span>MyKnownWorld</span><span>准确表达已知，不臆测未知。</span><span>模型版本 1.0</span></footer>
      </div>
    </main>

    {selected && <><button className="drawer-scrim" onClick={() => setSelected(null)} aria-label="关闭详情" /><aside className="detail-drawer"><div className="drawer-head"><span className="drawer-kicker">对象详情</span><button className="icon-button" onClick={() => setSelected(null)} aria-label="关闭"><X size={18} /></button></div><div className="detail-icon-wrap"><TypeIcon type={selected.type} /></div><div className="detail-type">{labels[selected.type]}</div><h2>{selected.name}</h2><div className="detail-id">{selected.id}</div><div className="detail-chips">{selected.lifecycle && <span className="chip lifecycle"><span />{selected.lifecycle}</span>}<span className="chip">模型 v{selected.modelVersion}</span></div><div className="drawer-section"><div className="section-heading">与 Self 的关系路径 <span>{path?.relations.length ?? 0} 条关系</span></div>{path ? path.relations.length ? <div className="path-stack">{path.nodes.map((node, index) => <div className="path-step" key={node.id}><div className={`path-icon ${typeColor[node.type]}`}><TypeIcon type={node.type} /></div><div className="path-copy"><strong>{node.name}</strong><small>{labels[node.type]}</small></div>{index < path.nodes.length - 1 && <div className="path-edge"><span>{path.relations[index]?.predicate}</span><ArrowDownRight size={14} /></div>}</div>)}</div> : <div className="empty-note">Self 节点本身是图谱锚点。</div> : <div className="loading-line"><span className="spinner small" /> 正在查询关系路径</div>}</div><div className="drawer-section"><div className="section-heading">已知信息</div><div className="fact-row"><span>生命周期</span><strong>{selected.lifecycle || '未记录'}</strong></div><div className="fact-row"><span>信息来源</span><strong>{selected.source === 'unspecified' ? '未说明' : selected.source}</strong></div><div className="fact-row"><span>证据类型 / 确定性</span><strong>{evidenceLabel[selected.evidenceKind]} / {certaintyLabel[selected.certainty]}</strong></div><div className="fact-row"><span>最近核验</span><strong>{selected.lastConfirmed ? new Date(selected.lastConfirmed).toLocaleDateString('zh-CN') : '尚未核验'}</strong></div><div className="fact-row"><span>节点标识</span><strong className="mono">{selected.id}</strong></div></div><div className="drawer-note"><ShieldCheck size={16} /><span>路径展示已记录的关系；可达不代表当前运行或状态正常。</span></div></aside></>}
  </div>
}

function SummaryCard({ label, value, hint, icon, tone }: { label: string; value: number; hint: string; icon: React.ReactNode; tone: string }) { return <div className="summary-card"><div className={`summary-icon ${tone}`}>{icon}</div><div className="summary-label">{label}</div><div className="summary-value">{value}<span className="summary-unit"> 个</span></div><div className="summary-hint">{hint}</div></div> }
function TypeIcon({ type }: { type: NodeType }) { const Icon = iconFor[type]; return <Icon size={17} strokeWidth={1.8} /> }

function ObjectTable({ nodes, impacts, selected, onSelect }: { nodes: Node[]; impacts: Set<string>; selected: Node | null; onSelect: (node: Node) => void }) {
  if (!nodes.length) return <div className="empty-state"><div className="empty-art"><Layers3 size={22} /></div><strong>还没有匹配的对象</strong><span>图谱只展示已明确建模的对象，不会根据名称推测关系。</span></div>
  return <div className="table-wrap"><table><thead><tr><th>对象</th><th>类型</th><th>生命周期</th><th>确定性</th><th>影响范围</th><th>最近核验</th><th /></tr></thead><tbody>{nodes.map(node => <tr key={node.id} className={selected?.id === node.id ? 'selected-row' : ''} onClick={() => onSelect(node)}><td><div className="object-cell"><div className={`type-icon ${typeColor[node.type]}`}><TypeIcon type={node.type} /></div><div><strong>{node.name}</strong><small>{node.id}</small></div></div></td><td><span className="type-label">{labels[node.type]}</span></td><td>{node.lifecycle ? <span className="lifecycle-cell"><span />{node.lifecycle}</span> : <span className="muted">未记录</span>}</td><td><span className={`certainty ${node.certainty}`}>{certaintyLabel[node.certainty]}</span></td><td>{impacts.has(node.id) ? <span className="impact-label"><Activity size={13} /> 可传播</span> : <span className="muted">—</span>}</td><td>{node.lastConfirmed ? new Date(node.lastConfirmed).toLocaleDateString('zh-CN') : <span className="muted">未核验</span>}</td><td><ArrowRight size={15} className="row-arrow" /></td></tr>)}</tbody></table></div>
}

function ImpactList({ impacts, onSelect }: { impacts: Impact[]; onSelect: (node: Node) => void }) {
  if (!impacts.length) return <div className="empty-state"><div className="empty-art"><Activity size={22} /></div><strong>暂无可传播的影响路径</strong><span>仅知晓关系不会进入影响分析。维护、拥有和明确运行关系才会参与传播。</span></div>
  return <div className="impact-list">{impacts.map(item => <button className="impact-row" key={item.node.id} onClick={() => onSelect(item.node)}><div className={`type-icon ${typeColor[item.node.type]}`}><TypeIcon type={item.node.type} /></div><div className="impact-main"><strong>{item.node.name}</strong><span>{item.relationPath.map(relation => relation.predicate).join('  →  ')}</span></div><span className="impact-kind">{labels[item.node.type]}</span><ArrowRight size={15} className="row-arrow" /></button>)}</div>
}

function GraphPreview({ self, impacts, onSelect }: { self?: Node; impacts: Impact[]; onSelect: (node: Node) => void }) {
  const items = impacts.slice(0, 4)
  if (!self || !items.length) return <div className="graph-empty"><div className="graph-empty-circle"><Network size={23} /></div><strong>关系图谱正在等待明确的业务关系</strong><span>仅知晓（knowsAbout）关系不会进入影响传播。</span><button className="text-action" onClick={() => onSelect(self ?? { id: 'self', name: 'Self', type: 'Self', source: 'unspecified', evidenceKind: 'unknown', certainty: 'unknown', modelVersion: 1 })}>查看 Self 锚点 <ArrowRight size={14} /></button></div>
  return <div className="graph-canvas"><div className="graph-grid" /><svg className="graph-lines" viewBox="0 0 600 244" preserveAspectRatio="none" aria-hidden="true">{items.map((_, index) => <path key={index} d={`M 300 120 C ${200 + index * 15} ${120 - 45 + index * 22}, 145 ${46 + index * 47}, 100 ${45 + index * 47}`} />)}</svg><button className="graph-node self-node" onClick={() => onSelect(self)}><span className="graph-node-mark"><Sparkles size={18} /></span><strong>Self</strong><small>认知锚点</small></button>{items.map((item, index) => <button key={item.node.id} className={`graph-node connected-node node-${index}`} onClick={() => onSelect(item.node)}><span className={`graph-node-mark ${typeColor[item.node.type]}`}><TypeIcon type={item.node.type} /></span><strong>{item.node.name}</strong><small>{item.relationPath.at(-1)?.predicate ?? labels[item.node.type]}</small></button>)}<div className="graph-legend"><span><i className="legend-dot self" />Self</span><span><i className="legend-dot relation" />明确关系</span></div></div>
}
