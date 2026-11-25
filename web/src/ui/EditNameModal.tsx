import { useState, useEffect } from 'react'
import { Modal, Input, Button, App as AntdApp } from 'antd'
import { Edit3 } from 'lucide-react'
import { type Node, type ManualMutation, type MutationPlan, api } from './api'

interface EditNameModalProps {
  node: Node | null
  open: boolean
  onClose: () => void
  onSuccess: (updated: Node) => void
  onNeedToken?: () => void
}

export function EditNameModal({ node, open, onClose, onSuccess, onNeedToken }: EditNameModalProps) {
  const { message, modal } = AntdApp.useApp()
  const [name, setName] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (open && node) {
      setName(node.name)
    }
  }, [open, node])

  const handleSave = async () => {
    if (!node) return
    const trimmed = name.trim()
    if (!trimmed) {
      message.warning('对象名称不能为空')
      return
    }
    if (trimmed === node.name) {
      onClose()
      return
    }

    let token = sessionStorage.getItem('mkw-write-token')
    if (!token) {
      message.warning('请先配置写操作令牌')
      onNeedToken?.()
      return
    }

    setSubmitting(true)
    try {
      const updated: Node = { ...node, name: trimmed }
      const mutation: ManualMutation = { kind: 'updateNode', id: node.id, node: updated }

      const plan: MutationPlan = await api.previewMutation(mutation)
      const confirmed = await new Promise<boolean>(resolve => {
        modal.confirm({
          title: '确认更新对象名称',
          content: (
            <div>
              <p>即将将对象 <strong>{node.id}</strong> 的名称修改为：</p>
              <div style={{ padding: '8px 12px', background: '#f5f7f4', borderRadius: 6, fontWeight: 600, color: '#3d5c39' }}>
                {trimmed}
              </div>
              <small style={{ display: 'block', marginTop: 8, color: '#888' }}>
                图谱版本：{plan.expectedRevision.slice(0, 12)}…
              </small>
            </div>
          ),
          okText: '确认更新',
          cancelText: '取消',
          onOk: () => resolve(true),
          onCancel: () => resolve(false)
        })
      })

      if (!confirmed) {
        setSubmitting(false)
        return
      }

      await api.applyMutation(mutation, plan, token)
      message.success('对象名称已成功修改')
      onSuccess(updated)
      onClose()
    } catch (err) {
      message.error(err instanceof Error ? err.message : '更新名称失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 16 }}>
          <Edit3 size={17} style={{ color: '#42633c' }} />
          <span>修改对象名称</span>
        </div>
      }
      footer={[
        <Button key="cancel" onClick={onClose}>
          取消
        </Button>,
        <Button key="save" type="primary" loading={submitting} onClick={handleSave}>
          保存修改
        </Button>
      ]}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12, marginTop: 14 }}>
        <div style={{ fontSize: 12, color: '#7a8274' }}>
          节点标识：<code style={{ background: '#eef2ec', padding: '2px 6px', borderRadius: 4 }}>{node?.id}</code>
        </div>
        <div>
          <label style={{ display: 'block', marginBottom: 6, fontSize: 13, fontWeight: 500, color: '#434c3f' }}>
            新对象名称
          </label>
          <Input
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="输入新的对象名称…"
            size="large"
            autoFocus
            onPressEnter={handleSave}
          />
        </div>
      </div>
    </Modal>
  )
}
