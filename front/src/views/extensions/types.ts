export type SkillTrigger = "llm" | "manual" | "schedule" | "system_event"
export type RunStatus = "pending" | "running" | "succeeded" | "failed" | "cancelled" | "timed_out" | "partially_succeeded"
export type PermissionDecision = "deny" | "allow_once" | "allow_session" | "allow_character" | "allow_always"
export type ScopeType = "global" | "character" | "conversation" | "channel" | "session"

export interface RunView {
  runId: string
  extensionId: string
  extensionVersion: string
  skillId: string
  userId: string
  characterId: string
  conversationId: string
  channel: string
  trigger: SkillTrigger
  status: RunStatus
  inputSummary: string
  outputSummary: string
  sideEffects?: Array<{ type: string; targetId?: string; confirmed: boolean }>
  idempotencyKey?: string
  startedAt: string
  finishedAt?: string
  durationMs: number
  errorCode?: string
  errorDetail?: string
  traceId: string
}

export interface SkillView {
  id: string
  modelName: string
  name: string
  description: string
  version: string
  source: string
  entry: { kind: string; name?: string; artifactId?: string }
  inputSchema: Record<string, unknown>
  outputSchema: Record<string, unknown>
  configSchema?: Record<string, unknown>
  defaultConfig?: Record<string, unknown>
  capabilities: string[]
  triggers: SkillTrigger[]
  timeoutMs: number
  hasSideEffects: boolean
  retryable: boolean
  idempotent: boolean
  enabled: boolean
  compatible: boolean
  compatibilityReason?: string
  author?: string
  license?: string
  manifest: Record<string, unknown>
  latestRun?: RunView
}

export interface PermissionGrant {
  id?: string
  capability: string
  risk: "low" | "medium" | "high"
  description: string
  decision: PermissionDecision
  scopeType: ScopeType
  scopeId: string
  expiresAt?: string
  consumedAt?: string
}

export interface CapabilityDefinition {
  name: string
  risk: "low" | "medium" | "high"
  description: string
}

export interface SkillDetail extends SkillView {
  permissions: PermissionGrant[]
  config: Record<string, unknown>
  recentRuns: RunView[]
  versions: Array<{ version: string; checksum: string; manifest: Record<string, unknown>; createdAt: string }>
}

export interface RunPage {
  items: RunView[]
  total: number
  page: number
  pageSize: number
}

export interface SkillResult {
  runId: string
  status: RunStatus
  output?: unknown
  sideEffects?: Array<{ type: string; targetId?: string; confirmed: boolean }>
  error?: { code: string; message: string; detail?: string; retryable: boolean }
  durationMs: number
  visibleText?: string
  forceVoice?: boolean
}

export type PluginLifecycle = "registered" | "loaded" | "enabled" | "disabled" | "error" | "circuit_open" | "unloading" | "unloaded"

export interface PluginManifest {
  $schema: string
  apiVersion: string
  kind: "Plugin"
  metadata: { id: string; name: string; description: string; version: string; author?: string; license?: string }
  compatibility: { engineMin: string; engineMaxExclusive?: string }
  entry: { kind: string; name: string }
  capabilities: string[]
  hooks: string[]
  subscriptions: string[]
  registeredSkills: string[]
  execution: { hookTimeoutMs: number; maxConcurrency: number; failureThreshold: number; circuitOpenMs: number; halfOpenMaxRequests?: number }
  configSchema?: Record<string, unknown>
  defaultConfig?: Record<string, unknown>
  state: { schemaVersion: string; schema?: Record<string, unknown>; default?: unknown }
  surface?: SurfaceDocument
  enabled: boolean
}

export interface PluginView {
  manifest: PluginManifest
  source: "builtin"
  lifecycle: PluginLifecycle
  health: string
  compatible: boolean
  enabled: boolean
  currentCircuits: number
  lastErrorCode?: string
  lastErrorAt?: string
}

export interface PluginState { pluginId: string; scopeType: ScopeType; scopeId: string; schemaVersion: string; revision: number; data: unknown; updatedAt: string }
export interface PluginSchedule { scheduleId: string; scope: { type: ScopeType; id: string }; type: string; expression: string; timezone: string; payload: unknown; enabled: boolean; nextRunAt?: string }
export interface ExtensionEvent { specversion: string; id: string; source: string; type: string; subject?: string; time: string; datacontenttype: string; data: unknown; traceId?: string; correlationId?: string; causationId?: string; depth: number }
export interface PluginRun { runId: string; pluginId: string; pluginVersion: string; hook: string; status: string; durationMs: number; errorCode?: string; traceId?: string; circuitState: string; createdAt: string }
export interface CircuitView { state: "closed" | "open" | "half_open"; failures: number; openedAt?: string; nextProbeAt?: string }
export interface PluginHealth { pluginId: string; lifecycle: PluginLifecycle; health: string; compatible: boolean; lastErrorCode?: string; lastErrorAt?: string; circuits: Record<string, CircuitView> }
export interface PluginDetail extends PluginView { permissions: PermissionGrant[]; config: Record<string, unknown>; states: PluginState[]; schedules: PluginSchedule[]; recentRuns: PluginRun[]; recentEvents: ExtensionEvent[] }
export interface PluginPage { items: PluginView[]; total: number; page: number; pageSize: number }
export interface PluginEventPage { items: ExtensionEvent[]; total: number; page: number; pageSize: number }

