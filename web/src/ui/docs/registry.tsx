import type { MenuProps } from 'antd'
import type { DocCategory, DocArticle } from './types'
import {
  Sparkles,
  ShieldCheck,
  Network,
  FolderTree,
  Tag as TagIcon,
  GitBranch,
  Layers3,
  Code2,
  Database
} from 'lucide-react'

// Import raw markdown files
import philosophyMd from './content/intro/philosophy.md?raw'
import principlesMd from './content/intro/principles.md?raw'
import selfPredicatesMd from './content/concepts/self-predicates.md?raw'
import nodeTypesMd from './content/concepts/node-types.md?raw'
import topologyPredicatesMd from './content/concepts/topology-predicates.md?raw'
import workspaceGuideMd from './content/guides/workspace-guide.md?raw'
import featuresGuideMd from './content/guides/features-guide.md?raw'
import mcpIntegrationMd from './content/api/mcp-integration.md?raw'
import writeApiMd from './content/api/write-api.md?raw'

export const docCategories: DocCategory[] = [
  {
    key: 'grp-intro',
    title: '认识与定位',
    articles: [
      {
        key: 'philosophy',
        slug: 'philosophy',
        title: '定位与设计哲学',
        description: '主观真实（Subjective Truth）与已知世界的记录哲学',
        icon: <Sparkles size={15} />,
        rawMarkdown: philosophyMd
      },
      {
        key: 'principles',
        slug: 'principles',
        title: '四大核心设计原则',
        description: 'Self 唯一锚点、零秘密、主观隔离与先预览后应用',
        icon: <ShieldCheck size={15} />,
        rawMarkdown: principlesMd
      }
    ]
  },
  {
    key: 'grp-model',
    title: '核心概念与规范',
    articles: [
      {
        key: 'self-predicates',
        slug: 'self-predicates',
        title: 'Self 与 5 种原初关系',
        description: 'owns / maintains / uses / canOperate / knowsAbout',
        icon: <Network size={15} />,
        rawMarkdown: selfPredicatesMd
      },
      {
        key: 'node-types',
        slug: 'node-types',
        title: '12 类核心实体模型',
        description: 'Machine、SoftwareProject、SoftwareInstance 等实体规范',
        icon: <FolderTree size={15} />,
        rawMarkdown: nodeTypesMd
      },
      {
        key: 'topology-predicates',
        slug: 'topology-predicates',
        title: '业务拓扑连线谓词',
        description: 'hosts、implementedFrom、hasCredential 等有向关系',
        icon: <TagIcon size={15} />,
        rawMarkdown: topologyPredicatesMd
      }
    ]
  },
  {
    key: 'grp-guides',
    title: '功能操作指南',
    articles: [
      {
        key: 'workspace-guide',
        slug: 'workspace-guide',
        title: '项目工作区指南',
        description: 'Project Workspace 四大聚合视图深度解析',
        icon: <GitBranch size={15} />,
        rawMarkdown: workspaceGuideMd
      },
      {
        key: 'features-guide',
        slug: 'features-guide',
        title: '探索、影响与批量同步',
        description: '子图展开、故障波及面计算与可逆快照撤回',
        icon: <Layers3 size={15} />,
        rawMarkdown: featuresGuideMd
      }
    ]
  },
  {
    key: 'grp-api',
    title: '接口与 AI 协作',
    articles: [
      {
        key: 'mcp-integration',
        slug: 'mcp-integration',
        title: '大模型只读 MCP 接入',
        description: 'Cursor / Claude / Antigravity 自动化挂载图谱',
        icon: <Code2 size={15} />,
        rawMarkdown: mcpIntegrationMd
      },
      {
        key: 'write-api',
        slug: 'write-api',
        title: '写入 API 与认证安全',
        description: '自动化写入端点规范与会话级令牌保障',
        icon: <Database size={15} />,
        rawMarkdown: writeApiMd
      }
    ]
  }
]

export const defaultDocKey = 'philosophy'

const allArticles = docCategories.flatMap(cat => cat.articles)
const articleMap = new Map<string, DocArticle>(allArticles.map(art => [art.key, art]))

export function getDocArticle(keyOrSlug: string): DocArticle | undefined {
  return articleMap.get(keyOrSlug) || allArticles.find(a => a.slug === keyOrSlug)
}

export function getDocTitle(keyOrSlug: string): string {
  return getDocArticle(keyOrSlug)?.title || '帮助与使用指南'
}

export const docMenuItems: MenuProps['items'] = docCategories.map(cat => ({
  key: cat.key,
  label: cat.title,
  type: 'group',
  children: cat.articles.map(art => ({
    key: art.key,
    icon: art.icon,
    label: art.title
  }))
}))

export const docDefaultOpenKeys = docCategories.map(c => c.key)
