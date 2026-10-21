import type { TaskAuthority } from "../kernel/tasks/authority";

export interface LocalExtensionPackage {
  id: string;
  name: string;
  version: string;
  publisher: string;
  moduleCount: number;
  installedAt: string;
}

export type SkillTrigger = string;

export type AgentSkillCompatibilityStatus =
  | "compatible"
  | "compatible_with_warnings"
  | "partially_compatible"
  | "blocked";
export type AgentSkillScope = "global" | "space" | "character" | "conversation";
export type AgentSkillResourceKind =
  | "skill"
  | "reference"
  | "asset"
  | "script"
  | "agent_metadata"
  | "other";
export interface AgentSkillResource {
  path: string;
  kind: AgentSkillResourceKind;
  mimeType: string;
  size: number;
  sha256: string;
  textReadable: boolean;
  executable: boolean;
  supported: boolean;
}
export interface AgentSkillToolMapping {
  sourceTool: string;
  targetSkillId?: string;
  status: "mapped" | "partially_mapped" | "unsupported" | "blocked";
  reason: string;
}
export interface AgentSkillWarning {
  code: string;
  message: string;
  path?: string;
}
export interface AgentSkillMCPDependency {
  id: string;
  description?: string;
  required: boolean;
  transport: "streamable_http" | "stdio" | "";
  url?: string;
  command?: string;
  args?: string[];
  authType: string;
  toolAllowlist?: string[];
  defaultScope: "global" | "space" | "character" | "conversation";
  autoConfigure?: boolean;
}
export interface AgentSkillCompatibilityReport {
  status: AgentSkillCompatibilityStatus;
  toolMappings: AgentSkillToolMapping[];
  requiredScripts: string[];
  missingFiles: string[];
  unsupported: string[];
  warnings: AgentSkillWarning[];
  errors: AgentSkillWarning[];
}
export interface AgentSkillDefinition {
  id: string;
  contributionId?: string;
  extensionId: string;
  name: string;
  description: string;
  license?: string;
  compatibility?: string;
  metadata: Record<string, string>;
  allowedTools?: string;
  displayName?: string;
  shortDescription?: string;
  defaultPrompt?: string;
  iconSmall?: string;
  iconLarge?: string;
  brandColor?: string;
  source: "bundled" | "local-directory" | "local-zip" | "local-import" | "agent_skill" | "workshop";
  scope: AgentSkillScope;
  scopeId?: string;
  artifactId?: string;
  contentHash: string;
  body?: string;
  rawSkillMd?: string;
  resources: AgentSkillResource[];
  toolMappings: AgentSkillToolMapping[];
  mcpDependencies: AgentSkillMCPDependency[];
  compatibilityStatus: AgentSkillCompatibilityStatus;
  warnings: AgentSkillWarning[];
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}
export interface AgentSkillPreview {
  previewId: string;
  definition: AgentSkillDefinition;
  compatibilityReport: AgentSkillCompatibilityReport;
  files: AgentSkillResource[];
  expiresAt: string;
  requiredConfirmations?: string[];
  riskFlags?: string[];
}
export interface AgentSkillPage {
  items: AgentSkillDefinition[];
  total: number;
  page: number;
  pageSize: number;
}
export interface AgentSkillActivation {
  activationId: string;
  extensionId: string;
  triggerType: string;
  explicit: boolean;
  status: string;
  loadedTokens: number;
  resourceReads: number;
  resourcePaths: string[];
  traceId: string;
  errorCode?: string;
  createdAt: string;
}
export interface AgentSkillDetail {
  definition: AgentSkillDefinition;
  compatibilityReport: AgentSkillCompatibilityReport;
  activations: AgentSkillActivation[];
}

export type PackageFormat =
  | "amitiax"
  | "agentskills-zip"
  | "agentskills-directory";