export interface SurfaceField {
  key: string
  label: string
  component: "text" | "number" | "switch" | "select" | "textarea" | "secret" | "action" | "status" | "table"
  description?: string
  placeholder?: string
  required?: boolean
  options?: string[]
}

export interface SurfaceSection { id: string; type: "form" | "action" | "status" | "table"; title?: string; label?: string; source?: string; skill?: string; fields?: SurfaceField[]; columns?: SurfaceField[] }
export interface SurfaceDocument { $schema: string; version: "1.0"; title: string; sections: SurfaceSection[] }

export type WorkshopStatus = "draft" | "generating" | "generated" | "validating" | "validation_failed" | "validated" | "awaiting_permission_confirmation" | "testing" | "test_failed" | "test_passed" | "installing" | "installed" | "enabled" | "disabled" | "archived" | "error"
export interface WorkshopSession { id: string; userId: string; characterId?: string; status: WorkshopStatus; requirement: string; currentRevision: number; currentDraftId?: string; validationSummary: unknown; riskSummary: unknown; testSummary: unknown; installedSkillId?: string; installedVersion?: string; testPermissionConfirmed: boolean; productionPermissionConfirmed: boolean; createdAt: string; updatedAt: string; archivedAt?: string }
export interface AnalysisIssue { level: "error" | "warning" | "info"; code: string; message: string; path?: string; stepId?: string }
export interface CapabilityAnalysis { required: string[]; declared: string[]; missing: string[]; excess: string[]; byStep: Record<string, string[]>; highRisk: string[] }
export interface WorkshopValidation { sessionId: string; revision: number; workflowChecksum: string; valid: boolean; issues: AnalysisIssue[]; capabilities: CapabilityAnalysis; hasSideEffects: boolean; idempotent: boolean; validatedAt: string }
export interface WorkflowStep { id: string; type: "http" | "condition" | "transform" | "template" | "call_skill" | "schedule" | "notification" | "memory_candidate" | "context_contribution"; input: Record<string, unknown>; when?: Record<string, unknown>; onError: { mode: "fail" | "continue" | "use_default"; default?: unknown } }
export interface ExtensionDraft { draftVersion: string; metadata: { id: string; name: string; version: string; description: string; author: string; license: string }; intent: { goal: string; triggers: SkillTrigger[]; sideEffects: string[]; idempotencyKey?: string }; manifest: Record<string, any>; inputSchema: Record<string, any>; outputSchema: Record<string, any>; configSchema: Record<string, any>; defaultConfig: Record<string, any>; workflow: { schemaVersion: string; steps: WorkflowStep[]; output: unknown; limits: Record<string, number> }; capabilities: string[]; dependencies: Array<{ skillId: string; version?: string; optional?: boolean }>; testCases: WorkshopTestCase[]; assumptions: Array<{ message: string }>; warnings: Array<{ code: string; message: string; path?: string }> }
export interface WorkshopTestCase { id: string; name: string; mode: "dry_run" | "mocked" | "controlled_live"; input: unknown; config: unknown; secretRefs: string[]; httpMocks: unknown[]; skillMocks: unknown[]; expectedOutput?: unknown; assertions: unknown[] }
export interface WorkshopPlan { goal: string; inputs: Array<{ name: string; type: string; required: boolean; description?: string }>; outputs: Array<{ name: string; type: string; required: boolean; description?: string }>; configs: Array<{ name: string; type: string; required: boolean; description?: string }>; steps: Array<{ id: string; type: WorkflowStep["type"]; purpose: string }>; dependencies: string[]; capabilities: string[]; sideEffects: string[]; risks: string[]; missingDetails: string[]; assumptions: string[] }
export interface WorkshopRevision { id: string; sessionId: string; revision: number; plan: WorkshopPlan; draft: ExtensionDraft; normalizedDraft: ExtensionDraft; workflowChecksum: string; validation?: WorkshopValidation; modelProvider?: string; modelName?: string; createdAt: string }
export interface WorkshopTestReport { testRunId: string; sessionId: string; revision: number; workflowChecksum: string; status: string; startedAt: string; finishedAt: string; durationMs: number; stepResults: Array<{ stepId: string; type: string; status: string; inputSummary: string; outputSummary: string; durationMs: number; mocked: boolean; error?: { code: string; message: string; detail?: string } }>; assertions: Array<{ type: string; passed: boolean; message?: string }>; sideEffects: Array<{ type: string; targetId?: string; confirmed: boolean }>; capabilities: string[]; warnings: Array<{ code: string; message: string }>; output?: unknown; error?: { code: string; message: string; detail?: string } }
export interface WorkshopSessionDetail extends WorkshopSession { revision?: WorkshopRevision; testReports: WorkshopTestReport[] }
export interface WorkshopPage { items: WorkshopSession[]; total: number; page: number; pageSize: number }
