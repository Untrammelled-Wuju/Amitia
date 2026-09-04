import { apiClient } from "@/composables/useApi";

const BASE = "/api/dev-console";

export interface DevConsoleOverview {
  generatedAt: string;
  extensions: number;
  modules: number;
  contributions: number;
  runtimes: number;
  activeInvocations: number;
  hostApiCalls: number;
  eventsLast5Min: number;
  hookInvocations: number;
  activeTasks: number;
  activeUiSessions: number;
  storageEntries: number;
  permissionGrants: number;
  activeScopes: number;
  resources: number;
  errors: number;
  warnings: number;
  lifecycleEvents: number;
  devWorkspaces: number;
  compatibilityIssues: number;
  topExtensions: ExtensionSummary[];
}

export interface ExtensionSummary {
  extensionId: string;
  publisher: string;
  version: string;
  moduleCount: number;
  enabled: boolean;
  status: string;
  errorCount: number;
  invocationCount: number;
}

export interface InvocationRecord {
  id: string;
  extensionId: string;
  moduleId: string;
  toolId: string;
  startedAt: string;
  completedAt?: string;
  status: string;
  durationMs: number;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  idempotencyKey?: string;
  trace?: string;
}

export interface EventRecord {
  id: string;
  type: string;
  source: string;
  emittedAt: string;
  consumed: boolean;
  consumer?: string;
  payload?: Record<string, unknown>;
}

export interface HookRecord {
  id: string;
  pipeline: string;
  stage: string;
  phase: string;
  extension: string;
  invokedAt: string;
  durationMs: number;
  vetoed: boolean;
}

export interface TaskRecord {
  id: string;
  taskId: string;
  extension: string;
  startedAt: string;
  completedAt?: string;
  status: string;
  progress: number;
  attempt: number;
}

export interface UISessionRecord {
  id: string;
  extension: string;
  contribution: string;
  startedAt: string;
  lastActive: string;
  origin: string;
  cspViolations: number;
}

export interface StorageEntryRecord {
  namespace: string;
  key: string;
  version: number;
  scope: string;
  updatedAt: string;
}

export interface PermissionGrantRecord {
  permission: string;
  extension: string;
  scope: string;
  granted: boolean;
  grantedAt: string;
  reason?: string;
}

export interface ScopeRecord {
  scope: string;
  characterId?: string;
  conversationId?: string;
  userId?: string;
  active: boolean;
}

export interface ResourceRecord {
  handle: string;
  kind: string;
  extension: string;
  createdAt: string;
  size: number;
}

export interface LifecycleEventRecord {
  extension: string;
  stage: string;
  at: string;
  success: boolean;
  reason?: string;
}

export interface LogEntry {
  extension: string;
  level: string;
  message: string;
  at: string;
  fields?: Record<string, unknown>;
}

export interface PerformanceRecord {
  extension: string;
  metric: string;
  value: number;
  at: string;
}

export interface MigrationRecord {
  stage: string;
  status: string;
  at: string;
  details?: string;
}

export interface CompatibilityRecord {
  extension: string;
  required: string;
  host: string;
  ok: boolean;
  reason?: string;
}

export interface HostAPIAuditEntry {
  callId: string;
  traceId: string;
  operationId: string;
  invocationId: string;
  extensionId: string;
  moduleId: string;
  method: string;
  generation: number;
  permissionSnapshotId: string;
  scopeSnapshotId: string;
  startedAt: string;
  finishedAt?: string;
  result: string;
  errorCode?: string;
  errorMessage?: string;
  sideEffect?: string;
  inputMasked?: string;
  phase: string;
}

export interface HostAPIAuditResponse {
  entries: HostAPIAuditEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface HostAPIAuditFilter {
  extension?: string;
  method?: string;
  result?: string;
  traceId?: string;
  limit?: number;
  offset?: number;
}

export interface ConsoleSession {
  sessionId: string;
  workspaceId: string;
  startedAt: string;
  expiresAt: string;
}

export interface ConsoleFilter {
  extension?: string;
  module?: string;
  severity?: string;
  stage?: string;
  search?: string;
  start?: number;
  end?: number;
}

function buildQuery(params?: ConsoleFilter): string {
  if (!params) return "";
  const sp = new URLSearchParams();
  if (params.extension) sp.set("extension", params.extension);
  if (params.module) sp.set("module", params.module);
  if (params.severity) sp.set("severity", params.severity);
  if (params.stage) sp.set("stage", params.stage);
  if (params.search) sp.set("search", params.search);
  if (params.start) sp.set("start", String(Math.floor(params.start)));
  if (params.end) sp.set("end", String(Math.floor(params.end)));
  const qs = sp.toString();
  return qs ? `?${qs}` : "";
}

export async function fetchOverview(): Promise<DevConsoleOverview> {
  const res = await apiClient.get(`${BASE}/overview`);
  return res.data;
}

export async function fetchInvocations(params?: ConsoleFilter): Promise<InvocationRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/invocations${qs}`);
  return res.data;
}

export async function fetchEvents(params?: ConsoleFilter): Promise<EventRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/events${qs}`);
  return res.data;
}

