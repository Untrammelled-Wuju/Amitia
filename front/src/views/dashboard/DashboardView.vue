<template>
  <div class="dashboard">
    <h2 class="page-title">概览</h2>

    <!-- Access Protection Risk Alert (Step 80) -->
    <div v-if="accessRisk && (accessRisk.overallLevel === 'error' || accessRisk.overallLevel === 'warn')" class="access-risk-alert" :class="'risk-' + accessRisk.overallLevel">
      <div class="ara-header">
        <el-icon :size="20"><Warning /></el-icon>
        <span class="ara-title">访问安全风险</span>
      </div>
      <div class="ara-list">
        <div v-for="c in accessRisk.checks.filter((x: any) => x.level !== 'ok')" :key="c.name" class="ara-item" :class="'ara-' + c.level">
          <span class="arai-dot"></span>
          <span class="arai-name">{{ c.name }}</span>
          <span class="arai-msg">{{ c.message }}</span>
        </div>
      </div>
      <div class="ara-footer">
        <router-link to="/settings#access-protection" class="ara-link">前往访问保护设置 →</router-link>
      </div>
    </div>
    <!-- Bridge Cloud Risk Alert (Step 89) -->
    <div v-if="cloudRisk && cloudRisk.hasRisk" class="access-risk-alert" :class="'risk-' + (cloudRisk.riskLevel === 'high' ? 'error' : 'warn')">
      <div class="ara-header">
        <el-icon :size="20"><Connection /></el-icon>
        <span class="ara-title">WeChat Bridge Cloud Risk - {{ cloudRisk.riskCount }} issue(s)</span>
      </div>
      <div class="ara-list">
        <div v-for="c in cloudRisk.items" :key="c.name" class="ara-item" :class="'ara-' + (c.status === 'error' ? 'error' : 'warn')">
          <span class="arai-dot"></span>
          <span class="arai-name">{{ c.name }}</span>
          <span class="arai-msg">{{ c.status === 'error' ? 'Error' : 'Warning' }}</span>
        </div>
      </div>
      <div class="ara-footer">
        <router-link to="/wechat" class="ara-link">Go to WeChat Cloud Check</router-link>
      </div>
    </div>

    <div class="status-grid">
      <div class="status-card" :class="deployClass">
        <div class="sc-icon"><el-icon :size="22"><Monitor /></el-icon></div>
        <div class="sc-body">
          <div class="sc-label">部署模式</div>
          <div class="sc-value">{{ deployLabel }}</div>
        </div>
      </div>

      <div class="status-card" :class="modelClass">
        <div class="sc-icon"><el-icon :size="22"><Cpu /></el-icon></div>
        <div class="sc-body">
          <div class="sc-label">模型状态</div>
          <div class="sc-value">{{ modelLabel }}</div>
          <div class="sc-sub" v-if="modelName">{{ modelName }}</div>
        </div>
      </div>

      <div class="status-card" :class="wechatClass">
        <div class="sc-icon"><el-icon :size="22"><Connection /></el-icon></div>
        <div class="sc-body">
          <div class="sc-label">微信连接</div>
          <div class="sc-value">{{ wechatLabel }}</div>
        </div>
      </div>

      <div class="status-card" :class="qqClass">
        <div class="sc-icon"><el-icon :size="22"><ChatDotSquare /></el-icon></div>
        <div class="sc-body">
          <div class="sc-label">QQ连接</div>
          <div class="sc-value">{{ qqLabel }}</div>
        </div>
      </div>

      <div class="status-card" :class="runtimeHealth?.overall === 'ok' ? 'status-ok' : runtimeHealth?.overall === 'warning' ? 'status-warn' : 'status-off'">
        <div class="sc-icon"><el-icon :size="22"><CircleCheck /></el-icon></div>
        <div class="sc-body">
          <div class="sc-label">系统健康</div>
          <div class="sc-value">{{ runtimeHealth?.overall === 'ok' ? '正常' : runtimeHealth?.overall === 'warning' ? '注意' : runtimeHealth?.overall === 'error' ? '异常' : '未知' }}</div>
        </div>
      </div>
    </div>

    <!-- Runtime Health Modules -->
    <div class="health-modules" v-if="runtimeHealth">
      <div class="health-header">
        <span class="panel-title">服务状态</span>
        <el-button size="small" :loading="runtimeHealthLoading" @click="runHealthCheck">
          <el-icon :size="14" style="margin-right:4px"><Refresh /></el-icon>立即检查
        </el-button>
      </div>
      <div class="health-module-grid">
        <div
          v-for="m in runtimeHealth.modules"
          :key="m.module"
          class="health-module-item"
          :class="'hm-' + m.status"
        >
          <div class="hmi-indicator" :class="m.status"></div>
          <div class="hmi-body">
            <div class="hmi-label">{{ healthModuleLabel(m.module) }}</div>
            <div class="hmi-status">{{ healthStatusLabel(m.status) }}</div>
          </div>
          <div class="hmi-detail" v-if="m.detail">{{ m.detail }}</div>
          <div class="hmi-suggestion" v-if="m.suggestion">{{ m.suggestion }}</div>
        </div>
      </div>
    </div>

    <div class="info-grid">
      <el-card shadow="never" class="info-panel">
        <template #header>
          <span class="panel-title">今日数据</span>
        </template>
        <div class="today-chart">
          <div class="tc-row">
            <span class="tc-label">消息</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-msg" :style="{ width: barPercent(todayMessages) }"></div>
            </div>
            <span class="tc-num">{{ todayMessages }}</span>
          </div>
          <div class="tc-row">
            <span class="tc-label">会话</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-conv" :style="{ width: barPercent(totalConvs) }"></div>
            </div>
            <span class="tc-num">{{ totalConvs }}</span>
          </div>
          <div class="tc-row">
            <span class="tc-label">记忆</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-mem" :style="{ width: barPercent(totalMemories) }"></div>
            </div>
            <span class="tc-num">{{ totalMemories }}</span>
          </div>
          <div class="tc-row">
            <span class="tc-label">角色</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-char" :style="{ width: barPercent(totalChars) }"></div>
            </div>
            <span class="tc-num">{{ totalChars }}</span>
          </div>
          <div class="tc-row">
            <span class="tc-label">模型调用</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-usage" :style="{ width: barPercent(todayCalls) }"></div>
            </div>
            <span class="tc-num">{{ todayCalls }}</span>
          </div>
          <div class="tc-row">
            <span class="tc-label">消耗Token</span>
            <div class="tc-bar-wrap">
              <div class="tc-bar tc-bar-token" :style="{ width: barPercent(Math.min(todayTokens, maxTodayStat)) }"></div>
            </div>
            <span class="tc-num">{{ formatTokens(todayTokens) }}</span>
          </div>
        </div>
      </el-card>

      <el-card shadow="never" class="info-panel">
        <template #header>
          <span class="panel-title">Feedback</span>
        </template>
        <div v-if="feedbackTotal > 0" class="feedback-summary">
          <div class="fb-total">{{ feedbackTotal }} total</div>
          <div class="fb-bars">
            <div v-for="(cnt, type) in feedbackByType" :key="type" class="fb-bar-row">
              <span class="fb-type">{{ type }}</span>
              <span class="fb-cnt">{{ cnt }}</span>
            </div>
          </div>
        </div>
        <div v-else class="empty-hint">No feedback yet</div>
      </el-card>
    </div>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header-row">
          <span class="panel-title">最近错误</span>
          <el-button size="small" text @click="fetchRecentErrors">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>
      </template>
      <div v-if="recentErrors.length > 0" class="error-list">
        <div v-for="(err, idx) in recentErrors" :key="idx" class="error-row">
          <div class="er-left">
            <el-tag :type="err.severity === 'error' ? 'danger' : 'warning'" size="small" effect="dark">
              {{ err.action || err.targetType || "错误" }}
            </el-tag>
            <span class="er-msg">{{ err.details || err.message || "未知错误" }}</span>
          </div>
          <span class="er-time">{{ fmtDateShort(err.createdAt) }}</span>
        </div>
      </div>
      <div v-else class="empty-hint ok">
        <el-icon><CircleCheck /></el-icon>
        暂无错误，一切正常
      </div>
    </el-card>

    <el-card shadow="never" class="section-card" v-if="diagResult">
      <template #header>
        <div class="section-header-row">
          <div class="header-left-group">
            <span class="panel-title">诊断报告</span>
            <span class="diag-time">{{ fmtDateShort(diagResult.timestamp) }}</span>
          </div>
          <el-button size="small" text :loading="diagLoading" @click="runDiagnostics">
            <el-icon v-if="!diagLoading"><Refresh /></el-icon>
            运行诊断
          </el-button>
        </div>
      </template>
      <div class="diag-summary">
        <div class="ds-overall" :class="diagResult.overallStatus">
          <el-tag :type="diagResult.overallStatus === 'healthy' ? 'success' : diagResult.overallStatus === 'degraded' ? 'warning' : 'danger'" size="large">
            {{ diagResult.overallStatus === 'healthy' ? '健康' : diagResult.overallStatus === 'degraded' ? '部分异常' : '存在错误' }}
          </el-tag>
        </div>
        <div class="ds-items">
          <div v-for="item in diagResult.items" :key="item.name" class="ds-item" :class="item.status">
            <span class="dsi-status" :class="item.status">
              <el-icon v-if="item.status === 'ok'"><CircleCheck /></el-icon>
              <el-icon v-else-if="item.status === 'warn'"><Warning /></el-icon>
              <el-icon v-else-if="item.status === 'error'"><CircleClose /></el-icon>
              <el-icon v-else><QuestionFilled /></el-icon>
            </span>
            <span class="dsi-name">{{ item.name }}</span>
            <span class="dsi-msg">{{ item.message }}</span>
          </div>
        </div>
        <div v-if="hasSuggestions" class="ds-suggestions">
          <div v-for="item in suggestionItems" :key="'sug-' + item.name" class="ds-suggestion-item">
            <el-icon :size="14"><Warning /></el-icon>
            <span class="dss-name">{{ item.name }}：</span>
            <span class="dss-text">{{ item.suggestion }}</span>
          </div>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="panel-title">最近导入批次</span>
      </template>
      <div v-if="recentImports.length > 0" class="list-compact">
        <div v-for="b in recentImports" :key="b.id" class="list-item">
          <span class="li-title">{{ b.title }}</span>
          <span class="li-meta">{{ b.itemCount || 0 }}条 · {{ fmtDateShort(b.createdAt) }}</span>
        </div>
      </div>
      <div v-else class="empty-hint">暂无导入记录</div>
    </el-card>

    <el-card shadow="never" class="quick-actions-panel">
      <template #header>
        <span class="panel-title">快速入口</span>
      </template>
      <div class="quick-actions">
        <router-link to="/chat" class="qa-item">
          <div class="qa-icon chat"><el-icon :size="20"><ChatDotRound /></el-icon></div>
          <span>聊天</span>
        </router-link>
        <router-link to="/wechat" class="qa-item">
          <div class="qa-icon wechat"><el-icon :size="20"><Connection /></el-icon></div>
          <span>微信接入</span>
        </router-link>
        <router-link to="/qq" class="qa-item">
          <div class="qa-icon wechat"><el-icon :size="20"><ChatDotSquare /></el-icon></div>
          <span>QQ接入</span>
        </router-link>
        <router-link to="/model" class="qa-item">
          <div class="qa-icon model"><el-icon :size="20"><Cpu /></el-icon></div>
          <span>模型设置</span>
        </router-link>
        <router-link to="/character" class="qa-item">
          <div class="qa-icon char"><el-icon :size="20"><UserFilled /></el-icon></div>
          <span>角色管理</span>
        </router-link>
        <router-link to="/import" class="qa-item">
          <div class="qa-icon import"><el-icon :size="20"><Upload /></el-icon></div>
          <span>导入数据</span>
        </router-link>
        <router-link to="/logs" class="qa-item">
          <div class="qa-icon logs"><el-icon :size="20"><Document /></el-icon></div>
          <span>系统日志</span>
        </router-link>
        <router-link to="/settings" class="qa-item">
          <div class="qa-icon logs"><el-icon :size="20"><Setting /></el-icon></div>
          <span>设置</span>
        </router-link>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import {
  Monitor, Cpu, Connection, CircleCheck, CircleClose,
  Warning, QuestionFilled, ChatDotRound, ChatDotSquare, UserFilled,
  Upload, Document, Setting, Refresh,
} from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const { get, post } = useApi()

