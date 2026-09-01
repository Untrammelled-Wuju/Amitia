import { apiClient } from "@/composables/useApi";

export interface WorkflowPosition { x: number; y: number }
export interface WorkflowRuntimeBinding {
  runtimeType?: string;
  runtimeId?: string;
  handlerName?: string;
  providerId?: string;
  endpoint?: string;
  metadata?: Record<string, unknown>;
}
export type WorkflowLocation = "local" | "cloud" | "device";
export type WorkflowExecutionPlacement = "auto" | "local" | "cloud" | "device";
export type WorkflowOfflinePolicy = "fail" | "wait";
export interface WorkflowTarget { location: WorkflowLocation; deviceId?: string }
export interface WorkflowExecutionTarget {
  placement?: WorkflowExecutionPlacement;
  deviceId?: string;
  runtimeId?: string;
  providerId?: string;
  providerInstanceId?: string;
  offlinePolicy?: WorkflowOfflinePolicy;
}
export interface WorkflowInstallation {
  installationId: string;
  workflowId: string;
  ownerUserId?: string;
  location: "local" | "cloud";
  hostDeviceId?: string;
  enabled: boolean;
  triggers?: WorkflowTrigger[];
  callableByAgent?: boolean;
  agentTool?: WorkflowAgentToolConfig;
  revision: number;
  createdAt?: string;
  updatedAt?: string;
}
export interface WorkflowDeviceDescriptor {
  deviceId: string;
  runtimeId?: string;
  label?: string;
  platform?: string;
  online: boolean;
  lastSeenAt?: string;
}
export interface WorkflowOnError { mode?: string; default?: unknown }
export interface WorkflowNodeRetryPolicy { maxAttempts?: number; initialBackoffMs?: number; maxBackoffMs?: number; multiplier?: number; jitter?: number }
export interface WorkflowStepInput { input?: unknown; when?: unknown; onError: WorkflowOnError }
export interface WorkflowNode {
  id: string;
  type: string;
  dependsOn?: string[];
  targetId?: string;
  runtime: WorkflowRuntimeBinding;
  executionTarget?: WorkflowExecutionTarget;
  permissions?: string[];
  scope?: string;
  position?: WorkflowPosition;
  label?: string;
  timeoutMs?: number;
  retry?: WorkflowNodeRetryPolicy;
  step: WorkflowStepInput;
}
export interface WorkflowEdge {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string;
  targetHandle?: string;
  label?: string;
  condition?: unknown;
}
export interface WorkflowTrigger {
  id: string;
  type: string;
  eventType?: string;
  schedule?: Record<string, unknown>;
  config: Record<string, any>;
  input?: unknown;
  enabled: boolean;
}
export interface WorkflowAgentToolConfig { name?: string; description?: string }
export interface WorkflowCatalogItem {
  id: string; modelName?: string; name: string; description?: string; source?: string; inputSchema?: unknown; outputSchema?: unknown; runtime?: WorkflowRuntimeBinding;
}
export interface WorkflowAIProposal { definition: WorkflowDefinition; summary: string; changes: string[]; warnings: string[] }
export interface WorkflowAIExplanation { summary: string; flow: string[]; issues: string[]; suggestions: string[] }
export interface WorkflowRevisionSummary {
  revisionId: string; workflowId: string; revisionNo: number; name: string; description?: string; definitionHash: string; note?: string; createdAt: string;
}
export interface WorkflowTemplateSummary {
  templateId: string; name: string; description?: string; definitionHash: string; nodeCount: number; triggerCount: number; createdAt: string; updatedAt: string;
}
export interface WorkflowExportEnvelope {
  format: "amitia-workflow"; formatVersion: number; exportedAt: string; workflow: WorkflowDefinition;
}

