import { useMemo, useState } from 'react'
import type { Node, NodeType } from './api'

type FieldKind = 'text' | 'list' | 'json' | 'boolean' | 'select'
type FieldSpec = { key: string; label: string; kind?: FieldKind; hint?: string; shape?: 'object' | 'array'; options?: Array<{ value: string; label: string }> }

const basicFields: FieldSpec[] = [
  { key: 'lifecycle', label: '生命周期状态' },
  { key: 'evidenceKind', label: '证据类型', kind: 'select', options: [{value:'unknown',label:'未知'},{value:'manual',label:'人工'},{value:'document',label:'文档'},{value:'observed',label:'观察'},{value:'derived',label:'派生'},{value:'imported',label:'导入'}] },
  { key: 'certainty', label: '确定性', kind: 'select', options: [{value:'unknown',label:'未知'},{value:'provisional',label:'暂定'},{value:'confirmed',label:'已确认'},{value:'disputed',label:'有冲突'},{value:'stale',label:'已过期'}] },
  { key: 'lastConfirmed', label: '最近确认时间', hint: 'RFC 3339 时间；留空表示未知' },
  { key: 'evidenceReference', label: '证据引用', hint: '只填安全引用，不填秘密或正文' },
]

const detailConfig: Partial<Record<NodeType, { key: string | null; fields: FieldSpec[] }>> = {
  RunningPeriod: { key: null, fields: [
    { key: 'startedAt', label: '开始时间', hint: 'RFC 3339，例如 2026-11-15T09:00:00Z' }, { key: 'endedAt', label: '结束时间', hint: '未知或仍在运行可留空' },
    { key: 'deployedVersion', label: '部署版本' }, { key: 'verificationAt', label: '验证时间', hint: 'RFC 3339' },
    { key: 'verificationResult', label: '验证结果' }, { key: 'evidenceRef', label: '安全证据引用' },
  ] },
  Hardware: { key: 'hardware', fields: [
    { key: 'category', label: '设备类别' }, { key: 'model', label: '型号' }, { key: 'location', label: '位置' }, { key: 'status', label: '状态' },
    { key: 'specifications', label: '规格（JSON 对象）', kind: 'json', shape: 'object' }, { key: 'accessReference', label: '访问说明引用' }, { key: 'maintenanceReference', label: '维护说明引用' },
  ] },
  Machine: { key: 'machine', fields: [
    { key: 'machineKind', label: '机器类别', hint: 'physical / virtual / shared' }, { key: 'hostname', label: '主机名' }, { key: 'os', label: '操作系统' }, { key: 'ips', label: 'IP 地址', kind: 'list' },
    { key: 'network', label: '网络' }, { key: 'status', label: '状态' }, { key: 'accessMethod', label: '访问方式' }, { key: 'accessPrerequisites', label: '访问前提', kind: 'list' },
    { key: 'allowedOperations', label: '允许操作', kind: 'list' }, { key: 'operationConstraints', label: '操作约束', kind: 'list' }, { key: 'forbiddenOperations', label: '禁止操作', kind: 'list' },
    { key: 'responsibilityBoundary', label: '责任边界' }, { key: 'resources', label: '资源声明（JSON 数组）', kind: 'json', shape: 'array' }, { key: 'lastVerified', label: '最近核验时间' },
  ] },
  SoftwareProject: { key: 'softwareProject', fields: [
    { key: 'purpose', label: '用途' }, { key: 'languages', label: '语言', kind: 'list' }, { key: 'form', label: '项目形态' }, { key: 'remoteRepository', label: '远程仓库引用' },
    { key: 'upstream', label: '上游引用' }, { key: 'maintenanceState', label: '维护状态' }, { key: 'recommendedVersion', label: '推荐版本' }, { key: 'successorId', label: '继任项目 ID' },
  ] },
  SoftwareInstance: { key: 'softwareInstance', fields: [
    { key: 'productName', label: '产品名称' }, { key: 'domains', label: '域名', kind: 'list' }, { key: 'deploymentPath', label: '部署路径' }, { key: 'deploymentMethod', label: '部署方式' },
    { key: 'runtimeIdentity', label: '运行身份（不含凭据）' }, { key: 'endpoints', label: '端点（JSON 数组）', kind: 'json', shape: 'array' }, { key: 'configurationReferences', label: '配置引用', kind: 'list' },
    { key: 'deploymentState', label: '部署状态' }, { key: 'operationalState', label: '运行状态' }, { key: 'maintenanceState', label: '维护状态' },
    { key: 'verificationResult', label: '验证结果' }, { key: 'verifiedAt', label: '验证时间' },
  ] },
  AccountCredential: { key: 'accountCredential', fields: [
    { key: 'account', label: '账号标识（非秘密）' }, { key: 'credentialType', label: '凭据类型（不含凭据值）' }, { key: 'purpose', label: '用途' }, { key: 'scope', label: '权限范围' },
    { key: 'accessReference', label: '安全访问引用' }, { key: 'rotationAt', label: '上次轮换时间' }, { key: 'status', label: '状态' }, { key: 'lastVerified', label: '最近核验时间' },
  ] },
  Product: { key: 'product', fields: [ { key: 'purpose', label: '用途' }, { key: 'commonConfigurationReference', label: '共性配置引用' }, { key: 'runbookReference', label: 'Runbook 引用' } ] },
  Document: { key: 'document', fields: [ { key: 'role', label: '文档角色' }, { key: 'authoritativeReference', label: '权威文档引用' }, { key: 'effective', label: '当前有效', kind: 'boolean' }, { key: 'lastVerified', label: '最近核验时间' }, { key: 'supersededBy', label: '替代文档 ID' } ] },
  Runbook: { key: 'runbook', fields: [
    { key: 'trigger', label: '触发条件' }, { key: 'preconditions', label: '前置条件', kind: 'list' }, { key: 'stepsReference', label: '步骤引用（不填正文）' },
    { key: 'stopConditionReference', label: '停止条件引用' }, { key: 'rollbackReference', label: '回退引用' }, { key: 'verificationMethod', label: '验证方法引用' },
    { key: 'lastVerified', label: '最近核验时间' }, { key: 'executableStatus', label: '可执行状态' }, { key: 'approvalRequired', label: '需要审批', kind: 'boolean' },
  ] },
  WorkItem: { key: 'workItem', fields: [ { key: 'kind', label: '事项类型' }, { key: 'status', label: '状态' }, { key: 'rationaleReference', label: '决策理由引用' }, { key: 'decisionEvidenceReference', label: '决策证据引用' } ] },
  Network: { key: 'network', fields: [
    { key: 'kind', label: '网络类型' }, { key: 'cidrs', label: 'CIDR 列表', kind: 'list' }, { key: 'gatewayReference', label: '网关引用' }, { key: 'accessLocations', label: '访问位置', kind: 'list' },
    { key: 'vpnReference', label: 'VPN 引用' }, { key: 'status', label: '状态' },
  ] },
}

