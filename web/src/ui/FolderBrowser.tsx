import { useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, ListTree, Search } from 'lucide-react'
import type { Node, Relation } from './api'
import { buildGrouping, collectKeys, filterTree, GROUPINGS, type BrowserEntry } from './folderTree'

interface FolderBrowserProps {
  nodes: Node[]
  relations: Relation[]
  // selectedId keeps the outline in sync when a node is opened elsewhere.
  selectedId?: string
  onOpen: (node: Node) => void
}

// FolderBrowser presents the graph as an expandable outline. The grouping is
// switchable because no single hierarchy fits every question: hosting order is
// right for operating the fleet, but useless when reviewing a delivery project,
// where the project's own boundary is what matters.
export function FolderBrowser({ nodes, relations, selectedId, onOpen }: FolderBrowserProps) {
  const [grouping, setGrouping] = useState('hosting')
  const [query, setQuery] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())

  const entries = useMemo(() => buildGrouping(grouping, nodes, relations), [grouping, nodes, relations])
  const visible = useMemo(() => filterTree(entries, query), [entries, query])
  const isSearching = query.trim().length > 0

  // Opening a node from elsewhere (detail drawer, search) should reveal its row.
  useEffect(() => {
    if (!selectedId) return
    setExpanded(prev => {
      const next = new Set(prev)
      const walk = (list: BrowserEntry[], trail: string[]): boolean => {
        for (const e of list) {
          if (e.node?.id === selectedId) {
            for (const k of trail) next.add(k)
            return true
          }
          if (e.children && walk(e.children, [...trail, e.key])) return true
        }
        return false
      }
      walk(entries, [])
      return next
    })
  }, [selectedId, entries])

  const isOpen = (key: string) => isSearching || expanded.has(key)
  const toggle = (key: string) => setExpanded(prev => {
    const next = new Set(prev)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    return next
  })

  const current = GROUPINGS.find(g => g.id === grouping)

  return (
    <section className="panel folder-panel">
      <div className="panel-head">
        <div>
          <div className="panel-title flex items-center gap-2"><ListTree size={15} /> 目录浏览</div>
          <div className="panel-sub">{current?.hint}</div>
        </div>
        <div className="folder-actions">
          <button className="text-action" onClick={() => setExpanded(new Set(collectKeys(entries)))}>全部展开</button>
          <button className="text-action" onClick={() => setExpanded(new Set())}>全部折叠</button>
        </div>
      </div>

      <div className="folder-toolbar">
        <div className="folder-groupings" role="tablist" aria-label="组织方式">
          {GROUPINGS.map(g => (
            <button
              key={g.id}
              role="tab"
              aria-selected={grouping === g.id}
              className={`folder-grouping ${grouping === g.id ? 'active' : ''}`}
              onClick={() => { setGrouping(g.id); setExpanded(new Set()) }}
            >
              {g.label}
            </button>
          ))}
        </div>
        <div className="search-box folder-search">
          <Search size={14} />
          <input
            value={query}
            onChange={e => setQuery(e.target.value)}
            placeholder="过滤名称、主机名、端口…"
            aria-label="过滤目录"
          />
        </div>
      </div>

      {visible.length === 0 ? (
        <div className="empty-state">
          <div className="empty-art"><Folder size={20} /></div>
          <strong>没有匹配的内容</strong>
          <span>换个关键词，或切换到别的组织方式。</span>
        </div>
      ) : (
        <div className="folder-tree">
          <TreeList entries={visible} depth={0} isOpen={isOpen} onToggle={toggle} onOpen={onOpen} selectedId={selectedId} />
        </div>
      )}
    </section>
  )
}

function TreeList({ entries, depth, isOpen, onToggle, onOpen, selectedId }: {
  entries: BrowserEntry[]
  depth: number
  isOpen: (key: string) => boolean
  onToggle: (key: string) => void
  onOpen: (node: Node) => void
  selectedId?: string
}) {
  return (
    <ul className="folder-list">
      {entries.map(entry => (
        <TreeRow key={entry.key} entry={entry} depth={depth} isOpen={isOpen} onToggle={onToggle} onOpen={onOpen} selectedId={selectedId} />
      ))}
    </ul>
  )
}

function TreeRow({ entry, depth, isOpen, onToggle, onOpen, selectedId }: {
  entry: BrowserEntry
  depth: number
  isOpen: (key: string) => boolean
  onToggle: (key: string) => void
  onOpen: (node: Node) => void
  selectedId?: string
}) {
  const hasChildren = !!entry.children && entry.children.length > 0
  const open = hasChildren && isOpen(entry.key)
  const active = !!entry.node && entry.node.id === selectedId

  return (
    <li className="folder-item">
      <div className={`folder-row ${active ? 'is-selected' : ''}`} style={{ paddingLeft: 8 + depth * 18 }}>
        <button
          className="folder-toggle"
          onClick={() => hasChildren && onToggle(entry.key)}
          aria-label={hasChildren ? (open ? '折叠' : '展开') : undefined}
          disabled={!hasChildren}
        >
          {hasChildren ? (open ? <ChevronDown size={13} /> : <ChevronRight size={13} />) : <span className="folder-toggle-spacer" />}
        </button>
        <span className="folder-icon">
          {hasChildren ? (open ? <FolderOpen size={14} /> : <Folder size={14} />) : <FileText size={14} />}
        </span>
        <button
          className="folder-label"
          onClick={() => { if (entry.node) onOpen(entry.node); else if (hasChildren) onToggle(entry.key) }}
        >
          <span className="folder-name">{entry.label}</span>
          {entry.detail && <span className="folder-detail">{entry.detail}</span>}
        </button>
        {hasChildren && <span className="folder-count">{entry.children?.length}</span>}
      </div>
      {open && entry.children && (
        <TreeList entries={entry.children} depth={depth + 1} isOpen={isOpen} onToggle={onToggle} onOpen={onOpen} selectedId={selectedId} />
      )}
    </li>
  )
}
