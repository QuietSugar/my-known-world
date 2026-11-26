export type NodeType = 'Self' | 'Hardware' | 'Machine' | 'SoftwareProject' | 'SoftwareInstance' | 'RunningPeriod' | 'AccountCredential' | 'Product' | 'Document' | 'Runbook' | 'WorkItem' | 'Network' | 'ProvisionalObject'
export type EvidenceKind = 'unknown' | 'manual' | 'document' | 'observed' | 'derived' | 'imported'
export type Certainty = 'unknown' | 'provisional' | 'confirmed' | 'disputed' | 'stale'
export interface HardwareDetails { category?: string; model?: string; location?: string; specifications?: Record<string,string>; status?: string; accessReference?: string; maintenanceReference?: string }
export interface ResourceClaim { kind: string; name: string; scope?: string; port?: number }
export interface MachineDetails { machineKind?: string; roles?: string[]; hostname?: string; os?: string; ips?: string[]; network?: string; status?: string; accessMethod?: string; accessPrerequisites?: string[]; allowedOperations?: string[]; forbiddenOperations?: string[]; responsibilityBoundary?: string; resources?: ResourceClaim[]; spec?: string; lastVerified?: string }
export interface SoftwareProjectDetails { purpose?: string; languages?: string[]; form?: string; remoteRepository?: string; upstream?: string; maintenanceState?: string; recommendedVersion?: string; successorId?: string }
export interface Endpoint { kind: string; address: string; port?: number }
export interface SoftwareInstanceDetails { productName?: string; domains?: string[]; ports?: number[]; deploymentPath?: string; deploymentMethod?: string; runtimeIdentity?: string; endpoints?: Endpoint[]; configurationReferences?: string[]; deploymentState?: string; operationalState?: string; maintenanceState?: string; notes?: string[]; verificationResult?: string; verifiedAt?: string }
export interface CredentialDetails { account?: string; credentialType?: string; purpose?: string; scope?: string; accessReference?: string; rotationAt?: string; status?: string; lastVerified?: string }
export interface ProductDetails { purpose?: string; commonConfigurationReference?: string; runbookReference?: string }
export interface DocumentDetails { role?: string; authoritativeReference?: string; effective?: boolean; lastVerified?: string; supersededBy?: string }
export interface RunbookDetails { scope?: string; trigger?: string; preconditions?: string[]; stepsReference?: string; stopConditionReference?: string; rollbackReference?: string; verificationMethod?: string; lastVerified?: string; executableStatus?: string; approvalRequired?: boolean }
export interface NetworkDetails { kind?:string; cidrs?:string[]; gatewayReference?:string; accessLocations?:string[]; vpnReference?:string; status?:string }
export interface WorkItemDetails { kind?: string; status?: string; rationaleReference?: string; decisionEvidenceReference?: string }
export interface Node { id: string; type: NodeType; name: string; lifecycle?: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; lastConfirmed?: string; modelVersion: number; candidateType?: NodeType; reviewStatus?: 'pending' | 'rejected'; reviewQuestions?: string[]; evidenceReference?: string; startedAt?: string; endedAt?: string; deployedVersion?: string; verificationAt?: string; verificationResult?: string; evidenceRef?: string; hardware?: HardwareDetails; machine?: MachineDetails; softwareProject?: SoftwareProjectDetails; softwareInstance?: SoftwareInstanceDetails; accountCredential?: CredentialDetails; product?: ProductDetails; document?: DocumentDetails; runbook?: RunbookDetails; workItem?: WorkItemDetails; network?:NetworkDetails }
export interface Relation { id: string; from: string; predicate: string; to: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; recordedAt: string }
export interface Exploration { centerId: string; direction: 'both'|'in'|'out'; depth: number; nodes: Node[]; relations: Relation[]; truncated: boolean }
export interface FieldAssertion { id: string; nodeId: string; fieldPath: string; valueHash: string; reference?: string; source: string; evidenceKind: EvidenceKind; certainty: Certainty; assertedAt: string }
export interface FieldAssertionView { id:string; nodeId:string; fieldPath:string; reference?:string; source:string; evidenceKind:EvidenceKind; certainty:Certainty; assertedAt:string; resolved:boolean; selected:boolean; resolutionId?:number }
export interface Impact { node: Node; relationPath: Relation[] }
export interface FieldConflict { nodeId:string; fieldPath:string; assertionIds:string[]; sources:string[] }
export interface SyncPreview { expectedRevision?:string; planHash?:string; source: string; sourceHash: string; parserVersion?: string; addedNodes: string[]; updatedNodes: string[]; addedRelations: string[]; updatedRelations: string[]; missingNodes: string[]; newlyMissingNodes: string[]; alreadyStaleNodes: string[]; missingRelations: string[]; newlyMissingRelations: string[]; alreadyStaleRelations: string[]; conflicts: string[]; addedAssertions: string[]; updatedAssertions: string[]; missingAssertions: string[]; newlyMissingAssertions: string[]; alreadyStaleAssertions: string[]; valueConflicts:FieldConflict[] }
export interface ManualMutation { kind:'createNode'|'updateNode'|'addRelation'|'updateRelation'|'deleteRelation'|'deleteNode'|'reviewProvisional'|'rejectProvisional'; node?:Node; relation?:Relation; id?:string; targetType?:NodeType; reviewEvidence?:{certainty?:Certainty;evidenceKind?:EvidenceKind;source?:string;reference?:string} }
export interface MutationPlan { expectedRevision:string; planHash:string; delta:{addedNodes:string[];updatedNodes:string[];deletedNodes:string[];addedRelations:string[];updatedRelations:string[];deletedRelations:string[];addedAssertions:string[];deletedAssertions:string[]} }
export interface SourcePolicy { source:string; priority:number; allowedNodeTypes:NodeType[]; allowedPredicates:string[]; allowedFieldPaths:string[] }
export interface SyncSource { source: string; sourceHash: string; parserVersion: string; appliedAt: string }
export interface FieldResolution { id:number; nodeId:string; fieldPath:string; selectedAssertionIds:string[]; rationaleReference:string; recordedAt:string }
export interface ChangeEvent { id:number; runId?:number; source:string; entityType:string; entityId:string; action:'created'|'updated'|'stale'|'deleted'; recordedAt:string; snapshot?:unknown }
export interface SyncRun { id: number; source: string; sourceHash: string; parserVersion: string; appliedAt: string; undone:boolean; preview: { addedNodes: string[]; updatedNodes: string[]; missingNodes: string[]; addedRelations: string[]; updatedRelations: string[]; missingRelations: string[]; conflicts: string[] } }
export interface ProjectWorkspaceData {
  project: Node
  instances: Node[]
  machines: Node[]
  credentials: Node[]
  documents: Node[]
  workItems: Node[]
  relations: Relation[]
}

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

