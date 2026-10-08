import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

export interface OwnedContinuityDocument { thread: ContinuityThread; waits: ContinuityWait[]; events: ContinuityEvent[]; ownerId: string; coreId: string; modeRevision: number; executionScope: OwnedExecutionScope; managementExecutionScope?: OwnedExecutionScope; persistedExecutionScope?: OwnedExecutionScope; readOnly?: boolean; pausedReason?: string; lease?: { id: string; state: string } }
const authority = (scope: OwnedExecutionScope) => JSON.stringify(Object.entries(scope).filter(([key]) => !["requestId", "turnId", "executionId"].includes(key)).sort(([left], [right]) => left.localeCompare(right)));
function ownedData<T>(response: any): T { return response.data?.code === 200 ? response.data.data : response.data; }
function managementDocument(document: OwnedContinuityDocument, expected?: OwnedExecutionScope): OwnedContinuityDocument {
  const scope = document.managementExecutionScope;
  if (!scope || document.ownerId !== scope.resourceOwnerId || document.thread?.characterId !== scope.roleId || (expected && authority(scope) !== authority(expected))) throw new Error("持续事项的管理范围或数据归属无效，请重新加载");
  return { ...document, persistedExecutionScope: document.executionScope, executionScope: scope };
}
async function ownedRole(characterId?: unknown): Promise<string | null> {
  const mesh = useDeviceOwnedConversation();
  if (!await mesh.refresh()) {
    if ((await getDeploymentConfig()).mode === "cloud") throw new Error("当前 Core 的持续事项服务尚未就绪");
    return null;
  }
  const role = typeof characterId === "string" && characterId ? characterId : mesh.selectInitialRole();
  if (!role) throw new Error("请先指定持续事项使用的角色");
  return role;
}
async function ownedDocument(id: string, intent?: OwnedContinuityDocument): Promise<OwnedContinuityDocument | null> {
  const characterId = await ownedRole(intent?.executionScope.roleId);
  if (!characterId) { if (intent) throw new Error("持续事项服务已变化，请重新加载"); return null; }
  if (!intent?.executionScope || intent.thread.id !== id) throw new Error("请先加载持续事项并确认原始数据归属");
  if (intent.readOnly) return intent;
  const result = managementDocument(ownedData<OwnedContinuityDocument>(await apiClient.get("/api/device-mesh/v1/business/continuity", { params: { id, characterId } })));
  if (result.thread.id !== id || result.ownerId !== intent.ownerId || authority(result.executionScope) !== authority(intent.executionScope)) throw new Error("持续事项所属设备或角色已变化，请重新加载");
  return result;
}
async function ownedMutation(action: string, payload: Record<string, unknown>, current?: OwnedContinuityDocument, expected?: OwnedExecutionScope): Promise<OwnedContinuityDocument | null> {
  const original = current?.executionScope || expected;
  const characterId = await ownedRole(original?.roleId || payload.characterId);
  if (!characterId) { if (original) throw new Error("持续事项服务已变化，请重新加载"); return null; }
  if (!original || current?.readOnly) throw new Error("旧持续事项只读或缺少原始数据归属，请重新加载");
  const mesh = useDeviceOwnedConversation();
  if (original.coreId !== mesh.coreId.value || original.providerEpoch !== mesh.policy.value?.providerEpoch || original.modeRevision !== mesh.policy.value?.modeRevision || original.permissionRevision !== mesh.policy.value?.permissionRevision) throw new Error("持续事项的数据归属已变化，请重新加载");
  const requestId = crypto.randomUUID();
  const result = ownedData<{ document: OwnedContinuityDocument; acknowledgement: { requestId: string; ownerId: string; versions: Record<string, number> } }>(await apiClient.post("/api/device-mesh/v1/business/continuity", { ...payload, action, characterId, requestId, expectedRevision: current?.thread.revision || 0, expectedCoreId: original.coreId, expectedOwnerId: original.resourceOwnerId, expectedModeRevision: original.modeRevision, expectedExecutionScope: original }));
  if (authority(result.document.executionScope) !== authority(original) || result.document.executionScope.requestId !== requestId || result.acknowledgement.requestId !== requestId || result.document.ownerId !== original.resourceOwnerId || result.acknowledgement.ownerId !== original.resourceOwnerId || result.document.thread.id !== (current?.thread.id || `continuity/${requestId}`) || result.document.thread.revision !== (current?.thread.revision || 0) + 1 || result.acknowledgement.versions[`continuity/${result.document.thread.id}`] !== result.document.thread.revision || result.acknowledgement.versions[`checkpoint/continuity-operation/${requestId}`] !== 1) throw new Error("持续事项的数据持有方尚未确认保存");
  return result.document;
}

