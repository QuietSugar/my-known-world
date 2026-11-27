import type { Node, Relation } from './api'

// BrowserEntry is one row in the folder-style browser: either a group that can
// be expanded, or a leaf node.
export interface BrowserEntry {
  kind: 'group' | 'node'
  // key is stable so expansion state survives a refresh.
  key: string
  label: string
  // detail is the short line under the label — host, type, or child count.
  detail?: string
  // group fields
  children?: BrowserEntry[]
  count?: number
  type?: string
  // node fields
  node?: Node
}

// hostsIndex maps a parent node id to the children linked by a hosts relation.
// The graph stores hosting as Host -> Child, so walking down from hardware
// through machines to services is a straightforward parent/child relation.
function hostsIndex(relations: Relation[]): Map<string, string[]> {
  const out = new Map<string, string[]>()
  for (const r of relations) {
    if (r.predicate !== 'hosts') continue
    const list = out.get(r.from)
    if (list) list.push(r.to)
    else out.set(r.from, [r.to])
  }
  return out
}

// relatedIndex maps a node id to the ids it points at, for predicates that
// group things without a hosting relationship (credentials, projects).
function predicateIndex(relations: Relation[], predicate: string): Map<string, string[]> {
  const out = new Map<string, string[]>()
  for (const r of relations) {
    if (r.predicate !== predicate) continue
    const list = out.get(r.from)
    if (list) list.push(r.to)
    else out.set(r.from, [r.to])
  }
  return out
}

function nodeLabel(n: Node): string {
  return n.name || n.id
}

function nodeDetail(n: Node, children: string[] | undefined, byId: Map<string, Node>): string {
  const parts: string[] = []
  if (children && children.length > 0) {
    const hosts = children.map(id => byId.get(id)).filter((x): x is Node => !!x)
    if (hosts.length > 0) {
      const names = hosts.slice(0, 3).map(nodeLabel).join('、')
      parts.push(hosts.length > 3 ? `${names} 等 ${hosts.length} 项` : names)
    }
  }
  if (n.machine?.hostname) parts.push(n.machine.hostname)
  if (n.softwareInstance?.ports?.length) parts.push(`端口 ${n.softwareInstance.ports.join('/')}`)
  if (n.lifecycle && n.lifecycle !== 'active') parts.push(n.lifecycle)
  return parts.join(' · ')
}

