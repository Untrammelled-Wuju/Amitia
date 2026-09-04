// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { apiClient } from "../ui-index";

export interface EpisodicMemory {
  id: string;
  userId: string;
  sceneType: string;
  title: string;
  content: string;
  contextBefore: string;
  contextAfter: string;
  triggerKeywords: string;
  sentimentScore: number;
  messageIdStart: string;
  messageIdEnd: string;
  sourceConvId: string;
  createdAt: string;
  updatedAt?: string;
  retentionLevel: number;
  memoryStrength: number;
  strengthUpdatedAt?: string | null;
  lastReinforcedAt?: string | null;
  reinforceCount: number;
  decayState: string;
  archivedAt?: string | null;
}

export interface EpisodicListResponse {
  items: EpisodicMemory[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

const sceneTypeLabels: Record<string, string> = {
  insight: "感悟",
  joke: "笑话",
  milestone: "里程碑",
  emotional_peak: "情感峰值",
  confession: "坦白",
};

const sceneTypeEmojis: Record<string, string> = {
  insight: "💡",
  joke: "😂",
  milestone: "🏆",
  emotional_peak: "💗",
  confession: "🗣️",
};

export function useEpisodic() {
  const memories = ref<EpisodicMemory[]>([]);
  const loading = ref(false);
  const total = ref(0);

  async function fetchMemories(params?: {
    userId?: string;
    sceneType?: string;
    retentionLevel?: number;
    decayState?: string;
    keyword?: string;
    page?: number;
    pageSize?: number;
  }) {
    loading.value = true;
    try {
      const res = await apiClient.get<EpisodicListResponse>("/api/episodic", {
        params,
      });
      memories.value = res.data.items || [];
      total.value = res.data.total || 0;
    } catch (e) {
      console.error("获取情景记忆失败", e);
    } finally {
      loading.value = false;
    }
  }

  async function deleteMemory(id: string) {
    await apiClient.delete(`/api/episodic/${id}`);
    await fetchMemories();
  }

  async function updateRetention(id: string, retentionLevel: number) {
    await apiClient.put(`/api/episodic/${id}/retention`, { retentionLevel });
  }

  async function restoreMemory(id: string) {
    await apiClient.post(`/api/episodic/${id}/restore`);
  }

  async function getDetail(id: string) {
    const res = await apiClient.get(`/api/episodic/${id}/detail`);
    return res.data;
  }

  function sceneLabel(t: string): string {
    return sceneTypeLabels[t] || t;
  }

  function sceneEmoji(t: string): string {
    return sceneTypeEmojis[t] || "📌";
  }

  function sentimentColor(score: number): string {
    if (score >= 5) return "var(--ac-color-success)";
    if (score >= 1) return "var(--ac-color-primary)";
    if (score >= -4) return "var(--ac-color-warning)";
    return "var(--ac-color-danger)";
  }

  function sentimentIntensity(score: number): {
    label: string;
    percent: number;
  } {
    const clamped = Math.max(-10, Math.min(10, score));
    const percent = ((clamped + 10) / 20) * 100;
    if (score >= 5) return { label: "强烈正面", percent };
    if (score >= 1) return { label: "正面", percent };
    if (score >= -4) return { label: "负面", percent };
    return { label: "强烈负面", percent };
  }

  return {
    memories,
    loading,
    total,
    fetchMemories,
    deleteMemory,
    updateRetention,
    restoreMemory,
    getDetail,
    sceneLabel,
    sceneEmoji,
    sentimentColor,
    sentimentIntensity,
  };
}
