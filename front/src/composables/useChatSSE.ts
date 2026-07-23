// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref, nextTick } from "vue";
import { resolveApiUrl } from "../runtime/runtime-adapter";
import {
  compareChatMessages,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

export function useChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
) {
  let eventSource: EventSource | null = null;
  let lastPolledMsgId: string | null = null;

  function getLastPolledMsgId() {
    return lastPolledMsgId;
  }
  function setLastPolledMsgId(id: string | null) {
    lastPolledMsgId = id;
  }

  function handleMessage(event: MessageEvent) {
    try {
      const msg = normalizeRealtimeMessage(JSON.parse(event.data));
      if (!msg.role || msg.role === "tool") return;
      if ((msg as any).tool_calls_json) return;
      if (mergeChatMessage(messages.value, msg)) {
        messages.value.sort(compareChatMessages);
        lastPolledMsgId = msg.id || lastPolledMsgId;
        return;
      }
      if (!messages.value.some((m: any) => m.id === msg.id)) {
        if (msg.role === "user") {
          const now = Date.now();
          const dup = messages.value.some(
            (m: any) =>
              m.role === "user" &&
              m.content === msg.content &&
              String(m.id).startsWith("user-") &&
              now - new Date(m.createdAt).getTime() < 15000,
          );
          if (dup) return;
        }
        lastPolledMsgId = msg.id || lastPolledMsgId;
        messages.value.push(msg);
        messages.value.sort(compareChatMessages);
        if (
          msg.source === "proactive" &&
          "Notification" in window &&
          (Notification as any).permission === "granted"
        ) {
          new Notification("日程提醒", {
            body: msg.content.slice(0, 200),
            tag: "reminder-" + msg.id,
          });
        }
        scrollToBottom();
        fetchWechatMsgCount();
        fetchQQStatus();
      }
    } catch {}
  }

  async function connectSSE() {
    disconnectSSE();
    if (!convId.value) return;
    const baseUrl = await resolveApiUrl("/api/messages/stream");
    const url =
      baseUrl +
      "?conversationId=" +
      encodeURIComponent(convId.value) +
      (lastPolledMsgId ? "&since=" + encodeURIComponent(lastPolledMsgId) : "");
    eventSource = new EventSource(url);
    eventSource.addEventListener("message", handleMessage);
    eventSource.onerror = () => {
      disconnectSSE();
      setTimeout(() => {
        if (convId.value) void connectSSE();
      }, 3000);
    };
  }

  function disconnectSSE() {
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  }

  let proactiveSSE: EventSource | null = null;
  async function connectProactiveSSE() {
    try {
      const url = await resolveApiUrl("/api/proactive-sse");
      proactiveSSE = new EventSource(url);
      proactiveSSE.addEventListener("proactive_message", (e) => {
        try {
          const msg = normalizeRealtimeMessage(JSON.parse(e.data));
          if (msg.conversationId === convId.value) {
            if (!mergeChatMessage(messages.value, msg))
              messages.value.push({
                ...msg,
                createdAt: msg.createdAt || new Date().toISOString(),
              });
            messages.value.sort(compareChatMessages);
            nextTick(() => scrollToBottom());
          }
        } catch {}
        fetchWechatMsgCount();
        fetchQQStatus();
      });
      proactiveSSE.onerror = () => {
        proactiveSSE?.close();
        setTimeout(() => void connectProactiveSSE(), 5000);
      };
    } catch {
      setTimeout(() => void connectProactiveSSE(), 5000);
    }
  }

  function disconnectProactiveSSE() {
    proactiveSSE?.close();
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
