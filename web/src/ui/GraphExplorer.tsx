import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Network } from 'lucide-react'
import { api, type Exploration, type Node, type NodeType } from './api'

const predicates = ['owns', 'maintains', 'knowsAbout', 'uses', 'canOperate', 'routesThrough', 'providesAccessTo', 'hosts', 'implementedFrom', 'hasRunningPeriod', 'hasCredential', 'documents', 'appliesTo', 'targets', 'supersedes', 'dependsOn']
const typeLabels: Record<NodeType, string> = { Self: 'Self', Hardware: '硬件', Machine: '机器', SoftwareProject: '软件项目', SoftwareInstance: '部署实例', RunningPeriod: '运行周期', AccountCredential: '凭据元信息', Product: '产品', Document: '文档', Runbook: 'Runbook', WorkItem: '工作事项', Network:'网络', ProvisionalObject: '待分类' }

export function GraphExplorer({ nodes, onSelect }: { nodes: Node[]; onSelect: (node: Node) => void }) {
  const [center, setCenter] = useState('')
  const [direction, setDirection] = useState('both')
  const [depth, setDepth] = useState(2)
  const [selectedPredicates, setSelectedPredicates] = useState<string[]>([])
  const [result, setResult] = useState<Exploration | null>(null)
  const [error, setError] = useState('')
  const byID = useMemo(() => new Map(nodes.map(node => [node.id, node])), [nodes])

  useEffect(() => { if (!center && nodes.length) setCenter(nodes.find(node => node.type === 'Self')?.id ?? nodes[0].id) }, [nodes, center])
  useEffect(() => {
    if (!center) return
    let active = true
    api.explore(center, direction, depth, selectedPredicates)
      .then(value => { if (active) { setResult(value); setError('') } })
      .catch((cause: unknown) => { if (active) { setResult(null); setError(cause instanceof Error ? cause.message : '关系探索失败') } })
    return () => { active = false }
  }, [center, direction, depth, selectedPredicates])

  const toggle = (predicate: string) => setSelectedPredicates(old => old.includes(predicate) ? old.filter(item => item !== predicate) : [...old, predicate])
  const positions = useMemo(() => {
    if (!result) return new Map<string, { x: number; y: number }>()
    const centerNode = result.nodes.find(node => node.id === result.centerId)
    const others = result.nodes.filter(node => node.id !== result.centerId)
    const map = new Map<string, { x: number; y: number }>()
    if (centerNode) map.set(centerNode.id, { x: 340, y: 174 })
    const radius = Math.min(134, 45 + Math.max(0, others.length - 1) * 8)
    others.forEach((node, index) => {
      const angle = -Math.PI / 2 + (2 * Math.PI * index) / Math.max(others.length, 1)
      map.set(node.id, { x: 340 + radius * Math.cos(angle), y: 174 + radius * Math.sin(angle) })
    })
    return map
  }, [result])

  return <section className="panel graph-explorer">
    <div className="panel-head"><div><div className="panel-title">关系子图探索</div><div className="panel-sub">任意对象为中心，筛选方向、深度与谓词。未知不补推。</div></div><Network size={18} /></div>
    <div className="explore-controls">
      <label>中心对象<select value={center} onChange={event => setCenter(event.target.value)}>{nodes.map(node => <option key={node.id} value={node.id}>{node.name} · {typeLabels[node.type]}</option>)}</select></label>
      <label>方向<select value={direction} onChange={event => setDirection(event.target.value)}><option value="both">双向</option><option value="out">沿出向</option><option value="in">沿入向</option></select></label>
      <label>深度<select value={depth} onChange={event => setDepth(Number(event.target.value))}>{[1, 2, 3, 4, 5].map(value => <option key={value} value={value}>{value}</option>)}</select></label>
    </div>
    <details className="predicate-filters"><summary>关系筛选（默认全部）</summary><div>{predicates.map(predicate => <label key={predicate}><input type="checkbox" checked={selectedPredicates.includes(predicate)} onChange={() => toggle(predicate)} />{predicate}</label>)}</div></details>
    {error && <p className="sync-conflict">{error}</p>}
    {result && <div className="explore-result">
      <div className="explore-counts">中心 {byID.get(result.centerId)?.name ?? result.centerId} · {result.nodes.length} 个对象 · {result.relations.length} 条关系{result.truncated&&' · 已达 250 节点上限，结果已截断'}</div>
      <svg className="explore-svg" viewBox="0 0 680 348" role="img" aria-label="筛选后的有向关系子图">
        <defs><marker id="explore-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M0,0 L8,4 L0,8 z" fill="#91a58b" /></marker></defs>
        {result.relations.map(relation => { const from = positions.get(relation.from); const to = positions.get(relation.to); if (!from || !to) return null; const x = (from.x + to.x) / 2; const y = (from.y + to.y) / 2; return <g key={relation.id}><line x1={from.x} y1={from.y} x2={to.x} y2={to.y} markerEnd="url(#explore-arrow)" /><text x={x} y={y - 6}>{relation.predicate}</text></g> })}
        {result.nodes.map(node => { const point = positions.get(node.id); if (!point) return null; return <g key={node.id} className={`explore-svg-node ${node.id === result.centerId ? 'center' : ''}`} role="button" tabIndex={0} aria-label={`${node.name} (${typeLabels[node.type]})`} onClick={() => onSelect(node)} onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') onSelect(node) }} transform={`translate(${point.x},${point.y})`}><circle r={node.id === result.centerId ? 29 : 23} /><text className="node-name" y="-3">{node.name.length > 16 ? `${node.name.slice(0, 15)}…` : node.name}</text><text className="node-type" y="10">{typeLabels[node.type]}</text></g> })}
      </svg>
      <div className="explore-edge-list">{result.relations.map(relation => <div className="explore-edge" key={relation.id}><button onClick={() => { const node = byID.get(relation.from); if (node) onSelect(node) }}>{byID.get(relation.from)?.name ?? relation.from}</button><span>{relation.predicate}<ArrowRight size={12} /></span><button onClick={() => { const node = byID.get(relation.to); if (node) onSelect(node) }}>{byID.get(relation.to)?.name ?? relation.to}</button></div>)}</div>
    </div>}
  </section>
}
