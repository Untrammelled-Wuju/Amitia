// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, nextTick } from "vue";
import {
  createAuthorizedRequestInit,
  resolveApiUrl,
} from "../runtime/runtime-adapter";
import { getAccessToken } from "../stores/refresh-coordinator";
import { calcTypingDelay } from "@/utils/typing";
import { emitConversationRuntimeEvent } from "@/ui-runtime/conversationProjection";
import {
  compareChatMessages,
  insertTransientModelError,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

export function isWebChatReplyEvent(message: any): boolean {
  return message?.role === "assistant";
}

export function isVisionErrorReplyEvent(message: any): boolean {
  return isWebChatReplyEvent(message) && message?.msgType === "vision_error";
}

export function isTransientModelErrorReplyEvent(message: any): boolean {
  return (
    isWebChatReplyEvent(message) &&
    ["vision_error", "text_error", "voice_error", "vector_error"].includes(
      message?.msgType,
    )
  );
}

export function shouldFinishSendingForModelError(message: any): boolean {
  return message?.msgType === "text_error";
}

export function useWebChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
  sending: Ref<boolean>,
) {
  let eventAbortController: AbortController | null = null;
  let lastPolledMsgId: string | null = null;
  let eventReconnectTimer: ReturnType<typeof setTimeout> | null = null;
  let proactiveListener: ((event: Event) => void) | null = null;
  let active = true;

  const typingQueue: any[] = [];
  let typingTimer: ReturnType<typeof setTimeout> | null = null;

  function getLastPolledMsgId() {
    return lastPolledMsgId;
  }
  function setLastPolledMsgId(id: string | null) {
    lastPolledMsgId = id;
  }

  function sortMessages() {
    messages.value.sort(compareChatMessages);
  }

  function processTypingQueue() {
    if (typingQueue.length === 0) return;
    const raw = typingQueue.shift()!;
    const delay = calcTypingDelay(raw.content || "");
    typingTimer = setTimeout(() => {
      raw.typingDone = true;
      messages.value.push(raw);
      sortMessages();
      lastPolledMsgId = raw.id || lastPolledMsgId;
      scrollToBottom();
      fetchWechatMsgCount();
      fetchQQStatus();
      typingTimer = null;
      setTimeout(() => processTypingQueue(), 300);
    }, delay);
  }

  function clearTypingQueue() {
    typingQueue.length = 0;
    if (typingTimer) {
      clearTimeout(typingTimer);
      typingTimer = null;
    }
  }

  function normalizeEventMessage(event: any): any | null {
    if (!event.messageId && !event.id) return null;
    const metadata =
      event.data && typeof event.data === "object" ? event.data : {};
    return {
      id: event.messageId || event.id,
      conversationId: event.conversationId,
      role: event.role,
      content: event.content || "",
      createdAt: event.createdAt || new Date().toISOString(),
      status: event.status,
      channel: event.channel,
      direction: event.direction,
      msgType: metadata.messageType,
      rawError: metadata.rawError,
      requestId: metadata.requestId,
      anchorMessageId: metadata.userMessageId,
      anchorSequence: metadata.userMessageSequence,
    };
  }

  function handleMessageEvent(event: MessageEvent) {
    try {
      const raw = JSON.parse(event.data);
      const msg = normalizeEventMessage(raw);
      if (!msg) return;
      if (msg.conversationId !== convId.value) return;
      if (!isWebChatReplyEvent(msg)) return;
      if ((msg as any).tool_calls_json) return;
      if (mergeChatMessage(messages.value, msg)) {
        lastPolledMsgId = msg.id || lastPolledMsgId;
        sortMessages();
        nextTick(() => scrollToBottom());
        return;
      }
      if (mergeChatMessage(typingQueue, msg)) {
        lastPolledMsgId = msg.id || lastPolledMsgId;
        return;
      }
      if (
        !messages.value.some((m: any) => m.id === msg.id) &&
        !typingQueue.some((m: any) => m.id === msg.id)
      ) {
        if (isTransientModelErrorReplyEvent(msg)) {
          if (shouldFinishSendingForModelError(msg)) sending.value = false;
          insertTransientModelError(messages.value, {
            ...msg,
            typingDone: true,
          });
          lastPolledMsgId = msg.id || lastPolledMsgId;
          sortMessages();
          nextTick(() => scrollToBottom());
          return;
        }
        sending.value = false;
        typingQueue.push(msg);
        if (!typingTimer) processTypingQueue();
      }
    } catch {}
  }

  function scheduleEventReconnect() {
    if (!active || !convId.value || eventReconnectTimer) return;
    eventReconnectTimer = setTimeout(() => {
      eventReconnectTimer = null;
      void connectSSE();
    }, 3000);
  }

  async function consumeMessageStream(response: Response, signal: AbortSignal) {
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
        while (boundary !== -1) {
          const block = buffer.slice(0, boundary);
          buffer = buffer.slice(boundary + 2);
          const type = block
            .split("\n")
            .find((line) => line.startsWith("event:"))
            ?.slice(6)
            .trim();
          const data = block
            .split("\n")
            .filter((line) => line.startsWith("data:"))
            .map((line) => line.slice(5).trimStart())
            .join("\n");
          if (data && type) {
            try {
              const payload = JSON.parse(data) as Record<string, unknown>;
              const conversationId = String(payload.conversationId ?? convId.value ?? "");
              if (conversationId) {
                emitConversationRuntimeEvent({
                  id: String(payload.messageId ?? payload.id ?? `${type}-${Date.now()}`),
                  eventType: type,
                  conversationId,
                  timestamp: String(payload.createdAt ?? new Date().toISOString()),
                  source: "message_sse",
                  payload,
                });
              }
            } catch {}
          }
          if (data && (type === "message_created" || type === "message_updated")) {
            handleMessageEvent({ data } as MessageEvent);
          }
          boundary = buffer.indexOf("\n\n");
        }
      }
    } finally {
      reader.releaseLock();
    }
  }

  async function connectSSE() {
    if (!active) return;
    disconnectSSE();
    if (!convId.value) return;
    const controller = new AbortController();
    eventAbortController = controller;
    try {
      const url =
        (await resolveApiUrl("/api/messages/events")) + "?channel=web";
      const init = await createAuthorizedRequestInit({
        headers: { Accept: "text/event-stream" },
        signal: controller.signal,
      });
      const token = getAccessToken();
      if (token) {
        (init.headers as Record<string, string>).Authorization = `Bearer ${token}`;
      }
      const response = await fetch(url, init);
      if (!response.ok || !response.headers.get("content-type")?.includes("text/event-stream")) {
        throw new Error("聊天事件流不可用");
      }
      await consumeMessageStream(response, controller.signal);
    } catch {}
    if (!controller.signal.aborted) scheduleEventReconnect();
  }

  function disconnectSSE() {
    if (eventReconnectTimer) {
      clearTimeout(eventReconnectTimer);
      eventReconnectTimer = null;
    }
    if (eventAbortController) {
      eventAbortController.abort();
      eventAbortController = null;
    }
  }

  async function connectProactiveSSE() {
    if (!active || proactiveListener) return;
    proactiveListener = (event) => {
      const data = (event as CustomEvent<string>).detail;
      if (!data) return;
      try {
        const e = { data };
        try {
          const msg = normalizeRealtimeMessage(JSON.parse(e.data));
          if (msg.conversationId === convId.value) {
            if (mergeChatMessage(messages.value, msg)) sortMessages();
            else if (!mergeChatMessage(typingQueue, msg)) {
              messages.value.push({
                ...msg,
                createdAt: msg.createdAt || new Date().toISOString(),
              });
              sortMessages();
            }
            nextTick(() => scrollToBottom());
          }
        } catch {}
        fetchWechatMsgCount();
        fetchQQStatus();
      } catch {}
    };
    window.addEventListener("amitia:proactive-message", proactiveListener);
  }

  function disconnectProactiveSSE() {
    if (!proactiveListener) return;
    window.removeEventListener("amitia:proactive-message", proactiveListener);
    proactiveListener = null;
  }

  function cleanup() {
    active = false;
    disconnectSSE();
    disconnectProactiveSSE();
    clearTypingQueue();
  }

  return {
    connectSSE,
    disconnectSSE,
    connectProactiveSSE,
    disconnectProactiveSSE,
    cleanup,
    getLastPolledMsgId,
    setLastPolledMsgId,
  };
}
