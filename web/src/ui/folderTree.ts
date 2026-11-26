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
