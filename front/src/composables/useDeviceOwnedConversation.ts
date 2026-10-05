import { computed, ref } from "vue";
import { apiClient } from "./useApi";
import { getDeploymentConfig, getNativeProviderTransition } from "@/runtime/runtime-adapter";
import { streamOwnedChat, type OwnedChatEvent, type OwnedChatRequest, type OwnedExecutionScope } from "@/runtime/device-owned-chat";
import { ownedImageURL, ownedAudioURL, ownedMessageText } from "@/runtime/device-owned-attachments";
import { conversationReference, parseConversationReference, ownedConversationRow, type OwnedConversationOrigin } from "@/runtime/device-owned-conversation-reference";
import { createOwnedConversationExport } from "@/runtime/device-owned-export";

interface MeshRole { id: string; name: string; revision: number }
interface MeshPolicy { coordinated: boolean; modeRevision: number; providerEpoch: number; permissionRevision: number; selectedRole: string }
interface MeshResource { kind: string; id: string; roleId: string; ownerId: string; revision: number; deleted: boolean; body: any; executionScope?: OwnedExecutionScope }
interface MeshSnapshot { ownerId: string; resources: MeshResource[]; nextCursors?: Record<string, string>; legacyMessages?: any[]; legacyMemories?: any[]; legacyProfiles?: any[]; legacyEpisodes?: any[]; legacyConversations?: any[]; legacySummary?: any }
interface MeshQueryResult { conversationReference?: string; conversationOrigin?: OwnedConversationOrigin; executionScope: OwnedExecutionScope; snapshot: MeshSnapshot; historicalSnapshot?: MeshSnapshot; historicalConversations?: any[]; nextHistoricalListCursor?: string; deliveryFailures?: Array<{ ownerId: string; requestId: string; conversationId?: string; errorCode: string; failedAt: string }> }

const enabled = ref(false);
const roles = ref<MeshRole[]>([]);
function readProviderState(): { coreId: string; notice: string; changeId?: string } {
  try { return JSON.parse(localStorage.getItem("amitia.mesh.provider-state") || "{}"); } catch { return { coreId: "", notice: "" }; }
}
const coreId = ref(String(readProviderState().coreId || ""));
const roleOwnerId = ref("");
const policy = ref<MeshPolicy | null>(null);
const notice = ref(String(readProviderState().notice || ""));
let providerChangeId = String(readProviderState().changeId || "");
let activeController: AbortController | null = null;
let activeRequest = "";
let revision = 0;
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
  try { localStorage.setItem("amitia.mesh.provider-state", JSON.stringify({ coreId: coreId.value, notice: notice.value, changeId: providerChangeId })); } catch {}
}

function unwrap<T>(response: any): T {
  return response?.data?.code === 200 && response.data.data !== undefined ? response.data.data : response.data;
}

function stamp(core: string, state: MeshPolicy | null) {
  return `${core}:${state?.providerEpoch}:${state?.modeRevision}:${state?.permissionRevision}`;
}

function stopLocal(message: string) {
  revision++;
  resourceCache.clear();
  summaryViews.clear();
  summaryPreparations.clear();
  historyPages.clear();
  historyRequests.clear();
  conversationLoads.clear();
  activeController?.abort();
  activeController = null;
  activeRequest = "";
  notice.value = message;
  saveProviderState();
}