export interface PackageRisk {
  code: string;
  severity: "low" | "medium" | "high";
  message: string;
}
export interface PackageFile {
  path: string;
  size: number;
  kind: string;
}
export interface PackageDependency {
  id: string;
  versionConstraint?: string;
  required: boolean;
  installed: boolean;
  version?: string;
}
export interface PackageUninstallPreview {
  extensionId: string;
  currentVersion: string;
  enabled: boolean;
  dependents: PackageDependency[];
  scheduleCount: number;
  grants: string[];
  configPresent: boolean;
  historicalRuns: number;
  artifactArchived: boolean;
  cleanup: string[];
  preserved: string[];
}
export interface PackagePermission {
  name: string;
  reason?: string;
  scope?: string;
  required: boolean;
}
export interface PackageImportPreview {
  sessionId: string;
  format: PackageFormat;
  skillType: "workflow" | "instructions";
  id: string;
  name: string;
  version: string;
  description: string;
  license: string;
  source: string;
  scopeType: "global" | "character";
  scopeId: string;
  packageHash: string;
  checksum: { valid: boolean; packageHash: string };
  signature: {
    status: "unsigned" | "valid-untrusted" | "valid-trusted" | "invalid";
    fingerprint?: string;
    algorithm?: string;
    displayName?: string;
  };
  compatible: boolean;
  compatibility: string;
  capabilities: string[];
  newPermissions?: PackagePermission[];
  highRiskCapabilities: string[];
  capabilityConfirmations: string[];
  triggers: SkillTrigger[];
  dependencies: PackageDependency[];
  workflowSteps?: string[];
  scripts: number;
  scriptsRequired: boolean;
  references: number;
  assets: number;
  files: PackageFile[];
  totalSize: number;
  fileCount: number;
  testStatus: string;
  testReport?: PackageDryRunReport;
  risks: PackageRisk[];
  warnings: string[];
  errors: string[];
  conflict: string;
  availableActions: string[];
  managementTarget?: "extension_center" | "game_center" | "pet_center" | string;
  contributionKinds?: string[];
  currentVersion?: string;
  rollbackVersion?: string;
  upgradeDiff?: Record<string, unknown>;
  expiresAt: string;
}
export interface PackageDryRunReport {
  status: string;
  caseCount: number;
  passedCount: number;
  failedCount: number;
  durationMs: number;
  capabilities: string[];
  sideEffects: Array<{ type: string; targetId?: string; confirmed: boolean }>;
  cases: PackageDryRunCaseReport[];
}
export interface PackageDryRunCaseReport {
  id: string;
  name: string;
  mode: string;
  status: string;
  durationMs: number;
  steps: Array<{
    stepId: string;
    type: string;
    status: string;
    mocked: boolean;
    durationMs: number;
    error?: { code: string; detail?: string };
  }>;
  assertions: Array<{ type: string; passed: boolean; message?: string }>;
  output?: unknown;
  error?: { code: string; message: string; detail?: string };
}
export interface PackageOperationResult {
  operationId: string;
  traceId: string;
  operation: string;
  extensionId: string;
  version: string;
  enabled: boolean;
  status: string;
}
export interface PackageOperation {
  id: string;
  operation: string;
  extensionId: string;
  previousVersion?: string;
  targetVersion?: string;
  source: string;
  packageHash: string;
  signatureStatus: string;
  signerFingerprint?: string;
  scopeType: string;
  scopeId: string;
  status: string;
  errorCode?: string;
  traceId: string;
  createdAt: string;
  completedAt?: string;
  currentStep?: string;
  steps?: PackageOperationStep[];
}
export interface PackageOperationStep {
  name: string;
  order: number;
  status: string;
  attemptCount: number;
  resultJson?: string;
  errorCode?: string;
  startedAt?: string;
  completedAt?: string;
}
export interface ExtensionUpdateMetadata {
  extensionId: string;
  version: string;
  manifestVersion: number;
  packageUrl: string;
  packageSha256: string;
  packageSha512?: string;
  packageSize: number;
  publisherId: string;
  publisherKeyId: string;
  signature?: string;
  minimumHostVersion?: string;
  maximumHostVersion?: string;
  supportedPlatforms?: string[];
  supportedArch?: string[];
  publishedAt: string;
  releaseChannel?: string;
  rollbackPolicy?: string;
  signedIndex: boolean;
}
export interface ExtensionUpdateCheck {
  available: boolean;
  extensionId: string;
  currentVersion: string;
  update?: ExtensionUpdateMetadata;
  items: ExtensionUpdateMetadata[];
  total: number;
}
export interface ExtensionUpdateDownload {
  downloadId: string;
  url: string;
  expectedHash: string;
  expectedSize: number;
  downloadedSize: number;
  status: string;
  startedAt: string;
  completedAt?: string;
  error?: string;
}
export interface PackageRollbackPreview {
  extensionId: string;
  currentVersion: string;
  targetVersion: string;
  currentGeneration: number;
  rollbackPointId: string;
  snapshotHash: string;
  retentionState: string;
  retentionUntil: string;
  manualRequired: boolean;
  manualReason?: string;
  dependents: string[];
  installable: boolean;
  previewSessionId: string;
  requiredConfirmations: string[];
}
export interface PackageRollbackConfirmation {
  confirmationToken: string;
  expiresAt: string;
  previewSessionId: string;
}
export interface PackageVersion {
  version: string;
  manifest: Record<string, unknown>;
  artifactId: string;
  artifactHash: string;
  packageHash: string;
  source: string;
  signatureStatus: string;
  signerFingerprint?: string;
  compatibilityStatus: string;
  capabilities: string[];
  installedAt: string;
  installedBy: string;
  active: boolean;
  validationStatus: string;
  testStatus: string;
  artifactStatus: string;
  activationStatus: string;
  operationId?: string;
  failureCode?: string;
  archived: boolean;
  versionState?: string;
  generationId?: string;
  installedPath?: string;
  installedTreeHash?: string;
  retainedUntil?: string;
}
export interface PackageSigner {
  fingerprint: string;
  algorithm: string;
  displayName: string;
  trusted: boolean;
  trustedAt?: string;
  revokedAt?: string;
}
export interface ExportedPackage {
  exportId: string;
  fileName: string;
  mime: string;
  size: number;
  hash: string;
  version: string;
  format: string;
  testsIncluded: boolean;
  readmeIncluded: boolean;
  sbomIncluded: boolean;
  scriptsIncluded: boolean;
  secretScan: string;
  signatureStatus: string;
  expiresAt: string;
}