// buildInfrastructureTree builds the hardware → machine → service → credential
// hierarchy that mirrors how the fleet is actually organised on disk and in
// inventory sheets.
export function buildInfrastructureTree(nodes: Node[], relations: Relation[]): BrowserEntry[] {
  const byId = new Map(nodes.map(n => [n.id, n]))
  const hosts = hostsIndex(relations)
  const credentials = predicateIndex(relations, 'hasCredential')

  const hardware = nodes.filter(n => n.type === 'Hardware')
  const machines = nodes.filter(n => n.type === 'Machine')
  const services = nodes.filter(n => n.type === 'SoftwareInstance')

  // A machine is hosted by hardware; a service is hosted by a machine.
  const machineChildren = (id: string): string[] =>
    (hosts.get(id) ?? []).filter(cid => byId.get(cid)?.type === 'Machine')
  const serviceChildren = (id: string): string[] =>
    (hosts.get(id) ?? []).filter(cid => byId.get(cid)?.type === 'SoftwareInstance')
  const credentialChildren = (id: string): string[] =>
    (credentials.get(id) ?? []).filter(cid => byId.get(cid)?.type === 'AccountCredential')

  const serviceEntry = (n: Node): BrowserEntry => {
    const creds = credentialChildren(n.id)
    const children = creds
      .map(cid => byId.get(cid))
      .filter((x): x is Node => !!x)
      .map(c => ({
        kind: 'node' as const,
        key: c.id,
        label: nodeLabel(c),
        detail: [c.accountCredential?.account, c.accountCredential?.purpose].filter(Boolean).join(' · '),
        node: c,
      }))
    return {
      kind: 'node',
      key: n.id,
      label: nodeLabel(n),
      detail: nodeDetail(n, creds, byId),
      node: n,
      children: children.length > 0 ? children : undefined,
      count: children.length,
    }
  }

  const machineEntry = (n: Node): BrowserEntry => {
    const svcIds = serviceChildren(n.id)
    const children = svcIds
      .map(sid => byId.get(sid))
      .filter((x): x is Node => !!x)
      .map(serviceEntry)
    const creds = credentialChildren(n.id)
    for (const c of creds.map(cid => byId.get(cid)).filter((x): x is Node => !!x)) {
      children.push({
        kind: 'node', key: c.id, label: nodeLabel(c),
        detail: [c.accountCredential?.account, c.accountCredential?.purpose].filter(Boolean).join(' · '),
        node: c,
      })
    }
    return {
      kind: 'node',
      key: n.id,
      label: nodeLabel(n),
      detail: nodeDetail(n, svcIds, byId),
      node: n,
      children: children.length > 0 ? children : undefined,
      count: children.length,
    }
  }

  const groups: BrowserEntry[] = []
  const assigned = new Set<string>()

  for (const hw of hardware.sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))) {
    const machineIds = machineChildren(hw.id)
    const children = machineIds
      .map(mid => byId.get(mid))
      .filter((x): x is Node => !!x)
      .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
      .map(m => {
        assigned.add(m.id)
        return machineEntry(m)
      })
    groups.push({
      kind: 'node',
      key: hw.id,
      label: nodeLabel(hw),
      detail: nodeDetail(hw, machineIds, byId),
      node: hw,
      children: children.length > 0 ? children : undefined,
      count: children.length,
    })
    for (const mid of machineIds) assigned.add(mid)
  }

  // Machines with no hardware parent (shared VPS, standalone hosts).
  const orphans = machines
    .filter(m => !assigned.has(m.id))
    .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
  if (orphans.length > 0) {
    groups.push({
      kind: 'group',
      key: 'group:unhosted-machines',
      label: '无硬件档案的机器',
      detail: `${orphans.length} 台`,
      count: orphans.length,
      children: orphans.map(m => {
        assigned.add(m.id)
        return machineEntry(m)
      }),
    })
  }

  // Services not attached to any machine appear under their own group so none
  // are lost; they are usually external or customer-hosted.
  const hostedServices = new Set<string>()
  for (const [, kids] of hosts) for (const k of kids) hostedServices.add(k)
  const loose = services.filter(s => !hostedServices.has(s.id))
  if (loose.length > 0) {
    groups.push({
      kind: 'group',
      key: 'group:unhosted-services',
      label: '未挂宿主机的服务',
      detail: `${loose.length} 个，多为外部或客户环境`,
      count: loose.length,
      children: loose
        .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
        .map(serviceEntry),
    })
  }

  return groups
}

// buildTypeGroups groups every node by type, for browsing what does not fit the
// infrastructure tree (documents, runbooks, projects).
export function buildTypeGroups(nodes: Node[]): BrowserEntry[] {
  const order = ['Document', 'Runbook', 'SoftwareProject', 'AccountCredential', 'Network', 'Product', 'WorkItem', 'ProvisionalObject']
  const groups: BrowserEntry[] = []
  for (const type of order) {
    const items = nodes.filter(n => n.type === type)
    if (items.length === 0) continue
    groups.push({
      kind: 'group',
      key: `type:${type}`,
      label: type,
      detail: `${items.length} 项`,
      count: items.length,
      children: items
        .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
        .map(n => ({ kind: 'node' as const, key: n.id, label: nodeLabel(n), detail: nodeDetail(n, undefined, new Map()), node: n })),
    })
  }
  return groups
}

// filterTree keeps entries whose subtree matches the query.
export function filterTree(entries: BrowserEntry[], query: string): BrowserEntry[] {
  const q = query.trim().toLowerCase()
  if (!q) return entries
  const walk = (entry: BrowserEntry): BrowserEntry | null => {
    const selfHit = entry.label.toLowerCase().includes(q) || (entry.detail ?? '').toLowerCase().includes(q)
    const kids = entry.children?.map(walk).filter((x): x is BrowserEntry => !!x) ?? []
    if (selfHit || kids.length > 0) return { ...entry, children: kids.length > 0 ? kids : entry.children }
    return null
  }
  return entries.map(walk).filter((x): x is BrowserEntry => !!x)
}

// collectKeys returns every group key so callers can expand or collapse all.
export function collectKeys(entries: BrowserEntry[]): string[] {
  const out: string[] = []
  const walk = (e: BrowserEntry) => {
    if (e.children && e.children.length > 0) out.push(e.key)
    e.children?.forEach(walk)
  }
  entries.forEach(walk)
  return out
}

// Grouping describes one way of organising the same set of nodes.
export interface Grouping {
  id: string
  label: string
  hint: string
}

