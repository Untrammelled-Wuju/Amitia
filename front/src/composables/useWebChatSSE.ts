// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { type Ref, nextTick } from "vue"
import { resolveApiUrl } from "../runtime/runtime-adapter"
import { calcTypingDelay } from "@/utils/typing"
import { compareChatMessages, mergeChatMessage, normalizeRealtimeMessage } from "@/utils/message-order"

export function useWebChatSSE(
  convId: Ref<string>,
  messages: Ref<any[]>,
  scrollToBottom: (smooth?: boolean) => void,
  fetchWechatMsgCount: () => void,
  fetchQQStatus: () => void,
  sending: Ref<boolean>,
) {
  let eventSource: EventSource | null = null
  let lastPolledMsgId: string | null = null
  let proactiveSSE: EventSource | null = null

  const typingQueue: any[] = []
  let typingTimer: ReturnType<typeof setTimeout> | null = null

  function getLastPolledMsgId() { return lastPolledMsgId }
  function setLastPolledMsgId(id: string | null) { lastPolledMsgId = id }

  function sortMessages() {
    messages.value.sort(compareChatMessages)
  }

  function processTypingQueue() {
    if (typingQueue.length === 0) return
    const raw = typingQueue.shift()!
    const delay = calcTypingDelay(raw.content || "")
    typingTimer = setTimeout(() => {
      raw.typingDone = true
      messages.value.push(raw)
      sortMessages()
      lastPolledMsgId = raw.id || lastPolledMsgId
      scrollToBottom()
      fetchWechatMsgCount()
      fetchQQStatus()
      typingTimer = null
      setTimeout(() => processTypingQueue(), 300)
    }, delay)
  }

  function clearTypingQueue() {
    typingQueue.length = 0
    if (typingTimer) {
      clearTimeout(typingTimer)
      typingTimer = null
    }
  }

  function normalizeEventMessage(event: any): any | null {
    if (!event.messageId && !event.id) return null
    return {
      id: event.messageId || event.id,
      conversationId: event.conversationId,
      role: event.role,
      content: event.content || "",
      createdAt: event.createdAt || new Date().toISOString(),
      status: event.status,
      channel: event.channel,
      direction: event.direction,
    }
  }

  function handleMessageEvent(event: MessageEvent) {
    try {
      const raw = JSON.parse(event.data)
      const msg = normalizeEventMessage(raw)
      if (!msg) return
      if (msg.conversationId !== convId.value) return
      if (!msg.role || msg.role === "tool") return
      if ((msg as any).tool_calls_json) return
      if (mergeChatMessage(messages.value, msg)) {
        lastPolledMsgId = msg.id || lastPolledMsgId
        sortMessages()
        nextTick(() => scrollToBottom())
        return
      }
      if (mergeChatMessage(typingQueue, msg)) {
        lastPolledMsgId = msg.id || lastPolledMsgId
        return
      }
      if (!messages.value.some((m: any) => m.id === msg.id) && !typingQueue.some((m: any) => m.id === msg.id)) {
        if (msg.role === "user") {
          if (sending.value) {
            lastPolledMsgId = msg.id || lastPolledMsgId
            return
          }
        }
        if (msg.role === "assistant") {
          sending.value = false
          typingQueue.push(msg)
          if (!typingTimer) processTypingQueue()
        } else {
          messages.value.push(msg)
          if (!sending.value) sortMessages()
          lastPolledMsgId = msg.id || lastPolledMsgId
          scrollToBottom()
          fetchWechatMsgCount()
          fetchQQStatus()
        }
      }
    } catch {}
  }

  async function connectSSE() {
    disconnectSSE()
    if (!convId.value) return
    const url = await resolveApiUrl("/api/messages/events") + "?channel=web"
    eventSource = new EventSource(url)
    eventSource.addEventListener("message_created", handleMessageEvent)
    eventSource.addEventListener("message_updated", handleMessageEvent)
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

  function connectProactiveSSE() {
    try {
      proactiveSSE = new EventSource("/api/proactive-sse")
      proactiveSSE.addEventListener("proactive_message", (e) => {
        try {
          const msg = normalizeRealtimeMessage(JSON.parse(e.data))
          if (msg.conversationId === convId.value) {
            if (mergeChatMessage(messages.value, msg)) sortMessages()
            else if (!mergeChatMessage(typingQueue, msg)) {
              messages.value.push({ ...msg, createdAt: msg.createdAt || new Date().toISOString() })
              sortMessages()
            }
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
    proactiveSSE = null
  }

  function cleanup() {
    disconnectSSE()
    disconnectProactiveSSE()
    clearTypingQueue()
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
