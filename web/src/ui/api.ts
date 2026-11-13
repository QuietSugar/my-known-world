export type NodeType = 'Self' | 'Hardware' | 'Machine' | 'SoftwareProject' | 'SoftwareInstance' | 'RunningPeriod' | 'AccountCredential' | 'Product' | 'Document' | 'Runbook' | 'WorkItem' | 'ProvisionalObject'
export type EvidenceKind = 'unknown' | 'manual' | 'document' | 'observed' | 'derived' | 'imported'
export type Certainty = 'unknown' | 'provisional' | 'confirmed' | 'disputed' | 'stale'
export interface Node { id: string; type: NodeType; name: string; lifecycle?: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; lastConfirmed?: string; modelVersion: number }
export interface Relation { id: string; from: string; predicate: string; to: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; recordedAt: string }
export interface Impact { node: Node; relationPath: Relation[] }

async function readJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { headers: { Accept: 'application/json' } })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { error?: { message?: string } } | null
    throw new Error(body?.error?.message ?? `Request failed (${response.status})`)
  }
  return response.json() as Promise<T>
}

export const api = {
  nodes: () => readJSON<{ items: Node[] }>('/api/nodes'),
  impact: () => readJSON<{ items: Impact[] }>('/api/impact'),
  path: (id: string) => readJSON<{ nodes: Node[]; relations: Relation[] }>(`/api/path/${encodeURIComponent(id)}`),
  health: () => readJSON<{ status: string }>('/healthz'),
}