export interface WorkflowDefinition {
  schemaVersion: string;
  id: string;
  extensionId?: string;
  moduleId?: string;
  name: string;
  description: string;
  inputSchema: unknown;
  outputSchema: unknown;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  triggers: WorkflowTrigger[];
  permissions?: string[];
  scope?: string;
  callableByAgent: boolean;
  agentTool: WorkflowAgentToolConfig;
  enabled: boolean;
  hasSideEffects?: boolean;
  idempotent?: boolean;
  limits?: Record<string, unknown>;
  version?: string;
  source?: string;
  metadata?: Record<string, unknown>;
  definitionHash?: string;
  installation?: WorkflowInstallation;
  cached?: boolean;
  offline?: boolean;
}
export interface WorkflowRun {
  executionId: string;
  workflowId: string;
  status: string;
  input?: unknown;
  output?: unknown;
  error?: string;
  attempt?: number;
  startedAt?: string;
  finishedAt?: string;
  updatedAt?: string;
}
export interface WorkflowStepRun {
  executionId: string;
  workflowId: string;
  nodeId: string;
  status: string;
  input?: unknown;
  output?: unknown;
  error?: string;
  attempt?: number;
  startedAt?: string;
  finishedAt?: string;
}
export interface WorkflowStepAttempt {
  executionId: string;
  workflowId: string;
  nodeId: string;
  attempt: number;
  generation: number;
  status: string;
  input?: unknown;
  output?: unknown;
  error?: string;
  nextBackoffMs?: number;
  startedAt: string;
  finishedAt: string;
}
export interface WorkflowCheckpoint { workflowId: string; executionId: string; nodeId: string; input?: unknown; output?: unknown; completedAt: string }
export interface WorkflowNodeStat { nodeId: string; runCount: number; succeeded: number; failed: number; timedOut: number; averageStepMs: number; averageAttempts: number }
export interface WorkflowExecutionStats { runCount: number; succeeded: number; failed: number; cancelled: number; compensated: number; successRate: number; averageRunMs: number; lastRunAt?: string; lastError?: string; nodeStatistics: WorkflowNodeStat[] }
export interface WorkflowRiskItem { level: "low"|"medium"|"high"|string; nodeId?: string; code: string; message: string }
export interface WorkflowNestedDependency { nodeId: string; workflowId: string; name?: string; status: "ok"|"missing"|"forbidden"|string; definitionHash?: string }
export interface WorkflowSafetyAnalysis { riskLevel: "low"|"medium"|"high"|string; declaredPermissions: string[]; secretReferences: string[]; risks: WorkflowRiskItem[]; nestedDependencies: WorkflowNestedDependency[]; hasSideEffects: boolean }
export interface WorkflowSyncEvent {
  cursor: number;
  eventId: string;
  type: string;
  aggregateId?: string;
  aggregateVersion?: number;
  occurredAt: string;
  payload?: unknown;
}
export interface WorkflowSyncPage { cursor: number; items: WorkflowSyncEvent[] }

const CLOUD_TARGET: WorkflowTarget = { location: "cloud" };

function normalizeTarget(target?: WorkflowTarget): WorkflowTarget {
  const value = target ?? CLOUD_TARGET;
  if (value.location === "device" && !String(value.deviceId || "").trim()) {
    throw new Error("device workflow target requires deviceId");
  }
  return value;
}

function workflowBase(target?: WorkflowTarget): string {
  const value = normalizeTarget(target);
  if (value.location === "local") return "/api/local/workflows";
  if (value.location === "device") return `/api/extensions/workflow-devices/${encodeURIComponent(String(value.deviceId))}/workflows`;
  return "/api/extensions/workflows";
}

function workflowRunBase(target?: WorkflowTarget): string {
  const value = normalizeTarget(target);
  if (value.location === "local") return "/api/local/workflow-runs";
  if (value.location === "device") throw new Error("remote device workflow run details are only available on that device");
  return "/api/extensions/workflow-runs";
}

function workflowSyncBase(target?: WorkflowTarget): string {
  const value = normalizeTarget(target);
  // Remote device mutations are projected into the Cloud Core outbox. Device-local
  // targets therefore consume the cloud sync stream rather than a device endpoint.
  return value.location === "local" ? "/api/local/workflows/sync-events" : "/api/extensions/workflows/sync-events";
}

function ensureKernelTarget(target: WorkflowTarget | undefined, feature: string): WorkflowTarget {
  const value = normalizeTarget(target);
  if (value.location === "device") throw new Error(`${feature} is not exposed through the remote device control plane`);
  return value;
}

