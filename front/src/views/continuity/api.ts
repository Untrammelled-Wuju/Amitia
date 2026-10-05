import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";

interface OwnedContinuityDocument { thread: ContinuityThread; waits: ContinuityWait[]; events: ContinuityEvent[]; ownerId: string; coreId: string; modeRevision: number; pausedReason?: string; lease?: { id: string; state: string } }
const ownedRoles = new Map<string, string>();
function ownedData<T>(response: any): T { return response.data?.code === 200 ? response.data.data : response.data; }
async function ownedRole(characterId?: unknown): Promise<string | null> {
  const mesh = useDeviceOwnedConversation();
  if (!await mesh.refresh()) return null;
  const role = typeof characterId === "string" && characterId ? characterId : mesh.selectInitialRole();
  if (!role) throw new Error("请先指定持续事项使用的角色");
  return role;
}
async function ownedDocument(id: string): Promise<OwnedContinuityDocument | null> {
  const characterId = await ownedRole(ownedRoles.get(id));
  if (!characterId) return null;
  const result = ownedData<OwnedContinuityDocument>(await apiClient.get("/api/device-mesh/v1/business/continuity", { params: { id, characterId } }));
  ownedRoles.set(id, result.thread.characterId || characterId);
  return result;
}
async function ownedMutation(action: string, payload: Record<string, unknown>, current?: OwnedContinuityDocument): Promise<OwnedContinuityDocument | null> {
  const characterId = await ownedRole(current?.thread.characterId || payload.characterId);
  if (!characterId) return null;
  const mesh = useDeviceOwnedConversation();
  const result = ownedData<{ document: OwnedContinuityDocument; acknowledgement: { ownerId: string; versions: Record<string, number> } }>(await apiClient.post("/api/device-mesh/v1/business/continuity", { ...payload, action, characterId, requestId: crypto.randomUUID(), expectedRevision: current?.thread.revision || 0, expectedCoreId: current?.coreId || mesh.coreId.value, expectedOwnerId: current?.ownerId || mesh.roleOwnerId.value, expectedModeRevision: current?.modeRevision || mesh.policy.value?.modeRevision }));
  if (result.acknowledgement.ownerId !== result.document.ownerId || result.acknowledgement.versions[`continuity/${result.document.thread.id}`] !== result.document.thread.revision) throw new Error("持续事项的数据持有方尚未确认保存");
  ownedRoles.set(result.document.thread.id, characterId);
  return result.document;
}

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
	lease?: { id: string; state: string };
	pausedReason?: string;
  thread: ContinuityThread;
  waits: ContinuityWait[];
  events: ContinuityEvent[];
  bindings: Array<Record<string, unknown>>;
}

export async function listContinuityThreads(params: Record<string, unknown> = {}): Promise<ContinuityThread[]> {
  const characterId = await ownedRole(params.characterId);
  if (characterId) {
    const collected = new Map<string, OwnedContinuityDocument>();
    const cursors = new Set<string>();
    let cursor = "";
    let scope = "";
    do {
      const page = ownedData<{ documents: OwnedContinuityDocument[]; executionScope: Record<string, unknown>; nextCursor?: string }>(await apiClient.get("/api/device-mesh/v1/business/continuity", { params: { characterId, pagination: "1", cursor } }));
      const currentScope = JSON.stringify(page.executionScope, (key, value) => ["requestId", "turnId", "executionId"].includes(key) ? undefined : value);
      if (scope && scope !== currentScope) throw new Error("持续事项的数据归属已变化，请重新加载");
      scope = currentScope;
      if (!Array.isArray(page.documents)) throw new Error("持续事项分页数据无效");
      for (const document of page.documents) collected.set(document.thread.id, document);
      if (collected.size > 32768) throw new Error("持续事项数量超过加载上限，请缩小查询范围");
      cursor = page.nextCursor || "";
      if (cursor && cursors.has(cursor)) throw new Error("持续事项分页游标重复");
      cursors.add(cursor);
    } while (cursor);
    const documents = Array.from(collected.values());
    for (const document of documents) ownedRoles.set(document.thread.id, document.thread.characterId || characterId);
    return documents.map((document) => document.thread).filter((thread) => (!params.status || thread.status === params.status) && (!params.q || `${thread.title} ${thread.goal || ""} ${thread.summary || ""}`.toLowerCase().includes(String(params.q).toLowerCase())));
  }
  return (await apiClient.get<ContinuityThread[]>("/api/continuity/threads", { params })).data;
}