// Health
const health = ref<any>({})

// Runtime health
const runtimeHealth = ref<any>(null)
const runtimeHealthLoading = ref(false)

// Access Protection (Step 80)
const accessRisk = ref<any>(null)
const cloudRisk = ref<any>(null)

async function fetchCloudRisk() {
  try {
    const r = await get<any>("/api/wechat/cloud-check/risk-summary")
    cloudRisk.value = r?.data || r
  } catch { /* ignore */ }
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

const healthModuleLabel = (m: string) => {
  const labels: Record<string, string> = {
    core: 'Core', bridge: 'Bridge', model: 'Model',
    database: 'DB', web: 'Web', storage: 'Storage'
  }
  return labels[m] || m
}

const healthStatusLabel = (s: string) => {
  const labels: Record<string, string> = {
    ok: 'OK', warning: 'Warning', error: 'Error', unknown: '-'
  }
  return labels[s] || s
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
      ]
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
      ]
    }
  } catch {}
  runtimeHealthLoading.value = false
}
const deployLabel = computed(() => health.value?.deployMode === "cloud-web" ? "私有云" : "本地桌面")
const deployClass = computed(() => health.value?.deployMode === "cloud-web" ? "status-warn" : "status-ok")
const modelLabel = computed(() => health.value?.model === "configured" ? "已配置" : "未配置")
const modelClass = computed(() => health.value?.model === "configured" ? "status-ok" : "status-warn")
const modelName = ref("")
const wechatLabel = computed(() => health.value?.wechat === "connected" ? "已连接" : "未连接")
const wechatClass = computed(() => health.value?.wechat === "connected" ? "status-ok" : "status-off")
const qqLabel = computed(() => health.value?.qq === "connected" ? "已连接" : "未连接")
const qqClass = computed(() => health.value?.qq === "connected" ? "status-ok" : "status-off")

