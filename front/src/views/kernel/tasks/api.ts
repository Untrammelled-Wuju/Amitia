import { apiClient } from "@/composables/useApi";
import type {
  TaskDefinition,
  TaskRun,
  TaskRunProgress,
  TaskCheckpoint,
  TaskRunResult,
  EnqueueTaskRequest,
  EnqueueTaskResult,
  ListTasksFilter,
} from "./types";

const TASKS_BASE = "/api/extensions/tasks";
const DEFS_BASE = "/api/extensions/task-definitions";

export async function listTasks(filter: ListTasksFilter = {}): Promise<{ items: TaskRun[]; total: number }> {
  const res = await apiClient.get(TASKS_BASE, { params: filter });
  return res.data;
}

export async function getTask(taskRunId: string): Promise<TaskRun> {
  const res = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}`);
  return res.data;
}

export async function enqueueTask(req: EnqueueTaskRequest): Promise<EnqueueTaskResult> {
  const res = await apiClient.post(TASKS_BASE, req);
  return res.data;
}

export async function cancelTask(taskRunId: string, reason?: string): Promise<{ taskRunId: string; status: string }> {
  const res = await apiClient.post(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/cancel`, { reason: reason || "user_requested" });
  return res.data;
}

export async function retryTask(taskRunId: string): Promise<TaskRun> {
  const res = await apiClient.post(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/retry`);
  return res.data;
}

export async function recoverTask(taskRunId: string): Promise<TaskRun> {
  const res = await apiClient.post(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/recover`);
  return res.data;
}

export async function getTaskProgress(taskRunId: string): Promise<TaskRunProgress | null> {
  const res = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/progress`);
  return res.data;
}

export async function getTaskResult(taskRunId: string): Promise<TaskRunResult | null> {
  const res = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/result`);
  return res.data;
}

export async function getTaskCheckpoint(taskRunId: string): Promise<TaskCheckpoint | null> {
  const res = await apiClient.get(`${TASKS_BASE}/${encodeURIComponent(taskRunId)}/checkpoint`);
  return res.data;
}

export async function listTaskDefinitions(extensionId?: string): Promise<{ items: TaskDefinition[]; total: number }> {
  const res = await apiClient.get(DEFS_BASE, { params: { extensionId } });
  return res.data;
}

export async function getTaskDefinition(defId: string): Promise<TaskDefinition> {
  const res = await apiClient.get(`${DEFS_BASE}/${encodeURIComponent(defId)}`);
  return res.data;
}

export async function createTaskDefinition(def: TaskDefinition): Promise<TaskDefinition> {
  const res = await apiClient.post(DEFS_BASE, def);
  return res.data;
}
