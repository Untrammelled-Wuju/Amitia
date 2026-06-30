// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { ref, computed, onMounted } from "vue"
import { useApi } from "../../../composables/useApi"

export function useDashboardData() {
  const { get, post } = useApi()

  const health = ref<any>({})
  const runtimeHealth = ref<any>(null)
  const runtimeHealthLoading = ref(false)
  const accessRisk = ref<any>(null)
  const cloudRisk = ref<any>(null)
  const modelName = ref("")
  const diagResult = ref<any>(null)
  const diagLoading = ref(false)
  const todayMessages = ref(0)
  const totalConvs = ref(0)
  const totalMemories = ref(0)
  const totalChars = ref(0)
  const todayCalls = ref(0)
  const todayTokens = ref(0)
  const recentErrors = ref<any[]>([])
  const recentImports = ref<any[]>([])
  const feedbackTotal = ref(0)
  const feedbackByType = ref<Record<string, number>>({})

  const deployLabel = computed(() => health.value?.deployMode === "cloud-web" ? "私有云" : "本地桌面")
  const deployClass = computed(() => health.value?.deployMode === "cloud-web" ? "status-warn" : "status-ok")
  const modelLabel = computed(() => health.value?.model === "configured" ? "已配置" : "未配置")
  const modelClass = computed(() => health.value?.model === "configured" ? "status-ok" : "status-warn")
  const wechatLabel = computed(() => health.value?.wechat === "connected" ? "已连接" : "未连接")
  const wechatClass = computed(() => health.value?.wechat === "connected" ? "status-ok" : "status-off")
  const qqLabel = computed(() => health.value?.qq === "connected" ? "已连接" : "未连接")
  const qqClass = computed(() => health.value?.qq === "connected" ? "status-ok" : "status-off")

  const suggestionItems = computed(() => {
    return diagResult.value?.items?.filter((i: any) => i.status !== "ok" && i.suggestion) || []
  })
  const hasSuggestions = computed(() => {
    return diagResult.value?.items?.some((i: any) => i.status !== "ok" && i.suggestion) || false
  })

  const maxTodayStat = computed(() => {
    const vals = [todayMessages.value, totalConvs.value, totalMemories.value, totalChars.value, todayCalls.value]
    return Math.max(...vals, 1)
  })

  function healthModuleLabel(m: string) {
    const labels: Record<string, string> = {
      core: "Core", bridge: "Bridge", model: "Model",
      database: "DB", web: "Web", storage: "Storage",
    }
    return labels[m] || m
  }

  function healthStatusLabel(s: string) {
    const labels: Record<string, string> = {
      ok: "OK", warning: "Warning", error: "Error", unknown: "-",
    }
    return labels[s] || s
  }

  function barPercent(val: number) {
    return (val / maxTodayStat.value * 100).toFixed(1) + "%"
  }

  function formatTokens(n: number): string {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + "M"
    if (n >= 1000) return (n / 1000).toFixed(1) + "K"
    return String(n)
  }

  function fmtDateShort(d: string) {
    if (!d) return ""
    try {
      const date = new Date(d)
      const now = new Date()
      const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24))
      if (diffDays === 0) return "今天 " + date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
      if (diffDays === 1) return "昨天"
      return date.toLocaleDateString("zh-CN", { month: "numeric", day: "numeric" })
    } catch { return d }
  }

  async function fetchHealth() {
    try {
      const data = await get<any>("/api/health") || {}
      health.value = { ...health.value, ...data }
    } catch {}
  }

  async function fetchModelInfo() {
    try {
      const configs = await get<any[]>("/api/model/configs")
      if (configs && configs.length > 0) {
        const active = configs.find((c: any) => c.isActive)
        if (active) modelName.value = active.modelName || active.name || ""
      }
    } catch {}
  }

  async function fetchQQStatus() {
    try {
      const r = await get<any>("/api/qq/status")
      const data = r?.data || r
      if (data) {
        health.value.qq = data.qqOnline || data.status === "online" ? "connected" : "disconnected"
      }
    } catch {
      health.value.qq = "disconnected"
    }
  }

  async function fetchAccessRisk() {
    try {
      accessRisk.value = await get<any>("/api/security/exposure-check")
    } catch {}
  }

  async function fetchCloudRisk() {
    try {
      const r = await get<any>("/api/wechat/cloud-check/risk-summary")
      cloudRisk.value = r?.data || r
    } catch {}
  }

  async function fetchRuntimeHealth() {
    try {
      const data = await get<any>("/api/runtime/status")
      runtimeHealth.value = {
        overall: data?.status === "running" ? "ok" : "warning",
        modules: [
          { module: "Core", status: data?.status === "running" ? "ok" : "warn", detail: data?.pid ? `PID: ${data.pid}` : "" },
          { module: "CPU", status: "ok", detail: data?.cpu ? `${data.cpu}%` : "" },
          { module: "Memory", status: "ok", detail: data?.memory?.rssMB ? `${data.memory.rssMB} MB` : "" },
          { module: "Uptime", status: "ok", detail: data?.uptime ? `${Math.floor(data.uptime / 60)}m` : "" },
        ],
      }
    } catch {}
  }

  async function runHealthCheck() {
    runtimeHealthLoading.value = true
    try {
      const data = await post<any>("/api/runtime/check-now")
      runtimeHealth.value = {
        overall: "ok",
        modules: [
          { module: "Core", status: "ok", detail: data?.startedAt || "Running" },
          { module: "Check", status: data?.started ? "ok" : "warn", detail: data?.started ? "Completed" : "Unknown" },
        ],
      }
    } catch {} finally {
      runtimeHealthLoading.value = false
    }
  }

  async function fetchDiagnostics() {
    try {
      const result = await get<any>("/api/diagnostics")
      if (result) {
        const checks = result.checks || []
        const passed = checks.filter((c: any) => c.status === "pass").length
        diagResult.value = {
          overallStatus: passed === checks.length ? "healthy" : passed > 0 ? "degraded" : "unhealthy",
          items: checks.map((c: any) => ({
            name: c.name, status: c.status === "pass" ? "ok" : c.status === "info" ? "ok" : "warn", message: c.detail || "",
          })),
          summary: { ok: passed, warn: checks.length - passed, error: 0 },
          timestamp: new Date().toISOString(),
        }
      }
    } catch {}
  }

  async function runDiagnostics() {
    diagLoading.value = true
    try {
      const result = await post<any>("/api/diagnostics/run")
      const checks = result?.checks || []
      const passed = checks.filter((c: any) => c.status === "pass").length
      diagResult.value = {
        overallStatus: passed === checks.length ? "healthy" : passed > 0 ? "degraded" : "unhealthy",
        items: checks.map((c: any) => ({
          name: c.name, status: c.status === "pass" ? "ok" : c.status === "info" ? "ok" : "warn", message: c.detail || "",
        })),
        summary: { ok: passed, warn: checks.length - passed, error: 0 },
        timestamp: new Date().toISOString(),
      }
    } catch {} finally {
      diagLoading.value = false
    }
  }

  async function fetchRecentErrors() {
    try {
      const r = await get<any>("/api/logs/recent/errors", { limit: 20 })
      recentErrors.value = r?.items || []
    } catch {}
  }

  async function fetchActiveChar() {
    try {
      const chars = await get<any[]>("/api/characters")
      if (chars && chars.length > 0) {
        totalChars.value = chars.length
      }
    } catch {}
  }

  async function fetchTodayStats() {
    try {
      const data = await get<any>("/api/chats/stats")
      if (data) {
        todayMessages.value = data.todayMessages || 0
        totalConvs.value = data.totalConversations || 0
      }
    } catch {}
    try {
      const mem = await get<any>("/api/memories", { limit: 1 })
      if (mem) totalMemories.value = mem.total || 0
    } catch {}
  }

  async function fetchUsageOverview() {
    try {
      const data = await get<any>("/api/usage/overview")
      if (data) {
        todayCalls.value = data.todayCalls || 0
        todayTokens.value = data.todayTokens || 0
      }
    } catch {}
  }

  async function fetchRecentImports() {
    try {
      const r = await get<any>("/api/imports/batches", { limit: 5 })
      recentImports.value = r?.items || []
    } catch {}
  }

  async function fetchFeedbackStats() {
    try {
      const res: any = await get("/api/messages/feedback/stats")
      const data = res?.data || res
      feedbackTotal.value = data?.total || 0
      feedbackByType.value = data?.byType || {}
    } catch {}
  }

  async function refreshAll() {
    await Promise.all([
      fetchHealth(), fetchModelInfo(), fetchDiagnostics(), fetchRuntimeHealth(),
      fetchRecentErrors(), fetchActiveChar(), fetchTodayStats(), fetchRecentImports(),
      fetchCloudRisk(), fetchFeedbackStats(), fetchAccessRisk(), fetchUsageOverview(),
      fetchQQStatus(),
    ])
  }

  onMounted(() => { refreshAll() })

  return {
    health, runtimeHealth, runtimeHealthLoading, accessRisk, cloudRisk, modelName,
    diagResult, diagLoading,
    todayMessages, totalConvs, totalMemories, totalChars, todayCalls, todayTokens,
    recentErrors, recentImports,
    feedbackTotal, feedbackByType,
    deployLabel, deployClass, modelLabel, modelClass, wechatLabel, wechatClass, qqLabel, qqClass,
    suggestionItems, hasSuggestions, maxTodayStat,
    healthModuleLabel, healthStatusLabel, barPercent, formatTokens, fmtDateShort,
    fetchHealth, fetchModelInfo, fetchQQStatus, fetchAccessRisk, fetchCloudRisk,
    fetchRuntimeHealth, runHealthCheck,
    fetchDiagnostics, runDiagnostics,
    fetchRecentErrors, fetchActiveChar, fetchTodayStats, fetchUsageOverview,
    fetchRecentImports, fetchFeedbackStats,
    refreshAll,
  }
}
