export type NodeType = 'Self' | 'Hardware' | 'Machine' | 'SoftwareProject' | 'SoftwareInstance' | 'RunningPeriod' | 'AccountCredential' | 'Product' | 'Document' | 'Runbook' | 'WorkItem' | 'Network' | 'ProvisionalObject'
export type EvidenceKind = 'unknown' | 'manual' | 'document' | 'observed' | 'derived' | 'imported'
export type Certainty = 'unknown' | 'provisional' | 'confirmed' | 'disputed' | 'stale'
export interface HardwareDetails { category?: string; model?: string; location?: string; specifications?: Record<string,string>; status?: string; accessReference?: string; maintenanceReference?: string }
export interface ResourceClaim { kind: string; name: string; scope?: string; port?: number }
export interface MachineDetails { hostname?: string; os?: string; ips?: string[]; network?: string; status?: string; accessMethod?: string; accessPrerequisites?: string[]; allowedOperations?: string[]; forbiddenOperations?: string[]; responsibilityBoundary?: string; resources?: ResourceClaim[]; lastVerified?: string }
export interface SoftwareProjectDetails { purpose?: string; languages?: string[]; form?: string; remoteRepository?: string; upstream?: string; maintenanceState?: string; recommendedVersion?: string; successorId?: string }
export interface Endpoint { kind: string; address: string; port?: number }
export interface SoftwareInstanceDetails { productName?: string; deploymentPath?: string; deploymentMethod?: string; runtimeIdentity?: string; endpoints?: Endpoint[]; configurationReferences?: string[]; deploymentState?: string; operationalState?: string; maintenanceState?: string; verificationResult?: string; verifiedAt?: string }
export interface CredentialDetails { account?: string; credentialType?: string; purpose?: string; scope?: string; accessReference?: string; rotationAt?: string; status?: string; lastVerified?: string }
export interface ProductDetails { purpose?: string; commonConfigurationReference?: string; runbookReference?: string }
export interface DocumentDetails { role?: string; authoritativeReference?: string; effective: boolean; lastVerified?: string; supersededBy?: string }
export interface RunbookDetails { trigger?: string; preconditions?: string[]; stepsReference?: string; stopConditionReference?: string; rollbackReference?: string; verificationMethod?: string; lastVerified?: string; executableStatus?: string; approvalRequired: boolean }
export interface NetworkDetails { kind?:string; cidrs?:string[]; gatewayReference?:string; accessLocations?:string[]; vpnReference?:string; status?:string }
export interface WorkItemDetails { kind?: string; status?: string; rationaleReference?: string; decisionEvidenceReference?: string }
export interface Node { id: string; type: NodeType; name: string; lifecycle?: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; lastConfirmed?: string; modelVersion: number; candidateType?: NodeType; reviewStatus?: 'pending' | 'rejected'; reviewQuestions?: string[]; startedAt?: string; endedAt?: string; deployedVersion?: string; verificationAt?: string; verificationResult?: string; evidenceRef?: string; hardware?: HardwareDetails; machine?: MachineDetails; softwareProject?: SoftwareProjectDetails; softwareInstance?: SoftwareInstanceDetails; accountCredential?: CredentialDetails; product?: ProductDetails; document?: DocumentDetails; runbook?: RunbookDetails; workItem?: WorkItemDetails; network?:NetworkDetails }
export interface Relation { id: string; from: string; predicate: string; to: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; recordedAt: string }
export interface Exploration { centerId: string; direction: 'both'|'in'|'out'; depth: number; nodes: Node[]; relations: Relation[]; truncated: boolean }
export interface FieldAssertion { id: string; nodeId: string; fieldPath: string; valueHash: string; reference?: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; assertedAt: string }
export interface FieldAssertionView { id:string; nodeId:string; fieldPath:string; reference?:string; source:string; evidenceKind:EvidenceKind; certainty:Certainty; assertedAt:string; resolved:boolean; selected:boolean; resolutionId?:number }
export interface Impact { node: Node; relationPath: Relation[] }
export interface FieldConflict { nodeId:string; fieldPath:string; assertionIds:string[]; sources:string[] }
export interface SyncPreview { source: string; sourceHash: string; parserVersion?: string; addedNodes: string[]; updatedNodes: string[]; addedRelations: string[]; updatedRelations: string[]; missingNodes: string[]; missingRelations: string[]; conflicts: string[]; addedAssertions: string[]; updatedAssertions: string[]; missingAssertions: string[]; valueConflicts:FieldConflict[] }
export interface ManualMutation { kind:'createNode'|'updateNode'|'addRelation'|'deleteRelation'|'deleteNode'; node?:Node; relation?:Relation; id?:string }
export interface MutationPlan { expectedRevision:string; planHash:string; delta:{addedNodes:string[];updatedNodes:string[];deletedNodes:string[];addedRelations:string[];updatedRelations:string[];deletedRelations:string[];addedAssertions:string[];deletedAssertions:string[]} }
export interface MutationOperation { id:number; action:string; recordedAt:string; reversible:boolean }
export interface SourcePolicy { source:string; priority:number; allowedNodeTypes:NodeType[]; allowedPredicates:string[]; allowedFieldPaths:string[] }
export interface SyncSource { source: string; sourceHash: string; parserVersion: string; appliedAt: string }
export interface FieldResolution { id:number; nodeId:string; fieldPath:string; selectedAssertionIds:string[]; rationaleReference:string; recordedAt:string }
export interface ChangeEvent { id:number; runId?:number; source:string; entityType:string; entityId:string; action:'created'|'updated'|'stale'|'deleted'; recordedAt:string; snapshot?:unknown }
export interface SyncRun { id: number; source: string; sourceHash: string; parserVersion: string; appliedAt: string; undone:boolean; preview: { addedNodes: string[]; updatedNodes: string[]; missingNodes: string[]; addedRelations: string[]; updatedRelations: string[]; missingRelations: string[]; conflicts: string[] } }

