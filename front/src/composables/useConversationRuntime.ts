import { type Ref, ref, shallowRef, watch } from "vue";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { projectAutomationStatus, type AutomationStatus } from "@/conversation/runtime/automationStatus";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import type { AssistantTurnData } from "@/conversation/rendering/types";
import { projectCitationSnapshot } from "@/conversation/runtime/citationProjection";
import {
  AgentEventReducer,
  type AgentUIEvent,
  type RuntimeTurnSnapshot,
  cloneTurn,
  isTerminalTurn,
  normalizedTurnStatus,
} from "@/conversation/runtime/agentEventReducer";
import {
  compareChatMessages,
  getMessageUIKey,
  normalizeRealtimeMessage,
} from "@/utils/message-order";
import { notifyDesktopPetChatState } from "@/runtime/desktop-pet-chat-state";
import { publishWorkbenchEvent } from "@/runtime/workbench-agent-event";
import { ownedChatRuntimeRequest, prepareOwnedChatRequest } from "@/runtime/owned-chat-management";
import { sameOwnedAuthority } from "@/runtime/owned-speech-result";
import { OwnedRuntimeSSEDecoder, type OwnedRuntimeSSEFrame } from "@/runtime/owned-runtime-sse";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";

interface ConversationSnapshot {
  version: number;
  revision: number;
  lastEventSequence: number;
  conversation?: Record<string, any>;
  workspace?: Record<string, any> | null;
  messages?: any[];
  turns?: AssistantTurnData[];
  messageHistory?: { nextBefore?: number; hasMore?: boolean };
  turnHistory?: { nextCursor?: string; hasMore?: boolean };
  approvals?: Array<Record<string, any>>;
  activeTurn?: RuntimeTurnSnapshot | null;
}

interface PendingAssistantRequest {
  requestId: string;
  characterId: string;
  createdAt: string;
}