function revisionParams(defOrRevision?: WorkflowDefinition | number): Record<string, number> | undefined {
  const revision = typeof defOrRevision === "number" ? defOrRevision : defOrRevision?.installation?.revision;
  return revision && revision > 0 ? { expectedRevision: revision } : undefined;
}

export function workflowTargetQuery(target: WorkflowTarget): Record<string, string> {
  return target.location === "device"
    ? { location: target.location, deviceId: String(target.deviceId || "") }
    : { location: target.location };
}

export function workflowTargetFromQuery(query: Record<string, unknown>): WorkflowTarget {
  const location = String(query.location || "cloud");
  if (location === "local") return { location: "local" };
  if (location === "device") return { location: "device", deviceId: String(query.deviceId || "") };
  return { location: "cloud" };
}

export async function listWorkflowDevices(): Promise<WorkflowDeviceDescriptor[]> {
  const res = await apiClient.get<{ items: WorkflowDeviceDescriptor[] }>("/api/extensions/workflow-devices");
  return res.data.items ?? [];
}

export async function listWorkflowSyncEvents(target: WorkflowTarget = CLOUD_TARGET, afterCursor?: number): Promise<WorkflowSyncPage> {
  const params = typeof afterCursor === "number" && afterCursor >= 0 ? { afterCursor } : undefined;
  const res = await apiClient.get<WorkflowSyncPage>(workflowSyncBase(target), { params });
  return {
    cursor: Number(res.data?.cursor ?? afterCursor ?? 0),
    items: Array.isArray(res.data?.items) ? res.data.items : [],
  };
}

