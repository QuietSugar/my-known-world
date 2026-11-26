import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider, App as AntdApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { App } from './ui/App'
import { NodePage } from './ui/NodePage'
import './ui/styles.css'
import './ui/cmdb.css'

const nodeMatch = window.location.pathname.match(/^\/node\/([^/]+)\/?$/)

function nodeIdFromMatch(match: RegExpMatchArray | null): string | null {
  if (!match) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}

const nodeId = nodeIdFromMatch(nodeMatch)

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#42633c',
          borderRadius: 8,
          fontFamily: "'DM Sans', system-ui, -apple-system, sans-serif",
        },
      }}
    >
      <AntdApp>
        {nodeId ? <NodePage id={nodeId} /> : <App />}
      </AntdApp>
    </ConfigProvider>
  </React.StrictMode>,
)
