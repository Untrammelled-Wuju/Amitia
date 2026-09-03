// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// API
export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data?: T;
  detail?: string;
}

// Error codes
export const ERR = {
  INTERNAL: 10000,
  SERVICE_UNAVAILABLE: 10001,
  TIMEOUT: 10002,
  DB_ERROR: 10003,
  VALIDATION: 10004,
  NOT_FOUND: 10005,
  BAD_REQUEST: 10006,
  CONFLICT: 10007,
  RATE_LIMITED: 10008,
  UNAUTHORIZED: 20000,
  INVALID_CREDENTIALS: 20001,
  TOKEN_EXPIRED: 20002,
  TOKEN_INVALID: 20003,
  FORBIDDEN: 20004,
  AUTH_SETUP_REQUIRED: 20005,
  AUTH_ALREADY_SETUP: 20006,
  CONFIG_ERROR: 30000,
  CONFIG_NOT_FOUND: 30001,
  CONFIG_INVALID: 30002,
  CONFIG_SAVE_FAILED: 30003,
  MODEL_NOT_CONFIGURED: 30004,
  MODEL_ERROR: 40000,
  MODEL_CONNECTION_FAILED: 40001,
  MODEL_TIMEOUT: 40002,
  MODEL_UNAUTHORIZED: 40003,
  MODEL_NOT_FOUND: 40004,
  MODEL_RATE_LIMITED: 40005,
  MODEL_INVALID_RESPONSE: 40006,
  MODEL_BASE_URL_UNREACHABLE: 40007,
  MODEL_NETWORK_ERROR: 40008,
  MODEL_CONFIG_INCOMPLETE: 40009,
  MODEL_UNSUPPORTED_TYPE: 40010,
  MODEL_TEST_FAILED: 40011,
  WECHAT_ERROR: 50000,
  WECHAT_NOT_CONNECTED: 50001,
  WECHAT_ACCOUNT_NOT_FOUND: 50002,
  WECHAT_SEND_FAILED: 50003,
  WECHAT_WEBHOOK_INVALID: 50004,
  AGENT_ERROR: 60000,
  AGENT_MODEL_FAILED: 60001,
  AGENT_NO_CHARACTER: 60002,
  AGENT_CONV_NOT_FOUND: 60003,
  AGENT_SAFETY_BLOCKED: 60004,
  AGENT_CHANNEL_UNSUPPORTED: 60005,
  AGENT_EMPTY_MESSAGE: 60006,
  IMPORT_ERROR: 70000,
  IMPORT_PARSE_FAILED: 70001,
  IMPORT_BATCH_NOT_FOUND: 70002,
  IMPORT_FILE_TOO_LARGE: 70003,
  IMPORT_FORMAT_UNSUPPORTED: 70004,
  IMPORT_SENSITIVE_CONTENT: 70005,
  IMPORT_MEMORY_FAILED: 70006,
  IMPORT_SUMMARY_FAILED: 70007,
  STORAGE_ERROR: 80000,
  BACKUP_FAILED: 80001,
  BACKUP_NOT_FOUND: 80002,
  RESTORE_FAILED: 80003,
  EXPORT_FAILED: 80004,
  IMPORT_FAILED: 80005,
  DATA_DIR_NOT_WRITABLE: 80006,
  DISK_SPACE_INSUFFICIENT: 80007,
} as const;

// Message
export interface Message {
  id: string;
  conversationId: string;
  role: "user" | "assistant" | "system";
  content: string;
  imageUrl?: string;
  videoUrl?: string;
  audioUrl?: string;
  msgType?: string;
  emoteId?: string;
  altText?: string;
  isAnimated?: boolean | number;
  width?: number;
  height?: number;
  originalAssetReference?: string;
  fallbackAssetReference?: string;
  responseGroupId?: string;
  deliverySequence?: number;
  sequence?: number;
  emoteDecisionStatus?: string;
  tokens?: number;
  source: string;
  importedItemId?: string | null;
  replyToMessageId?: string;
  replyToRole?: string;
  replyToExcerpt?: string;
  createdAt: string;
}

// Conversation
export interface Conversation {
  id: string;
  characterId: string;
  title: string;
  channel: string;
  source: string;
  peerId: string;
  importBatchId?: string | null;
  messageCount: number;
  createdAt: string;
  updatedAt: string;
}

