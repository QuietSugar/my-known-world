import { useEffect, useMemo, useState } from 'react'
import { ArrowRight, Network } from 'lucide-react'
import { api, type Exploration, type Node, type NodeType } from './api'
import { Alert, Card, Empty, Select, Space, Tag } from 'antd'
import { antdColorForType } from './theme'

const predicates = [
  'owns', 'maintains', 'knowsAbout', 'uses', 'canOperate', 'routesThrough',
  'providesAccessTo', 'hosts', 'implementedFrom', 'hasRunningPeriod', 'hasCredential',
  'documents', 'appliesTo', 'targets', 'supersedes', 'dependsOn', 'hasInstance', 'relatedTo'
]

const typeLabels: Record<NodeType, string> = {
  Self: 'Self',
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

export function GraphExplorer({ nodes, onSelect }: { nodes: Node[]; onSelect: (node: Node) => void }) {
  const [center, setCenter] = useState('')
  const [direction, setDirection] = useState('both')
  const [depth, setDepth] = useState(2)
  const [selectedPredicates, setSelectedPredicates] = useState<string[]>([])
  const [result, setResult] = useState<Exploration | null>(null)
  const [error, setError] = useState('')
  const byID = useMemo(() => new Map(nodes.map(node => [node.id, node])), [nodes])

  useEffect(() => {
    if (!center && nodes.length) {
      setCenter(nodes.find(node => node.type === 'Self')?.id ?? nodes[0].id)
    }
  }, [nodes, center])

  useEffect(() => {
    if (!center) return
    let active = true
    api.explore(center, direction, depth, selectedPredicates)
      .then(value => {
        if (active) {
          setResult(value)
          setError('')
        }
      })
      .catch((cause: unknown) => {
        if (active) {
          setResult(null)
          setError(cause instanceof Error ? cause.message : '关系探索失败')
        }
      })
    return () => {
      active = false
    }
  }, [center, direction, depth, selectedPredicates])

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

  return (
    <section className="panel graph-explorer">
      <div className="panel-head" style={{ marginBottom: 16 }}>
        <div>
          <div className="panel-title">关系子图探索 (Graph Explorer)</div>
          <div className="panel-sub">以任意对象为中心，筛选方向、深度与关系谓词。未知不补推。</div>
        </div>
        <Network size={20} style={{ color: '#42633c' }} />
      </div>

      <Card size="small" style={{ marginBottom: 16, background: '#fafafa' }}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap size="middle">
            <Space>
              <span style={{ fontSize: 12, color: '#666' }}>中心对象:</span>
              <Select
                showSearch
                style={{ width: 280 }}
                value={center || undefined}
                onChange={setCenter}
                filterOption={(input, option) =>
                  ((option?.label as string) ?? '').toLowerCase().includes(input.toLowerCase())
                }
                options={nodes.map(node => ({
                  label: `${node.name} (${typeLabels[node.type]})`,
                  value: node.id
                }))}
              />
            </Space>

            <Space>
              <span style={{ fontSize: 12, color: '#666' }}>方向:</span>
              <Select
                style={{ width: 120 }}
                value={direction}
                onChange={setDirection}
                options={[
                  { label: '双向链路', value: 'both' },
                  { label: '沿出向 (Out)', value: 'out' },
                  { label: '沿入向 (In)', value: 'in' }
                ]}
              />
            </Space>

            <Space>
              <span style={{ fontSize: 12, color: '#666' }}>探索深度:</span>
              <Select
                style={{ width: 90 }}
                value={depth}
                onChange={setDepth}
                options={[1, 2, 3, 4, 5].map(v => ({ label: `${v} 层`, value: v }))}
              />
            </Space>
          </Space>

          <Space style={{ width: '100%' }} align="start">
            <span style={{ fontSize: 12, color: '#666', marginTop: 4 }}>关系谓词过滤:</span>
            <Select
              mode="multiple"
              allowClear
              placeholder="默认探索所有关系谓词（可多选指定）"
              style={{ minWidth: 420, flex: 1 }}
              value={selectedPredicates}
              onChange={setSelectedPredicates}
              options={predicates.map(p => ({ label: p, value: p }))}
            />
          </Space>
        </Space>
      </Card>

      {error && <Alert type="error" message={error} showIcon style={{ marginBottom: 16 }} />}

      {result && (
        <div className="explore-result">
          <div className="explore-counts" style={{ marginBottom: 12, display: 'flex', alignItems: 'center', gap: 8 }}>
            <span>中心对象：<strong>{byID.get(result.centerId)?.name ?? result.centerId}</strong></span>
            <Tag color="blue">{result.nodes.length} 个对象</Tag>
            <Tag color="cyan">{result.relations.length} 条关系</Tag>
            {result.truncated && <Tag color="volcano">已达 250 节点上限，结果已截断</Tag>}
          </div>

          <svg className="explore-svg" viewBox="0 0 680 348" role="img" aria-label="筛选后的有向关系子图">
            <defs>
              <marker id="explore-arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
                <path d="M0,0 L8,4 L0,8 z" fill="#91a58b" />
              </marker>
            </defs>
            {result.relations.map(relation => {
              const from = positions.get(relation.from)
              const to = positions.get(relation.to)
              if (!from || !to) return null
              const x = (from.x + to.x) / 2
              const y = (from.y + to.y) / 2
              return (
                <g key={relation.id}>
                  <line x1={from.x} y1={from.y} x2={to.x} y2={to.y} markerEnd="url(#explore-arrow)" />
                  <text x={x} y={y - 6}>{relation.predicate}</text>
                </g>
              )
            })}
            {result.nodes.map(node => {
              const point = positions.get(node.id)
              if (!point) return null
              return (
                <g
                  key={node.id}
                  className={`explore-svg-node ${node.id === result.centerId ? 'center' : ''}`}
                  role="button"
                  tabIndex={0}
                  aria-label={`${node.name} (${typeLabels[node.type]})`}
                  onClick={() => onSelect(node)}
                  onKeyDown={event => {
                    if (event.key === 'Enter' || event.key === ' ') onSelect(node)
                  }}
                  transform={`translate(${point.x},${point.y})`}
                >
                  <circle r={node.id === result.centerId ? 29 : 23} />
                  <text className="node-name" y="-3">
                    {node.name.length > 16 ? `${node.name.slice(0, 15)}…` : node.name}
                  </text>
                  <text className="node-type" y="10">{typeLabels[node.type]}</text>
                </g>
              )
            })}
          </svg>

          <div className="explore-edge-list" style={{ marginTop: 16 }}>
            {result.relations.length === 0 ? (
              <Empty description="当前中心对象在指定深度和谓词下暂无关联边。" />
            ) : (
              result.relations.map(relation => {
                const fromNode = byID.get(relation.from)
                const toNode = byID.get(relation.to)
                return (
                  <div className="explore-edge" key={relation.id} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '6px 12px', borderBottom: '1px solid #f0f0f0' }}>
                    <button
                      onClick={() => { if (fromNode) onSelect(fromNode) }}
                      style={{ border: 0, background: 'none', cursor: 'pointer', textAlign: 'left', fontWeight: 600, color: '#333' }}
                    >
                      {fromNode?.name ?? relation.from} <Tag color={fromNode ? antdColorForType(fromNode.type) : 'default'}>{fromNode ? typeLabels[fromNode.type] : ''}</Tag>
                    </button>
                    <Space size={4}>
                      <Tag color="blue">{relation.predicate}</Tag>
                      <ArrowRight size={12} style={{ color: '#888' }} />
                    </Space>
                    <button
                      onClick={() => { if (toNode) onSelect(toNode) }}
                      style={{ border: 0, background: 'none', cursor: 'pointer', textAlign: 'right', fontWeight: 600, color: '#333' }}
                    >
                      <Tag color={toNode ? antdColorForType(toNode.type) : 'default'}>{toNode ? typeLabels[toNode.type] : ''}</Tag> {toNode?.name ?? relation.to}
                    </button>
                  </div>
                )
              })
            )}
          </div>
        </div>
      )}
    </section>
  )
}
