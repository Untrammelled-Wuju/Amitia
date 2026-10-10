// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, watch } from "vue";
import { apiClient } from "../ui-index";
import { useCoreConfigurationAccess, coreConfigurationRequestConfig } from "./useCoreConfigurationAccess";

export interface WorldBookEntry {
  id: string;
  matchType: string;
  matchPattern: string;
  matchScope: string;
  injectContent: string;
  priority: number;
  characterId: string;
  hitCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface WorldBookListResponse {
  items: WorldBookEntry[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface MatchResult {
  entry: WorldBookEntry;
  matchScope: string;
  hitText: string;
}

export interface TestMatchResponse {
  matches: MatchResult[];
}

const matchTypeLabels: Record<string, string> = {
  regex: "正则匹配",
  exact: "精确匹配",
  keyword: "关键词匹配",
};

const scopeLabels: Record<string, string> = {
  full_context: "全部上下文",
  user_message: "仅用户消息",
  assistant_reply: "仅AI回复",
};

export function useWorldBook() {
  const access = useCoreConfigurationAccess();
  const loadedContext = ref("");
  const rules = ref<WorldBookEntry[]>([]);
  const loading = ref(false);
  const total = ref(0);
  const page = ref(1);
  const totalPages = ref(1);
  watch(access.contextKey, (current) => {
    if (current !== loadedContext.value) {
      rules.value = [];
      total.value = 0;
      loadedContext.value = "";
    }
  });

  async function requireContext(expected = loadedContext.value) {
    if (!expected) throw new Error("请先加载当前 Core 世界书");
    return access.requireAccess(expected);
  }

  async function fetchRules(params?: {
    matchType?: string;
    keyword?: string;
    page?: number;
    pageSize?: number;
  }) {
    loading.value = true;
    try {
      const context = await access.requireAccess();
      const res = await apiClient.get<WorldBookListResponse>(
        "/api/world-book",
        { params, ...coreConfigurationRequestConfig(context) },
      );
      await access.requireAccess(context);
      loadedContext.value = context;
      rules.value = res.data.items || [];
      total.value = res.data.total || 0;
      page.value = res.data.page || 1;
      totalPages.value = res.data.totalPages || 1;
    } catch (e) {
      rules.value = [];
      total.value = 0;
      loadedContext.value = "";
      console.error("获取世界书规则失败", e);
    } finally {
      loading.value = false;
    }
  }

  async function createRule(data: Partial<WorldBookEntry>, expected = loadedContext.value) {
    const context = await requireContext(expected);
    await apiClient.post("/api/world-book", data, coreConfigurationRequestConfig(context));
    await requireContext(context);
    await fetchRules();
  }

  async function createRules(items: Partial<WorldBookEntry>[], expected = loadedContext.value) {
    for (const item of items) {
      const context = await requireContext(expected);
      await apiClient.post("/api/world-book", item, coreConfigurationRequestConfig(context));
      await requireContext(context);
    }
    await fetchRules();
  }

  async function updateRule(id: string, data: Partial<WorldBookEntry>, expected = loadedContext.value) {
    const context = await requireContext(expected);
    await apiClient.put(`/api/world-book/${id}`, data, coreConfigurationRequestConfig(context));
    await requireContext(context);
    await fetchRules();
  }

  async function deleteRule(id: string, expected = loadedContext.value) {
    const context = await requireContext(expected);
    await apiClient.delete(`/api/world-book/${id}`, coreConfigurationRequestConfig(context));
    await requireContext(context);
    await fetchRules();
  }

  async function testMatch(text: string): Promise<TestMatchResponse | null> {
    try {
      const context = await requireContext();
      const res = await apiClient.post<TestMatchResponse>(
        "/api/world-book/match",
        { text },
        coreConfigurationRequestConfig(context),
      );
      await requireContext(context);
      return res.data;
    } catch (e) {
      console.error("测试匹配失败", e);
      return null;
    }
  }

  async function deleteAll() {
    const context = await requireContext();
    await apiClient.delete("/api/world-book", coreConfigurationRequestConfig(context));
    await requireContext(context);
    await fetchRules();
  }

  function matchTypeLabel(t: string): string {
    return matchTypeLabels[t] || t;
  }

  function scopeLabel(s: string): string {
    return scopeLabels[s] || s;
  }

  return {
    access,
    loadedContext,
    rules,
    loading,
    total,
    page,
    totalPages,
    fetchRules,
    createRule,
    createRules,
    updateRule,
    deleteRule,
    testMatch,
    deleteAll,
    matchTypeLabel,
    scopeLabel,
  };
}