async function readJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, { headers: { Accept: 'application/json' } })
  if (!response.ok) {
    const body = await response.json().catch(() => null) as { error?: { message?: string } } | null
    throw new Error(body?.error?.message ?? `Request failed (${response.status})`)
  }
  return response.json() as Promise<T>
}

async function writeJSON<T>(url: string, token: string, body?: unknown, method='POST'): Promise<T> {
  const headers: Record<string,string> = { Accept: 'application/json', 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const response = await fetch(url, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
  const result = await response.json().catch(() => null) as ({ error?: { message?: string } } & T) | null
  if (!response.ok) throw new Error(result?.error?.message ?? `Request failed (${response.status})`)
  return result as T
}

export const api = {
  nodes: () => readJSON<{ items: Node[] }>('/api/nodes'),
  relations: () => readJSON<{ items: Relation[] }>('/api/relations'),
  assertions: (id: string) => readJSON<{ items: FieldAssertionView[] }>(`/api/nodes/${encodeURIComponent(id)}/assertions`),
  provisionals: () => readJSON<{ items: Node[] }>('/api/provisionals'),
  syncSources: () => readJSON<{ items: SyncSource[] }>('/api/sync/sources'),
  previewMutation: (mutation:ManualMutation) => writeJSON<MutationPlan>('/api/operations/preview','',{mutation}),
  applyMutation: (mutation:ManualMutation,plan:MutationPlan,token:string) => writeJSON<{applied:boolean;plan:MutationPlan}>('/api/operations/apply',token,{mutation,expectedRevision:plan.expectedRevision,planHash:plan.planHash}),
  mutationOperations: () => readJSON<{ items:MutationOperation[] }>('/api/operations'),
  undoOperation: (id:number,token:string) => writeJSON<{undone:number}>(`/api/operations/${id}/undo`,token,{}),
  sourcePolicies: () => readJSON<{ items: SourcePolicy[] }>('/api/sync/policies'),
  saveSourcePolicy: (policy:SourcePolicy,token:string) => writeJSON<{saved:boolean;policy:SourcePolicy}>('/api/sync/policies',token,policy),
  syncHistory: (source: string) => readJSON<{ items: SyncRun[] }>(`/api/sync/sources/${encodeURIComponent(source)}/history`),
  entityHistory: (kind:string,id:string) => readJSON<{ items:ChangeEvent[] }>(`/api/entities/${encodeURIComponent(kind)}/${encodeURIComponent(id)}/history`),
  fieldResolutions: (id:string) => readJSON<{ items:FieldResolution[] }>(`/api/nodes/${encodeURIComponent(id)}/resolutions`),
  createFieldResolution: (body:Omit<FieldResolution,'id'|'recordedAt'>,token:string) => writeJSON<FieldResolution>('/api/assertions/resolve',token,body),
  createNode: (node:Node,relation:Relation,token:string) => writeJSON<{created:string}>('/api/nodes',token,{node,relation}),
  updateNode: (id:string,node:Node,token:string) => writeJSON<{updated:string}>(`/api/nodes/${encodeURIComponent(id)}`,token,node,'PUT'),
  addRelation: (relation:Relation,token:string) => writeJSON<{created:string}>('/api/relations',token,relation),
  deleteRelation: (id:string,token:string) => writeJSON<{deleted:string}>(`/api/relations/${encodeURIComponent(id)}`,token,undefined,'DELETE'),
  reviewProvisional: (id: string, targetType: NodeType, token: string) => writeJSON<{ status: string }>(`/api/provisionals/${encodeURIComponent(id)}/review`, token, { targetType }),
  rejectProvisional: (id: string, token: string) => writeJSON<{ status: string }>(`/api/provisionals/${encodeURIComponent(id)}/reject`, token, {}),
  previewBatch: (batch: unknown) => writeJSON<SyncPreview>('/api/sync/preview', '', batch),
  applyBatch: (batch: unknown, token: string) => writeJSON<{ applied: boolean; preview: SyncPreview }>('/api/sync/apply', token, batch),
  impact: () => readJSON<{ items: Impact[] }>('/api/impact'),
  path: (id: string) => readJSON<{ nodes: Node[]; relations: Relation[] }>(`/api/path/${encodeURIComponent(id)}`),
  explore: (id: string,direction:string,depth:number,predicates:string[]) => readJSON<Exploration>(`/api/explore/${encodeURIComponent(id)}?direction=${encodeURIComponent(direction)}&depth=${depth}&predicates=${encodeURIComponent(predicates.join(','))}`),
  health: () => readJSON<{ status: string }>('/healthz'),
}
