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

export interface BackendUIActionTarget {
  type: string;
  command?: string;
  tool_id?: string;
  workflow_id?: string;
  workflow_action?: string;
  route_id?: string;
  dialog_id?: string;
  resource?: string;
}

export interface BackendUIActionDefinition {
  action_id: string;
  title?: BackendLocalizedText;
  icon?: string;
  input_schema?: unknown;
  target?: BackendUIActionTarget;
  risk_level?: string;
  confirmation?: string;
}

export interface BackendPermissionRequirement {
  permission: string;
  required: boolean;
  scope?: string;
}

export interface BackendContributionIntegrity {
  definition_hash: string;
  entry_hash?: string;
  schema_hash?: string;
  generation: number;
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
  integrity?: BackendContributionIntegrity;
}

export interface BackendSlotSnapshotEntry {
  slotId: string;
  contributions: BackendUIContributionDefinition[];
}

export interface BackendUIProviderEntry {
  contributionId?: string;
  type: string;
  path?: string;
  schemaPath?: string;
  exportName?: string;
  contentHash?: string;
}

export interface BackendUIProviderDefinition {
  providerId: string;
  extensionId: string;
  moduleId?: string;
  capability: string;
  mode: string;
  priority?: number;
  platforms?: string[];
  entries: Record<string, BackendUIProviderEntry>;
  fallbackProviderId?: string;
  trustLevel?: string;
  permissions?: string[];
  generation?: number;
  enabled: boolean;
  builtin?: boolean;
  metadata?: Record<string, unknown>;
}

export interface BackendUIProfile {
  profileId: string;
  name: string;
  selections: Record<string, string>;
  updatedAt?: number;
}

export interface BackendUIContributionSnapshot {
  slots: BackendSlotSnapshotEntry[];
  contributions?: BackendUIContributionDefinition[];
  timestamp: string;
  providers?: BackendUIProviderDefinition[];
  profile?: BackendUIProfile;
  resolved?: Record<string, BackendUIProviderDefinition>;
  providerVersion?: number;
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