export async function fetchHooks(params?: ConsoleFilter): Promise<HookRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/hooks${qs}`);
  return res.data;
}

export async function fetchTasks(params?: ConsoleFilter): Promise<TaskRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/tasks${qs}`);
  return res.data;
}

export async function fetchUISessions(params?: ConsoleFilter): Promise<UISessionRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/ui-sessions${qs}`);
  return res.data;
}

export async function fetchStorage(params?: ConsoleFilter): Promise<StorageEntryRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/storage${qs}`);
  return res.data;
}

export async function fetchPermissions(params?: ConsoleFilter): Promise<PermissionGrantRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/permissions${qs}`);
  return res.data;
}

export async function fetchScopes(): Promise<ScopeRecord[]> {
  const res = await apiClient.get(`${BASE}/scopes`);
  return res.data;
}

export async function fetchResources(params?: ConsoleFilter): Promise<ResourceRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/resources${qs}`);
  return res.data;
}

export async function fetchLifecycle(params?: ConsoleFilter): Promise<LifecycleEventRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/lifecycle${qs}`);
  return res.data;
}

export async function fetchLogs(params?: ConsoleFilter): Promise<LogEntry[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/logs${qs}`);
  return res.data;
}

export async function fetchPerformance(params?: ConsoleFilter): Promise<PerformanceRecord[]> {
  const qs = buildQuery(params);
  const res = await apiClient.get(`${BASE}/performance${qs}`);
  return res.data;
}

export async function fetchMigration(): Promise<MigrationRecord[]> {
  const res = await apiClient.get(`${BASE}/migration`);
  return res.data;
}

export async function fetchCompatibility(): Promise<CompatibilityRecord[]> {
  const res = await apiClient.get(`${BASE}/compatibility`);
  return res.data;
}

export async function fetchHostAPIAudits(params?: HostAPIAuditFilter): Promise<HostAPIAuditResponse> {
  const sp = new URLSearchParams();
  if (params?.extension) sp.set("extension", params.extension);
  if (params?.method) sp.set("method", params.method);
  if (params?.result) sp.set("result", params.result);
  if (params?.traceId) sp.set("traceId", params.traceId);
  if (params?.limit) sp.set("limit", String(params.limit));
  if (params?.offset != null) sp.set("offset", String(params.offset));
  const qs = sp.toString();
  const res = await apiClient.get(`${BASE}/host-api-audits${qs ? `?${qs}` : ""}`);
  return res.data;
}

export async function openSession(workspace?: string): Promise<ConsoleSession> {
  const sp = new URLSearchParams();
  if (workspace) sp.set("workspace", workspace);
  const qs = sp.toString();
  const res = await apiClient.post(`${BASE}/sessions${qs ? `?${qs}` : ""}`);
  return res.data;
}

export async function closeSession(id: string): Promise<void> {
  await apiClient.delete(`${BASE}/sessions?id=${encodeURIComponent(id)}`);
}

export async function exportDiagnostics(params?: ConsoleFilter): Promise<Blob> {
  const filter: ConsoleFilter = params || {};
  const [overview, invocations, events, hooks, tasks, uiSessions, storage, permissions, scopes, resources, lifecycle, logs, performance, migration, compatibility, hostApiAudits] = await Promise.all([
    fetchOverview().catch(() => null),
    fetchInvocations(filter).catch(() => []),
    fetchEvents(filter).catch(() => []),
    fetchHooks(filter).catch(() => []),
    fetchTasks(filter).catch(() => []),
    fetchUISessions(filter).catch(() => []),
    fetchStorage(filter).catch(() => []),
    fetchPermissions(filter).catch(() => []),
    fetchScopes().catch(() => []),
    fetchResources(filter).catch(() => []),
    fetchLifecycle(filter).catch(() => []),
    fetchLogs(filter).catch(() => []),
    fetchPerformance(filter).catch(() => []),
    fetchMigration().catch(() => []),
    fetchCompatibility().catch(() => []),
    fetchHostAPIAudits({ extension: filter.extension, limit: 1000 }).catch(() => ({ entries: [], total: 0, limit: 1000, offset: 0 })),
  ]);
  const payload = {
    exportedAt: new Date().toISOString(),
    filter,
    overview,
    invocations,
    events,
    hooks,
    tasks,
    uiSessions,
    storage,
    permissions,
    scopes,
    resources,
    lifecycle,
    logs,
    performance,
    migration,
    compatibility,
    hostApiAudits,
  };
  return new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
}
