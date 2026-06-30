// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed } from "vue"
import { useApi, isLoggedIn, getToken } from "./useApi"
import type { ApiResponse } from "@/types"

/**
 * Composable for WebChat state management.
 * Supports both regular and streaming (SSE) message sending.
 */
export function useChat() {
  const { get, post, del } = useApi()

  const conversations = ref<any[]>([])
  const messages = ref<any[]>([])
  const characters = ref<any[]>([])
  const importBatches = ref<any[]>([])
  const memories = ref<any[]>([])

  const loading = ref(false)
  const sending = ref(false)
  const streamingContent = ref("")
  const convId = ref("")
  const charId = ref("")

  let abortController: AbortController | null = null

    const replyStyle = ref<string>("natural")

  const canRegenerate = computed(() => {
    if (messages.value.length === 0) return false
    return messages.value[messages.value.length - 1]?.role === "assistant"
  })

  // ---- Characters ----
  async function fetchCharacters() {
    try {
      characters.value = await get<any[]>("/api/characters") || []
    } catch { /* ignore */ }
  }

  function getActiveCharacter() {
    return characters.value.find((c: any) => c.isActive) || characters.value[0] || null
  }

  // ---- Conversations ----
  async function fetchConversations() {
    try {
      const r = await get<any>("/api/web-chat/conversations")
      conversations.value = r?.items || []
    } catch { /* ignore */ }
  }

  async function createConversation(title?: string) {
    try {
      const r = await post<any>("/api/web-chat/conversations", {
        characterId: charId.value,
        title: title || "",
      })
      return r
    } catch {
      return null
    }
  }

  async function deleteConversation(id: string) {
    await del(`/api/web-chat/conversations/${id}`)
    if (convId.value === id) {
      convId.value = ""
      messages.value = []
    }
  }

  // ---- Messages ----
  async function fetchMessages(conversationId: string) {
    try {
      const r = await get<any>(`/api/web-chat/conversations/${conversationId}/messages`)
      messages.value = r?.items || []
    } catch { /* ignore */ }
  }

  /**
   * Send message via SSE streaming.
   * Returns true on success, false on failure.
   */
  async function sendMessageStream(text: string): Promise<boolean> {
    if (!text.trim() || sending.value) return false

    // Add temp user message
    const tempUserMsg = {
      id: "temp-" + Date.now(),
      role: "user",
      content: text,
      createdAt: new Date().toISOString(),
    }
    messages.value.push(tempUserMsg)

    // Add streaming placeholder
    const streamMsg = {
      id: "streaming",
      role: "assistant",
      content: "",
      createdAt: new Date().toISOString(),
    }
    messages.value.push(streamMsg)

    sending.value = true
    streamingContent.value = ""
    abortController = new AbortController()

    try {
      // Determine base URL for direct fetch (bypass axios for SSE)
      const apiBase = (import.meta as any).env.VITE_API_URL || ""
      const url = apiBase + "/api/web-chat/send-stream"

      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      }
      const token = getToken()
      if (token) {
        headers["Authorization"] = "Bearer " + token
      }

      const response = await fetch(url, {
        method: "POST",
        headers,
        body: JSON.stringify({
          conversationId: convId.value || undefined,
          characterId: charId.value || undefined,
          content: text,
          useMemory: true,
          replyStyle: replyStyle.value,
        }),
        signal: abortController.signal,
      })

      if (!response.ok) {
        throw new Error("HTTP " + response.status)
      }

      const reader = response.body?.getReader()
      if (!reader) throw new Error("No response body")

      const decoder = new TextDecoder()
      let buffer = ""
      let fullContent = ""
      const userMsgData: any = null
      const assistantMsgData: any = null

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split("\n")
        buffer = lines.pop() || ""

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed.startsWith("data: ")) continue

          try {
            const data = JSON.parse(trimmed.slice(6))

            if (data.type === "token") {
              fullContent += data.content
              streamingContent.value = fullContent
              // Update the streaming message in-place
              const idx = messages.value.findIndex(m => m.id === "streaming")
              if (idx >= 0) {
                messages.value[idx] = { ...messages.value[idx], content: fullContent }
              }
            } else if (data.type === "error") {
              console.error("[Stream] Error:", data.message)
              // Update streaming message with error
              const idx = messages.value.findIndex(m => m.id === "streaming")
              if (idx >= 0) {
                messages.value[idx] = {
                  ...messages.value[idx],
                  content: fullContent || "[Error: " + data.message + "]",
                }
              }
            } else if (data.type === "done") {
              // Remove streaming placeholder, add real messages
              const idx = messages.value.findIndex(m => m.id === "streaming")
              if (idx >= 0) messages.value.splice(idx, 1)

              // Remove temp user msg
              const tempIdx = messages.value.findIndex(m => m.id === tempUserMsg.id)
              if (tempIdx >= 0) messages.value.splice(tempIdx, 1)

              // Add real messages from response
              if (data.userMessage) messages.value.push(data.userMessage)
              if (data.assistantMessage) messages.value.push(data.assistantMessage)

              if (data.conversationId) convId.value = data.conversationId
              if (data.usedMemories) memories.value = data.usedMemories

              await fetchConversations()
              break
            }
          } catch {
            // Skip malformed SSE lines
          }
        }
      }

      return true
    } catch (err: any) {
      if (err?.name !== "AbortError") {
        console.error("[Stream] Failed:", err)
        // Remove streaming placeholder
        const idx = messages.value.findIndex(m => m.id === "streaming")
        if (idx >= 0) {
          if (streamingContent.value) {
            // Keep partial content
            messages.value[idx] = {
              ...messages.value[idx],
              content: streamingContent.value + "\n\n[Connection lost]",
            }
          } else {
            messages.value.splice(idx, 1)
            // Also remove temp user msg
            const tempIdx = messages.value.findIndex(m => m.id === tempUserMsg.id)
            if (tempIdx >= 0) messages.value.splice(tempIdx, 1)
          }
        }
      } else {
        // Aborted by user: keep partial content
        const idx = messages.value.findIndex(m => m.id === "streaming")
        if (idx >= 0 && streamingContent.value) {
          messages.value[idx] = {
            ...messages.value[idx],
            content: streamingContent.value,
          }
        }
      }
      return false
    } finally {
      sending.value = false
      streamingContent.value = ""
      abortController = null
    }
  }

  /** Legacy: send message via regular API (fallback) */
  async function sendMessage(text: string): Promise<boolean> {
    if (!text.trim() || sending.value) return false

    const tempMsg = {
      id: "temp-" + Date.now(),
      role: "user",
      content: text,
      createdAt: new Date().toISOString(),
    }
    messages.value.push(tempMsg)

    sending.value = true
    abortController = new AbortController()

    try {
      const res = await post<any>("/api/web-chat/send", {
        conversationId: convId.value || undefined,
        characterId: charId.value || undefined,
        content: text,
        useMemory: true,
        replyStyle: replyStyle.value,
      })

      const idx = messages.value.findIndex(m => m.id === tempMsg.id)
      if (idx >= 0) messages.value.splice(idx, 1)

      if (res) {
        if (res.userMessage) messages.value.push(res.userMessage)
        if (res.assistantMessage) messages.value.push(res.assistantMessage)
        if (res.conversationId) convId.value = res.conversationId
        if (res.usedMemories) memories.value = res.usedMemories
        await fetchConversations()
      }

      return true
    } catch (err: any) {
      if (err?.name !== "AbortError" && err?.name !== "CanceledError") {
        const idx = messages.value.findIndex(m => m.id === tempMsg.id)
        if (idx >= 0) messages.value.splice(idx, 1)
      }
      return false
    } finally {
      sending.value = false
      abortController = null
    }
  }

  function stopSending() {
    abortController?.abort()
    abortController = null
    sending.value = false
  }

  async function regenerateLast() {
    if (!canRegenerate.value || !convId.value) return false
    sending.value = true
    try {
      const res = await post<any>(`/api/web-chat/conversations/${convId.value}/regenerate`)
      if (res?.assistantMessage) {
        const last = messages.value[messages.value.length - 1]
        if (last?.role === "assistant") {
          messages.value[messages.value.length - 1] = res.assistantMessage
        } else {
          messages.value.push(res.assistantMessage)
        }
      }
      return true
    } catch {
      return false
    } finally {
      sending.value = false
    }
  }

  async function clearMessages() {
    if (convId.value) {
      await del(`/api/web-chat/conversations/${convId.value}/messages`)
    }
    messages.value = []
  }

  // ---- History pagination ----
  const historyPage = ref(1)
  const hasMoreHistory = ref(true)

  async function loadMoreHistory() {
    if (!convId.value || !hasMoreHistory.value) return 0
    try {
      const r = await get<any>(
        `/api/web-chat/conversations/${convId.value}/messages?page=${historyPage.value + 1}&pageSize=50`
      )
      const olderMessages = r?.items || []
      if (olderMessages.length > 0) {
        messages.value = [...olderMessages.reverse(), ...messages.value]
        historyPage.value++
      }
      if (olderMessages.length < 50) {
        hasMoreHistory.value = false
      }
      return olderMessages.length
    } catch {
      return 0
    }
  }

  function resetHistoryPagination() {
    historyPage.value = 1
    hasMoreHistory.value = true
  }

  // ---- Import batches ----
  async function fetchImportBatches() {
    try {
      const r = await get<any>("/api/imports/batches")
      importBatches.value = r?.items || []
    } catch { /* ignore */ }
  }

  // ---- Memories ----
  async function fetchMemories() {
    try {
      const r = await get<any>("/api/memories", { page: 1, pageSize: 10 })
      memories.value = r?.items || []
    } catch { /* ignore */ }
  }

  // ---- Message Polling (for reminders/proactive messages) ----
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let lastMessageId: string | null = null

  function startMessagePolling() {
    stopMessagePolling()
    pollTimer = setInterval(async () => {
      if (!convId.value || sending.value) return
      try {
        const r = await get<any>(`/api/web-chat/conversations/${convId.value}/messages`)
        const items: any[] = r?.items || []
        if (items.length === 0) return

        // Find new messages since last poll
        const lastIdx = items.findIndex((m: any) => m.id === lastMessageId)
        const newMsgs = lastIdx >= 0 ? items.slice(lastIdx + 1) : items

        if (newMsgs.length > 0) {
          // Append new messages to the list
          for (const msg of newMsgs) {
            const exists = messages.value.some((m: any) => m.id === msg.id)
            if (!exists) {
              messages.value.push(msg)
              // Show browser notification for reminder messages
              if ((msg.source === "reminder" || msg.source === "proactive") && "Notification" in window && Notification.permission === "granted") {
                new Notification("日程提醒", { body: msg.content.slice(0, 200), tag: "reminder-" + msg.id })
              }
            }
          }
          lastMessageId = items[items.length - 1]?.id || null
        }
      } catch { /* ignore poll errors */ }
    }, 5000)
  }

  function stopMessagePolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    lastMessageId = null
  }

  // ---- Init ----
  function setCharacter(id: string) {
    charId.value = id
  }

  function setConversation(id: string) {
    convId.value = id
    lastMessageId = null
    // Start polling when conversation is set
    if (id) {
      fetchMessages(id).then(() => {
        lastMessageId = messages.value[messages.value.length - 1]?.id || null
        startMessagePolling()
      })
    }
  }

  function newConversation() {
    convId.value = ""
    messages.value = []
    stopMessagePolling()
  }

  return {
    // State
    conversations, messages, characters, importBatches, memories, replyStyle,
    loading, sending, streamingContent, convId, charId,
    canRegenerate, hasMoreHistory,

    // Actions
    fetchCharacters, getActiveCharacter,
    fetchConversations, createConversation, deleteConversation,
    fetchMessages, sendMessage, sendMessageStream, stopSending, regenerateLast, clearMessages,
    loadMoreHistory, resetHistoryPagination,
    fetchImportBatches, fetchMemories,
    startMessagePolling, stopMessagePolling,
    setCharacter, setConversation, newConversation,
  }
}
