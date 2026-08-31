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
export interface WorkflowNodeRetryPolicy { maxAttempts?: number; initialBackoffMs?: number; maxBackoffMs?: number; multiplier?: number; jitter?: number }
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

export async function listWorkflows(): Promise<WorkflowDefinition[]> {
  const res = await apiClient.get<{ items: WorkflowDefinition[] }>("/api/extensions/workflows");
  return res.data.items ?? [];
}
export async function getWorkflowCatalog(): Promise<WorkflowCatalogItem[]> {
  const res = await apiClient.get<{ items: WorkflowCatalogItem[] }>("/api/extensions/workflows/catalog");
  return res.data.items ?? [];
}
export async function generateWorkflowWithAI(instruction: string): Promise<WorkflowAIProposal> {
  return (await apiClient.post<WorkflowAIProposal>("/api/extensions/workflows/ai/generate", { instruction })).data;
}
export async function editWorkflowWithAI(id: string, instruction: string): Promise<WorkflowAIProposal> {
  return (await apiClient.post<WorkflowAIProposal>(`/api/extensions/workflows/${encodeURIComponent(id)}/ai/edit`, { instruction })).data;
}
export async function repairWorkflowWithAI(id: string, instruction = ""): Promise<WorkflowAIProposal> {
  return (await apiClient.post<WorkflowAIProposal>(`/api/extensions/workflows/${encodeURIComponent(id)}/ai/repair`, { instruction })).data;
}
export async function explainWorkflowWithAI(id: string, instruction = ""): Promise<WorkflowAIExplanation> {
  return (await apiClient.post<WorkflowAIExplanation>(`/api/extensions/workflows/${encodeURIComponent(id)}/ai/explain`, { instruction })).data;
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
export async function getWorkflowRun(runId: string): Promise<{ run: WorkflowRun; stepRuns: WorkflowStepRun[]; attempts: WorkflowStepAttempt[]; checkpoints: WorkflowCheckpoint[]; workflow: WorkflowDefinition }> {
  return (await apiClient.get(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}`)).data;
}
export async function getWorkflowStats(id: string): Promise<WorkflowExecutionStats> {
  return (await apiClient.get<WorkflowExecutionStats>(`/api/extensions/workflows/${encodeURIComponent(id)}/stats`)).data;
}
export async function getWorkflowAnalysis(id: string): Promise<WorkflowSafetyAnalysis> {
  return (await apiClient.get<WorkflowSafetyAnalysis>(`/api/extensions/workflows/${encodeURIComponent(id)}/analysis`)).data;
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
export async function rerunWorkflowRun(runId: string, wait = false): Promise<{ accepted?: boolean; executionId: string; workflowId?: string; status?: string; sourceExecutionId?: string }> {
  return (await apiClient.post(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}/rerun`, { wait })).data;
}
export async function recoverWorkflowRun(runId: string): Promise<{ accepted: boolean; executionId: string; workflowId: string; status: string; generation: number; checkpointCount: number }> {
  return (await apiClient.post(`/api/extensions/workflow-runs/${encodeURIComponent(runId)}/recover`)).data;
}
export async function exportWorkflow(id: string): Promise<WorkflowExportEnvelope> {
  return (await apiClient.get<WorkflowExportEnvelope>(`/api/extensions/workflows/${encodeURIComponent(id)}/export`)).data;
}
export async function importWorkflow(payload: unknown): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>("/api/extensions/workflows/import", payload, { headers: { "Content-Type": "application/json" } })).data;
}
export async function listWorkflowTemplates(): Promise<WorkflowTemplateSummary[]> {
  const res = await apiClient.get<{ items: WorkflowTemplateSummary[] }>("/api/extensions/workflows/templates");
  return res.data.items ?? [];
}
export async function saveWorkflowTemplate(id: string, name = "", description = ""): Promise<void> {
  await apiClient.post(`/api/extensions/workflows/${encodeURIComponent(id)}/templates`, { name, description });
}
export async function instantiateWorkflowTemplate(templateId: string, name = ""): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>(`/api/extensions/workflows/templates/${encodeURIComponent(templateId)}/instantiate`, { name })).data;
}
export async function deleteWorkflowTemplate(templateId: string): Promise<void> {
  await apiClient.delete(`/api/extensions/workflows/templates/${encodeURIComponent(templateId)}`);
}
export async function listWorkflowRevisions(id: string, limit = 50): Promise<WorkflowRevisionSummary[]> {
  const res = await apiClient.get<{ items: WorkflowRevisionSummary[] }>(`/api/extensions/workflows/${encodeURIComponent(id)}/revisions`, { params: { limit } });
  return res.data.items ?? [];
}
export async function createWorkflowRevision(id: string, note = ""): Promise<WorkflowRevisionSummary> {
  return (await apiClient.post<WorkflowRevisionSummary>(`/api/extensions/workflows/${encodeURIComponent(id)}/revisions`, { note })).data;
}
export async function rollbackWorkflowRevision(id: string, revisionId: string): Promise<WorkflowDefinition> {
  return (await apiClient.post<WorkflowDefinition>(`/api/extensions/workflows/${encodeURIComponent(id)}/revisions/${encodeURIComponent(revisionId)}/rollback`)).data;
}

