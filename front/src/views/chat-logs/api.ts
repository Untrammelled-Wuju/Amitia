// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { get, post, put, del } from "@/composables/request"

export function fetchConvsApi(params: any) {
  return get<any>("/api/chats/conversations", params)
}

export function fetchMessagesApi(convId: string, params: any) {
  return get<any>(`/api/chats/conversations/${convId}/messages`, params)
}

export function deleteMessageApi(id: string) {
  return del(`/api/chats/messages/${id}`)
}

export function clearConversationApi(convId: string) {
  return del(`/api/chats/conversations/${convId}/messages`)
}

export function deleteConversationApi(convId: string) {
  return del(`/api/chats/conversations/${convId}`)
}

export function exportConversationApi(format: string, conversationIds: string[]) {
  return post("/api/chats/export", { format, conversationIds })
}

export function fetchFeedbackApi() {
  return get<any>("/api/messages/feedback/recent", { limit: 200 })
}

export function fetchMoodsApi(convId: string) {
  return get<any>(`/api/moods/conversations/${convId}`)
}

export function fetchSummaryApi(convId: string) {
  return get<any>(`/api/chats/conversations/${convId}/summary`)
}

export function generateSummaryApi(convId: string) {
  return post(`/api/chats/conversations/${convId}/summary/generate`)
}

export function deleteSummaryApi(convId: string) {
  return del(`/api/chats/conversations/${convId}/summary`)
}

export function switchCharacterApi(convId: string, characterId: string) {
  return put(`/api/chats/conversations/${convId}/character`, { characterId })
}

export function fetchContextPreviewApi(conversationId: string) {
  return get<any>(`/api/agent/context-preview?conversationId=${conversationId}`)
}

export function continueChatApi(body: any) {
  return post<any>("/api/web-chat/conversations/from-import", body)
}

export function loadCharactersApi() {
  return get<any[]>("/api/characters")
}
