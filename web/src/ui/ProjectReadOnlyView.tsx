import { useEffect, useMemo, useState } from 'react'
import {
  api,
  type Node,
  type ProjectWorkspaceData,
  type ProjectViewTemplate
} from './api'
import {
  Button,
  Input,
  Select,
  Space,
  Table,
  Modal,
  Form,
  Switch,
  Radio,
  App as AntdApp,
  Spin,
  Alert,
  Popconfirm,
  Tabs,
  Empty,
  Segmented
} from 'antd'
import {
  ArrowLeft,
  Search,
  Settings2,
  Trash2
} from 'lucide-react'
import './readonly-view.css'

interface ProjectReadOnlyViewProps {
  projectId: string
  projects?: Node[]
  onBackToGraph?: () => void
  onSwitchToWorkspace?: () => void
}

interface EnvGroup {
  key: string
  name: string
  shortName: string
  badgeName: string
  color: string
  instances: Node[]
  machines: Node[]
  credentials: Node[]
}

interface ServiceGroupItem {
  key: string
  name: string
  entityId: string
  type: 'instance' | 'machine' | 'standalone'
  host: string
  port?: string
  envTag?: { shortName: string; color: string } | null
  credentials: Node[]
}

function classifyEnvironments(
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
      color: '#15803d',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'test',
      name: '测试环境 (Testing / 一体化)',
      shortName: '测试环境',
      badgeName: 'TEST',
      color: '#0369a1',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'c0130',
      name: '雄安现场专区 (c0130 专区)',
      shortName: '现场专区 (C0130)',
      badgeName: 'C0130',
      color: '#c2410c',
      instances: [],
      machines: [],
      credentials: []
    },
    {
      key: 'infra',
      name: '公共基础设施与网关 (Infra)',
      shortName: '公共基础 (Infra)',
      badgeName: 'INFRA',
      color: '#6b21a8',
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

export function ProjectReadOnlyView({
  projectId,
  projects: _projects = [],
  onBackToGraph,
  onSwitchToWorkspace
}: ProjectReadOnlyViewProps) {
  const { message } = AntdApp.useApp()

  const [loading, setLoading] = useState(true)
  const [data, setData] = useState<ProjectWorkspaceData | null>(null)
  const [error, setError] = useState<string | null>(null)

  // 模板状态
  const [templates, setTemplates] = useState<ProjectViewTemplate[]>([])
  const [currentTemplateId, setCurrentTemplateId] = useState<string>('preset-secret-cards')
  const [templateModalOpen, setTemplateModalOpen] = useState(false)
  const [isEditingTemplate, setIsEditingTemplate] = useState(false)
  const [templateForm] = Form.useForm()

  // 复制与密码状态
  const [decryptedPasswords, setDecryptedPasswords] = useState<Record<string, string>>({})
  const [revealedPasswords, setRevealedPasswords] = useState<Record<string, boolean>>({})
  const [copyingState, setCopyingState] = useState<Record<string, boolean>>({})
  const [searchQuery, setSearchQuery] = useState('')

  // 环境 Tab
  const [activeEnvTab, setActiveEnvTab] = useState<string>('prod')
  const [viewMode, setViewMode] = useState<'aggregate' | 'flat'>('aggregate')

  // 加载项目数据与模板
  const loadData = async () => {
    setLoading(true)
    setError(null)
    try {
      const [workspaceData, templateList] = await Promise.all([
        api.projectWorkspace(projectId),
        api.getProjectTemplates(projectId).catch(() => [])
      ])
      setData(workspaceData)
      setTemplates(templateList)

      // 解析 URL 中的初始模板参数
      const urlParams = new URLSearchParams(window.location.search)
      const requestedTmpl = urlParams.get('template')

      if (requestedTmpl && templateList.some(t => t.id === requestedTmpl)) {
        setCurrentTemplateId(requestedTmpl)
      } else {
        const defaultTmpl = templateList.find(t => t.isDefault)
        if (defaultTmpl) {
          setCurrentTemplateId(defaultTmpl.id)
        } else if (templateList.length > 0) {
          setCurrentTemplateId(templateList[0].id)
        }
      }

      // 默认优先激活生产环境 Tab
      const envs = classifyEnvironments(workspaceData.instances || [], workspaceData.machines || [], workspaceData.credentials || [])
      if (envs.some(e => e.key === 'prod')) {
        setActiveEnvTab('prod')
      } else if (envs.length > 0) {
        setActiveEnvTab(envs[0].key)
      } else {
        setActiveEnvTab('all')
      }
    } catch (err) {
      console.error('Failed to load project read-only data:', err)
      setError(err instanceof Error ? err.message : '加载项目数据失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (projectId) {
      loadData()
    }
  }, [projectId])

  // 当前激活模板
  const currentTemplate = useMemo(() => {
    return templates.find(t => t.id === currentTemplateId) || {
      id: 'preset-secret-cards',
      projectId: '*',
      name: '密码速查流 (Quick Secret Cards)',
      layoutType: 'cards',
      config: { groupBy: 'environment', density: 'compact', highlightSecret: true },
      builtin: true
    } as ProjectViewTemplate
  }, [templates, currentTemplateId])

  // 环境归类
  const envGroups = useMemo(() => {
    if (!data) return []
    const envs = classifyEnvironments(data.instances || [], data.machines || [], data.credentials || [])
    const filterEnv = currentTemplate.config?.filterEnv
    if (filterEnv && filterEnv !== 'all') {
      return envs.filter(e => e.key === filterEnv)
    }
    return envs
  }, [data, currentTemplate])

  // 查找凭据所属的环境
  const getCredentialEnv = (credId: string) => {
    for (const env of envGroups) {
      if (env.credentials.some(c => c.id === credId)) {
        return env
      }
    }
    return null
  }


  // 根据凭据查找关联的主机或实例
  const findRelatedHostAndPort = (credId: string) => {
    if (!data) return { host: '', port: '' }
    for (const r of data.relations) {
      if (r.to === credId || r.from === credId) {
        const otherId = r.to === credId ? r.from : r.to
        const inst = data.instances.find(i => i.id === otherId)
        if (inst) {
          const ports = inst.softwareInstance?.ports || []
          const domains = inst.softwareInstance?.domains || []
          return {
            host: domains[0] || inst.name,
            port: ports[0] ? String(ports[0]) : ''
          }
        }
        const mach = data.machines.find(m => m.id === otherId)
        if (mach) {
          const ips = mach.machine?.ips || []
          return {
            host: mach.machine?.hostname || ips[0] || mach.name,
            port: ''
          }
        }
      }
    }
    return { host: '', port: '' }
  }

  // 查找凭据所属的宿主实体（服务实例或机器）
  const credToEntityMap = useMemo(() => {
    if (!data) return new Map<string, Node>()
    const map = new Map<string, Node>()
    for (const r of data.relations) {
      if (r.predicate === 'hasCredential') {
        const entity =
          data.instances.find(i => i.id === r.from) ||
          data.machines.find(m => m.id === r.from)
        if (entity) map.set(r.to, entity)
      }
    }
    return map
  }, [data])

  // 当前 Tab 下过滤后的凭据
  const currentTabCredentials = useMemo(() => {
    if (!data) return []
    let list: Node[] = []
    if (activeEnvTab === 'all') {
      list = data.credentials || []
    } else {
      const matched = envGroups.find(e => e.key === activeEnvTab)
      list = matched ? matched.credentials : []
    }
    if (!searchQuery) return list
    const q = searchQuery.toLowerCase()
    return list.filter(c => {
      const rel = findRelatedHostAndPort(c.id)
      const parentEntity = credToEntityMap.get(c.id)
      return (
        c.name.toLowerCase().includes(q) ||
        c.id.toLowerCase().includes(q) ||
        (c.accountCredential?.account || '').toLowerCase().includes(q) ||
        (rel.host || '').toLowerCase().includes(q) ||
        (parentEntity?.name || '').toLowerCase().includes(q)
      )
    })
  }, [data, activeEnvTab, envGroups, searchQuery, credToEntityMap])

  // 按服务/宿主实体聚合当前 Tab 下的凭据
  const aggregatedServiceGroups = useMemo<ServiceGroupItem[]>(() => {
    if (!data) return []
    const groupsMap = new Map<string, ServiceGroupItem>()

    for (const c of currentTabCredentials) {
      const parentEntity = credToEntityMap.get(c.id)
      let groupKey = parentEntity ? parentEntity.id : ''
      let groupName = parentEntity ? parentEntity.name : ''
      let groupType: 'instance' | 'machine' | 'standalone' = 'standalone'
      let host = ''
      let port = ''

      if (parentEntity) {
        if (parentEntity.id.startsWith('services/')) {
          groupType = 'instance'
          host = parentEntity.softwareInstance?.domains?.[0] || parentEntity.name
          const ports = parentEntity.softwareInstance?.ports || []
          if (ports && ports[0]) port = String(ports[0])
        } else if (parentEntity.id.startsWith('machine/')) {
          groupType = 'machine'
          host = parentEntity.machine?.hostname || parentEntity.machine?.ips?.[0] || parentEntity.name
        }
      }

      if (!groupKey) {
        groupKey = c.id
        groupName = c.name
        const rel = findRelatedHostAndPort(c.id)
        host = rel.host || ''
        port = rel.port || ''
      }

      const envTag = getCredentialEnv(c.id)

      if (!groupsMap.has(groupKey)) {
        groupsMap.set(groupKey, {
          key: groupKey,
          name: groupName,
          entityId: groupKey,
          type: groupType,
          host,
          port,
          envTag,
          credentials: []
        })
      }

      groupsMap.get(groupKey)!.credentials.push(c)
    }

    return Array.from(groupsMap.values())
  }, [data, currentTabCredentials, credToEntityMap])

  // 安全取密并复制到剪贴板
  const handleCopyPassword = async (node: Node) => {
    setCopyingState(prev => ({ ...prev, [node.id]: true }))
    try {
      let pw = decryptedPasswords[node.id]
      if (!pw) {
        const ref = node.accountCredential?.accessReference || node.id
        const res = await api.credentialSecret(ref)
        if (res.password) {
          pw = res.password
          setDecryptedPasswords(prev => ({ ...prev, [node.id]: pw }))
        } else {
          const res2 = await api.credentialSecret(node.id)
          if (res2.password) {
            pw = res2.password
            setDecryptedPasswords(prev => ({ ...prev, [node.id]: pw }))
          }
        }
      }

      if (!pw) {
        throw new Error('未检索到有效口令明文')
      }

      let success = false
      if (navigator.clipboard && navigator.clipboard.writeText) {
        try {
          await navigator.clipboard.writeText(pw)
          success = true
        } catch {
          success = false
        }
      }
      if (!success) {
        const textArea = document.createElement('textarea')
        textArea.value = pw
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.select()
        success = Boolean(document.execCommand('copy'))
        textArea.remove()
      }

      if (success) {
        message.success(`已复制「${node.name}」密码`)
      } else {
        message.info(`密码明文：${pw}`, 8)
      }
    } catch (err) {
      message.error(err instanceof Error ? err.message : '获取密码失败')
    } finally {
      setCopyingState(prev => ({ ...prev, [node.id]: false }))
    }
  }

  // 复制任意纯文本（例如账号）
  const handleCopyText = (text: string, label: string) => {
    if (!text) return
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text)
    }
    message.success(`已复制${label}: ${text}`)
  }

  // 切换明文/掩码显示
  const togglePasswordReveal = async (node: Node) => {
    if (revealedPasswords[node.id]) {
      setRevealedPasswords(prev => ({ ...prev, [node.id]: false }))
      return
    }
    try {
      if (!decryptedPasswords[node.id]) {
        const ref = node.accountCredential?.accessReference || node.id
        const res = await api.credentialSecret(ref)
        if (res.password) {
          setDecryptedPasswords(prev => ({ ...prev, [node.id]: res.password! }))
        }
      }
      setRevealedPasswords(prev => ({ ...prev, [node.id]: true }))
    } catch (err) {
      message.error(err instanceof Error ? err.message : '获取密码失败')
    }
  }

  // 模板选择切换
  const handleSelectTemplate = (tmplId: string) => {
    setCurrentTemplateId(tmplId)
    const url = new URL(window.location.href)
    url.searchParams.set('template', tmplId)
    window.history.replaceState({}, '', url.toString())
  }

  // 保存/新建自定义模板
  const handleSaveTemplate = async () => {
    try {
      const values = await templateForm.validateFields()
      const payload: Partial<ProjectViewTemplate> = {
        name: values.name,
        description: values.description || '',
        layoutType: values.layoutType,
        isDefault: values.isDefault || false,
        config: {
          groupBy: values.groupBy || 'environment',
          density: 'compact',
          filterEnv: values.filterEnv || 'all'
        }
      }

      if (isEditingTemplate && currentTemplate && !currentTemplate.builtin) {
        const updated = await api.updateProjectTemplate(projectId, currentTemplate.id, payload)
        message.success('模板更新成功')
        setTemplates(prev => prev.map(t => t.id === updated.id ? updated : t))
      } else {
        const created = await api.saveProjectTemplate(projectId, payload)
        message.success('模板已持久化保存至数据库')
        setTemplates(prev => [...prev, created])
        setCurrentTemplateId(created.id)
      }
      setTemplateModalOpen(false)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '保存模板失败')
    }
  }

  // 删除自定义模板
  const handleDeleteTemplate = async (tmplId: string) => {
    try {
      await api.deleteProjectTemplate(projectId, tmplId)
      message.success('自定义模板已删除')
      setTemplates(prev => prev.filter(t => t.id !== tmplId))
      setCurrentTemplateId('preset-secret-cards')
    } catch (err) {
      message.error(err instanceof Error ? err.message : '删除模板失败')
    }
  }

  // 打开创建新模板弹窗
  const openCreateModal = () => {
    setIsEditingTemplate(false)
    templateForm.resetFields()
    templateForm.setFieldsValue({
      name: `${data?.project.name || '项目'} 定制风格`,
      description: '朴素高密速查模板',
      layoutType: currentTemplate.layoutType || 'cards',
      filterEnv: 'all',
      isDefault: false
    })
    setTemplateModalOpen(true)
  }

  // 打开编辑当前模板弹窗
  const openEditModal = () => {
    if (!currentTemplate || currentTemplate.builtin) return
    setIsEditingTemplate(true)
    templateForm.resetFields()
    templateForm.setFieldsValue({
      name: currentTemplate.name,
      description: currentTemplate.description,
      layoutType: currentTemplate.layoutType,
      filterEnv: currentTemplate.config?.filterEnv || 'all',
      isDefault: currentTemplate.isDefault || false
    })
    setTemplateModalOpen(true)
  }

  // 发布自包含静态页面至 Nginx 托管目录
  const [deployingStatic, setDeployingStatic] = useState(false)
  const [lastStaticUrl, setLastStaticUrl] = useState<string>('')

  const handleDeployStaticPage = async () => {
    setDeployingStatic(true)
    try {
      const res = await api.exportStaticPage(projectId)
      setLastStaticUrl(res.url)
      message.success(`已生成并发布静态页至 Nginx 目录 (共包含 ${res.credentialCount} 项凭据)`)
    } catch (err) {
      message.error(err instanceof Error ? err.message : '生成持久静态页失败')
    } finally {
      setDeployingStatic(false)
    }
  }


  if (loading) {
    return (
      <div className="readonly-page-container" style={{ display: 'grid', placeItems: 'center', height: '60vh' }}>
        <Space size={8}>
          <Spin size="small" />
          <span style={{ color: '#6b7280', fontSize: 13 }}>加载中…</span>
        </Space>
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="readonly-page-container" style={{ padding: 30, maxWidth: 600, margin: '0 auto' }}>
        <Alert
          type="error"
          message="加载失败"
          description={error || '未检索到该项目的有效信息'}
          showIcon
          action={<Button size="small" onClick={loadData}>重试</Button>}
        />
      </div>
    )
  }

  const { project, instances, machines, credentials } = data

  return (
    <div className="readonly-page-container">
      {/* 顶部极简朴素 Header */}
      <header className="readonly-header">
        <div className="readonly-header-inner">
          <div className="readonly-title-area">
            {onBackToGraph && (
              <button
                type="button"
                className="plain-action-link secondary"
                onClick={onBackToGraph}
                style={{ fontSize: 13 }}
              >
                <ArrowLeft size={13} /> 图谱
              </button>
            )}
            <div className="readonly-title-text">
              <h1>{project.name}</h1>
              <span className="sub-title">/ 密码速查</span>
              <span style={{ fontSize: 12, color: '#9ca3af', marginLeft: 4 }}>
                ({credentials.length} 凭据 · {instances.length} 实例 · {machines.length} 机器)
              </span>
            </div>
          </div>

          <div className="readonly-toolbar">
            <Space size={8}>
              <span style={{ fontSize: 12, color: '#6b7280' }}>风格:</span>
              <Select
                size="small"
                value={currentTemplateId}
                onChange={handleSelectTemplate}
                style={{ width: 170 }}
                options={[
                  {
                    label: '预置样式',
                    options: templates.filter(t => t.builtin).map(t => ({
                      label: t.name.replace(/ \(.*?\)/, ''),
                      value: t.id
                    }))
                  },
                  {
                    label: '自定义模板',
                    options: templates.filter(t => !t.builtin).map(t => ({
                      label: t.name,
                      value: t.id
                    }))
                  }
                ]}
              />

              <button
                type="button"
                className="plain-action-link secondary"
                onClick={openCreateModal}
              >
                另存风格
              </button>

              {!currentTemplate.builtin && (
                <>
                  <button
                    type="button"
                    className="plain-action-link secondary"
                    onClick={openEditModal}
                    title="配置此模板"
                  >
                    <Settings2 size={13} />
                  </button>
                  <Popconfirm
                    title="删除自定义模板"
                    description="确定删除此模板吗？"
                    onConfirm={() => handleDeleteTemplate(currentTemplate.id)}
                    okText="删除"
                    cancelText="取消"
                  >
                    <button
                      type="button"
                      className="plain-action-link secondary"
                      style={{ color: '#dc2626' }}
                      title="删除此模板"
                    >
                      <Trash2 size={13} />
                    </button>
                  </Popconfirm>
                </>
              )}

              <button
                type="button"
                className="plain-action-link"
                disabled={deployingStatic}
                onClick={handleDeployStaticPage}
                title="将当前项目完整自包含静态页渲染并发布至 Nginx 静态托管目录"
              >
                {deployingStatic ? '发布中…' : '发布持久静态页'}
              </button>
              {lastStaticUrl && (
                <a
                  href={lastStaticUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="plain-action-link secondary"
                  title="在新窗口查看 Nginx 持久托管的静态页"
                >
                  打开静态页 ↗
                </a>
              )}

              {onSwitchToWorkspace && (
                <button
                  type="button"
                  className="plain-action-link"
                  onClick={onSwitchToWorkspace}
                >
                  管理工作区 ↗
                </button>
              )}
            </Space>
          </div>
        </div>
      </header>

      {/* 主体呈现区 */}
      <main className="readonly-content-wrapper">
        {/* 渲染方式 1：密码速查流 (cards/default) - 朴素极简 Tab + 高密度清爽数据表 */}
        {currentTemplate.layoutType === 'cards' && (
          <div className="plain-tabs-box">
            <Tabs
              activeKey={activeEnvTab}
              onChange={setActiveEnvTab}
              type="line"
              tabBarExtraContent={
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Segmented
                    size="small"
                    value={viewMode}
                    onChange={v => setViewMode(v as 'aggregate' | 'flat')}
                    options={[
                      { label: '按服务聚合', value: 'aggregate' },
                      { label: '平铺清单', value: 'flat' }
                    ]}
                  />
                  <Input
                    className="plain-search-input"
                    prefix={<Search size={12} color="#9ca3af" />}
                    placeholder="过滤服务、账号或IP…"
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    style={{ width: 160 }}
                    size="small"
                    allowClear
                  />
                  <span style={{ fontSize: 11.5, color: '#6b7280' }}>
                    {viewMode === 'aggregate' ? (
                      <><strong>{aggregatedServiceGroups.length}</strong> 服务 · <strong>{currentTabCredentials.length}</strong> 账号</>
                    ) : (
                      <>共 <strong>{currentTabCredentials.length}</strong> 条</>
                    )}
                  </span>
                </div>
              }
              items={[
                ...envGroups.map(env => ({
                  key: env.key,
                  label: (
                    <span>
                      {env.shortName} <span style={{ color: '#9ca3af', fontSize: 11 }}>({env.credentials.length})</span>
                    </span>
                  )
                })),
                {
                  key: 'all',
                  label: (
                    <span>
                      全部环境 <span style={{ color: '#9ca3af', fontSize: 11 }}>({credentials.length})</span>
                    </span>
                  )
                }
              ]}
            />

            {currentTabCredentials.length === 0 ? (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={searchQuery ? '未检索到匹配凭据' : '当前环境暂无凭据'}
                style={{ padding: '30px 0' }}
              />
            ) : viewMode === 'aggregate' ? (
              <div className="plain-service-grid">
                {aggregatedServiceGroups.map(group => (
                  <div key={group.key} className="plain-service-card">
                    <div className="plain-service-card-header">
                      <div className="plain-service-card-title">
                        <span className="service-name">{group.name}</span>
                        {group.entityId && group.entityId !== group.name && (
                          <span className="service-id" title={group.entityId}>
                            ({group.entityId.replace(/^services\/[^\/]+\/|^machine\/[^\/]+\/|^credentials\/[^\/]+\//, '')})
                          </span>
                        )}
                        {activeEnvTab === 'all' && group.envTag && (
                          <span className="service-env-badge">{group.envTag.shortName}</span>
                        )}
                      </div>
                      <div className="plain-service-card-meta">
                        {group.host && (
                          <span
                            className="service-host"
                            title="点击复制访问地址"
                            onClick={() => handleCopyText(`${group.host}${group.port ? `:${group.port}` : ''}`, '访问地址')}
                          >
                            {group.host}{group.port ? `:${group.port}` : ''}
                          </span>
                        )}
                        <span className="service-count">{group.credentials.length} 账号</span>
                      </div>
                    </div>

                    <div className="plain-service-card-body">
                      <table className="plain-micro-table">
                        <tbody>
                          {group.credentials.map(c => {
                            const details = c.accountCredential || {}
                            const pw = decryptedPasswords[c.id]
                            const isRevealed = revealedPasswords[c.id]
                            const isCopying = copyingState[c.id]

                            // 提取角色/说明：若凭据名称以服务主名开头，去掉冗余前缀
                            let roleName = c.name
                            const baseName = group.name.split(' ')[0]
                            if (baseName && roleName.startsWith(baseName)) {
                              const cleaned = roleName.slice(baseName.length).trim()
                              if (cleaned) roleName = cleaned
                            }

                            return (
                              <tr key={c.id}>
                                <td className="role-col" title={c.name}>
                                  <span className="role-label">{roleName}</span>
                                </td>
                                <td className="account-col">
                                  <span
                                    className="plain-code"
                                    style={{ cursor: 'pointer' }}
                                    title="点击复制账号"
                                    onClick={() => handleCopyText(details.account || '', '账号')}
                                  >
                                    {details.account || '默认'}
                                  </span>
                                </td>
                                <td className="secret-col">
                                  {isRevealed && pw ? (
                                    <span className="plain-revealed">{pw}</span>
                                  ) : (
                                    <span
                                      className="plain-masked"
                                      style={{ cursor: 'pointer' }}
                                      title="点击复制密码"
                                      onClick={() => handleCopyPassword(c)}
                                    >
                                      ••••••••
                                    </span>
                                  )}
                                </td>
                                <td className="action-col">
                                  <Space size={3}>
                                    <button
                                      type="button"
                                      className="plain-action-link"
                                      disabled={isCopying}
                                      onClick={() => handleCopyPassword(c)}
                                    >
                                      {isCopying ? '…' : '复制'}
                                    </button>
                                    <button
                                      type="button"
                                      className="plain-action-link secondary"
                                      onClick={() => togglePasswordReveal(c)}
                                    >
                                      {isRevealed ? '隐藏' : '查看'}
                                    </button>
                                  </Space>
                                </td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="plain-table-wrapper">
                <table className="plain-table">
                  <thead>
                    <tr>
                      <th style={{ width: '32%' }}>服务 / 凭据名称</th>
                      <th style={{ width: '18%' }}>登录账号</th>
                      <th style={{ width: '28%' }}>主机 / 端口</th>
                      <th style={{ width: '12%' }}>口令</th>
                      <th style={{ width: '10%', textAlign: 'right' }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {currentTabCredentials.map(c => {
                      const details = c.accountCredential || {}
                      const related = findRelatedHostAndPort(c.id)
                      const envTag = getCredentialEnv(c.id)
                      const pw = decryptedPasswords[c.id]
                      const isRevealed = revealedPasswords[c.id]
                      const isCopying = copyingState[c.id]

                      return (
                        <tr key={c.id}>
                          <td>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', overflow: 'hidden' }}>
                              <span style={{ fontWeight: 600, color: '#111827', fontSize: 12 }}>{c.name}</span>
                              <span style={{ fontSize: 11, color: '#9ca3af', fontFamily: 'monospace' }} title={c.id}>
                                ({c.id.replace('credentials/', '')})
                              </span>
                              {activeEnvTab === 'all' && envTag && (
                                <span style={{ fontSize: 10, color: '#6b7280', background: '#f3f4f6', padding: '0 3px', borderRadius: 2 }}>
                                  {envTag.shortName}
                                </span>
                              )}
                            </div>
                          </td>
                          <td>
                            <span
                              className="plain-code"
                              style={{ cursor: 'pointer' }}
                              title="点击复制账号"
                              onClick={() => handleCopyText(details.account || '', '账号')}
                            >
                              {details.account || '默认'}
                            </span>
                          </td>
                          <td>
                            {related.host ? (
                              <span style={{ fontFamily: 'monospace', fontSize: 11.5, color: '#4b5563', whiteSpace: 'nowrap' }}>
                                {related.host}{related.port ? `:${related.port}` : ''}
                              </span>
                            ) : (
                              <span style={{ color: '#d1d5db' }}>—</span>
                            )}
                          </td>
                          <td>
                            {isRevealed && pw ? (
                              <span className="plain-revealed">{pw}</span>
                            ) : (
                              <span
                                className="plain-masked"
                                style={{ cursor: 'pointer' }}
                                title="点击复制密码"
                                onClick={() => handleCopyPassword(c)}
                              >
                                ••••••••
                              </span>
                            )}
                          </td>
                          <td style={{ textAlign: 'right', whiteSpace: 'nowrap' }}>
                            <Space size={4}>
                              <button
                                type="button"
                                className="plain-action-link"
                                disabled={isCopying}
                                onClick={() => handleCopyPassword(c)}
                              >
                                {isCopying ? '…' : '复制'}
                              </button>
                              <button
                                type="button"
                                className="plain-action-link secondary"
                                onClick={() => togglePasswordReveal(c)}
                              >
                                {isRevealed ? '隐藏' : '查看'}
                              </button>
                            </Space>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {/* 渲染方式 2：多列环境看板 (board) - 朴素单色排布 */}
        {currentTemplate.layoutType === 'board' && (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 8 }}>
            {envGroups.map(env => (
              <div key={env.key} style={{ border: '1px solid #e5e7eb', borderRadius: 4, overflow: 'hidden' }}>
                <div style={{ padding: '4px 8px', background: '#f9fafb', borderBottom: '1px solid #e5e7eb', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <strong style={{ fontSize: 12, color: '#111827' }}>{env.shortName}</strong>
                  <span style={{ fontSize: 11, color: '#6b7280' }}>{env.credentials.length} 项</span>
                </div>
                <div style={{ padding: 6, display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {env.credentials.map(c => (
                    <div key={c.id} style={{ border: '1px solid #f3f4f6', borderRadius: 3, padding: '3px 6px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <div style={{ overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis', marginRight: 6 }}>
                        <div style={{ fontSize: 12, fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis' }}>{c.name}</div>
                        <div style={{ fontSize: 11, color: '#6b7280', fontFamily: 'monospace' }}>
                          账号: {c.accountCredential?.account || '默认'}
                        </div>
                      </div>
                      <button
                        type="button"
                        className="plain-action-link"
                        disabled={copyingState[c.id]}
                        onClick={() => handleCopyPassword(c)}
                      >
                        {copyingState[c.id] ? '…' : '复制'}
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* 渲染方式 3：紧凑资源矩阵 (table) - 朴素表格 */}
        {currentTemplate.layoutType === 'table' && (
          <div className="plain-table-wrapper">
            <Table
              dataSource={credentials.filter(c => {
                if (!searchQuery) return true
                const q = searchQuery.toLowerCase()
                return c.name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q)
              })}
              rowKey="id"
              pagination={false}
              size="small"
              columns={[
                {
                  title: '环境',
                  key: 'env',
                  width: 80,
                  render: (_, c) => {
                    const env = getCredentialEnv(c.id)
                    return <span style={{ fontSize: 11.5, color: '#6b7280' }}>{env ? env.shortName : '公共'}</span>
                  }
                },
                {
                  title: '服务 / 凭据名称',
                  key: 'name',
                  render: (_, c) => (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap' }}>
                      <span style={{ fontWeight: 600, fontSize: 12 }}>{c.name}</span>
                      <span style={{ fontSize: 11, color: '#9ca3af', fontFamily: 'monospace' }} title={c.id}>
                        ({c.id.replace('credentials/', '')})
                      </span>
                    </div>
                  )
                },
                {
                  title: '登录账号',
                  key: 'account',
                  width: 130,
                  render: (_, c) => (
                    <span
                      className="plain-code"
                      style={{ cursor: 'pointer' }}
                      title="点击复制账号"
                      onClick={() => handleCopyText(c.accountCredential?.account || '', '账号')}
                    >
                      {c.accountCredential?.account || '默认'}
                    </span>
                  )
                },
                {
                  title: '主机 / 端口',
                  key: 'host',
                  render: (_, c) => {
                    const rel = findRelatedHostAndPort(c.id)
                    return <span style={{ fontFamily: 'monospace', fontSize: 11.5, color: '#4b5563' }}>{rel.host ? `${rel.host}${rel.port ? `:${rel.port}` : ''}` : '—'}</span>
                  }
                },
                {
                  title: '口令',
                  key: 'password',
                  width: 100,
                  render: (_, c) => {
                    const isRevealed = revealedPasswords[c.id]
                    const pw = decryptedPasswords[c.id]
                    return isRevealed && pw ? (
                      <span className="plain-revealed">{pw}</span>
                    ) : (
                      <span className="plain-masked">••••••••</span>
                    )
                  }
                },
                {
                  title: '操作',
                  key: 'action',
                  width: 90,
                  align: 'right',
                  render: (_, c) => (
                    <Space size={4}>
                      <button
                        type="button"
                        className="plain-action-link"
                        disabled={copyingState[c.id]}
                        onClick={() => handleCopyPassword(c)}
                      >
                        {copyingState[c.id] ? '…' : '复制'}
                      </button>
                      <button
                        type="button"
                        className="plain-action-link secondary"
                        onClick={() => togglePasswordReveal(c)}
                      >
                        {revealedPasswords[c.id] ? '隐藏' : '查看'}
                      </button>
                    </Space>
                  )
                }
              ]}
            />
          </div>
        )}
      </main>

      {/* 自定义模板保存/编辑 Modal */}
      <Modal
        title={isEditingTemplate ? '配置模板' : '另存模板'}
        open={templateModalOpen}
        onOk={handleSaveTemplate}
        onCancel={() => setTemplateModalOpen(false)}
        okText="保存"
        cancelText="取消"
        width={440}
      >
        <Form form={templateForm} layout="vertical" style={{ marginTop: 12 }}>
          <Form.Item
            name="name"
            label="模板名称"
            rules={[{ required: true, message: '请输入模板名称' }]}
          >
            <Input placeholder="例如: 生产值班快速速查" size="small" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input size="small" placeholder="适用场景" />
          </Form.Item>

          <Form.Item name="layoutType" label="布局模式" rules={[{ required: true }]}>
            <Radio.Group size="small">
              <Radio value="cards">Tab 环境清单 (默认)</Radio>
              <Radio value="table">紧凑大矩阵</Radio>
              <Radio value="board">多列看板</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item name="filterEnv" label="环境过滤">
            <Select
              size="small"
              options={[
                { label: '全部环境', value: 'all' },
                { label: '仅看生产环境 (Prod)', value: 'prod' },
                { label: '仅看测试环境 (Test)', value: 'test' },
                { label: '仅看现场专区 (C0130)', value: 'c0130' }
              ]}
            />
          </Form.Item>

          <Form.Item name="isDefault" valuePropName="checked" label="设为默认">
            <Switch size="small" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