// Diagnostics
const diagResult = ref<any>(null)
const diagLoading = ref(false)
const suggestionItems = computed(() => {
  return diagResult.value?.items?.filter((i: any) => i.status !== "ok" && i.suggestion) || []
})
const hasSuggestions = computed(() => {
  return diagResult.value?.items?.some((i: any) => i.status !== "ok" && i.suggestion) || false
})
const healthLabel = computed(() => {
  if (diagResult.value?.overallStatus === "healthy") return "健康"
  if (diagResult.value?.overallStatus === "degraded") return "部分异常"
  if (diagResult.value?.overallStatus === "unhealthy") return "存在错误"
  return "未诊断"
})
const healthClass = computed(() => {
  if (!diagResult.value) return "status-off"
  if (diagResult.value.overallStatus === "healthy") return "status-ok"
  if (diagResult.value.overallStatus === "degraded") return "status-warn"
  return "status-off"
})
const healthSummary = computed(() => {
  if (!diagResult.value?.summary) return ""
  const s = diagResult.value.summary
  const parts = []
  if (s.ok) parts.push(`${s.ok} OK`)
  if (s.warn) parts.push(`${s.warn} 警告`)
  if (s.error) parts.push(`${s.error} 错误`)
  return parts.join(" / ")
})

// Active character
const todayMessages = ref(0)
const totalConvs = ref(0)
const totalMemories = ref(0)
const totalChars = ref(0)
const todayCalls = ref(0)
const todayTokens = ref(0)