// Character
export interface TtsConfig {
  id: number;
  name: string;
  apiKey: string;
  resourceId: string;
  voiceType: string;
  emotion: string;
  speed: number;
  pitch: number;
  volume: number;
  isActive: number;
  isCustom: number;
  customVoiceId: string;
  lastTestResult?: string;
  hasApiKey: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface VoicePreset {
  name: string;
  label: string;
  gender: string;
  language: string;
}

export interface Character {
  id: string;
  name: string;
  avatar: string;
  identity: string;
  personality: string;
  speakingStyle: string;
  relationshipStyle: string;
  characterBase: string;
  boundaryRules: string;
  personalitySliders: string;
  description: string;
  basePrompt: string;
  generatedPrompt: string;
  isDefault: number;
  status: string;
  personalityConfig: string;
  chatStyleConfig: string;
  sceneRules: string;
  isActive: number;
  sortOrder: number;
  conversationId: string;
  createdAt: string;
  updatedAt: string;
  gender: string;
  genderLabel?: string | null;
  pronoun: string;
  selfReference: string;
  userAddressingStyle?: string | null;
  voiceConfigId?: string;
  voiceType?: string;
  voiceSpeed?: number;
  voicePitch?: number;
  voiceVolume?: number;
  customVoiceId?: string;
}

// Import
export interface ImportResult {
  batchId: string;
  conversationId: string;
  messageCount: number;
  title: string;
}

// Memory
export interface Memory {
  id: string;
  characterId: string;
  memoryType: string;
  memorySubtype?: string;
  key: string;
  value: string;
  importance: number;
  confidence: number;
  source: string;
  scope: string;
  scopeType?: string;
  verifiedStatus: string;
  useCount: number;
  lastUsedAt?: string | null;
  retentionLevel?: number;
  memoryStrength?: number;
  strengthUpdatedAt?: string | null;
  lastReinforcedAt?: string | null;
  reinforceCount?: number;
  retrievedCount?: number;
  injectedCount?: number;
  decayState?: "active" | "fading" | "archived" | string;
  pinned?: boolean;
  archivedAt?: string | null;
  supersededBy?: string;
  expiresAt?: string | null;
  createdAt: string;
  updatedAt: string;
}

// LLM Config
export interface LLMConfig {
  baseUrl: string;
  apiKey: string;
  modelName: string;
  temperature: number;
  maxTokens: number;
  topP: number;
}
export interface RuntimeModeResponse {
  deployMode: string;
  host: string;
  port: number;
  web: { enabled: boolean; publicBaseUrl: string; requireAuth: boolean };
  bridge: { enabled: boolean; mode: string; host: string; port: number };
  storage: { dataDir: string };
}

export interface RuntimeModeValidationResult {
  valid: boolean;
  errors: string[];
  warnings?: string[];
  checks?: Array<{
    name: string;
    level: "info" | "warn" | "error";
    passed: boolean;
    message: string;
    suggestion?: string;
  }>;
}

export type DeployMode = "desktop-local" | "cloud-web";

export interface RuntimeDebugSnapshot {
  meta: {
    generatedAt: string;
    degraded: boolean;
  };
  summary: {
    activeInteractions: number;
    queuedTasks: number;
    reconciliationIssues: number;
  };
  interactions: Array<{
    scope: string;
    status: string;
    priority: string;
    path: string;
    stateVersion: number;
    deadlineAt?: string | null;
    cancelReason?: string | null;
  }>;
  budgets: Array<{
    scope: string;
    path: string;
    callsUsed: number;
    callsLimit: number;
    tokensUsed: number;
    tokensLimit: number;
    queueMs: number;
  }>;
  queues: Array<{
    name: string;
    priority: string;
    depth: number;
    oldestAgeMs: number;
    status: string;
  }>;
  deliveries: Array<{
    channel: string;
    leaseState: string;
    deliveryState: string;
    attempt: number;
    updatedAt?: string | null;
  }>;
  tools: Array<{
    tool: string;
    status: string;
    idempotencyKey: string;
    updatedAt?: string | null;
  }>;
  circuits: Array<{
    dependency: string;
    state: string;
    failures: number;
    openedAt?: string | null;
  }>;
  reconciliation: Array<{
    category: string;
    severity: "low" | "medium" | "high" | "ok";
    count: number;
    strategy: string;
    updatedAt?: string | null;
  }>;
  behaviorPlan?: {
    intention: string;
    strategy: string;
    mustInclude: string[];
    mayInclude: string[];
    mustAvoid: string[];
    questionPolicy: string;
    advicePolicy: string;
    deliveryPolicy: string;
    stateVersion: number;
    winnerCandidate: string;
    rejectedCandidates: string[];
  };
  expressionPlan?: {
    sentenceCount: number;
    maxLength: number;
    directness: number;
    warmth: number;
    emotionDisplay: string;
    useQuestion: boolean;
    voiceParams?: string | null;
    avoidTopics: string[];
  };
  hardConstraintFilters?: Array<{
    ruleId: string;
    candidateKey: string;
    reason: string;
    severity: "block" | "override";
  }>;
  softPreferenceScores?: Array<{
    dimension: string;
    rawScore: number;
    normalizedWeight: number;
    contribution: number;
  }>;
  copingStrategy?: {
    selected: string;
    alternatives: string[];
    selectionReason: string;
  };
  emotionExpression?: {
    displayMode: "show" | "suppress" | "reframe";
    internalIntensity: number;
    displayIntensity: number;
    overrideReason: string;
  };

