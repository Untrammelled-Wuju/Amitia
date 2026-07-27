export interface DesktopContributionDef {
  contributionId: string
  extensionId: string
  moduleId: string
  desktopType: string
  contractId: string
  contractVersion: number
  target: string
  order: { group?: string; priority: number; before?: string; after?: string }
  label: { default: string; translations?: Record<string, string> }
  description?: { default: string }
  iconResourceId?: string
  action: { actionType: string; targetId?: string; input?: unknown }
  shortcut?: { accelerator: string; scope?: string; global: boolean; repeatable?: boolean }
  permissionRequirements?: { permissionId: string; scope?: string; required: boolean }[]
  definitionHash: string
  version: string
}

export interface ResolvedContribution {
  definition: DesktopContributionDef
  status: string
  generation: number
  effectiveLabel: string
  effectiveIcon?: string
  resolvedAt: string
  conflictReason?: string
}

export interface DesktopSnapshot {
  generation: number
  contributions: ResolvedContribution[]
  hash: string
  createdAt: string
  menuTree: Record<string, ResolvedContribution[]>
  trayTree: Record<string, ResolvedContribution[]>
  shortcuts: ResolvedContribution[]
  conflicts: ConflictRecord[]
}

export interface ConflictRecord {
  conflictId: string
  type: string
  severity: string
  target: string
  existingContribId: string
  newContribId: string
  accelerator?: string
  resolved: boolean
  resolution?: string
  createdAt: string
}

export interface ExtensionUpdateMeta {
  extensionId: string
  version: string
  manifestVersion: number
  packageUrl: string
  packageSha256: string
  packageSize: number
  publisherId: string
  publisherKeyId?: string
  signature?: string
  minimumHostVersion?: string
  maximumHostVersion?: string
  supportedPlatforms?: string[]
  supportedArch?: string[]
  publishedAt: string
  releaseChannel?: string
}

export interface UpdateOperationInfo {
  operationId: string
  extensionId: string
  status: string
  version?: string
  createdAt: string
  updatedAt: string
  error?: string
}

export interface UpdateOperationStepInfo {
  stepId: string
  name: string
  status: string
  startedAt?: string
  endedAt?: string
  error?: string
}

export interface DesktopContract {
  contractId: string
  version: number
  desktopType: string
  allowedTargets: string[]
  status: string
  description: string
  maxItemsPerExt: number
  requiresPermission: boolean
}

export interface DesktopPermissionDef {
  id: string
  name: string
  category: string
  riskLevel: string
  description: string
}

export interface ResourceOwner {
  extensionId: string
  contributionId: string
  resourceType: string
  resourceHandle: string
  acquiredAt: string
}