export function useConversationRuntime(
  conversationId: Ref<string>,
  persistedMessages: Ref<any[]>,
  sending: Ref<boolean>,
  scrollToBottom: (smooth?: boolean) => void,
  onConversationSnapshot?: (
    conversation?: Record<string, any>,
    workspace?: Record<string, any> | null,
    snapshot?: ConversationSnapshot,
  ) => void | Promise<void>,
  onConversationActivityChanged?: (conversationId: string) => void | Promise<void>,
  groupSending?: Ref<boolean>,
) {
  const owned = useDeviceOwnedConversation();
  const reducer = new AgentEventReducer();
  const messages = shallowRef<any[]>([]);
  const activeTurnId = ref("");
  const activeExecutionId = ref("");
  const automationStatus = shallowRef<AutomationStatus | null>(null);
  const lastEventSequence = ref(0);
  const turnHistoryCursor = ref("");
  const hasMoreTurnHistory = ref(false);
  let projectionCache = new Map<string, any>();
  let projectionSignatures = new Map<string, string>();
  const pendingAssistantRequests = new Map<string, PendingAssistantRequest>();
  let abortController: AbortController | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let renderTimer: number | ReturnType<typeof setTimeout> | null = null;
  let snapshotEpoch = 0;
  let connectionEpoch = 0;
  let connectionTask: Promise<void> | null = null;
  let connectionTaskId = "";
  let streamOpen = false;
  let recoverySnapshotRequired = false;
  let disposed = false;

  function isGroupTurn(requestId: unknown): boolean {
    const id = String(requestId || "").trim();
    return Boolean(id && persistedMessages.value.some((message) => message.deliveryId && (message.requestId === id || message.id === `${id}/assistant`)));
  }

  function syncReducerState() {
    automationStatus.value = projectAutomationStatus(reducer.turns);
    activeTurnId.value = reducer.activeTurnId;
    activeExecutionId.value = reducer.activeExecutionId;
    lastEventSequence.value = reducer.lastEventSequence;
  }

  function publishApprovalState(detail: Record<string, any>) {
    window.dispatchEvent(new CustomEvent("amitia:agent-approval", { detail }));
  }

  function turnSignature(turn: AssistantTurnData): string {
    return [
      turn.id,
      turn.status,
      turn.sequence,
      ...(turn.items || []).map((item) => [
        item.id,
        item.type,
        item.status,
        item.revision,
        item.messageId,
        JSON.stringify(item.citationSnapshot || []),
        item.errorCode,
        item.durationMs,
      ].join("\u0001")),
    ].join("\u0002");
  }

  function messageSignature(message: any): string {
    return [
      message?.id,
      message?.uiKey,
      message?.role,
      message?.content,
      message?.status,
      message?.outputComplete,
      message?.reasoningComplete,
      message?.createdAt,
      message?.updatedAt,
      message?.clientMessageId,
      message?.requestId,
      message?.turnId,
      message?.conversationId,
      message?.imageUrl,
      message?.audioUrl,
      message?.videoUrl,
      message?.replyToMessageId,
      message?.replyToRole,
      message?.replyToExcerpt,
      JSON.stringify(message?.citationSnapshot || []),
    ].map((value) => String(value ?? "")).join("\u0002");
  }

  function assistantRowKey(turn: AssistantTurnData): string {
    const requestId = String(turn.requestId || "").trim();
    return requestId ? `request:${requestId}` : `turn:${turn.id}`;
  }

  function liveMessageForTurn(turn: AssistantTurnData): any {
    const textItems = (turn.items || []).filter((item) => item.type === "text");
    const markdown = textItems.map((item) => String(item.content || "")).join("");
    const finalItem = [...textItems].reverse().find((item) => item.messageId);
    return {
      id: finalItem?.messageId || `turn:${turn.id}`,
      uiKey: assistantRowKey(turn),
      role: "assistant",
      conversationId: turn.conversationId,
      turnId: turn.id,
      requestId: turn.requestId || "",
      content: markdown,
      outputComplete: textItems.length > 0 && textItems.every(item => isTerminalTurn(item.status)),
      status: isTerminalTurn(turn.status) ? normalizedTurnStatus(turn.status) : "streaming",
      createdAt: turn.createdAt || new Date().toISOString(),
      assistantTurn: cloneTurn(turn),
      ...(finalItem?.citationSnapshot ? { citationSnapshot: finalItem.citationSnapshot } : {}),
    };
  }

  function pendingAssistantMessage(request: PendingAssistantRequest): any {
    const turn: AssistantTurnData = {
      id: `pending:${request.requestId}`,
      conversationId: conversationId.value,
      requestId: request.requestId,
      sequence: 0,
      status: "queued",
      createdAt: request.createdAt,
      updatedAt: request.createdAt,
      items: [],
    };
    return {
      id: turn.id,
      uiKey: `request:${request.requestId}`,
      role: "assistant",
      conversationId: conversationId.value,
      characterId: request.characterId,
      requestId: request.requestId,
      turnId: "",
      content: "",
      status: "queued",
      createdAt: request.createdAt,
      assistantTurn: turn,
    };
  }

  function projectTurns() {
    renderTimer = null;
    const turnList = reducer.turns;
    const turnByMessageId = new Map<string, AssistantTurnData>();
    for (const turn of turnList) {
      const finalText = [...(turn.items || [])]
        .reverse()
        .find((item) => item.type === "text" && String(item.messageId || "").trim());
      if (finalText?.messageId) {
        turnByMessageId.set(String(finalText.messageId), turn);
      }
    }

    const nextCache = new Map<string, any>();
    const nextSignatures = new Map<string, string>();
    const renderedTurnIds = new Set<string>();
    const activeRequestIds = new Set<string>();
    const next = persistedMessages.value.map((message, index) => {
      if (message.role === "assistant" && message.requestId) { activeRequestIds.add(message.requestId); pendingAssistantRequests.delete(message.requestId); }
      const turn = turnByMessageId.get(String(message?.id || "")) || (message?.role === "assistant" && message?.requestId ? turnList.find((item) => item.requestId === message.requestId) : undefined);
      const requestId = String(message?.requestId || "").trim();
      const key = message.deliveryId ? `group-delivery:${message.deliveryId}` : turn ? assistantRowKey(turn) : message.role === "assistant" && requestId && message.id === `${requestId}/assistant` ? `request:${requestId}` : getMessageUIKey(message, index);
      const candidate = turn && (!message.deliveryId || turn.items?.some((item) => item.type === "tool_call"))
        ? {
            ...message,
            uiKey: key,
            turnId: turn.id,
            requestId: turn.requestId || message.requestId,
            assistantTurn: cloneTurn(turn),
            citationSnapshot: projectCitationSnapshot(
              message.citationSnapshot,
              [...(turn.items || [])].reverse().find((item) => item.type === "text" && item.messageId === String(message.id)),
            ),
            status: isTerminalTurn(turn.status) ? normalizedTurnStatus(turn.status) : message.status,
            state: isTerminalTurn(turn.status) ? normalizedTurnStatus(turn.status) : message.state,
          }
        : message.deliveryId || key.startsWith("request:") ? { ...message, uiKey: key } : message;
      const signature = `persisted:${messageSignature(candidate)}${turn ? `:${turnSignature(turn)}` : ""}`;
      const previous = projectionCache.get(key);
      const row = previous && projectionSignatures.get(key) === signature ? previous : candidate;
      nextCache.set(key, row);
      nextSignatures.set(key, signature);
      if (turn) {
        renderedTurnIds.add(turn.id);
        if (turn.requestId) activeRequestIds.add(String(turn.requestId));
      }
      return row;
    });

    for (const turn of turnList) {
      if (turn.requestId) activeRequestIds.add(String(turn.requestId));
      if (renderedTurnIds.has(turn.id)) continue;
      const uiKey = assistantRowKey(turn);
      const signature = `live:${turnSignature(turn)}`;
      const previous = projectionCache.get(uiKey);
      const row = previous && projectionSignatures.get(uiKey) === signature
        ? previous
        : liveMessageForTurn(turn);
      nextCache.set(uiKey, row);
      nextSignatures.set(uiKey, signature);
      next.push(row);
    }
    for (const request of pendingAssistantRequests.values()) {
      if (activeRequestIds.has(request.requestId)) continue;
      const uiKey = `request:${request.requestId}`;
      const message = pendingAssistantMessage(request);
      const signature = `pending:${request.requestId}:${request.createdAt}:${conversationId.value}`;
      const previous = projectionCache.get(uiKey);
      const row = previous && projectionSignatures.get(uiKey) === signature ? previous : message;
      nextCache.set(uiKey, row);
      nextSignatures.set(uiKey, signature);
      next.push(row);
    }
    next.sort(compareChatMessages);
    projectionCache = nextCache;
    projectionSignatures = nextSignatures;
    const unchanged = next.length === messages.value.length
      && next.every((message, index) => message === messages.value[index]);
    if (!unchanged) messages.value = next;
    scrollToBottom();
  }

  function scheduleProjection() {
    if (renderTimer !== null) return;
    if (typeof requestAnimationFrame === "function") {
      renderTimer = requestAnimationFrame(() => {
        renderTimer = null;
        projectTurns();
      });
      return;
    }
    renderTimer = setTimeout(projectTurns, 16);
  }

  async function applyEvent(event: AgentUIEvent) {
    if (!event || String(event.conversationId || "") !== String(conversationId.value || "")) return;
    const result = reducer.apply(event);
    if (result === "gap") {
      await recoverFromSnapshot();
      return;
    }
    if (result !== "applied") return;
    syncReducerState();
    publishWorkbenchEvent(event, String(conversationId.value || ""));
    if (event.requestId) pendingAssistantRequests.delete(String(event.requestId));

    if (event.type === "approval.requested") {
      publishApprovalState({
        action: "requested",
        id: String(event.payload?.approvalId || ""),
        conversationId: event.conversationId,
        turnId: event.turnId || "",
        toolCallId: event.callId || "",
        toolName: event.payload?.tool || event.payload?.toolName || "工具调用",
        arguments: event.payload?.arguments || "",
        riskLevel: event.payload?.risk || event.payload?.riskLevel || "",
        expiresAt: event.payload?.expiresAt || "",
      });
    } else if (["approval.approved", "approval.denied", "approval.expired"].includes(event.type)) {
      publishApprovalState({
        action: "resolved",
        id: String(event.payload?.approvalId || ""),
        conversationId: event.conversationId,
        turnId: event.turnId || "",
        approved: event.type === "approval.approved",
      });
    }

    const turn = event.turnId ? reducer.turn(event.turnId) : undefined;
    const groupTurn = isGroupTurn(event.requestId);
    if (event.type === "turn.waiting" && event.status === "needs_reconciliation") {
      if (!groupSending?.value && !groupTurn) sending.value = false;
    }
    if (event.type === "turn.queued" || event.type === "turn.started") {
      if (!groupTurn) sending.value = true;
      if (event.type === "turn.started") {
        notifyDesktopPetChatState("assistant_thinking", event.requestId || turn?.id || "");
      }
      void onConversationActivityChanged?.(event.conversationId);
    }
    if (event.type === "text.delta" || event.type === "text.started") {
      notifyDesktopPetChatState("assistant_speaking", event.requestId || turn?.id || "");
    }

    if (["turn.completed", "turn.failed", "turn.interrupted"].includes(event.type)) {
      if (!groupSending?.value && !groupTurn) sending.value = false;
      if (event.type === "turn.failed") {
        notifyDesktopPetChatState(
          "assistant_error",
          event.requestId || turn?.id || "",
          String(event.payload?.userMessage || event.payload?.error || "模型响应失败"),
        );
      } else {
        notifyDesktopPetChatState("assistant_finished", event.requestId || turn?.id || "");
      }
      scheduleProjection();
      await loadSnapshot();
      await onConversationActivityChanged?.(event.conversationId);
      return;
    }
    scheduleProjection();
  }

  function applySnapshot(snapshot: ConversationSnapshot) {
    if (Number(snapshot.version) !== 1) throw new Error(`Unsupported conversation snapshot version: ${snapshot.version}`);
    const completedTurns = reducer.turns.filter((turn) => isTerminalTurn(turn.status) && turn.conversationId === conversationId.value);
    reducer.reset(snapshot.turns || [], Number(snapshot.lastEventSequence || 0), snapshot.activeTurn || null);
    const savedRequestIds = new Set((snapshot.messages || []).filter((message) => message.role === "assistant").map((message) => String(message.requestId || message.request_id || "").trim()).filter(Boolean));
    reducer.mergeTurns(completedTurns.filter((turn) => turn.requestId && savedRequestIds.has(turn.requestId) && !(snapshot.turns || []).some((current) => current.requestId === turn.requestId && current.id !== turn.id)));
    turnHistoryCursor.value = String(snapshot.turnHistory?.nextCursor || "");
    hasMoreTurnHistory.value = snapshot.turnHistory?.hasMore === true;
    syncReducerState();
    const active = reducer.turn(reducer.activeTurnId);
    publishApprovalState({ action: "reset", conversationId: conversationId.value });
    for (const approval of snapshot.approvals || []) publishApprovalState({ action: "requested", ...approval });
    const previousGroupMessages = groupSending?.value ? persistedMessages.value.filter((message) => message.groupRoundId && message.deliveryId || message.role === "user" && message.status === "sending") : [];
    const snapshotMessages = (snapshot.messages || []).map((message) => normalizeRealtimeMessage(message));
    const snapshotIds = new Set(snapshotMessages.map((message) => String(message.id || "")));
    const sameDelivery = (a: any, b: any) => Boolean(a.deliveryId && b.deliveryId && a.deliveryId === b.deliveryId && a.groupRoundId && a.groupRoundId === b.groupRoundId);
    persistedMessages.value = [...snapshotMessages.map((message) => {
      const previous = previousGroupMessages.find((item) => item.id === message.id || sameDelivery(item, message));
      if (sameDelivery(previous || {}, message)) {
        if (message.status === "streaming") return { ...message, ...previous, uiKey: `group-delivery:${message.deliveryId}` };
        return { ...previous, ...message, createdAt: previous?.createdAt || message.createdAt, uiKey: `group-delivery:${message.deliveryId}`, saved: true, status: message.status || "completed" };
      }
      return previous && previous.status === "streaming" ? { ...message, ...previous } : message;
    }), ...previousGroupMessages.filter((message) => !snapshotIds.has(String(message.id || "")) && !snapshotMessages.some((item) => sameDelivery(message, item) || message.requestId && item.role === message.role && item.requestId === message.requestId))];
    sending.value = Boolean(groupSending?.value) || (!!active && !isGroupTurn(active.requestId) && active.status !== "needs_reconciliation");
    projectTurns();
  }

  function clear() {
    reducer.reset([], 0, null);
    pendingAssistantRequests.clear();
    syncReducerState();
    sending.value = false;
    projectTurns();
  }

  function revokeRuntimeAuthority(id: string) {
    disconnect();
    recoverySnapshotRequired = true;
    persistedMessages.value = [];
    turnHistoryCursor.value = '';
    hasMoreTurnHistory.value = false;
    automationStatus.value = null;
    clear();
    publishApprovalState({ action: 'reset', conversationId: id });
    window.dispatchEvent(new CustomEvent('amitia:workbench-task-revoked', { detail: { conversationId: id } }));
  }

  function beginPendingAssistant(requestId: string, characterId?: string) {
    const id = String(requestId || "").trim();
    if (!id) return;
    const request = {
      requestId: id,
      characterId: String(characterId || "").trim(),
      createdAt: new Date().toISOString(),
    };
    pendingAssistantRequests.set(id, request);
    const existingKeys = new Set(messages.value.map((message) => getMessageUIKey(message)));
    const optimisticRows = persistedMessages.value.filter((message) => message.requestId === id && (message.role === "user" || message.role === "assistant")).map((message) => message.role === "assistant" ? { ...pendingAssistantMessage(request), ...message } : message).filter((message) => !existingKeys.has(getMessageUIKey(message)));
    if (!optimisticRows.some((message) => message.role === "assistant") && !existingKeys.has(`request:${id}`)) optimisticRows.push(pendingAssistantMessage(request));
    if (optimisticRows.length) messages.value = [...messages.value, ...optimisticRows];
    scheduleProjection();
  }

  function failPendingAssistant(requestId: string) {
    const id = String(requestId || "").trim();
    if (!id || !pendingAssistantRequests.delete(id)) return;
    projectTurns();
  }

  async function loadSnapshot(notify = true): Promise<boolean> {
    const id = String(conversationId.value || "").trim();
    const epoch = ++snapshotEpoch;
    if (!id) {
      clear();
      return false;
    }
    const snapshot = await ownedChatRuntimeRequest<ConversationSnapshot>("snapshot", { conversationId: id });
    if (epoch !== snapshotEpoch || id !== String(conversationId.value || "").trim()) return false;
    if (snapshot.conversation) snapshot.conversation.id = id;
    snapshot.turns = snapshot.turns?.map((turn) => ({ ...turn, conversationId: id }));
    snapshot.approvals = snapshot.approvals?.map((approval) => ({ ...approval, conversationId: id }));
    applySnapshot(snapshot);
    window.dispatchEvent(new CustomEvent('amitia:workbench-task-resync', { detail: { conversationId: id } }));
    if (notify) await onConversationSnapshot?.(snapshot.conversation, snapshot.workspace ?? null, snapshot);
    return epoch === snapshotEpoch && id === String(conversationId.value || "").trim();
  }

  async function loadOlderTurns(): Promise<boolean> {
    const epoch = snapshotEpoch;
    const id = String(conversationId.value || "").trim();
    if (!id || !hasMoreTurnHistory.value || !turnHistoryCursor.value) return false;
    const response = await ownedChatRuntimeRequest("turns", { conversationId: id, cursor: turnHistoryCursor.value, limit: 50 });
    const rows = Array.isArray(response?.items) ? response.items.map((turn: any) => ({ ...turn, conversationId: id })) : [];
    if (epoch !== snapshotEpoch || id !== String(conversationId.value || "").trim()) return false;
    if (rows.length === 0) {
      hasMoreTurnHistory.value = false;
      return false;
    }
    reducer.mergeTurns(rows as AssistantTurnData[]);
    turnHistoryCursor.value = String(response?.nextCursor || "");
    hasMoreTurnHistory.value = response?.hasMore === true;
    syncReducerState();
    scheduleProjection();
    return true;
  }

  function currentConnection(epoch: number, id: string): boolean {
    return !disposed && connectionEpoch === epoch && id === String(conversationId.value || "").trim();
  }

  async function recoverFromSnapshot() {
    disconnect();
    const epoch = connectionEpoch;
    const id = String(conversationId.value || "").trim();
    recoverySnapshotRequired = true;
    try {
      if (!currentConnection(epoch, id) || !await loadSnapshot()) return;
      if (!currentConnection(epoch, id)) return;
      recoverySnapshotRequired = false;
      if (id) void connect(false);
    } catch {
      if (currentConnection(epoch, id)) scheduleReconnect();
    }
  }

  function scheduleReconnect() {
    if (disposed || !conversationId.value || reconnectTimer) return;
    if (automationStatus.value) automationStatus.value = {
      ...automationStatus.value,
      phase: "waiting",
      label: "正在同步自动化状态",
      detail: "连接恢复后确认操作状态",
    };
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      void connect(recoverySnapshotRequired);
    }, 1500);
  }

  async function consume(response: Response, signal: AbortSignal, expected: any, id: string, canonicalId: string, epoch: number) {
    if (!response.body) throw new Error('Runtime SSE 响应没有事件流');
    const reader = response.body.getReader();
    const decoder = new TextDecoder('utf-8', { fatal: true });
    const frames = new OwnedRuntimeSSEDecoder();
    async function accept(frame: OwnedRuntimeSSEFrame): Promise<boolean> {
      if (signal.aborted || !currentConnection(epoch, id)) return false;
      if (frame.type === 'snapshot_required') {
        await recoverFromSnapshot();
        return false;
      }
      let event: AgentUIEvent;
      try {
        event = JSON.parse(frame.data) as AgentUIEvent;
      } catch {
        await recoverFromSnapshot();
        return false;
      }
      const eventScope = (event as AgentUIEvent & { executionScope?: OwnedExecutionScope })?.executionScope;
      if (!event || typeof event !== 'object' || event.version !== 1 ||
          !Number.isSafeInteger(event.eventSequence) || event.eventSequence <= 0 ||
          event.conversationId !== canonicalId || !eventScope ||
          !sameOwnedAuthority(expected, eventScope) ||
          owned.coreId.value !== expected.coreId) {
        await recoverFromSnapshot();
        return false;
      }
      await applyEvent({ ...event, conversationId: id });
      return !signal.aborted && currentConnection(epoch, id);
    }
    try {
      while (!signal.aborted && currentConnection(epoch, id)) {
        const { done, value } = await reader.read();
        if (signal.aborted || !currentConnection(epoch, id)) break;
        const decoded = done ? frames.push(decoder.decode()).concat(frames.finish()) : frames.push(decoder.decode(value, { stream: true }));
        for (const frame of decoded) {
          if (!await accept(frame)) return;
        }
        if (done) break;
      }
    } catch {
      if (!signal.aborted && currentConnection(epoch, id)) await recoverFromSnapshot();
    } finally {
      reader.releaseLock();
    }
  }

  function connect(withSnapshot = true): Promise<void> {
    const id = String(conversationId.value || "").trim();
    if (!id || disposed) return Promise.resolve();
    if (connectionTask) {
      if (connectionTaskId === id) return connectionTask;
      disconnect();
    }
    const task = establishConnection(id, withSnapshot).finally(() => {
      if (connectionTask !== task) return;
      connectionTask = null;
      connectionTaskId = "";
      streamOpen = false;
    });
    connectionTask = task;
    connectionTaskId = id;
    return task;
  }

  async function establishConnection(id: string, withSnapshot: boolean) {
    const epoch = connectionEpoch;
    try {
      if (withSnapshot || recoverySnapshotRequired) {
        recoverySnapshotRequired = true;
        if (!await loadSnapshot() || !currentConnection(epoch, id)) return;
        recoverySnapshotRequired = false;
      }
      if (!currentConnection(epoch, id)) return;
      const captured = snapshotEpoch;
      const path = "/api/device-mesh/v1/business/chat-runtime/events";
      const afterSequence = lastEventSequence.value;
      const { payload, expected } = await prepareOwnedChatRequest({ conversationId: id, afterSequence });
      const canonicalId = String(payload.conversationId || id).trim();
      if (!currentConnection(epoch, id) || captured !== snapshotEpoch) return;
      if (owned.coreId.value !== expected.coreId) {
        recoverySnapshotRequired = true;
        scheduleReconnect();
        return;
      }
      const controller = new AbortController();
      abortController = controller;
      const [url, init] = await Promise.all([
        resolveApiUrl(path),
        createAuthenticatedFetchInit(path, {
          headers: {
            Accept: "text/event-stream",
            "Content-Type": "application/json",
            ...(afterSequence > 0 ? { "Last-Event-ID": String(afterSequence) } : {}),
          },
          signal: controller.signal,
          method: "POST",
          body: JSON.stringify(payload),
        }),
      ]);
      if (!currentConnection(epoch, id) || captured !== snapshotEpoch || controller.signal.aborted) return;
      const response = await fetch(url, init);
      if (!currentConnection(epoch, id) || captured !== snapshotEpoch || controller.signal.aborted) { await response.body?.cancel(); return; }
      if (!response.ok || !response.headers.get("content-type")?.includes("text/event-stream")) {
        await response.body?.cancel();
        if (!currentConnection(epoch, id) || controller.signal.aborted) return;
        if (response.status === 401 || response.status === 403) {
          revokeRuntimeAuthority(id);
          return;
        }
        if (response.status === 404 || response.status === 409 || response.ok) recoverySnapshotRequired = true;
        throw new Error(`HTTP ${response.status}`);
      }
      streamOpen = true;
      window.dispatchEvent(new CustomEvent('amitia:workbench-task-resync', { detail: { conversationId: id } }));
      await consume(response, controller.signal, expected, id, canonicalId, epoch);
      if (!controller.signal.aborted && currentConnection(epoch, id)) scheduleReconnect();
    } catch (cause: any) {
      if (!currentConnection(epoch, id)) return;
      const status = Number(cause?.response?.status || cause?.status);
      if (status === 401 || status === 403) {
        revokeRuntimeAuthority(id);
        return;
      }
      if ([404, 409].includes(status)) recoverySnapshotRequired = true;
      scheduleReconnect();
    }
  }

  function disconnect() {
    connectionEpoch++;
    snapshotEpoch++;
    connectionTask = null;
    connectionTaskId = "";
    streamOpen = false;
    automationStatus.value = null;
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    if (abortController) {
      abortController.abort();
      abortController = null;
    }
  }

  function cleanup() {
    disposed = true;
    automationStatus.value = null;
    disconnect();
    window.removeEventListener("amitia:owned-runtime-refresh", refreshRuntime);
    window.removeEventListener("amitia:runtime-connection-changed", invalidateRuntimeAuthority);
    window.removeEventListener("amitia:execution-scope-changed", invalidateRuntimeAuthority);
    if (renderTimer) {
      if (typeof requestAnimationFrame === "function") cancelAnimationFrame(renderTimer as number);
      else clearTimeout(renderTimer as ReturnType<typeof setTimeout>);
      renderTimer = null;
    }
  }

  function refreshRuntime(event: Event) {
    if ((event as CustomEvent).detail?.conversationId !== conversationId.value) return;
    if (streamOpen || connectionTask) return;
    void connect();
  }

  function invalidateRuntimeAuthority() {
    if (!disposed && conversationId.value) revokeRuntimeAuthority(conversationId.value);
  }

  window.addEventListener("amitia:owned-runtime-refresh", refreshRuntime);
  window.addEventListener("amitia:runtime-connection-changed", invalidateRuntimeAuthority);
  window.addEventListener("amitia:execution-scope-changed", invalidateRuntimeAuthority);

  watch(persistedMessages, scheduleProjection, { deep: true });
  if (groupSending) watch(groupSending, (current, previous) => { if (previous && !current) sending.value = false; });
  projectTurns();

  return {
    messages,
    automationStatus,
    activeTurnId,
    activeExecutionId,
    lastEventSequence,
    hasMoreTurnHistory,
    connect,
    disconnect,
    cleanup,
    loadSnapshot,
    loadOlderTurns,
    applyEvent,
    clear,
    beginPendingAssistant,
    failPendingAssistant,
  };
}