  traces?: Array<{
    index: number;
    stage: string;
    scope: string;
    interactionId: string;
    step: string;
    durationMs: number;
    status: string;
    detail: string;
    startedAt: string;
  }>;
  metrics?: {
    totalInteractions: number;
    activeInteractions: number;
    avgLatencyMs: number;
    p95LatencyMs: number;
    cancelRate: number;
    supersedeRate: number;
    deliverySuccessRate: number;
    toolUnknownRate: number;
    circuitBreakerRate: number;
    queueBackpressureRate: number;
    reconciliationOpenIssues: number;
    totalModelCalls: number;
    cacheHitRate: number;
    collectedAt: string;
    version: string;
  };
}

export interface RuntimeTraceFrame {
  index: number;
  stage: string;
  scope: string;
  interactionId: string;
  step: string;
  durationMs: number;
  status: string;
  detail: string;
  startedAt: string;
  metadata: Record<string, string>;
}

export interface RuntimeReconciliationDetail {
  category: string;
  severity: "low" | "medium" | "high" | "ok";
  count: number;
  strategy: string;
  updatedAt: string;
  samples: Array<{
    id: string;
    scope: string;
    description: string;
    autoFix: boolean;
  }>;
}

export interface RuntimeMetrics {
  totalInteractions: number;
  activeInteractions: number;
  avgLatencyMs: number;
  p95LatencyMs: number;
  cancelRate: number;
  supersedeRate: number;
  deliverySuccessRate: number;
  toolUnknownRate: number;
  circuitBreakerRate: number;
  queueBackpressureRate: number;
  reconciliationOpenIssues: number;
  totalModelCalls: number;
  cacheHitRate: number;
  collectedAt: string;
  version: string;
  traces?: Array<{
    index: number;
    stage: string;
    scope: string;
    interactionId: string;
    step: string;
    durationMs: number;
    status: string;
    detail: string;
    startedAt: string;
  }>;
  metrics?: {
    totalInteractions: number;
    activeInteractions: number;
    avgLatencyMs: number;
    p95LatencyMs: number;
    cancelRate: number;
    supersedeRate: number;
    deliverySuccessRate: number;
    toolUnknownRate: number;
    circuitBreakerRate: number;
    queueBackpressureRate: number;
    reconciliationOpenIssues: number;
    totalModelCalls: number;
    cacheHitRate: number;
    collectedAt: string;
    version: string;
  };
}

export interface PsycheStateSnapshot {
  emotion: {
    positive: number;
    negative: number;
    arousal: number;
    dominance: number;
  };
  mood: { valence: number; tension: number; pad: string };
  stress: number;
  energy: number;
  needs: Record<string, number>;
  beliefs: Array<{
    key: string;
    value: string;
    confidence: number;
    conflicted: boolean;
  }>;
  relationship: {
    trust: number;
    familiarity: number;
    tension: number;
    security: number;
  };
  affectLabel: string;
  collectedAt: string;
}

export interface ProspectiveMemory {
  id: string;
  title: string;
  description: string;
  priority: number;
  status: "pending" | "triggered" | "completed" | "cancelled";
  expiresAt: string | null;
  notBefore: string | null;
  triggerReason: string;
  createdAt: string;
  updatedAt: string;
}

export interface TriggerHistory {
  id: string;
  triggerId: string;
  triggerType: string;
  title: string;
  channel: string;
  state: "pending" | "sending" | "sent" | "failed" | "cancelled";
  priority: string;
  reason: string;
  attemptCount: number;
  lastError: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface TriggerQueueSummary {
  depth: number;
  pendingCount: number;
  oldestAgeMs: number;
  recentFailures: number;
  backpressure: boolean;
}

export type ReminderGroup = "overdue" | "upcoming" | "completed" | "disabled";

// ============================================================
// Extension Kernel - 新领域类型 解除Skill概念过载
// ============================================================

export type ToolSource = "builtin" | "plugin" | "mcp" | "workflow" | "internal" | "legacy_tool";

export type RiskLevel = "low" | "medium" | "high";

export type SideEffectLevel = "none" | "read_only" | "write" | "financial";

export interface ScopeRule {
  type: string;
  id?: string;
}

export interface PermissionRequirement {
  capability: string;
  description?: string;
  risk?: string;
}

export interface ToolDefinition {
  id: string;
  modelName: string;
  extensionId?: string;
  moduleId?: string;
  source: ToolSource;
  name: string;
  description: string;
  version?: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  permissions?: PermissionRequirement[];
  riskLevel?: RiskLevel;
  sideEffect?: SideEffectLevel;
  scope?: ScopeRule;
  enabled: boolean;
  compatible?: boolean;
  internal?: boolean;
  hasSideEffects: boolean;
  idempotent: boolean;
  retryable: boolean;
  timeoutMs: number;
  metadata?: Record<string, unknown>;
}

export interface ToolInvocation {
  toolId: string;
  input: Record<string, unknown>;
  idempotencyKey?: string;
  traceId?: string;
}

export interface ToolResult {
  invocationId: string;
  status: string;
  output?: Record<string, unknown>;
  error?: string;
  visibleText?: string;
  durationMs: number;
}

export interface ActivationRule {
  explicit: boolean;
  autoMatch?: string;
  keywords?: string[];
  priority?: number;
}

export interface ToolReference {
  toolId: string;
  constraint?: string;
}

export interface MCPReference {
  serverId: string;
  optional?: boolean;
}

export interface SkillResource {
  path: string;
  kind: string;
  mimeType?: string;
  size: number;
  textReadable?: boolean;
}

export type AgentSkillScope = "global" | "character";

export interface AgentSkillDefinition {
  id: string;
  extensionId: string;
  moduleId?: string;
  name: string;
  description: string;
  displayName?: string;
  instructions: string;
  activation: ActivationRule;
  requiredTools?: ToolReference[];
  requiredMCP?: MCPReference[];
  resources?: SkillResource[];
  tokenBudget?: number;
  scope: AgentSkillScope;
  scopeId?: string;
  enabled: boolean;
  compatible?: boolean;
  source: string;
  version?: string;
  license?: string;
  author?: string;
  compatibilityStatus?: string;
  toolMappings?: Record<string, unknown>[];
  metadata?: Record<string, unknown>;
}

export interface AgentSkillCatalogEntry {
  extensionId: string;
  name: string;
  description: string;
  displayName?: string;
  scope: AgentSkillScope;
  compatibility: string;
  source?: string;
}

export interface WorkflowNode {
  id: string;
  type: string;
  step: {
    input: Record<string, unknown>;
    when?: Record<string, unknown>;
    onError?: {
      mode: string;
      default?: Record<string, unknown>;
    };
  };
}

export type WorkflowConcurrencyMode = "ALLOW" | "SINGLETON" | "QUEUE" | "REPLACE" | "DROP" | "MAX_N";
export interface WorkflowConcurrencyPolicy { mode?: WorkflowConcurrencyMode; maxN?: number }

export interface WorkflowLimits {
  maxSteps: number;
  maxExecutionDurationMs: number;
  maxStepDurationMs: number;
  maxInputBytes: number;
  maxOutputBytes: number;
  maxIntermediateBytes: number;
  maxHttpResponseBytes: number;
  maxHttpRedirects: number;
  maxSkillCallDepth: number;
  maxSkillCalls: number;
  maxArrayItems: number;
  maxExpressionDepth: number;
  maxTemplateLength: number;
  maxEventsEmitted: number;
  maxSchedulesCreated: number;
  maxSideEffects: number;
}

export interface WorkflowDefinition {
  schemaVersion: string;
  id: string;
  extensionId?: string;
  moduleId?: string;
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  outputSchema: Record<string, unknown>;
  nodes: WorkflowNode[];
  permissions?: string[];
  scope?: string;
  callableByAgent: boolean;
  enabled: boolean;
  hasSideEffects?: boolean;
  idempotent?: boolean;
  limits?: WorkflowLimits;
  concurrencyPolicy?: WorkflowConcurrencyPolicy;
  version?: string;
  source?: string;
  metadata?: Record<string, unknown>;
}

export type ContributionType = "tool" | "agent_skill" | "workflow" | "mcp" | "ui" | "hook" | "background_task" | "provider" | "asset";

export interface BaseContribution {
  id: string;
  type: ContributionType;
  extensionId: string;
  moduleId?: string;
  enabled: boolean;
  metadata?: Record<string, unknown>;
}

export interface ToolContribution extends BaseContribution {
  toolId: string;
}

export interface AgentSkillContribution extends BaseContribution {
  agentSkillId: string;
}

export interface WorkflowContribution extends BaseContribution {
  workflowId: string;
}

export interface MCPContribution extends BaseContribution {
  serverId: string;
  descriptor?: Record<string, unknown>;
}
