// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref } from "vue"
import { apiClient } from "../ui-index"

export interface WorldBookEntry {
  id: string
  matchType: string
  matchPattern: string
  matchScope: string
  injectContent: string
  priority: number
  hitCount: number
  createdAt: string
  updatedAt: string
}

export interface WorldBookListResponse {
  items: WorldBookEntry[]
  total: number
  page: number
  pageSize: number
  totalPages: number
}

export interface MatchResult {
  entry: WorldBookEntry
  matchScope: string
  hitText: string
}

export interface TestMatchResponse {
  matches: MatchResult[]
}

const matchTypeLabels: Record<string, string> = {
  regex: "正则匹配",
  exact: "精确匹配",
  keyword: "关键词匹配",
}

const scopeLabels: Record<string, string> = {
  full_context: "全部上下文",
  user_message: "仅用户消息",
  assistant_reply: "仅AI回复",
}

export function useWorldBook() {
  const rules = ref<WorldBookEntry[]>([])
  const loading = ref(false)
  const total = ref(0)
  const page = ref(1)
  const totalPages = ref(1)

  async function fetchRules(params?: { matchType?: string; page?: number; pageSize?: number }) {
    loading.value = true
    try {
      const res = await apiClient.get<WorldBookListResponse>("/api/world-book", { params })
      rules.value = res.data.items || []
      total.value = res.data.total || 0
      page.value = res.data.page || 1
      totalPages.value = res.data.totalPages || 1
    } catch (e) {
      console.error("获取世界书规则失败", e)
    } finally {
      loading.value = false
    }
  }

  async function createRule(data: Partial<WorldBookEntry>) {
    await apiClient.post("/api/world-book", data)
    await fetchRules()
  }

  async function updateRule(id: string, data: Partial<WorldBookEntry>) {
    await apiClient.put(`/api/world-book/${id}`, data)
    await fetchRules()
  }

  async function deleteRule(id: string) {
    await apiClient.delete(`/api/world-book/${id}`)
    await fetchRules()
  }

  async function testMatch(text: string): Promise<TestMatchResponse | null> {
    try {
      const res = await apiClient.post<TestMatchResponse>("/api/world-book/match", { text })
      return res.data
    } catch (e) {
      console.error("测试匹配失败", e)
      return null
    }
  }

  async function deleteAll() {
    await apiClient.delete("/api/world-book")
    await fetchRules()
  }

  function matchTypeLabel(t: string): string {
    return matchTypeLabels[t] || t
  }

  function scopeLabel(s: string): string {
    return scopeLabels[s] || s
  }

  return {
    rules, loading, total, page, totalPages,
    fetchRules, createRule, updateRule, deleteRule,
    testMatch, deleteAll, matchTypeLabel, scopeLabel,
  }
}
