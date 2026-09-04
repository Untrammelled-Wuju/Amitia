// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue";
import { useApi } from "./useApi";
import { useProfile } from "./useProfile";
import { useEpisodic } from "./useEpisodic";
import { useGraph } from "./useGraph";
import { useWorldBook } from "./useWorldBook";

export interface MemoryHubData {
  memories: any[];
  memoriesTotal: number;
  profiles: any[];
  profilesTotal: number;
  episodics: any[];
  episodicsTotal: number;
  worldBooks: any[];
  worldBooksTotal: number;
  graphStats: any;
  pipelineStatus: any;
  retrievalStats: any;
  candidates: any[];
}

export function useMemoryHub() {
  const { get, post, del } = useApi();
  const profileApi = useProfile();
  const episodicApi = useEpisodic();
  const graphApi = useGraph();
  const worldBookApi = useWorldBook();

  const memories = ref<any[]>([]);
  const memoriesTotal = ref(0);
  const candidates = ref<any[]>([]);
  const pipelineStatus = ref<any>(null);
  const retrievalStats = ref<any>({ totalCount: 0 });
  const retrievalLogs = ref<any[]>([]);

  async function fetchMemories(params?: any) {
    try {
      const r = await get<any>("/api/memories", params);
      memories.value = r?.items || [];
      memoriesTotal.value = r?.total || 0;
    } catch {
      memories.value = [];
      memoriesTotal.value = 0;
    }
  }

  async function fetchCandidates() {
    try {
      const r = await get<any>("/api/memory-candidates");
      candidates.value = r?.candidates || [];
    } catch {
      candidates.value = [];
    }
  }

  async function fetchPipelineStatus() {
    try {
      const r = await get<any>("/api/memory/pipeline/status");
      pipelineStatus.value = r;
    } catch {
      pipelineStatus.value = null;
    }
  }

  async function fetchRetrievalStats() {
    try {
      const r = await get<any>("/api/memory/retrieval/stats");
      retrievalStats.value = { totalCount: r?.totalCount || 0 };
      retrievalLogs.value = r?.recentLogs || [];
    } catch {
      retrievalStats.value = { totalCount: 0 };
      retrievalLogs.value = [];
    }
  }

  async function batchDeleteMemories(ids: string[]) {
    await Promise.all(ids.map((id) => del(`/api/memories/${id}`)));
  }

  async function batchVerifyMemories(ids: string[], status = "user_verified") {
    await post("/api/memories/batch-verify", { ids, status });
  }

  async function batchSetImportance(ids: string[], importance: number) {
    await post("/api/memories/batch-importance", { ids, importance });
  }

  function containsQuery(value: unknown, keyword: string): boolean {
    if (value == null) return false;
    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
      return String(value).toLowerCase().includes(keyword);
    }
    if (Array.isArray(value)) {
      return value.some((item) => containsQuery(item, keyword));
    }
    if (typeof value === "object") {
      return Object.values(value as Record<string, unknown>).some((item) =>
        containsQuery(item, keyword),
      );
    }
    return false;
  }

  async function globalSearch(query: string, limitPerType = 5) {
    const keyword = query.trim();
    const results: any = {
      memories: [],
      profiles: [],
      episodics: [],
      worldBooks: [],
    };
    if (!keyword) return results;

    try {
      const mr = await post<any>("/api/memories/hybrid-search", {
        keyword,
        query: keyword,
        limit: limitPerType,
      });
      results.memories = (mr?.items || []).slice(0, limitPerType).map((r: any) => ({
        ...(r?.memory || r),
        score: r?.score,
        matchType: r?.matchType,
        memoryLayer: r?.memoryLayer,
      }));
    } catch {}

    const normalized = keyword.toLowerCase();
    try {
      const pr = await get<any>("/api/profiles", { keyword, page: 1, pageSize: 100 });
      results.profiles = (pr?.items || [])
        .filter((item: any) => containsQuery(item, normalized))
        .slice(0, limitPerType);
    } catch {}
    try {
      const er = await get<any>("/api/episodic", { keyword, page: 1, pageSize: 100 });
      results.episodics = (er?.items || [])
        .filter((item: any) => containsQuery(item, normalized))
        .slice(0, limitPerType);
    } catch {}
    try {
      const wr = await get<any>("/api/world-book", { keyword, page: 1, pageSize: 100 });
      results.worldBooks = (wr?.items || [])
        .filter((item: any) => containsQuery(item, normalized))
        .slice(0, limitPerType);
    } catch {}
    return results;
  }

  return {
    memories,
    memoriesTotal,
    candidates,
    pipelineStatus,
    retrievalStats,
    retrievalLogs,
    fetchMemories,
    fetchCandidates,
    fetchPipelineStatus,
    fetchRetrievalStats,
    batchDeleteMemories,
    batchVerifyMemories,
    batchSetImportance,
    globalSearch,
    profileApi,
    episodicApi,
    graphApi,
    worldBookApi,
  };
}
