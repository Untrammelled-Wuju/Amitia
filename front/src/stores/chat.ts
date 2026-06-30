// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineStore } from "pinia"
import { ref } from "vue"
import type { Message } from "@/types"

export const useChatStore = defineStore("chat", () => {
  const messages = ref<Message[]>([])
  const loading = ref(false)
  const currentConversationId = ref<string | null>(null)

  function setMessages(msgs: Message[]) { messages.value = msgs }
  function addMessage(msg: Message) { messages.value.push(msg) }
  function clearMessages() { messages.value = []; currentConversationId.value = null }
  function setConversationId(id: string) { currentConversationId.value = id }

  return { messages, loading, currentConversationId, setMessages, addMessage, clearMessages, setConversationId }
})
