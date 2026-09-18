// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { apiClient } from "@/composables/useApi";

export type MemorySearchMode = "hybrid" | "vector" | "keyword";

export type MemorySearchResult = {
  id: string;
  key: string;
  value: string;
  memoryType: string;
  score: number;
  matchType: string;
  memoryLayer: string;
  raw: Record<string, unknown>;
};

function normalizeResult(item: any, mode: MemorySearchMode): MemorySearchResult {
  const memory = item?.memory && typeof item.memory === "object" ? item.memory : item ?? {};
  return {
    id: String(memory.id ?? item?.id ?? ""),
    key: String(memory.key ?? item?.key ?? ""),
    value: String(memory.value ?? item?.value ?? ""),
    memoryType: String(memory.memoryType ?? item?.memoryType ?? "custom"),
    score: Number(item?.score ?? 0) || 0,
    matchType: String(item?.matchType ?? mode),
    memoryLayer: String(item?.memoryLayer ?? ""),
    raw: item && typeof item === "object" ? item : {},
  };
}

export async function searchMemories(
  query: string,
  mode: MemorySearchMode = "hybrid",
  limit = 20,
): Promise<MemorySearchResult[]> {
  const keyword = query.trim();
  if (!keyword) return [];

  const path = mode === "vector"
    ? "/api/memories/vector-search"
    : mode === "keyword"
      ? "/api/memories/search"
      : "/api/memories/hybrid-search";
  const response = await apiClient.post(path, {
    keyword,
    query: keyword,
    limit,
  });
  const data = response.data as any;
  const items = Array.isArray(data?.items) ? data.items : [];
  return items.map((item: any) => normalizeResult(item, mode));
}
