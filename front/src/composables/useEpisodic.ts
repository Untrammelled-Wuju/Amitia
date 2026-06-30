// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { apiClient } from "../ui-index"

export interface EpisodicMemory {
  id: string
  userId: string
  sceneType: string
  title: string
  content: string
  contextBefore: string
  contextAfter: string
  triggerKeywords: string
  sentimentScore: number
  messageIdStart: string
  messageIdEnd: string
  sourceConvId: string
  createdAt: string
}

export interface EpisodicListResponse {
  items: EpisodicMemory[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

const sceneTypeLabels: Record<string, string> = {
  insight: "感悟",
  joke: "笑话",
  milestone: "里程碑",
  emotional_peak: "情感峰值",
  confession: "坦白",
}

const sceneTypeEmojis: Record<string, string> = {
  insight: "💡",
  joke: "😂",
  milestone: "🏆",
  emotional_peak: "💗",
  confession: "🗣️",
}

export function useEpisodic() {
  const memories = ref<EpisodicMemory[]>([])
  const loading = ref(false)
  const total = ref(0)

  async function fetchMemories(params?: { userId?: string; sceneType?: string; page?: number; pageSize?: number }) {
    loading.value = true
    try {
      const res = await apiClient.get<EpisodicListResponse>("/api/episodic", { params })
      memories.value = res.data.items || []
      total.value = res.data.total || 0
    } catch (e) {
      console.error("获取情景记忆失败", e)
    } finally {
      loading.value = false
    }
  }

  async function deleteMemory(id: string) {
    await apiClient.delete(`/api/episodic/${id}`)
    await fetchMemories()
  }

  async function getDetail(id: string) {
    const res = await apiClient.get(`/api/episodic/${id}/detail`)
    return res.data
  }

  function sceneLabel(t: string): string {
    return sceneTypeLabels[t] || t
  }

  function sceneEmoji(t: string): string {
    return sceneTypeEmojis[t] || "📌"
  }

  function sentimentColor(score: number): string {
    if (score >= 5) return "#4caf50"
    if (score >= 1) return "#8bc34a"
    if (score >= -4) return "#ff9800"
    return "#f44336"
  }

  return {
    memories, loading, total,
    fetchMemories, deleteMemory, getDetail,
    sceneLabel, sceneEmoji, sentimentColor,
  }
}
