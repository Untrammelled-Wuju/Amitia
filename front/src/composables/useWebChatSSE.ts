// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, nextTick } from "vue"
import { resolveApiUrl } from "../runtime/runtime-adapter"

export function useWebChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
  fetchWebMsgCount: () => void,
  sending: Ref<boolean>,
) {
  let eventSource: EventSource | null = null
  let lastPolledMsgId: string | null = null
  let proactiveSSE: EventSource | null = null

  function getLastPolledMsgId() { return lastPolledMsgId }
  function setLastPolledMsgId(id: string | null) { lastPolledMsgId = id }

  function sortMessages() {
    messages.value.sort((a: any, b: any) => {
      const ta = a.createdAt ? new Date(a.createdAt).getTime() : 0
      const tb = b.createdAt ? new Date(b.createdAt).getTime() : 0
      return ta - tb
    })
  }

  async function connectSSE() {
    disconnectSSE()
    if (!convId.value) return
    const baseUrl = await resolveApiUrl("/api/messages/stream")
    const url = baseUrl + "?conversationId=" + encodeURIComponent(convId.value) + (lastPolledMsgId ? "&since=" + encodeURIComponent(lastPolledMsgId) : "")
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
          messages.value.push(msg)
          if (!sending.value) sortMessages()
          lastPolledMsgId = msg.id || lastPolledMsgId
          if (msg.source === "proactive" && "Notification" in window && (Notification as any).permission === "granted") {
            new Notification("日程提醒", { body: msg.content.slice(0, 200), tag: "reminder-" + msg.id })
          }
          scrollToBottom()
          fetchWechatMsgCount()
          fetchQQStatus()
          fetchWebMsgCount()
        }
      } catch { }
    }
    eventSource.onerror = () => {
      disconnectSSE()
      setTimeout(() => { if (convId.value) void connectSSE() }, 3000)
    }
  }

  function disconnectSSE() {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  async function connectProactiveSSE() {
    try {
      const url = await resolveApiUrl("/api/proactive-sse")
      proactiveSSE = new EventSource(url)
      proactiveSSE.addEventListener("proactive_message", (e) => {
        try {
          const msg = JSON.parse(e.data)
          if (msg.conversationId === convId.value) {
            if (!messages.value.some((m: any) => m.id === msg.messageId)) {
              messages.value.push({ id: msg.messageId, conversationId: msg.conversationId, role: msg.role, content: msg.content, source: msg.source, createdAt: msg.createdAt || new Date().toISOString() })
              if (!sending.value) sortMessages()
              nextTick(() => scrollToBottom())
            }
          }
        } catch {}
          fetchWechatMsgCount()
          fetchQQStatus()
          fetchWebMsgCount()
      })
      proactiveSSE.onerror = () => { proactiveSSE?.close(); setTimeout(() => void connectProactiveSSE(), 5000) }
    } catch { setTimeout(() => void connectProactiveSSE(), 5000) }
  }

  function disconnectProactiveSSE() {
    proactiveSSE?.close()
    proactiveSSE = null
  }

  function cleanup() {
    disconnectSSE()
    disconnectProactiveSSE()
  }

  return {
    connectSSE,
    disconnectSSE,
    connectProactiveSSE,
    disconnectProactiveSSE,
    cleanup,
    getLastPolledMsgId,
    setLastPolledMsgId,
  }
}