export type TaskRunStatus =
  | "created"
  | "queued"
  | "starting"
  | "running"
  | "checkpointing"
  | "pausing"
  | "paused"
  | "resuming"
  | "cancelling"
  | "cancelled"
  | "succeeded"
  | "failed"
  | "timed_out"
  | "recovery_required"
  | "manual_intervention";

export interface TaskProgress {
  taskRunId: string;
  sequence: number;
  current: number;
  total: number;
  percentage: number;
  stage: string;
  message: string;
  updatedAt: string;
}

export interface TaskCheckpoint {
  checkpointId: string;
  sequence: number;
  status: string;
  stage?: string;
  message?: string;
  payload?: unknown;
  createdAt: string;
}

export type TaskResultType = "inline_json" | "artifact";

export interface TaskResult {
  taskRunId: string;
  resultType: TaskResultType;
  resultJson?: unknown;
  artifactId?: string;
  resultHash?: string;
  artifactName?: string;
  artifactSize?: number;
  artifactMime?: string;
}

export interface TaskRun extends TaskAuthority {
  taskRunId: string;
  operationId?: string;
  invocationId?: string;
  taskDefinitionId?: string;
  extensionId: string;
  moduleId?: string;
  status: TaskRunStatus;
  priority: number;
  inputHash?: string;
  attempt: number;
  maxAttempts: number;
  createdAt: string;
  queuedAt?: string;
  startedAt?: string;
  finishedAt?: string;
  deadlineAt?: string;
  errorCode?: string;
  errorMessage?: string;
  progress?: TaskProgress;
  checkpoint?: TaskCheckpoint;
  result?: TaskResult;
}

export interface TaskRunPage {
  items: TaskRun[];
  total: number;
}

export interface TaskListFilters {
  extensionId?: string;
  status?: TaskRunStatus | "";
  page?: number;
  pageSize?: number;
}
