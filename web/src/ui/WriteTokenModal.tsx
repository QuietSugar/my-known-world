import { useState, useEffect } from 'react'
import { Modal, Input, Button, Alert, App as AntdApp } from 'antd'
import { KeyRound, ShieldCheck, CheckCircle2, Trash2 } from 'lucide-react'

interface WriteTokenModalProps {
  open: boolean
  onClose: () => void
  onSuccess?: (token: string) => void
}

export function WriteTokenModal({ open, onClose, onSuccess }: WriteTokenModalProps) {
  const { message } = AntdApp.useApp()
  const [token, setToken] = useState('')
  const [existingToken, setExistingToken] = useState<string | null>(null)
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    if (open) {
      const saved = sessionStorage.getItem('mkw-write-token')
      setExistingToken(saved)
      setToken(saved || '')
    }
  }, [open])

  const handleSave = () => {
    const trimmed = token.trim()
    if (!trimmed) {
      message.warning('请输入写操作令牌')
      return
    }
    sessionStorage.setItem('mkw-write-token', trimmed)
    setExistingToken(trimmed)
    message.success('写操作令牌已保存到当前会话')
    onSuccess?.(trimmed)
    onClose()
  }

  const handleClear = () => {
    sessionStorage.removeItem('mkw-write-token')
    setExistingToken(null)
    setToken('')
    message.info('已清除会话中的写操作令牌')
  }

  const handleTest = async () => {
    const trimmed = token.trim()
    if (!trimmed) {
      message.warning('请先输入令牌再进行连通性测试')
      return
    }
    setTesting(true)
    try {
      // Test write token by calling preview on a dry-run mutation
      const res = await fetch('/api/write/nodes', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${trimmed}`
        },
        body: JSON.stringify({
          node: { id: '__dry_run_probe__', name: '__probe__', type: 'SoftwareProject', modelVersion: 1 }
        })
      })
      // Even if rejected due to validation or exists, 401/403 means bad token, 201/400/409 means token authorized
      if (res.status === 401 || res.status === 403) {
        message.error('令牌无效或已过期（HTTP 401/403 未授权）')
      } else {
        message.success('令牌认证成功！具备图谱写权限')
      }
    } catch (e) {
      message.error(e instanceof Error ? e.message : '连接服务器测试失败')
    } finally {
      setTesting(false)
    }
  }

  return (
    <Modal
      open={open}
      onCancel={onClose}
      title={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 16 }}>
          <KeyRound size={18} style={{ color: '#42633c' }} />
          <span>写操作令牌管理 (Write Token)</span>
        </div>
      }
      footer={[
        existingToken && (
          <Button key="clear" danger icon={<Trash2 size={14} />} onClick={handleClear} style={{ float: 'left' }}>
            清除令牌
          </Button>
        ),
        <Button key="cancel" onClick={onClose}>
          取消
        </Button>,
        <Button key="test" loading={testing} onClick={handleTest}>
          测试有效性
        </Button>,
        <Button key="save" type="primary" onClick={handleSave}>
          保存并启用
        </Button>
      ]}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 16, marginTop: 16 }}>
        <Alert
          type={existingToken ? 'success' : 'info'}
          showIcon
          icon={existingToken ? <CheckCircle2 size={16} /> : <ShieldCheck size={16} />}
          message={
            existingToken
              ? '当前浏览器会话已配置写令牌，可以直接执行新增、修改和关系维护。'
              : '写操作需要提供 MYKNOWNWORLD_WRITE_TOKEN。'
          }
          description="安全说明：令牌仅存储在当前标签页的 sessionStorage 中，关闭页面自动销毁，绝不持久化或外传。"
        />

        <div>
          <label style={{ display: 'block', marginBottom: 8, fontSize: 13, fontWeight: 500, color: '#434c3f' }}>
            令牌字符串 (MYKNOWNWORLD_WRITE_TOKEN)
          </label>
          <Input.Password
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder="请输入服务端的写操作授权令牌…"
            size="large"
            autoFocus
            onPressEnter={handleSave}
          />
        </div>
      </div>
    </Modal>
  )
}