function inputValue(value: unknown, kind: FieldKind): string {
  if (kind === 'boolean') return value === true ? 'true' : 'false'
  if (kind === 'list') return Array.isArray(value) ? value.map(item => String(item)).join('\n') : ''
  if (kind === 'json') return value === undefined ? '' : JSON.stringify(value, null, 2)
  return typeof value === 'string' ? value : ''
}

export function DetailsEditor({ node, onClose, onSave }: { node: Node; onClose: () => void; onSave: (updated: Node) => Promise<void> }) {
  const config = detailConfig[node.type]
  const original = useMemo(() => config?.key ? ((node as unknown as Record<string, Record<string, unknown>>)[config.key] ?? {}) : {}, [node, config])
  const [values, setValues] = useState<Record<string, string>>(() => Object.fromEntries((config?.fields ?? []).map(field => [field.key, inputValue(original[field.key], field.kind ?? 'text')])))
  const [basicValues, setBasicValues] = useState<Record<string, string>>(() => Object.fromEntries(basicFields.map(field => [field.key, inputValue((node as unknown as Record<string, unknown>)[field.key], field.kind ?? 'text')])))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  if (!config) return null
  const save = async () => {
    try {
      const updatedDetails: Record<string, unknown> = { ...original }
      for (const field of config.fields) {
        const raw = values[field.key] ?? ''
        if (field.kind === 'boolean') updatedDetails[field.key] = raw === 'true'
        else if (field.kind === 'list') {
          const items = raw.split(/\r?\n/).map(item => item.trim()).filter(Boolean)
          if (items.length) updatedDetails[field.key] = items
          else delete updatedDetails[field.key]
        } else if (field.kind === 'json') {
          if (!raw.trim()) { delete updatedDetails[field.key]; continue }
          const parsed: unknown = JSON.parse(raw)
          if (parsed === null || typeof parsed !== 'object') throw new Error(`${field.label} 必须是 JSON 对象或数组`)
          if (field.shape === 'array' && !Array.isArray(parsed)) throw new Error(`${field.label} 必须是 JSON 数组`)
          if (field.shape === 'object' && Array.isArray(parsed)) throw new Error(`${field.label} 必须是 JSON 对象`)
          updatedDetails[field.key] = parsed
        } else if (raw.trim()) updatedDetails[field.key] = raw.trim()
        else delete updatedDetails[field.key]
      }
      const updatedBase = {
        ...node,
        lifecycle: basicValues.lifecycle?.trim() || undefined,
        evidenceKind: (basicValues.evidenceKind || 'unknown') as Node['evidenceKind'],
        certainty: (basicValues.certainty || 'unknown') as Node['certainty'],
        lastConfirmed: basicValues.lastConfirmed?.trim() || undefined,
        evidenceReference: basicValues.evidenceReference?.trim() || undefined,
      }
      const updated = config.key ? { ...updatedBase, [config.key]: updatedDetails } as Node : { ...updatedBase, ...updatedDetails } as Node
      setBusy(true); setError('')
      await onSave(updated)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '字段内容无效')
    } finally { setBusy(false) }
  }
  return <div className="modal-scrim" role="presentation"><section className="panel create-node-dialog details-editor-dialog" role="dialog" aria-modal="true" aria-label={`${node.type} 类型详情编辑器`}>
    <div className="panel-head"><div><div className="panel-title">编辑{node.type}详情</div><div className="panel-sub">来源标识不可在手动编辑中更改；列表每行一项，结构化字段使用 JSON。</div></div><button className="icon-button" onClick={onClose} aria-label="关闭">×</button></div>
    <div className="details-source-lock"><span>来源标识（保持不变）</span><strong>{node.source || 'unspecified'}</strong></div>
    <div className="details-editor-grid">{basicFields.map(field => <label className="details-editor-field" key={field.key}><span>{field.label}</span>{field.kind === 'select' ? <select value={basicValues[field.key] ?? ''} onChange={event => setBasicValues(old => ({ ...old, [field.key]: event.target.value }))}>{field.options?.map(option=><option value={option.value} key={option.value}>{option.label}</option>)}</select> : <input value={basicValues[field.key] ?? ''} onChange={event => setBasicValues(old => ({ ...old, [field.key]: event.target.value }))} placeholder={field.hint} />}</label>)}</div>
    <div className="details-editor-grid">{config.fields.map(field => <label className="details-editor-field" key={field.key}><span>{field.label}</span>{field.kind === 'boolean' ? <select value={values[field.key] ?? 'false'} onChange={event => setValues(old => ({ ...old, [field.key]: event.target.value }))}><option value="false">否</option><option value="true">是</option></select> : field.kind === 'list' || field.kind === 'json' ? <textarea value={values[field.key] ?? ''} onChange={event => setValues(old => ({ ...old, [field.key]: event.target.value }))} placeholder={field.kind === 'list' ? '每行一个值' : field.hint ?? '{}'} spellCheck={false} /> : <input value={values[field.key] ?? ''} onChange={event => setValues(old => ({ ...old, [field.key]: event.target.value }))} placeholder={field.hint} />}</label>)}</div>
    <small className="details-editor-warning">不要填写密码、令牌、私钥、文档正文或低熵敏感值；此界面仅保存元数据与安全引用。</small>
    {error && <p className="sync-conflict">{error}</p>}
    <div className="sync-batch-actions"><button onClick={() => void save()} disabled={busy}>{busy ? '保存中…' : '预览并保存'}</button><button className="text-button" onClick={onClose} disabled={busy}>取消</button></div>
  </section></div>
}
