import { type Ref, ref, shallowRef, watch } from "vue";
import { useDeviceOwnedConversation } from "./useDeviceOwnedConversation";
import { notifyReplyCompleted } from "./useReplyNotifications";
import { projectAutomationStatus, type AutomationStatus } from "@/conversation/runtime/automationStatus";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import type { AssistantTurnData } from "@/conversation/rendering/types";
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
  let renderTimer: ReturnType<typeof setTimeout> | null = null;
  let snapshotEpoch = 0;
  let disposed = false;

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
      status: isTerminalTurn(turn.status) ? normalizedTurnStatus(turn.status) : "streaming",
      createdAt: turn.createdAt || new Date().toISOString(),
      assistantTurn: cloneTurn(turn),
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
      const turn = turnByMessageId.get(String(message?.id || "")) || (message?.role === "assistant" && message?.requestId ? turnList.find((item) => item.requestId === message.requestId) : undefined);
      const key = turn ? assistantRowKey(turn) : getMessageUIKey(message, index);
      const candidate = turn
        ? {
            ...message,
            uiKey: key,
            turnId: turn.id,
            requestId: turn.requestId || message.requestId,
            assistantTurn: cloneTurn(turn),
            status: isTerminalTurn(turn.status) ? normalizedTurnStatus(turn.status) : message.status,
          }
        : message;
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
    if (renderTimer) return;
    renderTimer = setTimeout(projectTurns, 24);
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
    if (event.type === "turn.waiting" && event.status === "needs_reconciliation") {
      sending.value = false;
    }
    if (event.type === "turn.queued" || event.type === "turn.started") {
      sending.value = true;
      if (event.type === "turn.started") {
        notifyDesktopPetChatState("assistant_thinking", event.requestId || turn?.id || "");
      }
      void onConversationActivityChanged?.(event.conversationId);
    }
    if (event.type === "text.delta" || event.type === "text.started") {
      notifyDesktopPetChatState("assistant_speaking", event.requestId || turn?.id || "");
    }

    if (["turn.completed", "turn.failed", "turn.interrupted"].includes(event.type)) {
      sending.value = false;
      if (event.type === "turn.completed" && !event.parentTurnId && !turn?.parentTurnId) {
        void notifyReplyCompleted(event.conversationId, event.turnId || "");
      }
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
    reducer.reset(snapshot.turns || [], Number(snapshot.lastEventSequence || 0), snapshot.activeTurn || null);
    turnHistoryCursor.value = String(snapshot.turnHistory?.nextCursor || "");
    hasMoreTurnHistory.value = snapshot.turnHistory?.hasMore === true;
    syncReducerState();
    const active = reducer.turn(reducer.activeTurnId);
    sending.value = !!active && active.status !== "needs_reconciliation";
    publishApprovalState({ action: "reset", conversationId: conversationId.value });
    for (const approval of snapshot.approvals || []) publishApprovalState({ action: "requested", ...approval });
    persistedMessages.value = (snapshot.messages || []).map((message) => normalizeRealtimeMessage(message));
    projectTurns();
  }

  function clear() {
    reducer.reset([], 0, null);
    pendingAssistantRequests.clear();
    syncReducerState();
    sending.value = false;
    projectTurns();
  }

  function beginPendingAssistant(requestId: string, characterId?: string) {
    const id = String(requestId || "").trim();
    if (!id) return;
    pendingAssistantRequests.set(id, {
      requestId: id,
      characterId: String(characterId || "").trim(),
      createdAt: new Date().toISOString(),
    });
    projectTurns();
  }

  function failPendingAssistant(requestId: string) {
    const id = String(requestId || "").trim();
    if (!id || !pendingAssistantRequests.delete(id)) return;
    projectTurns();
  }

  async function loadSnapshot(notify = true) {
    const id = String(conversationId.value || "").trim();
    const epoch = ++snapshotEpoch;
    if (!id) {
      clear();
      return;
    }
    const snapshot = await ownedChatRuntimeRequest<ConversationSnapshot>("snapshot", { conversationId: id });
    if (epoch !== snapshotEpoch || id !== String(conversationId.value || "").trim()) return;
    if (snapshot.conversation) snapshot.conversation.id = id;
    snapshot.turns = snapshot.turns?.map((turn) => ({ ...turn, conversationId: id }));
    snapshot.approvals = snapshot.approvals?.map((approval) => ({ ...approval, conversationId: id }));
    applySnapshot(snapshot);
    if (notify) await onConversationSnapshot?.(snapshot.conversation, snapshot.workspace ?? null, snapshot);
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

  async function recoverFromSnapshot() {
    disconnect();
    try {
      await loadSnapshot();
    } finally {
      if (!disposed && conversationId.value) void connect(false);
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
      void connect(false);
    }, 1500);
  }

  async function consume(response: Response, signal: AbortSignal, expected: any, id: string) {
    if (!response.body) return;
    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    try {
      while (!signal.aborted) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");
        let boundary = buffer.indexOf("\n\n");
        while (boundary >= 0) {
          const block = buffer.slice(0, boundary);
          buffer = buffer.slice(boundary + 2);
          const type = block.split("\n").find((line) => line.startsWith("event:"))?.slice(6).trim() || "";
          const data = block.split("\n").filter((line) => line.startsWith("data:")).map((line) => line.slice(5).trimStart()).join("\n");
          if (type === "snapshot.required") {
            await recoverFromSnapshot();
            return;
          }
          if (type === "agent_ui_event" && data) {
            try {
              const event = JSON.parse(data);
              if (!sameOwnedAuthority(expected, event.executionScope) || owned.coreId.value !== expected.coreId) throw new Error("运行事件的数据归属已变化");
              await applyEvent({ ...event, conversationId: id } as AgentUIEvent);
            } catch {}
          }
          boundary = buffer.indexOf("\n\n");
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  async function connect(withSnapshot = true) {
    disconnect();
    const id = String(conversationId.value || "").trim();
    if (!id || disposed) return;
    try {
      if (withSnapshot) await loadSnapshot();
      if (id !== String(conversationId.value || "")) return;
      const captured = snapshotEpoch;
      const path = "/api/device-mesh/v1/business/chat-runtime/events";
      const { payload, expected } = await prepareOwnedChatRequest({ conversationId: id, afterSequence: lastEventSequence.value });
      const controller = new AbortController();
      abortController = controller;
      const [url, init] = await Promise.all([
        resolveApiUrl(path),
        createAuthenticatedFetchInit(path, {
          headers: {
            Accept: "text/event-stream",
            "Content-Type": "application/json",
            ...(lastEventSequence.value > 0 ? { "Last-Event-ID": String(lastEventSequence.value) } : {}),
          },
          signal: controller.signal,
          method: "POST",
          body: JSON.stringify(payload),
        }),
      ]);
      if (captured !== snapshotEpoch || controller.signal.aborted || id !== String(conversationId.value || "")) return;
      const response = await fetch(url, init);
      if (captured !== snapshotEpoch || controller.signal.aborted || id !== String(conversationId.value || "")) { await response.body?.cancel(); return; }
      if (!response.ok || !response.headers.get("content-type")?.includes("text/event-stream")) throw new Error(`HTTP ${response.status}`);
      await consume(response, controller.signal, expected, id);
      if (!controller.signal.aborted) scheduleReconnect();
    } catch {
      scheduleReconnect();
    }
  }

  function disconnect() {
    snapshotEpoch++;
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
    if (renderTimer) {
      clearTimeout(renderTimer);
      renderTimer = null;
    }
  }

  function refreshRuntime(event: Event) {
    if ((event as CustomEvent).detail?.conversationId === conversationId.value) void connect();
  }

  window.addEventListener("amitia:owned-runtime-refresh", refreshRuntime);

  watch(persistedMessages, scheduleProjection, { deep: true });
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
