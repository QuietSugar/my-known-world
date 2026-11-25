import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'MyKnownWorld 文档中心',
  description: '个人认知图谱 · 资产拓扑 · MCP 人机协作底座',
  base: '/docs/',
  lang: 'zh-CN',
  ignoreDeadLinks: true,

  themeConfig: {
    siteTitle: 'MyKnownWorld Docs',
    logo: '/logo.svg',

    nav: [
      { text: '认识与定位', link: '/intro/philosophy' },
      { text: '核心概念', link: '/concepts/node-types' },
      { text: '操作指南', link: '/guides/workspace-guide' },
      { text: 'API & MCP', link: '/api/mcp-integration' },
      {
        text: '系统设计',
        items: [
          { text: '系统需求与愿景', link: '/requirements' },
          { text: '增量同步协议', link: '/incremental-sync' },
          { text: '凭证提供者与边界', link: '/secret-providers' },
          { text: '数据库接管与演进', link: '/database-authoritative-cutover' },
          { text: '备份与恢复手册', link: '/backup-and-restore' },
          { text: '项目交接备忘', link: '/handover' }
        ]
      },
      { text: '进入工作区', link: '/' }
    ],

    sidebar: [
      {
        text: '认识与定位',
        collapsed: false,
        items: [
          { text: '哲理与设计定位', link: '/intro/philosophy' },
          { text: '核心原则与边界', link: '/intro/principles' }
        ]
      },
      {
        text: '核心概念与规范',
        collapsed: false,
        items: [
          { text: '节点类型体系', link: '/concepts/node-types' },
          { text: '自陈谓词规范', link: '/concepts/self-predicates' },
          { text: '拓扑关系谓词', link: '/concepts/topology-predicates' }
        ]
      },
      {
        text: '功能操作指南',
        collapsed: false,
        items: [
          { text: '工作区与表格操作', link: '/guides/workspace-guide' },
          { text: '关系维护与探索视图', link: '/guides/features-guide' },
          { text: 'CMDB 资产与凭证管理', link: '/guides/cmdb-guide' }
        ]
      },
      {
        text: '接口与 AI 协作',
        collapsed: false,
        items: [
          { text: 'MCP 协议与 Agent 协作', link: '/api/mcp-integration' },
          { text: 'HTTP 写入 API 规范', link: '/api/write-api' }
        ]
      },
      {
        text: '系统设计与技术规范',
        collapsed: true,
        items: [
          { text: '需求与愿景规范', link: '/requirements' },
          { text: '增量同步协议', link: '/incremental-sync' },
          { text: '凭证提供者与边界', link: '/secret-providers' },
          { text: '数据库接管与割接', link: '/database-authoritative-cutover' },
          { text: '备份与恢复流程', link: '/backup-and-restore' },
          { text: '交接状态与沉淀', link: '/handover' }
        ]
      }
    ],

    outline: {
      level: [2, 3],
      label: '本页大纲'
    },

    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: '搜索文档',
            buttonAriaLabel: '搜索文档'
          },
          modal: {
            noResultsText: '无法找到相关结果',
            resetButtonTitle: '清除查询条件',
            footer: {
              selectText: '选择',
              navigateText: '切换',
              closeText: '关闭'
            }
          }
        }
      }
    },

    docFooter: {
      prev: '上一篇',
      next: '下一篇'
    },

    darkModeSwitchLabel: '深浅外观',
    lightModeSwitchTitle: '切换到浅色模式',
    darkModeSwitchTitle: '切换到深色模式',
    sidebarMenuLabel: '目录菜单',
    returnToTopLabel: '返回顶部'
  }
})
