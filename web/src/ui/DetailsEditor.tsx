import { useMemo, useState } from 'react'
import { Modal, Form, Input, Select, Tabs, Alert, Space, Typography, App as AntdApp } from 'antd'
import { Settings, ShieldCheck } from 'lucide-react'
import type { Node, NodeType } from './api'

const { Text } = Typography

type FieldKind = 'text' | 'list' | 'json' | 'boolean' | 'select'
type FieldSpec = {
  key: string
  label: string
  kind?: FieldKind
  hint?: string
  shape?: 'object' | 'array'
  options?: Array<{ value: string; label: string }>
}

const basicFields: FieldSpec[] = [
  { key: 'lifecycle', label: '生命周期状态', hint: '例如 active, running, retired' },
  {
    key: 'evidenceKind',
    label: '证据类型',
    kind: 'select',
    options: [
      { value: 'unknown', label: '未知 (unknown)' },
      { value: 'manual', label: '人工确认 (manual)' },
      { value: 'document', label: '文档声明 (document)' },
      { value: 'observed', label: '现场核验 (observed)' },
      { value: 'derived', label: '系统派生 (derived)' },
      { value: 'imported', label: '外部导入 (imported)' }
    ]
  },
  {
    key: 'certainty',
    label: '确定性',
    kind: 'select',
    options: [
      { value: 'unknown', label: '未知 (unknown)' },
      { value: 'provisional', label: '暂定 (provisional)' },
      { value: 'confirmed', label: '已确认 (confirmed)' },
      { value: 'disputed', label: '有冲突 (disputed)' },
      { value: 'stale', label: '已过期 (stale)' }
    ]
  },
  { key: 'lastConfirmed', label: '最近确认时间 (RFC 3339)', hint: '例如 2026-11-20T10:00:00Z；留空表示未知' },
  { key: 'evidenceReference', label: '安全证据引用', hint: '安全路径或指针，严禁填密码或私钥' }
]

