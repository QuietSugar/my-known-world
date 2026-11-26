import { useMemo, useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, Search } from 'lucide-react'
import type { Node } from './api'
import { buildInfrastructureTree, buildTypeGroups, collectKeys, filterTree, type BrowserEntry } from './folderTree'

interface FolderBrowserProps {
  nodes: Node[]
  relations: import('./api').Relation[]
  onOpen: (node: Node) => void
}

// FolderBrowser presents the graph as an expandable outline instead of a flat
// table or a force-directed subgraph. Nesting follows the real hosting
// hierarchy: hardware → machine → service → credential.
export function FolderBrowser({ nodes, relations, onOpen }: FolderBrowserProps) {
  const [query, setQuery] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set())

  const infra = useMemo(() => buildInfrastructureTree(nodes, relations), [nodes, relations])
  const others = useMemo(() => buildTypeGroups(nodes), [nodes])

  const visibleInfra = useMemo(() => filterTree(infra, query), [infra, query])
  const visibleOthers = useMemo(() => filterTree(others, query), [others, query])

  // While searching, show every match instead of making the user expand.
  const isSearching = query.trim().length > 0
  const isOpen = (key: string) => isSearching || expanded.has(key)

  const toggle = (key: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <section className="panel folder-panel">
      <div className="panel-head">
        <div>
          <div className="panel-title">目录浏览</div>
          <div className="panel-sub">按实际承载关系组织：硬件 → 机器 → 服务 → 凭据</div>
        </div>
        <div className="folder-actions">
          <button className="text-action" onClick={() => setExpanded(new Set(collectKeys([...infra, ...others])))}>全部展开</button>
          <button className="text-action" onClick={() => setExpanded(new Set())}>全部折叠</button>
        </div>
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

      {visibleInfra.length === 0 && visibleOthers.length === 0 ? (
        <div className="empty-state">
          <div className="empty-art"><Folder size={20} /></div>
          <strong>没有匹配的内容</strong>
          <span>试试更短的关键词，或清空过滤框。</span>
        </div>
      ) : (
        <div className="folder-tree">
          <TreeList entries={visibleInfra} depth={0} isOpen={isOpen} onToggle={toggle} onOpen={onOpen} />
          {visibleOthers.length > 0 && (
            <>
              <div className="folder-section-label">其他</div>
              <TreeList entries={visibleOthers} depth={0} isOpen={isOpen} onToggle={toggle} onOpen={onOpen} />
            </>
          )}
        </div>
      )}
    </section>
  )
}

function TreeList({ entries, depth, isOpen, onToggle, onOpen }: {
  entries: BrowserEntry[]
  depth: number
  isOpen: (key: string) => boolean
  onToggle: (key: string) => void
  onOpen: (node: Node) => void
}) {
  return (
    <ul className="folder-list">
      {entries.map(entry => (
        <TreeRow key={entry.key} entry={entry} depth={depth} isOpen={isOpen} onToggle={onToggle} onOpen={onOpen} />
      ))}
    </ul>
  )
}

function TreeRow({ entry, depth, isOpen, onToggle, onOpen }: {
  entry: BrowserEntry
  depth: number
  isOpen: (key: string) => boolean
  onToggle: (key: string) => void
  onOpen: (node: Node) => void
}) {
  const hasChildren = !!entry.children && entry.children.length > 0
  const open = hasChildren && isOpen(entry.key)

  return (
    <li className="folder-item">
      <div
        className={`folder-row ${entry.kind === 'group' ? 'is-group' : ''}`}
        style={{ paddingLeft: 8 + depth * 18 }}
      >
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
        <button className="folder-label" onClick={() => entry.node && onOpen(entry.node)} disabled={!entry.node}>
          <span className="folder-name">{entry.label}</span>
          {entry.detail && <span className="folder-detail">{entry.detail}</span>}
        </button>
        {hasChildren && <span className="folder-count">{entry.children?.length}</span>}
      </div>
      {open && entry.children && (
        <TreeList entries={entry.children} depth={depth + 1} isOpen={isOpen} onToggle={onToggle} onOpen={onOpen} />
      )}
    </li>
  )
}