export interface PagedResult<T> {
  items: T[]
  totalCount: number
  count: number
  hasMore: boolean
  nextCursor?: string
}

export interface ProjectViewTemplate {
  id: string
  projectId: string
  name: string
  description?: string
  layoutType: 'cards' | 'board' | 'table' | 'list'
  config: {
    groupBy?: 'environment' | 'machine' | 'none'
    density?: 'compact' | 'comfortable'
    visibleSections?: ('instances' | 'machines' | 'credentials' | 'documents')[]
    showPort?: boolean
    showHost?: boolean
    showAccount?: boolean
    highlightSecret?: boolean
    filterEnv?: string
    [key: string]: any
  }
  isDefault?: boolean
  builtin?: boolean
  createdAt?: string
  updatedAt?: string
}

export interface ExportStaticResult {
  status: string
  projectId: string
  projectSlug: string
  filePath: string
  historyPath: string
  url: string
  historyUrl: string
  credentialCount: number
  decryptedCount: number
  generatedAt: string
}

export function getStoredToken(): string {
  return sessionStorage.getItem('mkw-write-token') || localStorage.getItem('mkw-write-token') || ''
}

export const api = {
  nodes: () => readJSON<PagedResult<Node>>('/api/nodes'),
  node: (id: string) => readJSON<Node>(`/api/nodes/${encodeURIComponent(id)}`),
  relations: () => readJSON<PagedResult<Relation>>('/api/relations'),
  assertions: (id: string) => readJSON<{ items: FieldAssertionView[] }>(`/api/nodes/${encodeURIComponent(id)}/assertions`),
  provisionals: () => readJSON<{ items: Node[] }>('/api/provisionals'),
  syncSources: () => readJSON<{ items: SyncSource[] }>('/api/sync/sources'),
  previewMutation: (mutation:ManualMutation) => writeJSON<MutationPlan>('/api/operations/preview','',{mutation}),
  applyMutation: (mutation:ManualMutation,plan:MutationPlan,token:string) => writeJSON<{applied:boolean;plan:MutationPlan}>('/api/operations/apply',token,{mutation,expectedRevision:plan.expectedRevision,planHash:plan.planHash}),
  sourcePolicies: () => readJSON<{ items: SourcePolicy[] }>('/api/sync/policies'),
  previewSourcePolicy: (policy:SourcePolicy) => writeJSON<{plan:MutationPlan;previousPolicy:SourcePolicy|null;policy:SourcePolicy}>('/api/sync/policies/preview','',{policy}),
  saveSourcePolicy: (policy:SourcePolicy,plan:MutationPlan,token:string) => writeJSON<{saved:boolean;policy:SourcePolicy}>('/api/sync/policies/apply',token,{policy,expectedRevision:plan.expectedRevision,planHash:plan.planHash}),
  syncHistory: (source: string) => readJSON<{ items: SyncRun[] }>(`/api/sync/sources/${encodeURIComponent(source)}/history`),
  entityHistory: (kind:string,id:string) => readJSON<{ items:ChangeEvent[] }>(`/api/entities/${encodeURIComponent(kind)}/${encodeURIComponent(id)}/history`),
  fieldResolutions: (id:string) => readJSON<{ items:FieldResolution[] }>(`/api/nodes/${encodeURIComponent(id)}/resolutions`),
  previewFieldResolution: (resolution:Omit<FieldResolution,'id'|'recordedAt'>) => writeJSON<{plan:MutationPlan;resolution:Omit<FieldResolution,'id'|'recordedAt'>}>('/api/assertions/resolve/preview','',{resolution}),
  createFieldResolution: (resolution:Omit<FieldResolution,'id'|'recordedAt'>,plan:MutationPlan,token:string) => writeJSON<FieldResolution>('/api/assertions/resolve/apply',token,{resolution,expectedRevision:plan.expectedRevision,planHash:plan.planHash}),
  previewBatch: (batch: unknown) => writeJSON<SyncPreview>('/api/sync/preview', '', batch),
  applyBatch: (batch: unknown, preview: SyncPreview, token: string) => writeJSON<{ applied: boolean; preview: SyncPreview }>('/api/sync/apply', token, { batch, expectedRevision: preview.expectedRevision, planHash: preview.planHash }),
  impact: () => readJSON<{ items: Impact[] }>('/api/impact'),
  path: (id: string) => readJSON<{ nodes: Node[]; relations: Relation[] }>(`/api/path/${encodeURIComponent(id)}`),
  explore: (id: string,direction:string,depth:number,predicates:string[]) => readJSON<Exploration>(`/api/explore/${encodeURIComponent(id)}?direction=${encodeURIComponent(direction)}&depth=${depth}&predicates=${encodeURIComponent(predicates.join(','))}`),
  projectWorkspace: (id: string) => readJSON<ProjectWorkspaceData>(`/api/workspace/projects/${encodeURIComponent(id)}`),
  credentialSecret: (ref: string) => readJSON<{ status: string; ref: string; password?: string; opaqueId?: string }>(`/api/credentials/secret?ref=${encodeURIComponent(ref)}`),
  uploadCredentials: (refs: string[], upstream: 'bitwarden' | 'file' = 'file') =>
    writeJSON<{ status: string; report: { total: number; succeeded: number; failed: number; upstream: string; results: { ref: string; newUUID?: string; accessReference?: string; success: boolean; error?: string }[] }; message: string }>(
      '/api/credentials/upload-to-upstream',
      '',
      { refs, upstream }
    ),
  credentialStatus: () => readJSON<{ status: string; managed: Record<string, string> }>('/api/credentials/status'),
  syncCredentials: () => writeJSON<{ status: string; msg: string }>('/api/credentials/sync', '', {}),
  health: () => readJSON<{ status: string }>('/healthz'),
  getProjectTemplates: (projectId: string) =>
    readJSON<ProjectViewTemplate[]>(`/api/workspace/projects/${encodeURIComponent(projectId)}/templates`),
  saveProjectTemplate: (projectId: string, template: Partial<ProjectViewTemplate>) =>
    writeJSON<ProjectViewTemplate>(`/api/workspace/projects/${encodeURIComponent(projectId)}/templates`, getStoredToken(), template, 'POST'),
  updateProjectTemplate: (projectId: string, templateId: string, template: Partial<ProjectViewTemplate>) =>
    writeJSON<ProjectViewTemplate>(`/api/workspace/projects/${encodeURIComponent(projectId)}/templates/${encodeURIComponent(templateId)}`, getStoredToken(), template, 'PUT'),
  deleteProjectTemplate: (projectId: string, templateId: string) =>
    writeJSON<{ status: string; id: string }>(`/api/workspace/projects/${encodeURIComponent(projectId)}/templates/${encodeURIComponent(templateId)}`, getStoredToken(), undefined, 'DELETE'),
  exportStaticPage: (projectId: string) =>
    writeJSON<ExportStaticResult>(`/api/workspace/projects/${encodeURIComponent(projectId)}/export-static`, '', {}),
}
