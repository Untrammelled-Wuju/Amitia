// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, type Ref, nextTick } from "vue"

export function useChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
) {
  let eventSource: EventSource | null = null
  let lastPolledMsgId: string | null = null

  function getLastPolledMsgId() { return lastPolledMsgId }
  function setLastPolledMsgId(id: string | null) { lastPolledMsgId = id }

  function connectSSE() {
    disconnectSSE()
    if (!convId.value) return
    const apiBase = (import.meta as any).env?.VITE_API_URL || ""
    const url = apiBase + "/api/messages/stream?conversationId=" + encodeURIComponent(convId.value) + (lastPolledMsgId ? "&since=" + encodeURIComponent(lastPolledMsgId) : "")
    eventSource = new EventSource(url)
    eventSource.onmessage = function(event) {
      try {
        const msg = JSON.parse(event.data)
        if (!msg.role || msg.role === "tool") return
        if ((msg as any).tool_calls_json) return
        if (!messages.value.some((m: any) => m.id === msg.id)) {
          if (msg.role === "user") {
            const now = Date.now()
            const dup = messages.value.some((m: any) =>
              m.role === "user" && m.content === msg.content &&
              String(m.id).startsWith("user-") &&
              (now - new Date(m.createdAt).getTime()) < 15000
            )
            if (dup) return
          }
          lastPolledMsgId = msg.id || lastPolledMsgId
          messages.value.push(msg)
          if (msg.source === "proactive" && "Notification" in window && (Notification as any).permission === "granted") {
            new Notification("日程提醒", { body: msg.content.slice(0, 200), tag: "reminder-" + msg.id })
          }
          scrollToBottom()
          fetchWechatMsgCount()
          fetchQQStatus()
        }
      } catch { }
    }
    eventSource.onerror = () => {
      disconnectSSE()
      setTimeout(() => { if (convId.value) connectSSE() }, 3000)
    }
  }

  function disconnectSSE() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  let proactiveSSE: EventSource | null = null
  function connectProactiveSSE() {
    try {
      proactiveSSE = new EventSource("/api/proactive-sse")
      proactiveSSE.addEventListener("proactive_message", (e) => {
        try {
          const msg = JSON.parse(e.data)
          if (msg.conversationId === convId.value) {
            messages.value.push({ id: msg.messageId, conversationId: msg.conversationId, role: msg.role, content: msg.content, source: msg.source, createdAt: new Date().toISOString() })
            nextTick(() => scrollToBottom())
          }
        } catch {}
        fetchWechatMsgCount()
        fetchQQStatus()
      })
      proactiveSSE.onerror = () => { proactiveSSE?.close(); setTimeout(connectProactiveSSE, 5000) }
    } catch { setTimeout(connectProactiveSSE, 5000) }
  }

  function disconnectProactiveSSE() {
    proactiveSSE?.close()
  }

  return {
    connectSSE,
    disconnectSSE,
    connectProactiveSSE,
    disconnectProactiveSSE,
    getLastPolledMsgId,
    setLastPolledMsgId,
  }
}
