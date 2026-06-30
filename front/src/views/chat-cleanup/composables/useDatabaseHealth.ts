// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, reactive } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../../composables/useApi"

export function useDatabaseHealth() {
  const stats = reactive({
    totalConversations: 0,
    totalMessages: 0,
    dbSize: "计算中...",
  })

  const vacuumLoading = ref(false)
  const vacuumResult = ref<any>(null)

  async function loadStats() {
    try {
      const res = await apiClient.get("/api/chats/stats")
      const d = res.data?.data || res.data
      stats.totalConversations = d?.totalConversations ?? 0

      const emptyRes = await apiClient.post("/api/chats/cleanup/preview", {
        channels: [],
        sources: [],
      })
      const ed = emptyRes.data?.data || emptyRes.data
      stats.totalMessages = ed?.messageCount ?? 0

      try {
        const vRes = await apiClient.post("/api/chats/cleanup/vacuum")
        const vd = vRes.data?.data || vRes.data
        if (vd?.sizeAfterFormatted) {
          stats.dbSize = vd.sizeBeforeFormatted || vd.sizeAfterFormatted
        }
      } catch {
        stats.dbSize = "--"
      }
    } catch {
      stats.totalConversations = 0
      stats.totalMessages = 0
      stats.dbSize = "--"
    }
  }

  async function runVacuum() {
    vacuumLoading.value = true
    try {
      const res = await apiClient.post("/api/chats/cleanup/vacuum")
      const d = res.data?.data || res.data
      vacuumResult.value = d
      ElMessage.success(`优化完成，释放 ${d.freedFormatted}`)
    } catch (err: any) {
      ElMessage.error("优化失败: " + (err.response?.data?.message || err.message))
    } finally {
      vacuumLoading.value = false
    }
  }

  return {
    stats,
    vacuumLoading,
    vacuumResult,
    loadStats,
    runVacuum,
  }
}