const maxTodayStat = computed(() => {
  const vals = [todayMessages.value, totalConvs.value, totalMemories.value, totalChars.value, todayCalls.value]
  return Math.max(...vals, 1)
})
function barPercent(val: number) {
  return (val / maxTodayStat.value * 100).toFixed(1) + '%'
}
function formatTokens(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}
const feedbackTotal = ref(0)
const feedbackByType = ref<Record<string, number>>({})

// Recent errors
const recentErrors = ref<any[]>([])

// Recent imports
const recentImports = ref<any[]>([])

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

async function fetchDiagnostics() {
  try {
    const result = await get<any>("/api/diagnostics")
    if (result) {
      const checks = result.checks || []
      const passed = checks.filter((c: any) => c.status === "pass").length
      diagResult.value = {
        overallStatus: passed === checks.length ? "healthy" : passed > 0 ? "degraded" : "unhealthy",
        items: checks.map((c: any) => ({ name: c.name, status: c.status === "pass" ? "ok" : c.status === "info" ? "ok" : "warn", message: c.detail || "" })),
        summary: { ok: passed, warn: checks.length - passed, error: 0 },
        timestamp: new Date().toISOString()
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
      items: checks.map((c: any) => ({ name: c.name, status: c.status === "pass" ? "ok" : c.status === "info" ? "ok" : "warn", message: c.detail || "" })),
      summary: { ok: passed, warn: checks.length - passed, error: 0 },
      timestamp: new Date().toISOString()
    }
  } catch (err: any) {
    // Silently fail
  } finally {
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

async function fetchFeedbackStats() {
  try {
    const res: any = await get("/api/messages/feedback/stats")
    const data = res?.data || res
    feedbackTotal.value = data?.total || 0
    feedbackByType.value = data?.byType || {}
  } catch {}
}

onMounted(async () => {
  await Promise.all([
    fetchHealth(),
    fetchModelInfo(),
    fetchDiagnostics(),
    fetchRuntimeHealth(),
    fetchRecentErrors(),
    fetchActiveChar(),
    fetchTodayStats(),
    fetchRecentImports(),
    fetchCloudRisk(),
    fetchFeedbackStats(),
    fetchAccessRisk(),
    fetchUsageOverview(),
    fetchQQStatus(),
  ])
})
</script>

<style scoped>
.dashboard { }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 16px; color: var(--ac-color-text); }

.status-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 10px; margin-bottom: 14px; }
.status-card { display: flex; align-items: center; gap: 12px; padding: 14px 16px; border-radius: var(--ac-radius-md); background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); transition: border-color var(--ac-transition-fast); }
.status-card.status-ok { border-left: 3px solid var(--ac-color-success); }
.status-card.status-warn { border-left: 3px solid var(--ac-color-warning); }
.status-card.status-off { border-left: 3px solid var(--ac-color-text-muted); }
.sc-icon { flex-shrink: 0; color: var(--ac-color-text-secondary); }
.sc-body { flex: 1; min-width: 0; }
.sc-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-bottom: 2px; }
.sc-value { font-size: var(--ac-font-size-base); font-weight: 600; color: var(--ac-color-text); }
.sc-sub { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* Runtime Health Modules */
.health-modules { margin-top: 12px; margin-bottom: 14px; }
.health-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.health-module-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }
.health-module-item {
  background: var(--ac-color-bg-secondary);
  border-radius: 6px;
  padding: 12px;
  border: 1px solid var(--ac-color-border-light);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.health-module-item.hm-error { border-left: 3px solid #d4644c; background: #fdf5f4; }
.health-module-item.hm-warning { border-left: 3px solid #b8952e; background: #fdf8ef; }
.hmi-indicator { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.hmi-indicator.ok { background: #5a9e6f; }
.hmi-indicator.warning { background: #b8952e; }
.hmi-indicator.error { background: #d4644c; }
.hmi-indicator.unknown { background: #aaa; }
.hmi-body { display: flex; align-items: center; gap: 10px; }
.hmi-label { font-size: 13px; font-weight: 600; color: var(--ac-color-text); }
.hmi-status { font-size: 11px; color: var(--ac-color-text-muted); }
.hmi-detail { font-size: 11px; color: var(--ac-color-text-secondary); word-break: break-all; }
.hmi-suggestion { font-size: 11px; color: #8b6914; margin-top: 2px; }

@media (max-width: 768px) {
  .health-module-grid { grid-template-columns: 1fr; }
}

.info-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 14px; }
@media (max-width: 640px) { .info-grid { grid-template-columns: 1fr; } }
.info-panel { min-height: 100px; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }

.char-info { display: flex; align-items: center; gap: 12px; }
.char-detail { flex: 1; min-width: 0; }
.char-name { font-size: var(--ac-font-size-base); font-weight: 500; }
.char-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.today-chart { display: flex; flex-direction: column; gap: 10px; }
.tc-row { display: flex; align-items: center; gap: 10px; }
.tc-label { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); width: 32px; flex-shrink: 0; text-align: right; }
.tc-bar-wrap { flex: 1; height: 20px; background: var(--ac-color-bg-secondary); border-radius: 4px; overflow: hidden; }
.tc-bar { height: 100%; border-radius: 4px; transition: width 0.6s ease; min-width: 2px; }
.tc-bar-msg { background: var(--ac-color-primary); }
.tc-bar-conv { background: #5a9e6f; }
.tc-bar-mem { background: #b8952e; }
.tc-bar-char { background: #8b7ec8; }
.tc-num { font-size: var(--ac-font-size-sm); font-weight: 700; color: var(--ac-color-text); width: 40px; flex-shrink: 0; text-align: right; }

/* Section card */
.section-card { margin-bottom: 14px; }
.section-header-row { display: flex; justify-content: space-between; align-items: center; }

/* Error list */
.error-list { display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto; }
.error-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; padding: 8px 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); }
.er-left { display: flex; align-items: flex-start; gap: 8px; flex: 1; min-width: 0; }
.er-msg { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); line-height: 1.4; word-break: break-all; }
.er-time { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); white-space: nowrap; flex-shrink: 0; }

/* Diagnostic summary */
.diag-summary { display: flex; flex-direction: column; gap: 10px; }
.ds-overall { display: flex; align-items: center; gap: 8px; }
.diag-time { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }
.ds-items { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
@media (max-width: 640px) { .ds-items { grid-template-columns: 1fr; } }
.ds-item { display: flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); font-size: var(--ac-font-size-sm); }
.dsi-status.ok { color: var(--ac-color-success); }
.dsi-status.warn { color: var(--ac-color-warning); }
.dsi-status.error { color: var(--ac-color-error, #f56c6c); }
.dsi-status.unknown { color: var(--ac-color-text-muted); }
.dsi-name { font-weight: 500; white-space: nowrap; flex-shrink: 0; }
.dsi-msg { color: var(--ac-color-text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

/* Lists */
.list-compact { display: flex; flex-direction: column; gap: 6px; }
.list-item { display: flex; justify-content: space-between; align-items: center; padding: 4px 0; font-size: var(--ac-font-size-sm); }
.li-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.li-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); white-space: nowrap; margin-left: 8px; }

/* Empty states */
.empty-hint { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-muted); padding: 8px 0; }
.empty-hint.ok { color: var(--ac-color-success); display: flex; align-items: center; gap: 6px; }

/* Access Risk Alert (Step 80) */
.access-risk-alert {
  padding: 14px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.access-risk-alert.risk-error {
  background: #fef0f0;
  border: 1px solid #fbc4c4;
}
.access-risk-alert.risk-warn {
  background: #fef7e0;
  border: 1px solid #fae29c;
}
.ara-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}
.risk-error .ara-header { color: #f56c6c; }
.risk-warn .ara-header { color: #e6a23c; }
.ara-title {
  font-size: 15px;
  font-weight: 700;
}
.ara-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}
.ara-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
  padding: 4px 0;
}
.arai-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}
.ara-error .arai-dot { background: #f56c6c; }
.ara-warn .arai-dot { background: #e6a23c; }
.arai-name {
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
  min-width: 80px;
  color: var(--ac-color-text);
}
.arai-msg {
  color: var(--ac-color-text-secondary);
  line-height: 1.4;
}
.ara-footer {
  text-align: right;
}
.ara-link {
  font-size: 13px;
  font-weight: 600;
  text-decoration: none;
}
.risk-error .ara-link { color: #f56c6c; }
.risk-warn .ara-link { color: #e6a23c; }

/* Quick Actions */
.quick-actions-panel { margin-bottom: 14px; }
.quick-actions { display: grid; grid-template-columns: repeat(auto-fill, minmax(120px, 1fr)); gap: 10px; }
.qa-item { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 16px 10px; border-radius: var(--ac-radius-md); background: var(--ac-color-bg-secondary); text-decoration: none; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); transition: all var(--ac-transition-fast); cursor: pointer; }
.qa-item:hover { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); }
.qa-icon { width: 40px; height: 40px; border-radius: var(--ac-radius-sm); display: flex; align-items: center; justify-content: center; background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light); }
.qa-icon.chat { color: var(--ac-color-primary); }
.qa-icon.wechat { color: var(--ac-color-success); }
.qa-icon.model { color: var(--ac-color-warning); }
.qa-icon.char { color: #8b7ec8; }
.qa-icon.import { color: #c8806a; }
.qa-icon.logs { color: var(--ac-color-text-secondary); }

/* Diagnostic suggestions */
.header-left-group { display: flex; align-items: center; gap: 10px; }
.ds-item.warn { border-left: 2px solid var(--ac-color-warning); }
.ds-item.error { border-left: 2px solid var(--ac-color-error, #f56c6c); }
.ds-suggestions { margin-top: 10px; display: flex; flex-direction: column; gap: 6px; }
.ds-suggestion-item { display: flex; align-items: flex-start; gap: 6px; padding: 6px 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-warning-bg, #fef0e6); font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); line-height: 1.5; }
.dss-name { font-weight: 600; white-space: nowrap; flex-shrink: 0; }
.dss-text { word-break: break-all; }

</style>