export type ThreadStatus = "active" | "waiting" | "blocked" | "paused" | "completed" | "cancelled";
export type WaitStatus = "waiting" | "resolved" | "cancelled";

export interface ContinuityThread {
  ownedDocument?: OwnedContinuityDocument;
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
  executionScope?: OwnedExecutionScope;
  ownerId?: string;
  coreId?: string;
  modeRevision?: number;
  readOnly?: boolean;
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
    if (params.historicalRoleId) {
      const mesh = useDeviceOwnedConversation();
      const selected = String(params.historicalRoleId);
      if (!(await mesh.historicalRoles(characterId)).some((role) => role.id === selected)) throw new Error("旧设备事项角色已变化，请重新选择");
      const collected = new Map<string, ContinuityThread>();
      let cursor = "";
      let expected = "";
      const cursors = new Set<string>();
      do {
        const page = await mesh.data("continuity", characterId, "", "", { historicalRoleId: selected, historicalCursor: cursor });
        const currentScope = authority(page.executionScope);
        if (expected && expected !== currentScope) throw new Error("旧持续事项的数据归属已变化，请重新加载");
        expected = currentScope;
        if (!page.historicalSnapshot) throw new Error("旧设备事项暂不可用");
        for (const row of page.historicalSnapshot.resources) {
          if (row.kind !== "continuity") continue;
          const document = row.body as OwnedContinuityDocument;
          if (row.ownerId !== page.historicalSnapshot.ownerId || row.roleId !== selected || document.thread?.id !== row.id) throw new Error("旧设备事项归属无效");
          collected.set(`${row.ownerId}/${row.id}`, { ...document.thread, ownedDocument: { ...document, ownerId: row.ownerId, executionScope: page.executionScope, readOnly: true } });
        }
        cursor = page.historicalSnapshot.nextCursors?.continuity || "";
        if (collected.size > 32768 || (cursor && cursors.has(cursor))) throw new Error("旧持续事项分页超过上限或游标重复");
        cursors.add(cursor);
      } while (cursor);
      return [...collected.values()].filter((thread) => (!params.status || thread.status === params.status) && (!params.q || `${thread.title} ${thread.goal || ""} ${thread.summary || ""}`.toLowerCase().includes(String(params.q).toLowerCase())));
    }
    const collected = new Map<string, OwnedContinuityDocument>();
    const cursors = new Set<string>();
    let cursor = "";
    let scope = "";
    do {
      const page: { documents: OwnedContinuityDocument[]; executionScope: OwnedExecutionScope; nextCursor?: string } = ownedData(await apiClient.get("/api/device-mesh/v1/business/continuity", { params: { characterId, pagination: "1", cursor } }));
      const currentScope = authority(page.executionScope);
      if (scope && scope !== currentScope) throw new Error("持续事项的数据归属已变化，请重新加载");
      scope = currentScope;
      if (!Array.isArray(page.documents)) throw new Error("持续事项分页数据无效");
      const mesh = useDeviceOwnedConversation();
      if (page.executionScope.coreId !== mesh.coreId.value || page.executionScope.roleId !== characterId) throw new Error("持续事项的数据归属已变化，请重新加载");
      for (const document of page.documents) {
        const presented = managementDocument(document, page.executionScope);
        collected.set(`${document.ownerId}/${document.thread.id}`, presented);
      }
      if (collected.size > 32768) throw new Error("持续事项数量超过加载上限，请缩小查询范围");
      cursor = page.nextCursor || "";
      if (cursor && cursors.has(cursor)) throw new Error("持续事项分页游标重复");
      cursors.add(cursor);
    } while (cursor);
    const documents = Array.from(collected.values());
    return documents.map((document) => ({ ...document.thread, ownedDocument: document })).filter((thread) => (!params.status || thread.status === params.status) && (!params.q || `${thread.title} ${thread.goal || ""} ${thread.summary || ""}`.toLowerCase().includes(String(params.q).toLowerCase())));
  }
  return (await apiClient.get<ContinuityThread[]>("/api/continuity/threads", { params })).data;
}

