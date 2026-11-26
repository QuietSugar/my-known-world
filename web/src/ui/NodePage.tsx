import { useEffect, useMemo, useState } from 'react'
import { Alert, Card, Descriptions, Space, Spin, Tag, Typography } from 'antd'
import { api } from './api'
import type { Exploration, Node, Relation } from './api'

const TYPE_LABELS: Record<string, string> = {
  Self: '自身',
  Hardware: '硬件',
  Machine: '机器',
  SoftwareProject: '软件项目',
  SoftwareInstance: '服务实例',
  RunningPeriod: '运行期',
  AccountCredential: '凭据',
  Product: '产品',
  Document: '文档',
  Runbook: '运维手册',
  WorkItem: '工作项',
  Network: '网络',
  ProvisionalObject: '暂存对象',
}

const TYPE_COLORS: Record<string, string> = {
  Self: 'purple',
  Hardware: 'gold',
  Machine: 'blue',
  SoftwareProject: 'green',
  SoftwareInstance: 'cyan',
  RunningPeriod: 'default',
  AccountCredential: 'red',
  Product: 'lime',
  Document: 'geekblue',
  Runbook: 'orange',
  WorkItem: 'magenta',
  Network: 'volcano',
  ProvisionalObject: 'default',
}

const DETAIL_SECTIONS: Array<{ key: string; label: string }> = [
  { key: 'hardware', label: '硬件信息' },
  { key: 'machine', label: '机器信息' },
  { key: 'softwareProject', label: '软件项目' },
  { key: 'softwareInstance', label: '服务实例' },
  { key: 'accountCredential', label: '凭据' },
  { key: 'product', label: '产品' },
  { key: 'document', label: '文档' },
  { key: 'runbook', label: '运维手册' },
  { key: 'workItem', label: '工作项' },
  { key: 'network', label: '网络' },
]

const FIELD_LABELS: Record<string, string> = {
  category: '类别',
  model: '型号',
  location: '位置',
  specifications: '规格',
  status: '状态',
  machineKind: '机器类型',
  roles: '角色',
  hostname: '主机名',
  os: '操作系统',
  ips: 'IP 地址',
  network: '网络',
  accessMethod: '访问方式',
  accessPrerequisites: '访问前提',
  allowedOperations: '允许操作',
  forbiddenOperations: '禁止操作',
  responsibilityBoundary: '责任边界',
  resources: '资源占用',
  spec: '规格',
  lastVerified: '最近核验',
  purpose: '用途',
  languages: '语言',
  form: '形态',
  remoteRepository: '远程仓库',
  upstream: '上游',
  maintenanceState: '维护状态',
  recommendedVersion: '推荐版本',
  successorId: '继任节点',
  productName: '所属产品',
  domains: '域名',
  ports: '端口',
  deploymentPath: '部署路径',
  deploymentMethod: '部署方式',
  runtimeIdentity: '运行时标识',
  endpoints: '端点',
  configurationReferences: '恢复资产',
  deploymentState: '部署状态',
  operationalState: '运行状态',
  notes: '备注',
  verificationResult: '核验结果',
  verifiedAt: '核验时间',
  account: '账号',
  credentialType: '凭据类型',
  scope: '范围',
  accessReference: '凭据引用',
  rotationAt: '轮换时间',
  commonConfigurationReference: '通用配置',
  runbookReference: '运维手册',
  role: '角色',
  authoritativeReference: '权威来源',
  effective: '是否生效',
  supersededBy: '被取代于',
  trigger: '触发条件',
  preconditions: '前置条件',
  stepsReference: '步骤',
  stopConditionReference: '停止条件',
  rollbackReference: '回滚',
  verificationMethod: '验证方法',
  executableStatus: '可执行状态',
  approvalRequired: '需批准',
  kind: '类型',
  cidrs: '网段',
  gatewayReference: '网关',
  accessLocations: '访问位置',
  vpnReference: 'VPN',
  rationaleReference: '决策依据',
  decisionEvidenceReference: '证据',
}

function labelFor(key: string): string {
  return FIELD_LABELS[key] || key
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return ''
  if (Array.isArray(value)) {
    return value.map(formatValue).filter(Boolean).join('、')
  }
  if (typeof value === 'object') {
    return Object.entries(value as Record<string, unknown>)
      .map(([key, item]) => {
        const text = formatValue(item)
        return text ? `${labelFor(key)}: ${text}` : ''
      })
      .filter(Boolean)
      .join('；')
  }
  if (typeof value === 'boolean') return value ? '是' : '否'
  return String(value)
}

function relationRows(node: Node, exploration: Exploration | null) {
  if (!exploration) return []
  const names = new Map(exploration.nodes.map(item => [item.id, item]))
  return exploration.relations
    .filter((relation: Relation) => relation.from === node.id || relation.to === node.id)
    .map((relation: Relation) => {
      const outgoing = relation.from === node.id
      const otherId = outgoing ? relation.to : relation.from
      const other = names.get(otherId)
      return { relation, outgoing, otherId, other }
    })
}

