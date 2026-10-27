import { computed, ref } from "vue";
import { apiClient } from "./useApi";
import { getDeploymentConfig, getNativeProviderTransition } from "@/runtime/runtime-adapter";
import { streamOwnedChat, streamOwnedGroupChat, type OwnedChatEvent, type OwnedChatRequest, type OwnedExecutionScope, type OwnedGroupChatRequest, type OwnedGroupChatResponse, type OwnedGroupDelivery, type OwnedGroupState } from "@/runtime/device-owned-chat";
import { ownedImageURL, ownedAudioURL, ownedMessageText, ownedAttachmentURL } from "@/runtime/device-owned-attachments";
import { conversationReference, parseConversationReference, ownedConversationRow, type OwnedConversationOrigin } from "@/runtime/device-owned-conversation-reference";
import { createOwnedConversationExport } from "@/runtime/device-owned-export";
import { OwnedChatStateChangedError } from "@/runtime/owned-chat-poll";
import { memoryRecordScope, memoryStorageRole } from "@/runtime/owned-memory-scope";
import { ownedWorkbenchTaskChange, workbenchTaskEventName } from "@/runtime/owned-workbench-task-event";
import {
  deviceProviderChangedNotice,
  deviceProviderTransitionNotice,
  deviceUnavailableNotice,
  type DeviceServiceMode,
} from "@/runtime/device-service-label";

interface MeshRole { id: string; name: string; avatar?: string; revision: number }
export interface OwnedProjectWorkspace { id: string; deviceId: string; name: string; kind: string; rootUri: string; readOnly?: boolean }
export interface OwnedProject { id: string; title: string; workspaces: OwnedProjectWorkspace[]; primaryWorkspaceId: string; hiddenAt?: string | null; revision: number; ownerId: string; roleId: string; readOnly: boolean; createdAt: string; updatedAt: string; pinnedAt?: string | null; executionScope: OwnedExecutionScope }
interface MeshPolicy { coordinated: boolean; modeRevision: number; providerEpoch: number; permissionRevision: number; selectedRole: string }
interface MeshResource { kind: string; id: string; roleId: string; ownerId: string; revision: number; deleted: boolean; origin?: string; readOnly?: boolean; body: any; executionScope?: OwnedExecutionScope }
interface MeshSnapshot { ownerId: string; resources: MeshResource[]; nextCursors?: Record<string, string>; group?: OwnedGroupState | null }
interface MeshQueryResult { conversationReference?: string; conversationOrigin?: OwnedConversationOrigin; executionScope: OwnedExecutionScope; snapshot: MeshSnapshot; historicalSnapshot?: MeshSnapshot; historicalConversations?: any[]; nextHistoricalListCursor?: string; deliveryFailures?: Array<{ ownerId: string; requestId: string; conversationId?: string; errorCode: string; failedAt: string }> }

const enabled = ref(false);
const roles = ref<MeshRole[]>([]);
export const ownedConversationSwitchNotice = "会话已切换，原回复已中断";
const providerStateKey = "amitia.mesh.provider-state";
const providerStateVersion = 2;
interface StoredProviderState {
  version: number;
  mode: DeviceServiceMode;
  binding: string;
  changeId?: string;
}
function readProviderState(): StoredProviderState | null {
  try {
    const state = JSON.parse(localStorage.getItem(providerStateKey) || "null");
    if (state?.version !== providerStateVersion || !["local", "cloud"].includes(state?.mode) || typeof state?.binding !== "string" || !state.binding) return null;
    return { version: providerStateVersion, mode: state.mode, binding: state.binding, changeId: typeof state.changeId === "string" ? state.changeId : undefined };
  } catch { return null; }
}
const coreId = ref("");
const roleOwnerId = ref("");
const policy = ref<MeshPolicy | null>(null);
const notice = ref("");
let transientNotice = "";
let providerChangeId = "";
let providerMode: DeviceServiceMode = "local";
let providerBinding = "";
let activeController: AbortController | null = null;
let activeRequest = "";
let revision = 0;
let refreshInFlight: Promise<boolean> | undefined;
const resourceCache = new Map<string, MeshResource>();
const summaries = new Map<string, string>();
const summaryViews = new Map<string, { summaryViewId: string; summaryText: string; sourceOwnerId: string; sourceResourceId: string; sourceRevision: number; sourceScope: OwnedExecutionScope; editable: boolean }>();
const historyPages = new Map<string, { scope: string; cursors: Record<string, string> }>();
interface MeshQueryResult { conversationId?: string }
const summaryPreparations = new Map<string, { viewId: string; scope: OwnedExecutionScope; expectedRevision: number; resourceId: string }>();

function authorityStamp(scope: OwnedExecutionScope) {
  return JSON.stringify(Object.entries(scope).filter(([key]) => !["requestId", "turnId", "executionId"].includes(key)).sort(([left], [right]) => left.localeCompare(right)));
}
const historyRequests = new Map<string, symbol>();
const conversationLoads = new Map<string, Promise<any[]>>();

function saveProviderState() {
  if (!providerBinding) return;
  try {
    const state: StoredProviderState = { version: providerStateVersion, mode: providerMode, binding: providerBinding, changeId: providerChangeId || undefined };
    localStorage.setItem(providerStateKey, JSON.stringify(state));
  } catch {}
}

function unwrap<T>(response: any): T {
  return response?.data?.code === 200 && response.data.data !== undefined ? response.data.data : response.data;
}

function stamp(core: string, state: MeshPolicy | null) {
  return `${core}:${state?.providerEpoch}:${state?.modeRevision}:${state?.permissionRevision}`;
}

function clearOwnedCaches() {
  resourceCache.clear();
  summaryViews.clear();
  summaryPreparations.clear();
  historyPages.clear();
  historyRequests.clear();
  conversationLoads.clear();
}