export async function createContinuityThread(payload: Record<string, unknown>): Promise<ContinuityThread> {
  const document = await ownedMutation("create", payload);
  if (document) return document.thread;
  return (await apiClient.post<ContinuityThread>("/api/continuity/threads", payload)).data;
}

export async function getContinuityThread(id: string): Promise<ContinuityDetail> {
  const document = await ownedDocument(id);
  if (document) return { ...document, bindings: [] };
  return (await apiClient.get<ContinuityDetail>(`/api/continuity/threads/${encodeURIComponent(id)}`)).data;
}

export async function updateContinuityThread(id: string, payload: Record<string, unknown>): Promise<ContinuityThread> {
  const current = await ownedDocument(id);
  if (current) {
    const action = payload.status === "paused" ? "pause" : payload.status === "active" ? "resume" : payload.status === "completed" ? "complete" : payload.status === "cancelled" ? "cancel" : "update";
    const document = await ownedMutation(action, { ...payload, id, updateGoal: Object.hasOwn(payload, "goal"), updateNextAction: Object.hasOwn(payload, "nextAction") }, current);
    if (!document) throw new Error("服务提供者已变化，请重新加载持续事项");
    return document.thread;
  }
  return (await apiClient.patch<ContinuityThread>(`/api/continuity/threads/${encodeURIComponent(id)}`, payload)).data;
}

export async function resolveContinuityWait(threadId: string, waitId: string, resume = true): Promise<ContinuityWait> {
  const current = await ownedDocument(threadId);
  if (current) {
    const document = await ownedMutation("resolve_wait", { id: threadId, waitId, resume }, current);
    const wait = document?.waits.find((item) => item.id === waitId);
    if (!wait) throw new Error("等待条件已变化，请重新加载");
    return wait;
  }
  return (await apiClient.post<ContinuityWait>(
    `/api/continuity/threads/${encodeURIComponent(threadId)}/waits/${encodeURIComponent(waitId)}/resolve`,
    { resume },
  )).data;
}

export async function confirmContinuityExecution(id: string, leaseId: string, outcome: "completed" | "abandoned", result = ""): Promise<void> {
  const current = await ownedDocument(id);
  if (!current || current.lease?.id !== leaseId || current.lease.state !== "unknown") throw new Error("执行状态已变化，请刷新后再确认");
  if (!await ownedMutation("confirm_execution", { id, leaseId, outcome, result }, current)) throw new Error("服务提供者已变化，请重新加载持续事项");
}

export async function cancelContinuityWait(threadId: string, waitId: string): Promise<ContinuityWait> {
  const current = await ownedDocument(threadId);
  if (current) {
    const document = await ownedMutation("cancel_wait", { id: threadId, waitId }, current);
    const wait = document?.waits.find((item) => item.id === waitId);
    if (!wait) throw new Error("等待条件已变化，请重新加载");
    return wait;
  }
  return (await apiClient.post<ContinuityWait>(
    `/api/continuity/threads/${encodeURIComponent(threadId)}/waits/${encodeURIComponent(waitId)}/cancel`,
  )).data;
}

export async function createContinuityWait(threadId: string, payload: Record<string, unknown>): Promise<ContinuityWait> {
  const current = await ownedDocument(threadId);
  if (current) {
    const document = await ownedMutation("add_wait", { id: threadId, wait: { ...payload, waitType: payload.waitType || payload.type, conditionJson: payload.conditionJson || JSON.stringify(payload.condition || {}), autoResume: payload.autoResume === undefined ? (payload.waitType || payload.type) === "time" : payload.autoResume } }, current);
    if (!document?.waits.length) throw new Error("等待条件尚未确认保存");
    return document.waits[document.waits.length - 1];
  }
  return (await apiClient.post<ContinuityWait>(`/api/continuity/threads/${encodeURIComponent(threadId)}/waits`, payload)).data;
}
