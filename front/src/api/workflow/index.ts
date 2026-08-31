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
export interface WorkflowOnError { mode?: string; default?: unknown }
export interface WorkflowStepInput { input?: unknown; when?: unknown; onError: WorkflowOnError }
export interface WorkflowNode {
  id: string;
  type: string;
  dependsOn?: string[];
  targetId?: string;
  runtime: WorkflowRuntimeBinding;
  permissions?: string[];
  scope?: string;
  position?: WorkflowPosition;
  label?: string;
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
  enabled: boolean;
  hasSideEffects?: boolean;
  idempotent?: boolean;
  limits?: Record<string, unknown>;
  version?: string;
  source?: string;
  metadata?: Record<string, unknown>;
  definitionHash?: string;
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

export async function listWorkflows(): Promise<WorkflowDefinition[]> {
  const res = await apiClient.get<{ items: WorkflowDefinition[] }>("/api/extensions/workflows");
  return res.data.items ?? [];
}
export async function createWorkflow(def: Partial<WorkflowDefinition>): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>("/api/extensions/workflows", def)).data;
}
export async function getWorkflow(id: string): Promise<WorkflowDefinition> {
  return (await apiClient.get<WorkflowDefinition>(`/api/extensions/workflows/${encodeURIComponent(id)}`)).data;
}
export async function updateWorkflow(id: string, def: WorkflowDefinition): Promise<WorkflowDefinition> {
  return (await apiClient.put<WorkflowDefinition>(`/api/extensions/workflows/${encodeURIComponent(id)}`, def)).data;
}
export async function patchWorkflow(id: string, patch: Partial<WorkflowDefinition>): Promise<WorkflowDefinition> {
  return (await apiClient.patch<WorkflowDefinition>(`/api/extensions/workflows/${encodeURIComponent(id)}`, patch)).data;
}
export async function duplicateWorkflow(id: string): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>(`/api/extensions/workflows/${encodeURIComponent(id)}/duplicate`)).data;
}
export async function deleteWorkflow(id: string): Promise<void> {
  await apiClient.delete(`/api/extensions/workflows/${encodeURIComponent(id)}`);
}
export async function setWorkflowEnabled(id: string, enabled: boolean): Promise<void> {
  await apiClient.post(`/api/extensions/workflows/${encodeURIComponent(id)}/${enabled ? "enable" : "disable"}`);
}
export async function validateWorkflow(def: WorkflowDefinition): Promise<{ valid: boolean; error?: string; topologicalOrder?: string[] }> {
  return (await apiClient.post("/api/extensions/workflows/validate", def)).data;
}
export async function runWorkflow(id: string, input: unknown = {}, wait = false): Promise<{ accepted?: boolean; executionId: string; status?: string }> {
  return (await apiClient.post(`/api/extensions/workflows/${encodeURIComponent(id)}/run`, { input, wait })).data;
}
export async function dispatchWorkflowEvent(eventType: string, payload: unknown = {}): Promise<{ accepted: boolean; eventType: string }> {
  return (await apiClient.post(`/api/extensions/workflows/events/${encodeURIComponent(eventType)}`, payload)).data;
}
export async function listWorkflowRuns(id: string, limit = 50): Promise<{ items: WorkflowRun[]; total: number }> {
  return (await apiClient.get(`/api/extensions/workflows/${encodeURIComponent(id)}/runs`, { params: { limit } })).data;
}
export async function getWorkflowRun(runId: string): Promise<{ run: WorkflowRun; stepRuns: WorkflowStepRun[]; workflow: WorkflowDefinition }> {
  return (await apiClient.get(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}`)).data;
}
export async function cancelWorkflowRun(runId: string): Promise<void> {
  await apiClient.post(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}/cancel`);
}
export async function pauseWorkflowRun(runId: string, reason = "Paused from Creative Workshop"): Promise<void> {
  await apiClient.post(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}/pause`, { reason });
}
export async function resumeWorkflowRun(runId: string): Promise<void> {
  await apiClient.post(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}/resume`);
}