export function useDeviceOwnedConversation() {
	async function data(kind: string, characterId: string, conversationId = "", cursor = "", historical: { legacyCursor?: string; historicalRoleId?: string; historicalCursor?: string; historicalLegacyCursor?: string } = {}): Promise<MeshQueryResult> {
	  const captured = revision;
	  const result = unwrap<MeshQueryResult>(await apiClient.get("/api/device-mesh/v1/business/data", { params: { kind, characterId, conversationId, cursor, ...historical } }));
	  if (captured !== revision || result.executionScope.coreId !== coreId.value) throw new Error("服务状态已变化，请重新加载记忆");
	  for (const resource of result.snapshot.resources) resourceCache.set(`${resource.kind}/${resource.id}`, { ...resource, executionScope: result.executionScope });
	  return result;
	}
	async function historicalRoles(characterId: string) {
	  const captured = revision;
	  const result = unwrap<{ roles: Array<{ id: string; name: string }>; executionScope: OwnedExecutionScope }>(await apiClient.get("/api/device-mesh/v1/business/historical-roles", { params: { characterId } }));
	  if (captured !== revision || result.executionScope.coreId !== coreId.value) throw new Error("服务状态已变化，旧记忆目录已丢弃");
	  return result.roles;
	}
  async function refresh() {
	const transition = await getNativeProviderTransition();
	if (transition?.providerChangePending) {
	  enabled.value = true;
	  roles.value = [];
	  stopLocal(`云端服务提供者正在从「${transition.coreId || coreId.value}」切换至「${transition.successorCoreId || "新 Core"}」，当前回复已中断，等待新服务批准并连接。`);
	  throw new Error(notice.value);
	}
    const deployment = await getDeploymentConfig();
    const current = unwrap<any>(await apiClient.get("/api/device-mesh/v1/coordination/me"));
    const latestDeployment = await getDeploymentConfig();
    if (latestDeployment.mode !== deployment.mode || latestDeployment.serverURL !== deployment.serverURL) throw new Error("服务提供者已变化，旧状态已丢弃");
    const previous = stamp(coreId.value, policy.value);
    const next = stamp(String(current.coreId || ""), current.policy || null);
    if ((enabled.value && previous !== next) || (coreId.value && coreId.value !== current.coreId)) {
      const message = coreId.value !== current.coreId
        ? `云端服务提供者已从「${coreId.value}」切换为「${current.coreId}」，当前回复已中断。`
        : "统筹模式或调用权限已变化，当前回复已中断；继续发送时将使用当前服务和数据归属。";
      stopLocal(message);
    }
    if (transition?.coreId === current.coreId && transition?.providerChangeId && transition.previousCoreId && transition.previousCoreId !== current.coreId && providerChangeId !== transition.providerChangeId) {
      providerChangeId = transition.providerChangeId;
      stopLocal(`云端服务提供者已从「${transition.previousCoreId}」切换为「${current.coreId}」，当前回复已中断。`);
    }
    coreId.value = String(current.coreId || "");
    saveProviderState();
    policy.value = current.policy || null;
    enabled.value = current.coordinationAvailable === true;
    if (!enabled.value) return false;
    const available = unwrap<any>(await apiClient.get("/api/device-mesh/v1/business/roles"));
    const finalDeployment = await getDeploymentConfig();
    if (finalDeployment.mode !== deployment.mode || finalDeployment.serverURL !== deployment.serverURL) throw new Error("服务提供者已变化，旧角色已丢弃");
    roles.value = Array.isArray(available.roles) ? available.roles : [];
    roleOwnerId.value = String(available.roleOwnerId || "");
    return true;
  }

  function selectInitialRole(saved?: string): string {
    const requested = policy.value?.selectedRole || saved || "";
    if (requested) return roles.value.some((role) => role.id === requested) ? requested : "";
    return roles.value.length === 1 ? roles.value[0].id : "";
  }

  const historyKey = (conversationId: string, characterId: string, keyword = "") => JSON.stringify([conversationId, characterId, keyword]);

  async function query(conversationId: string, characterId: string, older = false, keyword = ""): Promise<MeshQueryResult> {
    const origin = parseConversationReference(conversationId);
    const path = "/api/device-mesh/v1/business/conversations" + (conversationId ? `/${encodeURIComponent(origin?.id || conversationId)}` : "");
    const captured = revision;
    const key = historyKey(conversationId, characterId, keyword);
    const ticket = Symbol(key);
    historyRequests.set(key, ticket);
    if (historyRequests.size > 128) historyRequests.delete(historyRequests.keys().next().value!);
    const previous = historyPages.get(key);
    const kind = conversationId ? "message" : "conversation";
    const legacyKind = conversationId ? "legacyMessage" : "legacyConversation";
    const cursors = older ? previous?.cursors || {} : {};
    const params = { characterId, ...(keyword ? { keyword } : {}), ...(origin ? { conversationOwnerId: origin.ownerId } : {}), ...(older ? { resourceKind: kind, ...cursors } : {}) };
    const result = unwrap<MeshQueryResult>(await apiClient.get(path, { params }));
    if (captured !== revision || historyRequests.get(key) !== ticket) throw new Error("服务状态或加载请求已变化，旧上下文已丢弃");
    const scope = JSON.stringify([result.executionScope.spaceId, result.executionScope.initiatorDeviceId, result.executionScope.targetDeviceId, result.executionScope.coreId, result.executionScope.providerEpoch, result.executionScope.targetProviderEpoch, result.executionScope.coordinated, result.executionScope.modeRevision, result.executionScope.permissionRevision, result.executionScope.targetPermissionRevision, result.executionScope.roleOwnerId, result.executionScope.resourceOwnerId, result.executionScope.roleId, result.executionScope.roleRevision]);
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
      const text = String(summary?.summary || summary?.text || (typeof summary === "string" ? summary : result.snapshot.legacySummary?.summaryText || result.snapshot.legacySummary?.summary_text || "")).slice(0, 16000);
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
        for (const value of [...(snapshot.legacyConversations || []), ...snapshot.resources.filter((resource) => resource.kind === "conversation").map((resource) => resource.body)]) {
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

  async function edit(kind: "conversation" | "message" | "memory" | "summary", id: string, changes: Record<string, unknown> = {}, options: { deleted?: boolean; clear?: boolean; characterId?: string; expectedExecutionScope?: OwnedExecutionScope; expectedOwnerId?: string; expectedRevision?: number } = {}) {
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
    const result = await query(conversationId, characterId);
    summaryViews.delete(conversationId);
    for (const snapshot of [result.snapshot, result.historicalSnapshot]) {
      if (!snapshot) continue;
      const row = snapshot.resources.find((resource) => resource.kind === "summary" && !resource.deleted && resource.body);
      if (row) {
        const content = row.body.content;
        const value = { summaryViewId: crypto.randomUUID(), summaryText: String(typeof content === "object" && content !== null ? content.summary || content.text || "" : content || ""), sourceOwnerId: snapshot.ownerId, sourceResourceId: row.id, sourceRevision: row.revision, sourceScope: { ...result.executionScope }, editable: snapshot === result.snapshot && row.ownerId === result.executionScope.resourceOwnerId };
        summaryViews.set(conversationId, value);
        if (summaryViews.size > 64) summaryViews.delete(summaryViews.keys().next().value!);
        return { ...value, sourceScope: { ...value.sourceScope } };
      }
      if (snapshot.legacySummary) return { ...snapshot.legacySummary, sourceOwnerId: snapshot.ownerId, editable: false };
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

  async function projections(characterId: string, expectedExecutionScope?: OwnedExecutionScope) {
    if (!enabled.value) throw new Error("设备索引服务尚未就绪");
    const captured = revision;
    const result = unwrap<any>(expectedExecutionScope
      ? await apiClient.post("/api/device-mesh/v1/business/projections/rebuild", { characterId, expectedExecutionScope })
      : await apiClient.get("/api/device-mesh/v1/business/projections", { params: { characterId } }));
    if (captured !== revision || result?.executionScope?.coreId !== coreId.value || result?.executionScope?.roleId !== characterId || result?.status?.ownerId !== result?.executionScope?.resourceOwnerId || result?.status?.roleId !== characterId || !Array.isArray(result?.status?.layers)) throw new Error("服务状态已变化，索引状态已丢弃");
    return result;
  }

  function messages(result: MeshQueryResult): any[] {
    const resultMessages: any[] = [];
    const seen = new Set<string>();
    for (const snapshot of [result.historicalSnapshot, result.snapshot]) {
      if (!snapshot) continue;
      const rows = [...(snapshot.legacyMessages || []).map((row: any) => ({ ...row, sourceRevision: null })), ...snapshot.resources.filter((resource) => resource.kind === "message").map((resource) => ({ ...resource.body, sourceRevision: resource.revision }))];
      for (const row of rows) {
        if (!row?.id || !["user", "assistant"].includes(row.role)) continue;
        const key = `${snapshot.ownerId}:${row.id}`;
        if (seen.has(key)) continue;
        seen.add(key);
        resultMessages.push({ ...row, ...(result.conversationReference ? { conversationId: result.conversationReference } : {}), content: ownedMessageText(row), imageUrl: ownedImageURL(row.attachments) || row.imageUrl, audioUrl: ownedAudioURL(row.attachments) || row.audioUrl, uiKey: key, ownerId: snapshot.ownerId, executionScope: result.executionScope, status: row.status || "completed" });
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
        onEvent(publicEvent(event));
      });
      if (captured !== revision) throw new Error("服务状态已变化，迟到回复已拦截");
      return { ...response, conversationId: conversationReference(response.conversationOrigin || origin || { ownerId: response.executionScope.resourceOwnerId, id: response.conversationId }) };
    } finally {
      if (activeController === controller) {
        activeController = null;
        activeRequest = "";
      }
    }
  }

  async function interrupt() {
    if (!activeRequest) return;
    await apiClient.post(`/api/device-mesh/v1/business/messages/${encodeURIComponent(activeRequest)}/interrupt`);
  }

  const previousSummary = (previousCoreId: string, conversationId: string) => summaries.get(`${previousCoreId}/${conversationId}`);
  return { enabled, roles, coreId, roleOwnerId, policy, notice, refresh, selectInitialRole, query, data, historicalRoles, projections, hasMore, conversations, allMessages, exportConversation, conversationSummary, editConversationSummary, prepareSummaryGeneration, generateConversationSummary, edit, messages, send, interrupt, stopLocal, previousSummary, coordinated: computed(() => policy.value?.coordinated === true) };
}
