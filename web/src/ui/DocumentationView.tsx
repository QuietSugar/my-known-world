import { useEffect, useState } from 'react'
import {
  Menu,
  Tag,
  Button,
  Divider
} from 'antd'
import {
  BookOpen,
  ExternalLink,
  ArrowLeft,
  ChevronLeft,
  ChevronRight
} from 'lucide-react'
import {
  docMenuItems,
  docDefaultOpenKeys,
  defaultDocKey,
  getDocArticle,
  getDocTitle,
  docCategories
} from './docs/registry'
import { MarkdownRenderer } from './docs/MarkdownRenderer'

interface DocumentationViewProps {
  initialSlug?: string
  onBackToGraph: () => void
}

export function DocumentationView({ initialSlug, onBackToGraph }: DocumentationViewProps) {
  const getSlugFromLocation = (): string => {
    // 1. Check path: /docs/:slug
    const pathname = window.location.pathname
    if (pathname.startsWith('/docs')) {
      const slug = pathname.replace(/^\/docs\/?/, '').split('/')[0]
      if (slug && getDocArticle(slug)) return slug
    }
    // 2. Check query: ?doc=:slug
    const docQuery = new URLSearchParams(window.location.search).get('doc')
    if (docQuery && getDocArticle(docQuery)) return docQuery

    return initialSlug && getDocArticle(initialSlug) ? initialSlug : defaultDocKey
  }

  const [selectedKey, setSelectedKey] = useState<string>(getSlugFromLocation)

  // Listen to popstate for back/forward navigation within docs
  useEffect(() => {
    const handlePopState = () => {
      setSelectedKey(getSlugFromLocation())
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const article = getDocArticle(selectedKey) || getDocArticle(defaultDocKey)!

  // Previous and next articles for bottom navigation
  const allArticles = docCategories.flatMap(c => c.articles)
  const currentIndex = allArticles.findIndex(a => a.key === article.key)
  const prevArticle = currentIndex > 0 ? allArticles[currentIndex - 1] : null
  const nextArticle = currentIndex >= 0 && currentIndex < allArticles.length - 1 ? allArticles[currentIndex + 1] : null

  const switchArticle = (key: string) => {
    setSelectedKey(key)
    // Update independent URL path: /docs/:slug
    const nextPath = `/docs/${key}`
    window.history.pushState({}, '', nextPath)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <div className="app-shell" style={{ minHeight: '100vh', display: 'flex' }}>
      {/* Documentation Modular Sidebar */}
      <aside className="sidebar docs-sidebar" style={{ overflowY: 'auto' }}>
        <div className="brand" style={{ marginBottom: 12 }}>
          <div className="brand-mark"><BookOpen size={18} strokeWidth={2.3} /></div>
          <span>my<span className="brand-light">known</span>world</span>
          <Tag color="green" style={{ marginLeft: 'auto', marginRight: 0 }}>文档中心</Tag>
        </div>

        <div style={{ padding: '0 4px', marginBottom: 16 }}>
          <Button
            type="dashed"
            icon={<ArrowLeft size={14} />}
            onClick={onBackToGraph}
            block
            style={{
              height: 36,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              gap: 6,
              color: '#42633c',
              borderColor: '#b8d2b2'
            }}
          >
            返回图谱工作区
          </Button>
        </div>

        <div className="side-label" style={{ paddingLeft: 8 }}>文档章节</div>
        <Menu
          mode="inline"
          selectedKeys={[article.key]}
          defaultOpenKeys={docDefaultOpenKeys}
          onClick={({ key }) => switchArticle(key)}
          style={{ border: 0, background: 'transparent' }}
          items={docMenuItems}
        />

        <div className="sidebar-bottom" style={{ marginTop: 'auto', paddingTop: 16 }}>
          <Button
            type="text"
            icon={<ExternalLink size={14} />}
            href="https://github.com/QuietSugar/my-known-world"
            target="_blank"
            block
            style={{ textAlign: 'left', color: '#666', fontSize: 12 }}
          >
            GitHub 开源代码仓
          </Button>
          <div style={{ fontSize: 11, color: '#aaa', padding: '6px 8px 0' }}>
            MyKnownWorld Docs v1.0
          </div>
        </div>
      </aside>

      {/* Main Documentation Reading Content */}
      <main className="main-area">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>我的世界</span>
            <span className="crumb-slash">/</span>
            <span>帮助与文档</span>
            <span className="crumb-slash">/</span>
            <strong>{getDocTitle(article.key)}</strong>
          </div>
          <div className="top-actions">
            <Button icon={<ArrowLeft size={14} />} onClick={onBackToGraph}>
              返回图谱
            </Button>
            <Button
              type="primary"
              icon={<ExternalLink size={14} />}
              href="https://github.com/QuietSugar/my-known-world"
              target="_blank"
            >
              GitHub 仓库
            </Button>
          </div>
        </header>

        <div className="content" style={{ maxWidth: 960, padding: '32px 40px 60px' }}>
          {/* Article Header info */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 20 }}>
            <div style={{ width: 38, height: 38, borderRadius: 10, background: '#eaf0e7', color: '#42633c', display: 'grid', placeItems: 'center', flexShrink: 0 }}>
              {article.icon || <BookOpen size={20} />}
            </div>
            <div>
              <div style={{ fontSize: 12, color: '#888' }}>
                独立路由: <code style={{ color: '#42633c' }}>/docs/{article.slug}</code> · 原始文件: <code style={{ color: '#666' }}>{article.slug}.md</code>
              </div>
            </div>
          </div>

          {/* Render Raw Markdown content */}
          <MarkdownRenderer content={article.rawMarkdown} />

          {/* Bottom Pagination Links */}
          <Divider style={{ margin: '48px 0 24px' }} />
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 16 }}>
            {prevArticle ? (
              <Button
                style={{ height: 44, display: 'flex', alignItems: 'center', gap: 8 }}
                onClick={() => switchArticle(prevArticle.key)}
              >
                <ChevronLeft size={16} />
                <div style={{ textAlign: 'left' }}>
                  <div style={{ fontSize: 10, color: '#888' }}>上一篇</div>
                  <div style={{ fontSize: 12, fontWeight: 600 }}>{prevArticle.title}</div>
                </div>
              </Button>
            ) : <div />}

            {nextArticle ? (
              <Button
                type="primary"
                style={{ height: 44, display: 'flex', alignItems: 'center', gap: 8 }}
                onClick={() => switchArticle(nextArticle.key)}
              >
                <div style={{ textAlign: 'right' }}>
                  <div style={{ fontSize: 10, color: '#e8f0e5' }}>下一篇</div>
                  <div style={{ fontSize: 12, fontWeight: 600 }}>{nextArticle.title}</div>
                </div>
                <ChevronRight size={16} />
              </Button>
            ) : <div />}
          </div>
        </div>
      </main>
    </div>
  )
}