export async function listWorkflows(target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition[]> {
  const res = await apiClient.get<{ items: Array<WorkflowDefinition & { workflowId?: string }>; cached?: boolean; offline?: boolean }>(workflowBase(target));
  return (res.data.items ?? []).map((item: any) => ({
    schemaVersion: item.schemaVersion || "workflow-v2",
    id: item.id || item.workflowId,
    name: item.name || item.workflowId || "Workflow",
    description: item.description || "",
    inputSchema: item.inputSchema || {},
    outputSchema: item.outputSchema || {},
    nodes: item.nodes || [],
    edges: item.edges || [],
    triggers: item.triggers || [],
    callableByAgent: Boolean(item.callableByAgent),
    agentTool: item.agentTool || {},
    enabled: item.installation?.enabled ?? item.enabled ?? false,
    ...item,
    cached: Boolean(res.data.cached || item.cached),
    offline: Boolean(res.data.offline || item.offline),
  }));
}
export async function getWorkflowCatalog(target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowCatalogItem[]> {
  const value = ensureKernelTarget(target, "workflow catalog");
  const res = await apiClient.get<{ items: WorkflowCatalogItem[] }>(`${workflowBase(value)}/catalog`);
  return res.data.items ?? [];
}
export async function generateWorkflowWithAI(instruction: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowAIProposal> {
  const value = ensureKernelTarget(target, "AI workflow generation");
  return (await apiClient.post<WorkflowAIProposal>(`${workflowBase(value)}/ai/generate`, { instruction })).data;
}
export async function editWorkflowWithAI(id: string, instruction: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowAIProposal> {
  const value = ensureKernelTarget(target, "AI workflow editing");
  return (await apiClient.post<WorkflowAIProposal>(`${workflowBase(value)}/${encodeURIComponent(id)}/ai/edit`, { instruction })).data;
}
export async function repairWorkflowWithAI(id: string, instruction = "", target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowAIProposal> {
  const value = ensureKernelTarget(target, "AI workflow repair");
  return (await apiClient.post<WorkflowAIProposal>(`${workflowBase(value)}/${encodeURIComponent(id)}/ai/repair`, { instruction })).data;
}
export async function explainWorkflowWithAI(id: string, instruction = "", target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowAIExplanation> {
  const value = ensureKernelTarget(target, "AI workflow explanation");
  return (await apiClient.post<WorkflowAIExplanation>(`${workflowBase(value)}/${encodeURIComponent(id)}/ai/explain`, { instruction })).data;
}

export async function createWorkflow(def: Partial<WorkflowDefinition>, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>(workflowBase(target), def)).data;
}
export async function getWorkflow(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  return (await apiClient.get<WorkflowDefinition>(`${workflowBase(target)}/${encodeURIComponent(id)}`)).data;
}
export async function updateWorkflow(id: string, def: WorkflowDefinition, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  return (await apiClient.put<WorkflowDefinition>(`${workflowBase(target)}/${encodeURIComponent(id)}`, def, { params: revisionParams(def) })).data;
}
export async function patchWorkflow(id: string, patch: Partial<WorkflowDefinition>, target: WorkflowTarget = CLOUD_TARGET, expectedRevision?: number): Promise<WorkflowDefinition> {
  const value = ensureKernelTarget(target, "workflow patch");
  return (await apiClient.patch<WorkflowDefinition>(`${workflowBase(value)}/${encodeURIComponent(id)}`, patch, { params: revisionParams(expectedRevision) })).data;
}
export async function duplicateWorkflow(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  const value = ensureKernelTarget(target, "workflow duplication");
  return (await apiClient.post<WorkflowDefinition>(`${workflowBase(value)}/${encodeURIComponent(id)}/duplicate`)).data;
}
export async function deleteWorkflow(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  await apiClient.delete(`${workflowBase(target)}/${encodeURIComponent(id)}`);
}
export async function setWorkflowEnabled(id: string, enabled: boolean, target: WorkflowTarget = CLOUD_TARGET, expectedRevision?: number): Promise<WorkflowInstallation | void> {
  const res = await apiClient.post(`${workflowBase(target)}/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`, undefined, { params: revisionParams(expectedRevision) });
  return (res.data as any)?.installation ?? res.data;
}
export async function validateWorkflow(def: WorkflowDefinition, target: WorkflowTarget = CLOUD_TARGET): Promise<{ valid: boolean; error?: string; topologicalOrder?: string[] }> {
  const value = ensureKernelTarget(target, "workflow validation");
  return (await apiClient.post(`${workflowBase(value)}/validate`, def)).data;
}
export async function runWorkflow(id: string, input: unknown = {}, wait = false, target: WorkflowTarget = CLOUD_TARGET): Promise<{ accepted?: boolean; executionId: string; status?: string }> {
  const value = normalizeTarget(target);
  const payload = value.location === "device" ? { input } : { input, wait };
  return (await apiClient.post(`${workflowBase(value)}/${encodeURIComponent(id)}/run`, payload)).data;
}
export async function dispatchWorkflowEvent(eventType: string, payload: unknown = {}, target: WorkflowTarget = CLOUD_TARGET): Promise<{ accepted: boolean; eventType: string }> {
  const value = ensureKernelTarget(target, "workflow event dispatch");
  return (await apiClient.post(`${workflowBase(value)}/events/${encodeURIComponent(eventType)}`, payload)).data;
}
export async function listWorkflowRuns(id: string, limit = 50, target: WorkflowTarget = CLOUD_TARGET): Promise<{ items: WorkflowRun[]; total: number }> {
  const value = ensureKernelTarget(target, "workflow run history");
  return (await apiClient.get(`${workflowBase(value)}/${encodeURIComponent(id)}/runs`, { params: { limit } })).data;
}
export async function getWorkflowRun(runId: string, target: WorkflowTarget = CLOUD_TARGET): Promise<{ run: WorkflowRun; stepRuns: WorkflowStepRun[]; attempts: WorkflowStepAttempt[]; checkpoints: WorkflowCheckpoint[]; workflow: WorkflowDefinition }> {
  return (await apiClient.get(`${workflowRunBase(target)}/${encodeURIComponent(runId)}`)).data;
}
export async function getWorkflowStats(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowExecutionStats> {
  const value = ensureKernelTarget(target, "workflow statistics");
  return (await apiClient.get<WorkflowExecutionStats>(`${workflowBase(value)}/${encodeURIComponent(id)}/stats`)).data;
}
export async function getWorkflowAnalysis(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowSafetyAnalysis> {
  const value = ensureKernelTarget(target, "workflow analysis");
  return (await apiClient.get<WorkflowSafetyAnalysis>(`${workflowBase(value)}/${encodeURIComponent(id)}/analysis`)).data;
}
export async function cancelWorkflowRun(runId: string, target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  await apiClient.post(`${workflowRunBase(target)}/${encodeURIComponent(runId)}/cancel`);
}
export async function pauseWorkflowRun(runId: string, reason = "Paused from Creative Workshop", target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  await apiClient.post(`${workflowRunBase(target)}/${encodeURIComponent(runId)}/pause`, { reason });
}
export async function resumeWorkflowRun(runId: string, target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  await apiClient.post(`${workflowRunBase(target)}/${encodeURIComponent(runId)}/resume`);
}
export async function rerunWorkflowRun(runId: string, wait = false, target: WorkflowTarget = CLOUD_TARGET): Promise<{ accepted?: boolean; executionId: string; workflowId?: string; status?: string; sourceExecutionId?: string }> {
  return (await apiClient.post(`${workflowRunBase(target)}/${encodeURIComponent(runId)}/rerun`, { wait })).data;
}
export async function recoverWorkflowRun(runId: string, target: WorkflowTarget = CLOUD_TARGET): Promise<{ accepted: boolean; executionId: string; workflowId: string; status: string; generation: number; checkpointCount: number }> {
  return (await apiClient.post(`${workflowRunBase(target)}/${encodeURIComponent(runId)}/recover`)).data;
}
export async function exportWorkflow(id: string, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowExportEnvelope> {
  const value = ensureKernelTarget(target, "workflow export");
  return (await apiClient.get<WorkflowExportEnvelope>(`${workflowBase(value)}/${encodeURIComponent(id)}/export`)).data;
}
export async function importWorkflow(payload: unknown, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  const value = ensureKernelTarget(target, "workflow import");
  return (await apiClient.post<WorkflowDefinition>(`${workflowBase(value)}/import`, payload, { headers: { "Content-Type": "application/json" } })).data;
}
export async function listWorkflowTemplates(target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowTemplateSummary[]> {
  const value = ensureKernelTarget(target, "workflow templates");
  const res = await apiClient.get<{ items: WorkflowTemplateSummary[] }>(`${workflowBase(value)}/templates`);
  return res.data.items ?? [];
}
export async function saveWorkflowTemplate(id: string, name = "", description = "", target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  const value = ensureKernelTarget(target, "workflow templates");
  await apiClient.post(`${workflowBase(value)}/${encodeURIComponent(id)}/templates`, { name, description });
}
export async function instantiateWorkflowTemplate(templateId: string, name = "", target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowDefinition> {
  const value = ensureKernelTarget(target, "workflow templates");
  return (await apiClient.post<WorkflowDefinition>(`${workflowBase(value)}/templates/${encodeURIComponent(templateId)}/instantiate`, { name })).data;
}
export async function deleteWorkflowTemplate(templateId: string, target: WorkflowTarget = CLOUD_TARGET): Promise<void> {
  const value = ensureKernelTarget(target, "workflow templates");
  await apiClient.delete(`${workflowBase(value)}/templates/${encodeURIComponent(templateId)}`);
}
export async function listWorkflowRevisions(id: string, limit = 50, target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowRevisionSummary[]> {
  const value = ensureKernelTarget(target, "workflow revisions");
  const res = await apiClient.get<{ items: WorkflowRevisionSummary[] }>(`${workflowBase(value)}/${encodeURIComponent(id)}/revisions`, { params: { limit } });
  return res.data.items ?? [];
}
export async function createWorkflowRevision(id: string, note = "", target: WorkflowTarget = CLOUD_TARGET): Promise<WorkflowRevisionSummary> {
  const value = ensureKernelTarget(target, "workflow revisions");
  return (await apiClient.post<WorkflowRevisionSummary>(`${workflowBase(value)}/${encodeURIComponent(id)}/revisions`, { note })).data;
}
export async function rollbackWorkflowRevision(id: string, revisionId: string, target: WorkflowTarget = CLOUD_TARGET, expectedRevision?: number): Promise<WorkflowDefinition> {
  const value = ensureKernelTarget(target, "workflow revisions");
  return (await apiClient.post<WorkflowDefinition>(`${workflowBase(value)}/${encodeURIComponent(id)}/revisions/${encodeURIComponent(revisionId)}/rollback`, undefined, { params: revisionParams(expectedRevision) })).data;
}