export const GROUPINGS: Grouping[] = [
  { id: 'hosting', label: '按承载关系', hint: '硬件 → 机器 → 服务 → 凭据' },
  { id: 'type', label: '按类型', hint: '按节点类型归类' },
  { id: 'source', label: '按来源', hint: '按数据来路归类，便于看交付批次' },
  { id: 'project', label: '按项目', hint: '按实现的软件项目归类' },
  { id: 'lifecycle', label: '按状态', hint: '活跃 / 已退役 / 已销毁' },
]

// buildFlatGroups groups nodes by an arbitrary key function, producing a
// two-level outline. It is the generic fallback for every grouping that is not
// the hosting hierarchy.
function buildFlatGroups(
  nodes: Node[],
  relations: Relation[],
  keyOf: (n: Node) => string | undefined,
  labelFor: (key: string) => string,
  emptyLabel: string,
): BrowserEntry[] {
  const byId = new Map(nodes.map(n => [n.id, n]))
  const groups = new Map<string, Node[]>()
  for (const n of nodes) {
    const key = keyOf(n) ?? emptyLabel
    const list = groups.get(key)
    if (list) list.push(n)
    else groups.set(key, [n])
  }
  const entries: BrowserEntry[] = []
  for (const [key, items] of [...groups.entries()].sort((a, b) => a[0].localeCompare(b[0], 'zh'))) {
    entries.push({
      kind: 'group',
      key: `group:${key}`,
      label: labelFor(key),
      detail: `${items.length} 项`,
      count: items.length,
      children: items
        .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
        .map(n => ({
          kind: 'node' as const,
          key: n.id,
          label: nodeLabel(n),
          detail: nodeDetail(n, undefined, byId),
          node: n,
        })),
      // relations 参数保留：未来可在此基础上按关系再分层
      ...(relations.length < 0 ? {} : {}),
    })
  }
  return entries
}

// buildGrouping returns the outline for the requested grouping id.
export function buildGrouping(
  grouping: string,
  nodes: Node[],
  relations: Relation[],
): BrowserEntry[] {
  switch (grouping) {
    case 'type':
      return buildFlatGroups(nodes.filter(n => n.type !== 'Self'), relations, n => n.type, k => k, '未分类')
    case 'source':
      return buildFlatGroups(nodes.filter(n => n.type !== 'Self'), relations, n => n.source || undefined, k => k, '未标注来源')
    case 'lifecycle':
      return buildFlatGroups(nodes.filter(n => n.type !== 'Self'), relations, n => n.lifecycle || 'active', k => k, '未标注状态')
    case 'project': {
      // 项目视图把「某个软件项目实现了哪些服务」摆在同一层，便于交付核对。
      const byId = new Map(nodes.map(n => [n.id, n]))
      const impl = new Map<string, Node[]>()
      for (const r of relations) {
        if (r.predicate !== 'implementedFrom') continue
        const from = byId.get(r.from)
        if (!from) continue
        const list = impl.get(r.to)
        if (list) list.push(from)
        else impl.set(r.to, [from])
      }
      const projects = nodes.filter(n => n.type === 'SoftwareProject')
      const entries: BrowserEntry[] = projects
        .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
        .map(p => {
          const kids = (impl.get(p.id) ?? []).sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
          return {
            kind: 'node' as const,
            key: p.id,
            label: nodeLabel(p),
            detail: kids.length > 0 ? `${kids.length} 个实现` : '暂无实现',
            node: p,
            count: kids.length,
            children: kids.map(n => ({
              kind: 'node' as const,
              key: n.id,
              label: nodeLabel(n),
              detail: nodeDetail(n, undefined, byId),
              node: n,
            })),
          }
        })
      // 没有归属项目的东西单独列出，避免看着像丢了
      const claimed = new Set(projects.map(p => p.id))
      for (const kids of impl.values()) for (const k of kids) claimed.add(k.id)
      const rest = nodes.filter(n => n.type !== 'Self' && n.type !== 'SoftwareProject' && !claimed.has(n.id))
      if (rest.length > 0) {
        entries.push({
          kind: 'group',
          key: 'group:unassigned',
          label: '未归属项目',
          detail: `${rest.length} 项`,
          count: rest.length,
          children: rest
            .sort((a, b) => nodeLabel(a).localeCompare(nodeLabel(b), 'zh'))
            .map(n => ({ kind: 'node' as const, key: n.id, label: nodeLabel(n), detail: nodeDetail(n, undefined, byId), node: n })),
        })
      }
      return entries
    }
    default:
      return buildInfrastructureTree(nodes, relations)
  }
}