export function NodePage({ id }: { id: string }) {
  const [node, setNode] = useState<Node | null>(null)
  const [exploration, setExploration] = useState<Exploration | null>(null)
  const [error, setError] = useState('')

  useEffect(() => {
    let alive = true
    setNode(null)
    setError('')
    api.node(id)
      .then(result => {
        if (!alive) return
        setNode(result)
        document.title = `${result.name} · MyKnownWorld`
      })
      .catch((err: Error) => {
        if (!alive) return
        setError(err.message || '节点不存在')
        document.title = '节点不存在 · MyKnownWorld'
      })
    api.explore(id, 'both', 1, [])
      .then(result => { if (alive) setExploration(result) })
      .catch(() => { if (alive) setExploration(null) })
    return () => { alive = false }
  }, [id])

  const relations = useMemo(() => (node ? relationRows(node, exploration) : []), [node, exploration])

  return (
    <div style={{ minHeight: '100vh', background: '#f7f8f5' }}>
      <header style={{ borderBottom: '1px solid #e9ebe6', background: '#fff' }}>
        <div style={{ maxWidth: 900, margin: '0 auto', padding: '0 20px', height: 56, display: 'flex', alignItems: 'center', gap: 12 }}>
          <a href="/" style={{ color: '#42633c', fontWeight: 600, textDecoration: 'none' }}>← MyKnownWorld</a>
          <span style={{ color: '#b3b8b0', fontSize: 12 }}>节点详情</span>
        </div>
      </header>

      <main style={{ maxWidth: 900, margin: '0 auto', padding: '28px 20px 64px' }}>
        {error && (
          <Alert
            type="warning"
            showIcon
            message={`没有找到这个节点：${error}`}
            description={<span style={{ fontSize: 12 }}>节点 ID：<code>{id}</code>。可能是 UUID 写错，或该节点已被删除。<a href="/" style={{ marginLeft: 8 }}>返回首页</a></span>}
          />
        )}

        {!error && !node && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, color: '#8f958b', padding: '60px 0' }}>
            <Spin size="small" /> 正在读取节点…
          </div>
        )}

        {node && (
          <>
            <Space wrap align="center" style={{ marginBottom: 10 }}>
              <Tag color={TYPE_COLORS[node.type] || 'default'}>{TYPE_LABELS[node.type] || node.type}</Tag>
              {node.lifecycle && <Tag>生命周期: {node.lifecycle}</Tag>}
              {node.certainty && <Tag color="default">确定性: {node.certainty}</Tag>}
            </Space>
            <Typography.Title level={2} style={{ margin: '4px 0 6px' }}>{node.name}</Typography.Title>
            <div style={{ fontFamily: 'monospace', fontSize: 12, color: '#92968f', marginBottom: 22 }}>{node.id}</div>

            <Card size="small" title="基本已知信息" style={{ marginBottom: 16 }}>
              <Descriptions size="small" column={1} bordered>
                <Descriptions.Item label="类型">{TYPE_LABELS[node.type] || node.type}</Descriptions.Item>
                <Descriptions.Item label="生命周期">{node.lifecycle || '未记录'}</Descriptions.Item>
                <Descriptions.Item label="来源">{node.source === 'unspecified' ? '未说明' : node.source}</Descriptions.Item>
                <Descriptions.Item label="证据类型">{node.evidenceKind || '未记录'}</Descriptions.Item>
                <Descriptions.Item label="确定性">{node.certainty || '未记录'}</Descriptions.Item>
                <Descriptions.Item label="最近核验">{node.lastConfirmed ? new Date(node.lastConfirmed).toLocaleString('zh-CN') : '尚未核验'}</Descriptions.Item>
              </Descriptions>
            </Card>

            {DETAIL_SECTIONS.map(section => {
              const details = (node as unknown as Record<string, unknown>)[section.key]
              if (!details || typeof details !== 'object' || Array.isArray(details)) return null
              const entries = Object.entries(details as Record<string, unknown>)
                .map(([key, value]) => [key, formatValue(value)] as const)
                .filter(([, value]) => Boolean(value))
              if (!entries.length) return null
              return (
                <Card key={section.key} size="small" title={section.label} style={{ marginBottom: 16 }}>
                  <Descriptions size="small" column={1} bordered>
                    {entries.map(([key, value]) => (
                      <Descriptions.Item key={key} label={labelFor(key)}>
                        <span style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{value}</span>
                      </Descriptions.Item>
                    ))}
                  </Descriptions>
                </Card>
              )
            })}

            <Card size="small" title={`直接关系（${relations.length} 条）`}>
              {relations.length === 0 ? (
                <div style={{ color: '#92968f', fontSize: 12 }}>没有查到直接关系。</div>
              ) : (
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  {relations.map(({ relation, outgoing, otherId, other }) => (
                    <div key={relation.id} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 0', borderBottom: '1px solid #f0f1ed', fontSize: 13 }}>
                      {outgoing ? (
                        <>
                          <span style={{ color: '#8a9087' }}>—[{relation.predicate}]→</span>
                          <a href={`/node/${otherId}`} style={{ color: '#42633c' }}>{other?.name || otherId}</a>
                          {other && <Tag>{TYPE_LABELS[other.type] || other.type}</Tag>}
                        </>
                      ) : (
                        <>
                          <a href={`/node/${otherId}`} style={{ color: '#42633c' }}>{other?.name || otherId}</a>
                          {other && <Tag>{TYPE_LABELS[other.type] || other.type}</Tag>}
                          <span style={{ color: '#8a9087' }}>—[{relation.predicate}]→ 本节点</span>
                        </>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </Card>

            <p style={{ marginTop: 24, fontSize: 12, color: '#9aa096' }}>
              本页只读，数据来自 MyKnownWorld 图谱；wiki 正文见对应档案文件。
            </p>
          </>
        )}
      </main>
    </div>
  )
}
