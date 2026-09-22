import { apiClient } from "@/composables/useApi";

export type ThreadStatus = "active" | "waiting" | "blocked" | "paused" | "completed" | "cancelled";
export type WaitStatus = "waiting" | "resolved" | "cancelled";

export interface ContinuityThread {
  id: string;
  spaceId: string;
  characterId?: string;
  parentThreadId?: string;
  title: string;
  goal?: string;
  status: ThreadStatus;
  summary?: string;
  currentState?: string;
  nextAction?: string;
  priority: number;
  confidence: number;
  revision: number;
  createdAt: string;
  updatedAt: string;
  lastActiveAt: string;
  completedAt?: string;
}

export interface ContinuityWait {
  id: string;
  threadId: string;
  waitType: "user" | "time" | "device" | "external" | "approval" | "dependency";
  status: WaitStatus;
  description?: string;
  conditionJson: string;
  resumeHint?: string;
  dueAt?: string;
  resolvedAt?: string;
  resolvedBy?: string;
  autoResume: boolean;
  wakeState?: string;
  wakeAttempts: number;
  lastWakeError?: string;
  createdAt: string;
}

export interface ContinuityEvent {
  id: string;
  eventType: string;
  sourceType?: string;
  sourceId?: string;
  payloadJson: string;
  occurredAt: string;
}

export interface ContinuityDetail {
  thread: ContinuityThread;
  waits: ContinuityWait[];
  events: ContinuityEvent[];
  bindings: Array<Record<string, unknown>>;
}

export async function listContinuityThreads(params: Record<string, unknown> = {}): Promise<ContinuityThread[]> {
  return (await apiClient.get<ContinuityThread[]>("/api/continuity/threads", { params })).data;
}

export async function createContinuityThread(payload: Record<string, unknown>): Promise<ContinuityThread> {
  return (await apiClient.post<ContinuityThread>("/api/continuity/threads", payload)).data;
}

export async function getContinuityThread(id: string): Promise<ContinuityDetail> {
  return (await apiClient.get<ContinuityDetail>(`/api/continuity/threads/${encodeURIComponent(id)}`)).data;
}

export async function updateContinuityThread(id: string, payload: Record<string, unknown>): Promise<ContinuityThread> {
  return (await apiClient.patch<ContinuityThread>(`/api/continuity/threads/${encodeURIComponent(id)}`, payload)).data;
}

export async function resolveContinuityWait(threadId: string, waitId: string, resume = true): Promise<ContinuityWait> {
  return (await apiClient.post<ContinuityWait>(
    `/api/continuity/threads/${encodeURIComponent(threadId)}/waits/${encodeURIComponent(waitId)}/resolve`,
    { resume },
  )).data;
}

export async function cancelContinuityWait(threadId: string, waitId: string): Promise<ContinuityWait> {
  return (await apiClient.post<ContinuityWait>(
    `/api/continuity/threads/${encodeURIComponent(threadId)}/waits/${encodeURIComponent(waitId)}/cancel`,
  )).data;
}

export async function createContinuityWait(threadId: string, payload: Record<string, unknown>): Promise<ContinuityWait> {
  return (await apiClient.post<ContinuityWait>(`/api/continuity/threads/${encodeURIComponent(threadId)}/waits`, payload)).data;
}