function invalidateWithoutInterrupt() {
  revision++;
  clearOwnedCaches();
}

function stopLocal(message: string, _temporary = false) {
  revision++;
  refreshInFlight = undefined;
  clearOwnedCaches();
  activeController?.abort();
  activeController = null;
  activeRequest = "";
  notice.value = message;
  transientNotice = message;
  window.dispatchEvent(new CustomEvent("amitia:execution-scope-changed", { detail: { reason: message } }));
}

function noteWithoutInterrupt(message: string) {
  transientNotice = message;
  notice.value = message;
}

function markUnavailable() {
  const message = deviceUnavailableNotice(providerMode);
  if (notice.value === message && !activeController) return;
  stopLocal(message, true);
}

window.addEventListener("amitia:runtime-connection-changed", () => {
  const interrupted = activeController !== null;
  coreId.value = "";
  providerBinding = "";
  providerChangeId = "";
  notice.value = "";
  transientNotice = "";
  enabled.value = false;
  roles.value = [];
  roleOwnerId.value = "";
  policy.value = null;
  if (interrupted) stopLocal("设备服务连接正在变化，当前回复已中断；请等待当前服务确认");
  else invalidateWithoutInterrupt();
});

export function useDeviceOwnedConversation() {
	async function data(kind: string, characterId: string, conversationId = "", cursor = "", historical: { memoryView?: string; legacyCursor?: string; historicalRoleId?: string; historicalCursor?: string; historicalLegacyCursor?: string } = {}): Promise<MeshQueryResult> {
	  const captured = revision;
	  const result = unwrap<MeshQueryResult>(await apiClient.get("/api/device-mesh/v1/business/data", { params: { kind, characterId, conversationId, cursor, ...historical } }));
	  if (captured !== revision || result.executionScope.coreId !== coreId.value) throw new Error("服务状态已变化，请重新加载记忆");
	  for (const resource of result.snapshot.resources) resourceCache.set(`${resource.kind}/${resource.id}`, { ...resource, executionScope: memoryRecordScope(result.executionScope, resource.roleId, resource.kind) });
	  return result;
	}
	async function historicalRoles(characterId: string) {
	  const captured = revision;
	  const result = unwrap<{ roles: Array<{ id: string; name: string }>; executionScope: OwnedExecutionScope }>(await apiClient.get("/api/device-mesh/v1/business/historical-roles", { params: { characterId } }));
	  if (captured !== revision || result.executionScope.coreId !== coreId.value) throw new Error("服务状态已变化，旧记忆目录已丢弃");
	  return result.roles;
	}
  function refresh(): Promise<boolean> {
    if (refreshInFlight) return refreshInFlight;
    const pending = refreshState().finally(() => {
      if (refreshInFlight === pending) refreshInFlight = undefined;
    });
    refreshInFlight = pending;
    return pending;
  }

  async function refreshState() {
    const deployment = await getDeploymentConfig();
    const mode = deployment.mode;
    const binding = mode === "cloud" ? `cloud:${deployment.serverURL || ""}` : "local";
    const bindingChanged = providerBinding !== "" && (providerMode !== mode || providerBinding !== binding);
    providerMode = mode;
    providerBinding = binding;
    const stored = readProviderState();
    providerChangeId = stored?.mode === mode && stored.binding === binding ? String(stored.changeId || "") : "";
    if (bindingChanged) {
      const interrupted = activeController !== null;
      notice.value = "";
      transientNotice = "";
      coreId.value = "";
      policy.value = null;
      roles.value = [];
      roleOwnerId.value = "";
      enabled.value = false;
      if (interrupted) stopLocal("设备服务连接正在变化，当前回复已中断；请等待当前服务确认");
      else invalidateWithoutInterrupt();
    }
    let transition: Awaited<ReturnType<typeof getNativeProviderTransition>> = null;
    if (mode === "cloud") {
      transition = await getNativeProviderTransition();
      if (transition?.providerChangePending) {
        enabled.value = true;
        roles.value = [];
        stopLocal(deviceProviderTransitionNotice(mode, transition.coreId || coreId.value, transition.successorCoreId || "新 Core"));
        throw new Error(notice.value);
      }
    }
    if (mode === "cloud") enabled.value = true;
    const captured = revision;
    const current = unwrap<any>(await apiClient.get("/api/device-mesh/v1/coordination/me"));
    const latestDeployment = await getDeploymentConfig();
    if (captured !== revision || latestDeployment.mode !== deployment.mode || latestDeployment.serverURL !== deployment.serverURL) throw new OwnedChatStateChangedError("服务提供者已变化，旧状态已丢弃");
    const previous = stamp(coreId.value, policy.value);
    const next = stamp(String(current.coreId || ""), current.policy || null);
    const coreChanged = Boolean(coreId.value && coreId.value !== current.coreId);
    const authorityChanged = Boolean(enabled.value && policy.value && previous !== next);
    if (coreChanged || authorityChanged) {
      const message = coreChanged
        ? deviceProviderChangedNotice(mode, coreId.value, String(current.coreId || ""), activeController ? "当前回复已中断。" : "旧数据已失效。")
        : activeController
          ? "数据归属或调用权限已变化，当前回复已中断；继续发送时将使用当前 Core。"
          : "数据归属或调用权限已更新，旧数据已失效。";
      stopLocal(message);
    }
    if (transition?.coreId === current.coreId && transition?.providerChangeId && transition.previousCoreId && transition.previousCoreId !== current.coreId && providerChangeId !== transition.providerChangeId) {
      providerChangeId = transition.providerChangeId;
      stopLocal(deviceProviderChangedNotice(mode, transition.previousCoreId, current.coreId, "当前回复已中断。"));
    }
    coreId.value = String(current.coreId || "");
    saveProviderState();
    policy.value = current.policy || null;
    enabled.value = current.coordinationAvailable === true;
    if (!enabled.value && mode === "cloud") { enabled.value = true; roles.value = []; roleOwnerId.value = ""; stopLocal("云端设备对话服务尚未就绪，当前回复已中断"); throw new Error(notice.value); }
    if (!enabled.value) return false;
    const roleGeneration = revision;
    const available = unwrap<any>(await apiClient.get("/api/device-mesh/v1/business/roles"));
    const finalDeployment = await getDeploymentConfig();
    if (roleGeneration !== revision || finalDeployment.mode !== deployment.mode || finalDeployment.serverURL !== deployment.serverURL) throw new OwnedChatStateChangedError("服务提供者已变化，旧角色已丢弃");
    const nextRoles: MeshRole[] = Array.isArray(available.roles) ? available.roles : [];
    if (roles.value.some(role => nextRoles.find(item => item.id === role.id)?.revision !== role.revision)) noteWithoutInterrupt("角色配置已变化");
    roles.value = nextRoles;
    roleOwnerId.value = String(available.roleOwnerId || "");
    const unavailable = deviceUnavailableNotice(mode);
    if (notice.value === unavailable) {
      notice.value = "";
      if (transientNotice === unavailable) transientNotice = "";
      saveProviderState();
    }
    return true;
  }

  function selectInitialRole(saved?: string): string {
    const requested = policy.value?.selectedRole || saved || "";
    if (requested) return roles.value.some((role) => role.id === requested) ? requested : "";
    return roles.value.length === 1 ? roles.value[0].id : "";
  }

  const historyKey = (conversationId: string, characterId: string, keyword = "") => JSON.stringify([conversationId, characterId, keyword]);

  async function query(conversationId: string, characterId: string, older = false, keyword = "", requestScope = ""): Promise<MeshQueryResult> {
    const origin = parseConversationReference(conversationId);
    const path = "/api/device-mesh/v1/business/conversations" + (conversationId ? `/${encodeURIComponent(origin?.id || conversationId)}` : "");
    const captured = revision;
    const key = historyKey(conversationId, characterId, keyword);
    const ticketKey = requestScope ? `${key}\u0000${requestScope}` : key;
    const ticket = Symbol(ticketKey);
    historyRequests.set(ticketKey, ticket);
    if (historyRequests.size > 128) historyRequests.delete(historyRequests.keys().next().value!);
    const previous = historyPages.get(key);
    const kind = conversationId ? "message" : "conversation";
    const legacyKind = conversationId ? "legacyMessage" : "legacyConversation";
    const cursors = older ? previous?.cursors || {} : {};
    const params = { characterId, ...(keyword ? { keyword } : {}), ...(origin ? { conversationOwnerId: origin.ownerId } : {}), ...(older ? { resourceKind: kind, ...cursors } : {}) };
    const result = unwrap<MeshQueryResult>(await apiClient.get(path, { params }));
    if (captured !== revision || historyRequests.get(ticketKey) !== ticket) throw new Error("服务状态或加载请求已变化，旧上下文已丢弃");
    const scope = JSON.stringify([result.executionScope.spaceId, result.executionScope.initiatorDeviceId, result.executionScope.targetDeviceId, result.executionScope.coreId, result.executionScope.providerEpoch, result.executionScope.targetProviderEpoch, result.executionScope.coordinated, result.executionScope.modeRevision, result.executionScope.permissionRevision, result.executionScope.targetPermissionRevision, result.executionScope.roleOwnerId, result.executionScope.resourceOwnerId, result.executionScope.roleId, result.executionScope.roleRevision, result.executionScope.conversationRoleId]);
    if (older && previous?.scope !== scope) throw new Error("服务或角色已变化，请重新加载当前对话");
    const next: Record<string, string> = {};
    for (const [parameter, snapshot, cursorKind] of [
      ["cursor", result.snapshot, kind], ["legacyCursor", result.snapshot, legacyKind],
      ["historicalCursor", result.historicalSnapshot, kind], ["historicalLegacyCursor", result.historicalSnapshot, legacyKind],
    ] as const) {
      if (older && !cursors[parameter]) continue;
      const value = snapshot?.nextCursors?.[cursorKind];
      if (value) next[parameter] = value;
    }
    if (!conversationId && (!older || cursors.historicalListCursor) && result.nextHistoricalListCursor) next.historicalListCursor = result.nextHistoricalListCursor;
    historyPages.set(key, { scope, cursors: next });
    if (historyPages.size > 64) {
      const oldest = historyPages.keys().next().value!;
      historyPages.delete(oldest);
      historyRequests.delete(oldest);
    }
    for (const resource of result.snapshot.resources) {
      const cached = { ...resource, executionScope: result.executionScope };
      resourceCache.set(`${resource.kind}/${resource.id}`, cached);
      if (resource.kind === "conversation") resourceCache.set(`conversation/${ownedConversationRow(resource.body, result.snapshot.ownerId).id}`, cached);
    }
    if (conversationId) {
      const summary = result.snapshot.resources.find((resource) => resource.kind === "summary")?.body?.content;
      const text = String(summary?.summary || summary?.text || (typeof summary === "string" ? summary : "")).slice(0, 16000);
      if (text) {
        summaries.set(`${result.executionScope.coreId}/${conversationId}`, text);
        if (summaries.size > 32) summaries.delete(summaries.keys().next().value!);
      }
    }
    return { ...result, ...(conversationId ? { conversationReference: conversationId } : {}) };
  }

  function hasMore(conversationId: string, characterId: string, keyword = ""): boolean {
    return Object.keys(historyPages.get(historyKey(conversationId, characterId, keyword))?.cursors || {}).length > 0;
  }

  async function loadConversations(characterId: string, keyword: string): Promise<any[]> {
    const rows = new Map<string, any>();
    const visited = new Set<string>();
    let result = await query("", characterId, false, keyword);
    for (;;) {
      for (const value of result.historicalConversations || []) {
        if (value?.id) {
          const row = ownedConversationRow(value, value.ownerId || result.executionScope.targetDeviceId);
          if (!rows.has(row.id)) rows.set(row.id, row);
        }
      }
      for (const snapshot of [result.historicalSnapshot, result.snapshot]) {
        if (!snapshot) continue;
        for (const value of snapshot.resources.filter((resource) => resource.kind === "conversation").map((resource) => resource.body)) {
          if (value?.id) {
            const row = ownedConversationRow(value, snapshot.ownerId);
            rows.set(row.id, row);
          }
        }
      }
      if (!hasMore("", characterId, keyword)) return [...rows.values()];
      const cursor = JSON.stringify(historyPages.get(historyKey("", characterId, keyword))?.cursors);
      if (visited.has(cursor) || rows.size > 32768) throw new Error("会话分页结果无效，请重新加载");
      visited.add(cursor);
      result = await query("", characterId, true, keyword);
    }
  }

  async function conversations(characterId: string, keyword = ""): Promise<any[]> {
    keyword = keyword.trim().toLowerCase();
    const key = historyKey("", characterId, keyword);
    const previous = conversationLoads.get(key);
    if (previous) return previous;
    const pending = loadConversations(characterId, keyword);
    conversationLoads.set(key, pending);
    try { return await pending; }
    finally { if (conversationLoads.get(key) === pending) conversationLoads.delete(key); }
  }

  async function allMessages(conversationId: string, characterId: string): Promise<any[]> {
    const captured = revision;
    const rows = new Map<string, any>();
    const visited = new Set<string>();
    const sizes = new Map<string, number>();
    let bytes = 0;
    let result = await query(conversationId, characterId);
    for (;;) {
      if (captured !== revision) throw new Error("服务状态已变化，旧历史已丢弃");
      for (const row of messages(result)) {
        const key = row.uiKey || `${row.ownerId}:${row.id}`;
        const previous = rows.get(key);
        if (previous && previous.sourceRevision !== row.sourceRevision) throw new Error("历史记录在读取期间已变化，请重新加载");
        const size = new TextEncoder().encode(JSON.stringify(row)).byteLength;
        bytes += size - (sizes.get(key) || 0);
        sizes.set(key, size);
        rows.set(key, { ...row, conversationId });
      }
      if (rows.size > 4096 || bytes > 32 * 1024 * 1024) throw new Error("历史记录超过单次读取上限，请分段读取");
      if (!hasMore(conversationId, characterId)) return [...rows.values()].sort((a, b) => String(a.createdAt).localeCompare(String(b.createdAt)));
      const cursor = JSON.stringify(historyPages.get(historyKey(conversationId, characterId))?.cursors);
      if (visited.has(cursor)) throw new Error("历史分页结果无效，请重新加载");
      visited.add(cursor);
      result = await query(conversationId, characterId, true);
    }
  }

  async function exportConversation(conversationId: string, characterId: string, format = "json") {
    if (!enabled.value) throw new Error("设备数据服务尚未就绪");
    const captured = revision;
    const rows = await allMessages(conversationId, characterId);
    if (captured !== revision) throw new Error("服务状态已变化，旧导出已丢弃");
    return createOwnedConversationExport(conversationId, rows, format);
  }

  async function edit(kind: "conversation" | "message" | "memory" | "summary" | "project" | "emotion", id: string, changes: Record<string, unknown> = {}, options: { deleted?: boolean; clear?: boolean; characterId?: string; expectedExecutionScope?: OwnedExecutionScope; expectedOwnerId?: string; expectedRevision?: number } = {}) {
    if (!enabled.value) throw new Error("设备数据服务尚未就绪");
    const captured = revision;
    let resource = resourceCache.get(`${kind}/${id}`);
    const origin = kind === "conversation" ? parseConversationReference(id) : undefined;
    const characterId = options.characterId || resource?.roleId || selectInitialRole();
    if (!characterId) throw new Error("请先指定当前数据所属的角色");
    if (!resource) {
      if (origin) throw new Error("请先加载该会话并确认当前数据来源后再修改");
      const result = unwrap<{ resource: MeshResource | null; executionScope: OwnedExecutionScope }>(await apiClient.get("/api/device-mesh/v1/business/resources", { params: { kind, id, characterId } }));
      resource = result.resource ? { ...result.resource, executionScope: result.executionScope } : undefined;
    }
    if (captured !== revision) throw new Error("服务状态已变化，请重新加载后再操作");
    if (!resource || resource.deleted) throw new Error("该记录不在当前数据来源中，请在原设备管理历史数据");
    if (resource.readOnly || resource.origin === "legacy") throw new Error("旧系统记录只读，请在当前数据所有者中创建新记录");
    id = resource.id;
    if (options.expectedOwnerId && resource.ownerId !== options.expectedOwnerId) throw new Error("记录所属设备已变化，请在原设备管理历史数据");
    if (!resource.executionScope) throw new Error("记录缺少数据来源版本，请重新加载");
    const expectedRevision = options.expectedRevision ?? resource.revision;
    const response = unwrap<any>(await apiClient.post("/api/device-mesh/v1/business/resources/edit", {
      requestId: crypto.randomUUID(), kind, id, characterId, expectedRevision,
      expectedExecutionScope: options.expectedExecutionScope || resource.executionScope,
      changes, deleted: options.deleted === true, clear: options.clear === true,
    }));
    if (captured !== revision) throw new Error("服务状态已变化，请重新加载确认操作结果");
    if (response.ownerId !== resource.ownerId || response.versions?.[`${kind}/${id}`] !== expectedRevision + 1) throw new Error("数据所有者尚未确认操作结果");
    resourceCache.clear();
    return response;
  }

  async function conversationSummary(conversationId: string, characterId: string) {
    const result = await query(conversationId, characterId, false, "", "summary");
    summaryViews.delete(conversationId);
    for (const snapshot of [result.snapshot, result.historicalSnapshot]) {
      if (!snapshot) continue;
      const row = snapshot.resources.find((resource) => resource.kind === "summary" && !resource.deleted && resource.body);
      if (row) {
        const content = row.body.content;
        const value = { summaryViewId: crypto.randomUUID(), summaryText: String(typeof content === "object" && content !== null ? content.summary || content.text || "" : content || ""), sourceOwnerId: snapshot.ownerId, sourceResourceId: row.id, sourceRevision: row.revision, sourceScope: { ...result.executionScope }, editable: snapshot === result.snapshot && row.ownerId === result.executionScope.resourceOwnerId && row.origin !== "legacy" && row.readOnly !== true };
        summaryViews.set(conversationId, value);
        if (summaryViews.size > 64) summaryViews.delete(summaryViews.keys().next().value!);
        return { ...value, sourceScope: { ...value.sourceScope } };
      }
    }
    return null;
  }

  async function editConversationSummary(conversationId: string, text?: string, deleted = false, displayedViewId?: string) {
    const displayed = summaryViews.get(conversationId);
    if (!displayed?.editable || !Number.isSafeInteger(displayed.sourceRevision)) throw new Error("请先加载当前所有者的摘要，历史摘要需在原设备管理");
    if (!displayedViewId || displayedViewId !== displayed.summaryViewId) throw new Error("摘要视图已变化，请重新打开编辑");
    await edit("summary", displayed.sourceResourceId, deleted ? {} : { content: { summary: String(text || "").trim() } }, { deleted, characterId: displayed.sourceScope.roleId, expectedOwnerId: displayed.sourceOwnerId, expectedRevision: displayed.sourceRevision, expectedExecutionScope: displayed.sourceScope });
    summaryViews.delete(conversationId);
    return deleted ? null : { ...displayed, summaryText: String(text || "").trim(), sourceRevision: displayed.sourceRevision + 1 };
  }

  async function prepareSummaryGeneration(conversationId: string, characterId: string) {
    if (!enabled.value) throw new Error("设备摘要服务尚未就绪");
    const captured = revision;
    const result = await query(conversationId, characterId);
    const origin = parseConversationReference(conversationId);
    const resourceId = `${result.conversationId || origin?.id || conversationId}/summary`;
    const current = unwrap<{ resource: MeshResource | null; executionScope: OwnedExecutionScope }>(await apiClient.get("/api/device-mesh/v1/business/resources", { params: { kind: "summary", id: resourceId, characterId: result.executionScope.roleId } }));
    if (captured !== revision || authorityStamp(current.executionScope) !== authorityStamp(result.executionScope)) throw new Error("摘要数据来源已变化，请重新加载");
    if (current.resource && (current.resource.ownerId !== result.executionScope.resourceOwnerId || current.resource.roleId !== result.executionScope.roleId)) throw new Error("摘要数据来源不一致");
    const prepared = { viewId: crypto.randomUUID(), scope: { ...result.executionScope }, expectedRevision: current.resource?.revision || 0, resourceId };
    if (!Number.isSafeInteger(prepared.expectedRevision) || prepared.expectedRevision < 0) throw new Error("摘要数据版本无效");
    summaryPreparations.set(conversationId, prepared);
    if (summaryPreparations.size > 64) summaryPreparations.delete(summaryPreparations.keys().next().value!);
    return prepared.viewId;
  }

  async function generateConversationSummary(conversationId: string, viewId: string) {
    const prepared = summaryPreparations.get(conversationId);
    if (!prepared || prepared.viewId !== viewId) throw new Error("摘要生成页面已变化，请重新加载");
    const captured = revision;
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<any>(await apiClient.post(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/summary/generate`, {
      requestId, characterId: prepared.scope.roleId, expectedRevision: prepared.expectedRevision, expectedExecutionScope: prepared.scope, ...(origin ? { conversationOrigin: origin } : {}),
    }));
    if (captured !== revision || summaryPreparations.get(conversationId) !== prepared || authorityStamp(result.executionScope) !== authorityStamp(prepared.scope)) throw new Error("服务状态已变化，旧摘要已丢弃");
    if (!result.saved || result.executionScope.requestId !== requestId || result.sourceResourceId !== prepared.resourceId || result.sourceOwnerId !== prepared.scope.resourceOwnerId || result.sourceRevision !== prepared.expectedRevision + 1 || result.acknowledgement?.ownerId !== result.sourceOwnerId || result.acknowledgement?.requestId !== `${requestId}|summary-result` || result.acknowledgement?.versions?.[`summary/${prepared.resourceId}`] !== result.sourceRevision) throw new Error("摘要所有者尚未确认保存");
    summaryPreparations.delete(conversationId);
    summaryViews.delete(conversationId);
    return result;
  }

  async function projections(characterId: string, expectedExecutionScope?: OwnedExecutionScope, memoryView?: string) {
    if (!enabled.value) throw new Error("设备索引服务尚未就绪");
    const captured = revision;
    const result = unwrap<any>(expectedExecutionScope
      ? await apiClient.post("/api/device-mesh/v1/business/projections/rebuild", { characterId, expectedExecutionScope })
      : await apiClient.get("/api/device-mesh/v1/business/projections", { params: { characterId, memoryView } }));
    if (captured !== revision || result?.executionScope?.coreId !== coreId.value || result?.executionScope?.roleId !== characterId || result?.status?.ownerId !== result?.executionScope?.resourceOwnerId || result?.status?.roleId !== memoryStorageRole(result.executionScope, "memory") || !Array.isArray(result?.status?.layers)) throw new Error("服务状态已变化，索引状态已丢弃");
    return result;
  }

  function messages(result: MeshQueryResult): any[] {
    const resultMessages: any[] = [];
    const seen = new Set<string>();
    for (const snapshot of [result.historicalSnapshot, result.snapshot]) {
      if (!snapshot) continue;
      const rows = snapshot.resources.filter((resource) => resource.kind === "message").map((resource) => ({ ...resource.body, origin: resource.origin, readOnly: resource.readOnly || resource.origin === "legacy", sourceRevision: resource.origin === "legacy" || resource.readOnly ? null : resource.revision }));
      for (const row of rows) {
        if (!row?.id || !["user", "assistant"].includes(row.role)) continue;
        const key = `${snapshot.ownerId}:${row.id}`;
        if (seen.has(key)) continue;
        seen.add(key);
        const files = (Array.isArray(row.attachments) ? row.attachments : []).filter((item: any) => ["file", "video"].includes(item?.kind)).map((item: any) => ({ ...item, url: ownedAttachmentURL(item), downloadUrl: ownedAttachmentURL(item) }));
        resultMessages.push({ ...row, sourceConversationId: row.conversationId, attachments: files, ...(row.quote ? { replyToMessageId: row.quote.messageId, replyToRole: row.quote.role, replyToExcerpt: String(row.quote.content || "").slice(0, 100) } : {}), ...(result.conversationReference ? { conversationId: result.conversationReference } : {}), content: ownedMessageText(row), imageUrl: ownedImageURL(row.attachments) || row.imageUrl, audioUrl: ownedAudioURL(row.attachments) || row.audioUrl, uiKey: key, ownerId: snapshot.ownerId, executionScope: result.executionScope, status: row.status || "completed" });
      }
    }
		const failures = (result.deliveryFailures || []).filter((row) => row.ownerId === result.executionScope.resourceOwnerId && ["mesh.owned_resource_version", "mesh.owned_request_conflict"].includes(row.errorCode));
		if (failures.length) {
			const id = `save-failures:${result.executionScope.coreId}:${result.executionScope.resourceOwnerId}:${result.executionScope.roleId}:${failures[0].conversationId || ""}`;
			resultMessages.push({ id, uiKey: id, role: "system", type: "system_notice", content: `有 ${failures.length} 项保存请求被数据所有者拒绝，未确认保存。请重新加载记录后处理版本或请求编号冲突；系统不会继续重试这些旧请求。`, createdAt: failures[0].failedAt, ownerId: result.executionScope.resourceOwnerId, executionScope: result.executionScope, sourceRevision: null, status: "completed" });
		}
    return resultMessages;
  }

  async function send(request: OwnedChatRequest, onEvent: (event: OwnedChatEvent) => void) {
    if (activeController) throw new Error("当前回复尚未结束");
    if (!enabled.value) throw new Error("设备对话服务尚未就绪");
    if (transientNotice && notice.value === transientNotice) {
      notice.value = "";
      transientNotice = "";
    }
    const origin = parseConversationReference(request.conversationId || "");
    const controller = new AbortController();
    activeController = controller;
    activeRequest = request.requestId;
    const captured = revision;
    const wireRequest = origin ? { ...request, conversationId: origin.id, conversationOrigin: origin, ...(request.context ? { context: { ...request.context, conversationId: origin.id } } : {}) } : request;
    const publicEvent = (event: OwnedChatEvent): OwnedChatEvent => {
      const id = event.data?.conversationId || event.conversationId;
      const ownerId = event.data?.executionScope.resourceOwnerId || event.executionScope?.resourceOwnerId;
      if (!id || !ownerId) return event;
      const reference = conversationReference(event.data?.conversationOrigin || origin || { ownerId, id });
      return { ...event, conversationId: reference, ...(event.data ? { data: { ...event.data, conversationId: reference } } : {}) };
    };
    try {
      const response = await streamOwnedChat(wireRequest, controller.signal, (event) => {
        if (captured !== revision) return;
		const mapped = publicEvent(event);
		if (mapped.type === "task" && mapped.executionScope?.coreId === coreId.value && mapped.executionScope?.roleId === request.characterId) {
		  const change = ownedWorkbenchTaskChange(mapped);
		  if (change) window.dispatchEvent(new CustomEvent(workbenchTaskEventName, { detail: change }));
		}
        onEvent(mapped);
      });
      if (captured !== revision) throw new Error("服务状态已变化，迟到回复已拦截");
      return { ...response, resourceConversationId: response.conversationId, conversationId: conversationReference(response.conversationOrigin || origin || { ownerId: response.executionScope.resourceOwnerId, id: response.conversationId }) };
    } finally {
      if (activeController === controller) {
        activeController = null;
        activeRequest = "";
      }
    }
  }

  async function groupState(conversationId: string, characterId: string): Promise<OwnedGroupState | null> {
    const id = parseConversationReference(conversationId)?.id || conversationId;
    if (!id || !characterId) return null;
    const result = await query(id, characterId);
    if (result.snapshot.group) return result.snapshot.group;
    const conversation = result.snapshot.resources.find((resource) => resource.kind === "conversation")?.body;
    if (conversation?.conversationType !== "group") return null;
    return {
      conversationId: id,
      conversationRoleId: String(conversation.conversationRoleId || conversation.characterId || ""),
      roleOwnerId: String(conversation.roleOwnerId || result.executionScope.roleOwnerId),
      title: String(conversation.title || ""),
      conversationType: "group",
      membershipRevision: Number(conversation.membershipRevision || 1),
      settingsRevision: Number(conversation.settingsRevision || 1),
      lastMessageSequence: Number(conversation.lastMessageSequence || 0),
      groupSettings: {
        allowCharacterMentions: conversation.groupSettings?.allowCharacterMentions === true,
        maxAutoReplies: Number(conversation.groupSettings?.maxAutoReplies || 6),
      },
      members: Array.isArray(conversation.members) ? conversation.members : [],
    };
  }

  async function sendGroup(request: OwnedGroupChatRequest, onEvent: (event: OwnedChatEvent) => void): Promise<OwnedGroupChatResponse> {
    if (activeController) throw new Error("当前回复尚未结束");
    if (!enabled.value) throw new Error("设备对话服务尚未就绪");
    const origin = parseConversationReference(request.conversationId || "");
    const controller = new AbortController();
    activeController = controller;
    activeRequest = request.requestId;
    const captured = revision;
    const wireRequest = origin ? { ...request, conversationId: origin.id, conversationOrigin: origin } : request;
    try {
      const response = await streamOwnedGroupChat(wireRequest, controller.signal, (event) => {
        if (captured !== revision) return;
        onEvent(event);
      });
      if (captured !== revision) throw new Error("服务状态已变化，迟到群聊已拦截");
      const reference = conversationReference(origin || { ownerId: response.deliveries[0]?.executionScope.resourceOwnerId || roleOwnerId.value, id: response.conversationId });
      return { ...response, conversationId: reference };
    } finally {
      if (activeController === controller) {
        activeController = null;
        activeRequest = "";
      }
    }
  }

  async function inviteGroupMembers(conversationId: string, characterId: string, members: Array<{ characterId: string; shareHistory: boolean }>, expectedMembershipRevision: number) {
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<{ group: OwnedGroupState; saved: boolean }>(await apiClient.post(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/members`, { requestId, characterId, members, expectedMembershipRevision, ...(origin ? { conversationOrigin: origin } : {}) }));
    if (!result.saved) throw new Error("群成员尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return result.group;
  }

  async function createGroupConversation(characterId: string, members: Array<{ characterId: string; shareHistory: boolean }>) {
    if (!enabled.value) throw new Error("设备对话服务尚未就绪");
    if (!characterId || members.length === 0) throw new Error("请选择当前发言角色和至少一个邀请角色");
    const requestId = crypto.randomUUID();
    const result = unwrap<{ group: OwnedGroupState; acknowledgement: { ownerId: string }; saved: boolean }>(await apiClient.post("/api/device-mesh/v1/business/conversations/group", { requestId, characterId, members }));
    if (!result.saved || !result.group?.conversationId || !result.acknowledgement?.ownerId) throw new Error("群聊尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return {
      ...result.group,
      conversationId: conversationReference({ ownerId: result.acknowledgement.ownerId, id: result.group.conversationId }),
    };
  }

  async function removeGroupMember(conversationId: string, characterId: string, memberId: string, expectedMembershipRevision: number) {
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<{ group: OwnedGroupState; saved: boolean }>(await apiClient.delete(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/members/${encodeURIComponent(memberId)}`, { data: { requestId, characterId, expectedMembershipRevision } }));
    if (!result.saved) throw new Error("群成员变更尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return result.group;
  }

  async function updateGroupSettings(conversationId: string, characterId: string, expectedSettingsRevision: number, changes: { name?: string; allowCharacterMentions?: boolean; maxAutoReplies?: number; memberOrder?: string[] }) {
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<{ group: OwnedGroupState; saved: boolean }>(await apiClient.patch(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/group-settings`, { requestId, characterId, expectedSettingsRevision, ...changes }));
    if (!result.saved) throw new Error("群设置尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return result.group;
  }

  async function requestGroupReplies(conversationId: string, characterId: string, sourceMessageId: string, members: Array<{ memberId: string; characterId: string; roleOwnerId?: string }>) {
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<OwnedGroupChatResponse>(await apiClient.post(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/reply-requests`, { requestId, characterId, sourceMessageId, members }));
    if (!result.saved) throw new Error("点名回复尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return result;
  }

  async function retryGroupDelivery(conversationId: string, characterId: string, roundId: string, deliveryId: string) {
    const origin = parseConversationReference(conversationId);
    const requestId = crypto.randomUUID();
    const result = unwrap<OwnedGroupChatResponse>(await apiClient.post(`/api/device-mesh/v1/business/conversations/${encodeURIComponent(origin?.id || conversationId)}/group-rounds/retry?roundId=${encodeURIComponent(roundId)}`, { requestId, characterId, deliveryId }));
    if (!result.saved) throw new Error("群成员重试尚未获得数据所有者确认");
    invalidateWithoutInterrupt();
    return result;
  }

  async function failedGroupDeliveries(conversationId: string, characterId: string): Promise<OwnedGroupDelivery[]> {
    const result = await query(conversationId, characterId);
    const rounds = result.snapshot.resources.filter((resource) => resource.kind === "group-round" && resource.body && !resource.deleted);
    const superseded = new Set<string>();
    for (const round of rounds) {
      for (const delivery of Array.isArray(round.body.deliveries) ? round.body.deliveries as OwnedGroupDelivery[] : []) {
        if (delivery.source.startsWith("retry:")) superseded.add(delivery.source.slice("retry:".length));
      }
    }
    return rounds
      .flatMap((round) => Array.isArray(round.body.deliveries) ? round.body.deliveries as OwnedGroupDelivery[] : [])
      .filter((delivery) => delivery.status === "failed" && !superseded.has(delivery.deliveryId))
      .sort((left, right) => String(right.updatedAt || "").localeCompare(String(left.updatedAt || "")));
  }

  async function interrupt() {
    if (!activeRequest) return;
    await apiClient.post(`/api/device-mesh/v1/business/messages/${encodeURIComponent(activeRequest)}/interrupt`);
  }

  const previousSummary = (previousCoreId: string, conversationId: string) => summaries.get(`${previousCoreId}/${conversationId}`);
  async function projects(characterId: string, selectedHistoricalRole = "") {
    const captured = revision;
    const rows = new Map<string, OwnedProject>();
    const historical = new Map<string, OwnedProject>();
    const availableHistoricalRoles = policy.value?.coordinated ? await historicalRoles(characterId) : [];
    const historicalRole = selectedHistoricalRole || (availableHistoricalRoles.length === 1 ? availableHistoricalRoles[0].id : "");
    if (historicalRole && !availableHistoricalRoles.some((role) => role.id === historicalRole)) throw new Error("旧项目角色已变化，请重新选择");
    const cursors = new Set<string>();
    let cursor = "";
    let historicalCursor = "";
    let currentDone = false;
    let historicalDone = !historicalRole;
    let expected: OwnedExecutionScope | undefined;
    for (let page = 0; page < 32; page++) {
      const result = unwrap<{ projects: Omit<OwnedProject, "executionScope">[]; historicalProjects?: Omit<OwnedProject, "executionScope">[]; executionScope: OwnedExecutionScope; nextCursor?: string; nextHistoricalCursor?: string }>(await apiClient.get("/api/device-mesh/v1/business/projects", { params: { characterId, cursor, ...(!historicalDone ? { historicalRoleId: historicalRole, historicalCursor } : {}) } }));
      if (captured !== revision || result.executionScope?.coreId !== coreId.value || result.executionScope?.roleId !== characterId || (expected && authorityStamp(expected) !== authorityStamp(result.executionScope))) throw new Error("项目数据归属已变化，请重新加载");
      expected = result.executionScope;
      if (!Array.isArray(result.projects) || result.projects.length > 128) throw new Error("项目分页数据无效");
      for (const project of currentDone ? [] : result.projects) {
        if (!project.id || project.ownerId !== expected.resourceOwnerId || project.roleId !== characterId || project.readOnly !== false || !Number.isSafeInteger(project.revision) || project.revision < 1) throw new Error("项目所属设备或版本无效");
        rows.set(project.id, { ...project, executionScope: expected });
        resourceCache.set(`project/${project.id}`, { kind: "project", id: project.id, roleId: characterId, ownerId: project.ownerId, revision: project.revision, deleted: false, body: project, executionScope: expected });
      }
      if (!Array.isArray(result.historicalProjects || []) || (result.historicalProjects?.length || 0) > 128) throw new Error("旧项目分页数据无效");
      for (const project of historicalDone ? [] : result.historicalProjects || []) {
        if (!historicalRole || !project.id || project.ownerId !== expected.targetDeviceId || project.ownerId === expected.resourceOwnerId || project.roleId !== historicalRole || project.readOnly !== true || !Number.isSafeInteger(project.revision) || project.revision < 1) throw new Error("旧项目所属设备或角色无效");
        historical.set(project.id, { ...project, executionScope: expected });
      }
      currentDone ||= !result.nextCursor;
      historicalDone ||= !result.nextHistoricalCursor;
      if (rows.size + historical.size > 4096) throw new Error("项目分页超过上限");
      if (currentDone && historicalDone) return { projects: [...rows.values()], historicalProjects: [...historical.values()], historicalRoles: availableHistoricalRoles, selectedHistoricalRole: historicalRole, executionScope: expected };
      if (!currentDone) cursor = result.nextCursor!;
      if (!historicalDone) historicalCursor = result.nextHistoricalCursor!;
      const next = JSON.stringify([currentDone, cursor, historicalDone, historicalCursor]);
      if (cursors.has(next) || rows.size + historical.size > 4096) throw new Error("项目分页超过上限或游标重复");
      cursors.add(next);
    }
    throw new Error("项目分页超过上限");
  }

  async function createProject(title: string, expected: OwnedExecutionScope, workspaces: OwnedProjectWorkspace[], primaryWorkspaceId: string) {
    if (!enabled.value || !expected || expected.coreId !== coreId.value) throw new Error("项目归属无法确认，请重新加载");
    if (!Array.isArray(workspaces) || (workspaces.length > 0 && (!primaryWorkspaceId || !workspaces.some((workspace) => workspace.id === primaryWorkspaceId)))) throw new Error("项目工作区无效");
    const captured = revision;
    const requestId = crypto.randomUUID();
    const identity = Array.from(new Uint8Array(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(`${expected.coreId}\0${expected.initiatorDeviceId}\0${requestId}`))), (value) => value.toString(16).padStart(2, "0")).join("");
    if (captured !== revision) throw new Error("项目归属已变化，请重新加载");
    const result = unwrap<any>(await apiClient.post("/api/device-mesh/v1/business/projects", { requestId, title, workspaces, primaryWorkspaceId, characterId: expected.roleId, expectedExecutionScope: expected }));
    if (captured !== revision || result.saved !== true || result.project?.id !== `project-${identity}` || result.project?.primaryWorkspaceId !== primaryWorkspaceId || result.executionScope?.requestId !== requestId || authorityStamp(result.executionScope) !== authorityStamp(expected) || result.acknowledgement?.ownerId !== expected.resourceOwnerId || result.acknowledgement?.requestId !== requestId || result.acknowledgement?.versions?.[`project/${result.project.id}`] !== 1 || result.acknowledgement?.versions?.[`checkpoint/project/${requestId}`] !== 1) throw new Error("项目尚未获得数据所有者保存确认");
    return { ...result.project, ownerId: expected.resourceOwnerId, roleId: expected.roleId, readOnly: false, revision: 1, executionScope: result.executionScope } as OwnedProject;
  }
  return { enabled, roles, coreId, roleOwnerId, policy, notice, refresh, markUnavailable, selectInitialRole, query, data, historicalRoles, projections, hasMore, conversations, allMessages, exportConversation, conversationSummary, editConversationSummary, prepareSummaryGeneration, generateConversationSummary, edit, messages, send, groupState, sendGroup, createGroupConversation, inviteGroupMembers, removeGroupMember, updateGroupSettings, requestGroupReplies, retryGroupDelivery, failedGroupDeliveries, interrupt, stopLocal, previousSummary, projects, createProject };
}