const detailConfig: Partial<Record<NodeType, { key: string | null; fields: FieldSpec[] }>> = {
  RunningPeriod: {
    key: null,
    fields: [
      { key: 'startedAt', label: '开始时间', hint: 'RFC 3339，例如 2026-11-15T09:00:00Z' },
      { key: 'endedAt', label: '结束时间', hint: '未知或仍在运行可留空' },
      { key: 'deployedVersion', label: '部署版本' },
      { key: 'verificationAt', label: '验证时间', hint: 'RFC 3339' },
      { key: 'verificationResult', label: '验证结果' },
      { key: 'evidenceRef', label: '安全证据引用' }
    ]
  },
  Hardware: {
    key: 'hardware',
    fields: [
      { key: 'category', label: '设备类别' },
      { key: 'model', label: '型号' },
      { key: 'location', label: '位置' },
      { key: 'status', label: '状态' },
      { key: 'specifications', label: '规格（JSON 对象）', kind: 'json', shape: 'object' },
      { key: 'accessReference', label: '访问说明引用' },
      { key: 'maintenanceReference', label: '维护说明引用' }
    ]
  },
  Machine: {
    key: 'machine',
    fields: [
      { key: 'machineKind', label: '机器类别', hint: 'physical / virtual / shared / rig' },
      { key: 'hostname', label: '主机名' },
      { key: 'os', label: '操作系统' },
      { key: 'ips', label: 'IP 地址（每行一个）', kind: 'list' },
      { key: 'network', label: '所属网络' },
      { key: 'status', label: '状态' },
      { key: 'accessMethod', label: '访问方式（如 ssh user@ip）' },
      { key: 'accessPrerequisites', label: '访问前提（每行一个）', kind: 'list' },
      { key: 'allowedOperations', label: '允许操作（每行一个）', kind: 'list' },
      { key: 'operationConstraints', label: '操作约束（每行一个）', kind: 'list' },
      { key: 'forbiddenOperations', label: '禁止操作（每行一个）', kind: 'list' },
      { key: 'responsibilityBoundary', label: '责任边界说明' },
      { key: 'resources', label: '资源声明（JSON 数组）', kind: 'json', shape: 'array' },
      { key: 'lastVerified', label: '最近核验时间' }
    ]
  },
  SoftwareProject: {
    key: 'softwareProject',
    fields: [
      { key: 'purpose', label: '用途说明' },
      { key: 'languages', label: '开发语言（每行一个）', kind: 'list' },
      { key: 'form', label: '项目形态（service / library / tool）' },
      { key: 'remoteRepository', label: '远程代码仓引用' },
      { key: 'upstream', label: '上游引用' },
      { key: 'maintenanceState', label: '维护状态（active / deprecated / maintenance）' },
      { key: 'recommendedVersion', label: '推荐版本' },
      { key: 'successorId', label: '继任项目 ID' }
    ]
  },
  SoftwareInstance: {
    key: 'softwareInstance',
    fields: [
      { key: 'productName', label: '所属产品名称' },
      { key: 'domains', label: '域名列表（每行一个）', kind: 'list' },
      { key: 'deploymentPath', label: '部署路径' },
      { key: 'deploymentMethod', label: '部署方式（docker / systemd / manual）' },
      { key: 'runtimeIdentity', label: '运行身份（不含凭据）' },
      { key: 'endpoints', label: '服务端口与端点（JSON 数组）', kind: 'json', shape: 'array' },
      { key: 'configurationReferences', label: '配置引用（每行一个）', kind: 'list' },
      { key: 'deploymentState', label: '部署状态' },
      { key: 'operationalState', label: '运行状态' },
      { key: 'maintenanceState', label: '维护状态' },
      { key: 'verificationResult', label: '验证结果' },
      { key: 'verifiedAt', label: '验证时间' }
    ]
  },
  AccountCredential: {
    key: 'accountCredential',
    fields: [
      { key: 'account', label: '账号标识（非秘密）' },
      { key: 'credentialType', label: '凭据类型（如 ssh_key / token / password）' },
      { key: 'purpose', label: '用途' },
      { key: 'scope', label: '权限范围' },
      { key: 'accessReference', label: '安全访问引用（如 vaultwarden:path）' },
      { key: 'rotationAt', label: '上次轮换时间' },
      { key: 'status', label: '状态' },
      { key: 'lastVerified', label: '最近核验时间' }
    ]
  },
  Product: {
    key: 'product',
    fields: [
      { key: 'purpose', label: '产品用途' },
      { key: 'commonConfigurationReference', label: '共性配置引用' },
      { key: 'runbookReference', label: 'Runbook 引用' }
    ]
  },
  Document: {
    key: 'document',
    fields: [
      { key: 'role', label: '文档角色' },
      { key: 'authoritativeReference', label: '权威文档引用（URL 或 file://）' },
      { key: 'effective', label: '当前有效', kind: 'boolean' },
      { key: 'lastVerified', label: '最近核验时间' },
      { key: 'supersededBy', label: '替代文档 ID' }
    ]
  },
  Runbook: {
    key: 'runbook',
    fields: [
      { key: 'trigger', label: '触发条件' },
      { key: 'preconditions', label: '前置条件（每行一个）', kind: 'list' },
      { key: 'stepsReference', label: '步骤引用（文档地址，不填正文）' },
      { key: 'stopConditionReference', label: '停止条件引用' },
      { key: 'rollbackReference', label: '回退引用' },
      { key: 'verificationMethod', label: '验证方法引用' },
      { key: 'lastVerified', label: '最近核验时间' },
      { key: 'executableStatus', label: '可执行状态' },
      { key: 'approvalRequired', label: '需要审批', kind: 'boolean' }
    ]
  },
  WorkItem: {
    key: 'workItem',
    fields: [
      { key: 'kind', label: '事项类型' },
      { key: 'status', label: '状态' },
      { key: 'rationaleReference', label: '决策理由引用' },
      { key: 'decisionEvidenceReference', label: '决策证据引用' }
    ]
  },
  Network: {
    key: 'network',
    fields: [
      { key: 'kind', label: '网络类型' },
      { key: 'cidrs', label: 'CIDR 列表（每行一个）', kind: 'list' },
      { key: 'gatewayReference', label: '网关引用' },
      { key: 'accessLocations', label: '访问位置（每行一个）', kind: 'list' },
      { key: 'vpnReference', label: 'VPN 引用' },
      { key: 'status', label: '状态' }
    ]
  }
}

function inputValue(value: unknown, kind: FieldKind): string {
  if (kind === 'boolean') return value === true ? 'true' : 'false'
  if (kind === 'list') return Array.isArray(value) ? value.map(item => String(item)).join('\n') : ''
  if (kind === 'json') return value === undefined ? '' : JSON.stringify(value, null, 2)
  return typeof value === 'string' ? value : ''
}