export async function createContinuityThread(payload: Record<string, unknown>, expected?: OwnedExecutionScope): Promise<ContinuityThread> {
  const document = await ownedMutation("create", payload, undefined, expected);
  if (document) return { ...document.thread, ownedDocument: document };
  return (await apiClient.post<ContinuityThread>("/api/continuity/threads", payload)).data;
}

export async function getContinuityThread(id: string, intent?: OwnedContinuityDocument): Promise<ContinuityDetail> {
  const document = await ownedDocument(id, intent);
  if (document) return { ...document, bindings: [] };
  return (await apiClient.get<ContinuityDetail>(`/api/continuity/threads/${encodeURIComponent(id)}`)).data;
}

export async function updateContinuityThread(id: string, payload: Record<string, unknown>, intent?: OwnedContinuityDocument): Promise<ContinuityThread> {
  const current = intent || await ownedDocument(id);
  if (current) {
    const action = payload.status === "paused" ? "pause" : payload.status === "active" ? "resume" : payload.status === "completed" ? "complete" : payload.status === "cancelled" ? "cancel" : "update";
    const document = await ownedMutation(action, { ...payload, id, updateGoal: Object.hasOwn(payload, "goal"), updateNextAction: Object.hasOwn(payload, "nextAction") }, current);
    if (!document) throw new Error("服务提供者已变化，请重新加载持续事项");
    return document.thread;
  }
  return (await apiClient.patch<ContinuityThread>(`/api/continuity/threads/${encodeURIComponent(id)}`, payload)).data;
}

export async function resolveContinuityWait(threadId: string, waitId: string, resume = true, intent?: OwnedContinuityDocument): Promise<ContinuityWait> {
  const current = intent || await ownedDocument(threadId);
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

export async function confirmContinuityExecution(id: string, leaseId: string, outcome: "completed" | "abandoned", result = "", intent?: OwnedContinuityDocument): Promise<void> {
  const current = intent || await ownedDocument(id);
  if (!current || current.lease?.id !== leaseId || current.lease.state !== "unknown") throw new Error("执行状态已变化，请刷新后再确认");
  if (!await ownedMutation("confirm_execution", { id, leaseId, outcome, result }, current)) throw new Error("服务提供者已变化，请重新加载持续事项");
}

export async function cancelContinuityWait(threadId: string, waitId: string, intent?: OwnedContinuityDocument): Promise<ContinuityWait> {
  const current = intent || await ownedDocument(threadId);
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

export async function createContinuityWait(threadId: string, payload: Record<string, unknown>, intent?: OwnedContinuityDocument): Promise<ContinuityWait> {
  const current = intent || await ownedDocument(threadId);
  if (current) {
    const document = await ownedMutation("add_wait", { id: threadId, wait: { ...payload, waitType: payload.waitType || payload.type, conditionJson: payload.conditionJson || JSON.stringify(payload.condition || {}), autoResume: payload.autoResume === undefined ? (payload.waitType || payload.type) === "time" : payload.autoResume } }, current);
    if (!document?.waits.length) throw new Error("等待条件尚未确认保存");
    return document.waits[document.waits.length - 1];
  }
  return (await apiClient.post<ContinuityWait>(`/api/continuity/threads/${encodeURIComponent(threadId)}/waits`, payload)).data;
}
