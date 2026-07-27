export interface BackendLocalizedText {
  default: string;
  i18n?: Record<string, string>;
}

export interface BackendUISlotReference {
  slot_id: string;
  contract_version: number;
}

export interface BackendUIDisplayMetadata {
  title: BackendLocalizedText;
  description?: BackendLocalizedText;
  icon?: string;
  badge?: unknown;
  category?: string;
  keywords?: string[];
}

export interface BackendUIEntryDefinition {
  type: string;
  path: string;
  schema_path?: string;
  runtime_id?: string;
  entry_name?: string;
  content_hash: string;
}

export interface BackendUIOrderingRule {
  priority: number;
  before?: string[];
  after?: string[];
  category?: string;
  sort_key?: string;
}

export interface BackendUISandboxPolicy {
  type: string;
  csp?: string;
  allowed_origins?: string[];
  enable_scripts?: boolean;
  allow_forms?: boolean;
  allow_popups?: boolean;
  max_memory_mb?: number;
}

export interface BackendUIActionDefinition {
  action_id: string;
  title?: BackendLocalizedText;
  icon?: string;
  risk_level?: string;
}

export interface BackendPermissionRequirement {
  permission: string;
  required: boolean;
  scope?: string;
}

export interface BackendUIContributionDefinition {
  contribution_id: string;
  extension_id: string;
  module_id: string;
  kind: string;
  slot: BackendUISlotReference;
  contract_version: number;
  display: BackendUIDisplayMetadata;
  entry: BackendUIEntryDefinition;
  visibility?: unknown;
  data_contract?: unknown;
  actions?: BackendUIActionDefinition[];
  permissions?: BackendPermissionRequirement[];
  scope_rule?: unknown;
  ordering?: BackendUIOrderingRule;
  conflict_policy?: unknown;
  sandbox: BackendUISandboxPolicy;
  lifecycle?: unknown;
  integrity?: unknown;
}

export interface BackendSlotSnapshotEntry {
  slotId: string;
  contributions: BackendUIContributionDefinition[];
}

export interface BackendUIContributionSnapshot {
  slots: BackendSlotSnapshotEntry[];
  timestamp: string;
}

export interface BackendBridgeSessionResponse {
  sessionId: string;
  contributionId: string;
  extensionId: string;
  moduleId: string;
  generation: number;
  origin: string;
  contractVersion: number;
  grantedScopes: string[];
  grantedPerms: string[];
  createdAt: string;
  expiresAt: string;
}

export interface BackendBridgeResponse {
  ok: boolean;
  error?: string;
  code?: string;
  result?: unknown;
  payload?: unknown;
}

export interface BackendOpenPageResult {
  sessionId: string;
  state: string;
  definition?: unknown;
  missingPermissions?: string[];
  reason?: string;
}

export interface BackendPageSessionStatus {
  sessionId: string;
  state: string;
  definition?: unknown;
  missingPermissions: string[];
  reason: string;
}
