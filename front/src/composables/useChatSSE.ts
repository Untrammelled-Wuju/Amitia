// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { nextTick, type Ref } from "vue";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import { createAuthenticatedFetchInit } from "../runtime/request-auth";
import {
  compareChatMessages,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

type ParsedSSEEvent = { event: string; data: string };

async function consumeSSE(
  response: Response,
  onEvent: (event: ParsedSSEEvent) => void,
): Promise<void> {
  const reader = response.body?.getReader();
  if (!reader) throw new Error("SSE response body is empty");
  const decoder = new TextDecoder();
  let buffer = "";
  let eventName = "message";
  let dataLines: string[] = [];

  const dispatch = () => {
    if (dataLines.length === 0) {
      eventName = "message";
      return;
    }
    onEvent({ event: eventName || "message", data: dataLines.join("\n") });
    eventName = "message";
    dataLines = [];
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split(/\r?\n/);
      buffer = lines.pop() || "";
      for (const line of lines) {
        if (line === "") {
          dispatch();
          continue;
        }
        if (line.startsWith(":")) continue;
        if (line.startsWith("event:")) {
          eventName = line.slice(6).trim() || "message";
        } else if (line.startsWith("data:")) {
          dataLines.push(line.slice(5).trimStart());
        }
      }
    }
    if (buffer.startsWith("data:")) dataLines.push(buffer.slice(5).trimStart());
    dispatch();
  } finally {
    reader.releaseLock();
  }
}

export function useChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
) {
  let messageController: AbortController | null = null;
  let proactiveController: AbortController | null = null;
  let messageReconnectTimer: number | null = null;
  let proactiveReconnectTimer: number | null = null;
  let messageEnabled = false;
  let proactiveEnabled = false;
  let lastPolledMsgId: string | null = null;

  function getLastPolledMsgId() {
    return lastPolledMsgId;
  }

  function setLastPolledMsgId(id: string | null) {
    lastPolledMsgId = id;
  }

  function clearTimer(timer: number | null) {
    if (timer !== null) window.clearTimeout(timer);
  }

  function handleMessageData(data: string) {
    try {
      const msg = normalizeRealtimeMessage(JSON.parse(data));
      if (!msg.role || msg.role === "tool") return;
      if ((msg as any).tool_calls_json) return;
      if (mergeChatMessage(messages.value, msg)) {
        messages.value.sort(compareChatMessages);
        lastPolledMsgId = msg.id || lastPolledMsgId;
        return;
      }
      if (messages.value.some((m: any) => m.id === msg.id)) return;
      if (msg.role === "user") {
        const now = Date.now();
        const duplicate = messages.value.some(
          (m: any) =>
            m.role === "user" &&
            m.content === msg.content &&
            String(m.id).startsWith("user-") &&
            now - new Date(m.createdAt).getTime() < 15000,
        );
        if (duplicate) return;
      }
      lastPolledMsgId = msg.id || lastPolledMsgId;
      messages.value.push(msg);
      messages.value.sort(compareChatMessages);
      if (
        msg.source === "proactive" &&
        "Notification" in window &&
        Notification.permission === "granted"
      ) {
        new Notification("日程提醒", {
          body: String(msg.content || "").slice(0, 200),
          tag: "reminder-" + msg.id,
        });
      }
      scrollToBottom();
      fetchWechatMsgCount();
      fetchQQStatus();
    } catch {}
  }

  async function openMessageStream() {
    if (!messageEnabled || !convId.value) return;
    messageController?.abort();
    const controller = new AbortController();
    messageController = controller;
    const path = "/api/messages/stream";
    try {
      const baseUrl = await resolveApiUrl(path);
      const query =
        "?conversationId=" +
        encodeURIComponent(convId.value) +
        (lastPolledMsgId ? "&since=" + encodeURIComponent(lastPolledMsgId) : "");
      const init = await createAuthenticatedFetchInit(path, {
        method: "GET",
        headers: { Accept: "text/event-stream" },
        cache: "no-store",
        signal: controller.signal,
      });
      const response = await fetch(baseUrl + query, init);
      if (!response.ok) throw new Error(`SSE HTTP ${response.status}`);
      await consumeSSE(response, ({ event, data }) => {
        if (event === "message" || event === "chat_message") handleMessageData(data);
      });
    } catch (error: any) {
      if (error?.name !== "AbortError") console.warn("[ChatSSE] message stream closed:", error);
    } finally {
      if (messageController === controller) messageController = null;
      if (messageEnabled && convId.value && !controller.signal.aborted) {
        clearTimer(messageReconnectTimer);
        messageReconnectTimer = window.setTimeout(() => void openMessageStream(), 3000);
      }
    }
  }

  function connectSSE() {
    messageEnabled = true;
    clearTimer(messageReconnectTimer);
    messageReconnectTimer = null;
    void openMessageStream();
  }

  function disconnectSSE() {
    messageEnabled = false;
    clearTimer(messageReconnectTimer);
    messageReconnectTimer = null;
    messageController?.abort();
    messageController = null;
  }

  function handleProactiveData(data: string) {
    try {
      const msg = normalizeRealtimeMessage(JSON.parse(data));
      if (msg.conversationId === convId.value) {
        if (!mergeChatMessage(messages.value, msg)) {
          messages.value.push({
            ...msg,
            createdAt: msg.createdAt || new Date().toISOString(),
          });
        }
        messages.value.sort(compareChatMessages);
        nextTick(() => scrollToBottom());
      }
    } catch {}
    fetchWechatMsgCount();
    fetchQQStatus();
  }

  async function openProactiveStream() {
    if (!proactiveEnabled) return;
    proactiveController?.abort();
    const controller = new AbortController();
    proactiveController = controller;
    const path = "/api/proactive-sse";
    try {
      const url = await resolveApiUrl(path);
      const init = await createAuthenticatedFetchInit(path, {
        method: "GET",
        headers: { Accept: "text/event-stream" },
        cache: "no-store",
        signal: controller.signal,
      });
      const response = await fetch(url, init);
      if (!response.ok) throw new Error(`SSE HTTP ${response.status}`);
      await consumeSSE(response, ({ event, data }) => {
        if (event === "proactive_message" || event === "message") handleProactiveData(data);
      });
    } catch (error: any) {
      if (error?.name !== "AbortError") console.warn("[ChatSSE] proactive stream closed:", error);
    } finally {
      if (proactiveController === controller) proactiveController = null;
      if (proactiveEnabled && !controller.signal.aborted) {
        clearTimer(proactiveReconnectTimer);
        proactiveReconnectTimer = window.setTimeout(() => void openProactiveStream(), 5000);
      }
    }
  }

  function connectProactiveSSE() {
    proactiveEnabled = true;
    clearTimer(proactiveReconnectTimer);
    proactiveReconnectTimer = null;
    void openProactiveStream();
  }

  function disconnectProactiveSSE() {
    proactiveEnabled = false;
    clearTimer(proactiveReconnectTimer);
    proactiveReconnectTimer = null;
    proactiveController?.abort();
    proactiveController = null;
  }

  return {
    connectSSE,
    disconnectSSE,
    connectProactiveSSE,
    disconnectProactiveSSE,
    getLastPolledMsgId,
    setLastPolledMsgId,
  };
}