export function DetailsEditor({
  node,
  onClose,
  onSave
}: {
  node: Node
  onClose: () => void
  onSave: (updated: Node) => Promise<void>
}) {
  const { message } = AntdApp.useApp()
  const config = detailConfig[node.type]
  const original = useMemo(
    () => (config?.key ? ((node as unknown as Record<string, Record<string, unknown>>)[config.key] ?? {}) : {}),
    [node, config]
  )

  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries((config?.fields ?? []).map(field => [field.key, inputValue(original[field.key], field.kind ?? 'text')]))
  )
  const [basicValues, setBasicValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(
      basicFields.map(field => [field.key, inputValue((node as unknown as Record<string, unknown>)[field.key], field.kind ?? 'text')])
    )
  )
  const [busy, setBusy] = useState(false)

  if (!config) return null

  const handleSave = async () => {
    try {
      const updatedDetails: Record<string, unknown> = { ...original }
      for (const field of config.fields) {
        const raw = values[field.key] ?? ''
        if (field.kind === 'boolean') {
          updatedDetails[field.key] = raw === 'true'
        } else if (field.kind === 'list') {
          const items = raw.split(/\r?\n/).map(item => item.trim()).filter(Boolean)
          if (items.length) updatedDetails[field.key] = items
          else delete updatedDetails[field.key]
        } else if (field.kind === 'json') {
          if (!raw.trim()) {
            delete updatedDetails[field.key]
            continue
          }
          const parsed: unknown = JSON.parse(raw)
          if (parsed === null || typeof parsed !== 'object') {
            throw new Error(`${field.label} 必须是有效 JSON 对象或数组`)
          }
          if (field.shape === 'array' && !Array.isArray(parsed)) {
            throw new Error(`${field.label} 必须是 JSON 数组`)
          }
          if (field.shape === 'object' && Array.isArray(parsed)) {
            throw new Error(`${field.label} 必须是 JSON 对象`)
          }
          updatedDetails[field.key] = parsed
        } else if (raw.trim()) {
          updatedDetails[field.key] = raw.trim()
        } else {
          delete updatedDetails[field.key]
        }
      }

      const updatedBase = {
        ...node,
        lifecycle: basicValues.lifecycle?.trim() || undefined,
        evidenceKind: (basicValues.evidenceKind || 'unknown') as Node['evidenceKind'],
        certainty: (basicValues.certainty || 'unknown') as Node['certainty'],
        lastConfirmed: basicValues.lastConfirmed?.trim() || undefined,
        evidenceReference: basicValues.evidenceReference?.trim() || undefined
      }

      const updated = config.key
        ? ({ ...updatedBase, [config.key]: updatedDetails } as Node)
        : ({ ...updatedBase, ...updatedDetails } as Node)

      setBusy(true)
      await onSave(updated)
      onClose()
    } catch (cause) {
      message.error(cause instanceof Error ? cause.message : '字段校验未通过')
    } finally {
      setBusy(false)
    }
  }

  const renderField = (field: FieldSpec, isBasic: boolean) => {
    const val = isBasic ? basicValues[field.key] ?? '' : values[field.key] ?? ''
    const onChangeVal = (newVal: string) => {
      if (isBasic) {
        setBasicValues(prev => ({ ...prev, [field.key]: newVal }))
      } else {
        setValues(prev => ({ ...prev, [field.key]: newVal }))
      }
    }

    if (field.kind === 'select') {
      return (
        <Form.Item label={field.label} key={field.key} style={{ marginBottom: 14 }}>
          <Select
            value={val || undefined}
            onChange={onChangeVal}
            options={field.options}
            placeholder={field.hint}
          />
        </Form.Item>
      )
    }

    if (field.kind === 'boolean') {
      return (
        <Form.Item label={field.label} key={field.key} style={{ marginBottom: 14 }}>
          <Select
            value={val || 'false'}
            onChange={onChangeVal}
            options={[
              { value: 'false', label: '否 (false)' },
              { value: 'true', label: '是 (true)' }
            ]}
          />
        </Form.Item>
      )
    }

    if (field.kind === 'list' || field.kind === 'json') {
      return (
        <Form.Item
          label={field.label}
          key={field.key}
          extra={field.hint || (field.kind === 'list' ? '每行一个值' : '有效 JSON')}
          style={{ marginBottom: 14 }}
        >
          <Input.TextArea
            rows={field.kind === 'json' ? 4 : 3}
            value={val}
            onChange={e => onChangeVal(e.target.value)}
            placeholder={field.kind === 'json' ? '{}' : '一行一项…'}
            style={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace', fontSize: 12 }}
          />
        </Form.Item>
      )
    }

    return (
      <Form.Item label={field.label} key={field.key} extra={field.hint} style={{ marginBottom: 14 }}>
        <Input
          value={val}
          onChange={e => onChangeVal(e.target.value)}
          placeholder={field.hint}
        />
      </Form.Item>
    )
  }

  const tabItems = [
    {
      key: 'type-specific',
      label: `${node.type} 专属属性`,
      children: (
        <Form layout="vertical">
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '0 16px' }}>
            {config.fields.map(field => renderField(field, false))}
          </div>
        </Form>
      )
    },
    {
      key: 'basic',
      label: '通用认知与证据',
      children: (
        <Form layout="vertical">
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '0 16px' }}>
            {basicFields.map(field => renderField(field, true))}
          </div>
        </Form>
      )
    }
  ]

  return (
    <Modal
      open={true}
      onCancel={onClose}
      onOk={handleSave}
      confirmLoading={busy}
      okText="预览并保存"
      cancelText="取消"
      width={720}
      title={
        <Space>
          <Settings size={18} style={{ color: '#42633c' }} />
          <span>编辑「{node.name}」的 {node.type} 详情</span>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%', marginTop: 8 }} size="middle">
        <Alert
          type="info"
          showIcon
          icon={<ShieldCheck size={16} />}
          message={
            <span>
              来源标识锁定：<Text code>{node.source || 'unspecified'}</Text>（不可在人工界面更改，防止溯源断链）
            </span>
          }
          description="零秘密红线提醒：严禁在此处输入密码、Token、私钥或低熵敏感正文；仅记录安全引用与架构元数据。"
        />

        <Tabs defaultActiveKey="type-specific" items={tabItems} />
      </Space>
    </Modal>
  )
}
