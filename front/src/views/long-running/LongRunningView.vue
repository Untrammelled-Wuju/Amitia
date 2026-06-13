<template>
  <div class="lr-page">
    <h2 class="page-title">长期运行维护</h2>

    <!-- Status Overview -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header-row">
          <span class="section-title">运行状态</span>
          <el-button size="small" :loading="loading" @click="refresh">刷新</el-button>
        </div>
      </template>

      <div v-if="status" class="lr-status-grid">
        <div class="lr-stat-card">
          <div class="lr-stat-label">运行状态</div>
          <div class="lr-stat-value">
            <el-tag :type="status.running ? 'success' : 'info'" size="large">
              {{ status.running ? '运行中' : '空闲' }}
            </el-tag>
          </div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">活跃任务数</div>
          <div class="lr-stat-value">{{ status.tasks?.length ?? 0 }}</div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">最后活动</div>
          <div class="lr-stat-value lr-sm">
            {{ status.tasks?.length > 0 ? fmtTime(status.tasks[0].updated_at) : '-' }}
          </div>
        </div>
      </div>

      <div v-if="status && status.tasks && status.tasks.length > 0" class="lr-task-list">
        <div class="lr-task-list-title">任务列表</div>
        <div v-for="task in status.tasks" :key="task.id" class="lr-task-row">
          <span class="lr-task-title">{{ task.title || '未命名任务' }}</span>
          <span class="lr-task-time">{{ fmtTime(task.updated_at) }}</span>
        </div>
      </div>
    </el-card>



    <!-- Manual Actions -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">手动操作</span></template>

      <div class="lr-actions">
        <div class="lr-action-row">
          <div class="lr-action-info">
            <div class="lr-action-name">清理临时文件</div>
            <div class="lr-action-desc">删除过期的临时文件、导入导出暂存文件</div>
          </div>
          <el-button
            size="small"
            type="warning"
            :loading="cleanupLoading"
            @click="runCleanup"
          >
            {{ cleanupLoading ? '清理中...' : '立即清理' }}
          </el-button>
        </div>

        <el-divider />

        <div class="lr-action-row">
          <div class="lr-action-info">
            <div class="lr-action-name">日志轮转</div>
            <div class="lr-action-desc">对超过大小限制的日志文件进行轮转归档</div>
          </div>
          <el-button
            size="small"
            type="primary"
            :loading="rotateLoading"
            @click="runLogRotate"
          >
            {{ rotateLoading ? '轮转中...' : '立即轮转' }}
          </el-button>
        </div>

        <el-divider />

        <div class="lr-action-row">
          <div class="lr-action-info">
            <div class="lr-action-name">数据库完整性检查</div>
            <div class="lr-action-desc">检查数据库文件完整性及外键约束</div>
          </div>
          <el-button
            size="small"
            type="success"
            :loading="dbCheckLoading"
            @click="runDbCheck"
          >
            {{ dbCheckLoading ? '检查中...' : '立即检查' }}
          </el-button>
        </div>
      </div>

      <!-- Results -->
      <div v-if="actionResult" class="lr-action-result">
        <el-alert
          :title="actionResult.message"
          :type="actionResult.type"
          :closable="true"
          @close="actionResult = null"
          show-icon
        />
        <div v-if="actionResult.detail" class="lr-action-detail">{{ actionResult.detail }}</div>
      </div>
    </el-card>

    <!-- Config -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">配置</span></template>

      <div class="lr-config-form">
        <div class="lr-cfg-row">
          <span class="lr-cfg-label">最大任务数</span>
          <el-input-number v-model="config.maxTasks" :min="1" :max="20" size="small" />
        </div>
        <div class="lr-cfg-row">
          <span class="lr-cfg-label">超时时间 (分钟)</span>
          <el-input-number v-model="config.timeoutMinutes" :min="5" :max="120" size="small" />
        </div>
        <div class="lr-cfg-actions">
          <el-button type="primary" size="small" @click="saveConfig">保存配置</el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { CircleCheck, CircleClose } from "@element-plus/icons-vue"
import { get, post, put } from "../../composables/request.js"

interface LongRunningStatus {
  running: boolean
  tasks: Array<{
    id: string
    title: string
    character_id: string
    updated_at: string
  }>
}

interface LongRunningConfig {
  maxTasks: number
  timeoutMinutes: number
}

const status = ref<LongRunningStatus | null>(null)
const config = ref<LongRunningConfig>({
  maxTasks: 5,
  timeoutMinutes: 30,
})

const loading = ref(false)
const cleanupLoading = ref(false)
const rotateLoading = ref(false)
const dbCheckLoading = ref(false)
const actionResult = ref<{ message: string; type: "success" | "warning" | "error" | "info"; detail?: string } | null>(null)

async function refresh() {
  loading.value = true
  try {
    const data = await get<{ status: LongRunningStatus; config: LongRunningConfig }>("/api/runtime/long-running/status")
    // The API returns the status directly
    status.value = data as any

    // Also refresh config
    try {
      const cfgData = await get<LongRunningConfig>("/api/runtime/long-running/config")
      if (cfgData) { config.value = cfgData }
    } catch { /* config fetch is separate */ }
  } catch {
    // Error handled by request interceptor
  } finally {
    loading.value = false
  }
}

async function loadConfig() {
  try {
    // Load config from server via status endpoint includes it?
    // Let's fetch separately if needed
    // For now, use a GET on the config endpoint
    // There's no dedicated GET; we'll update from status
  } catch { /* ignore */ }
}

async function saveConfig() {
  try {
    await put("/api/runtime/long-running/config", {
      maxTasks: config.value.maxTasks,
      timeoutMinutes: config.value.timeoutMinutes,
    })
    ElMessage.success("配置已保存")
  } catch {
  }
}

async function runCleanup() {
  cleanupLoading.value = true
  actionResult.value = null
  try {
    const result = await post<{ deleted: number; freedBytes: number }>("/api/runtime/cleanup-temp")
    actionResult.value = {
      message: `清理完成: 删除 ${result.deleted} 个文件`,
      type: "success",
      detail: `释放空间: ${fmtBytes(result.freedBytes)}`,
    }
    await refresh()
  } catch {
    actionResult.value = { message: "清理失败", type: "error" }
  } finally {
    cleanupLoading.value = false
  }
}

async function runLogRotate() {
  rotateLoading.value = true
  actionResult.value = null
  try {
    const result = await post<{ rotated: string[]; skipped: string[] }>("/api/runtime/rotate-logs")
    actionResult.value = {
      message: result.rotated.length > 0
        ? `已轮转 ${result.rotated.length} 个日志文件`
        : "所有日志文件未超过大小限制",
      type: "success",
      detail: result.rotated.length > 0 ? `轮转文件: ${result.rotated.join(", ")}` : undefined,
    }
    await refresh()
  } catch {
    actionResult.value = { message: "日志轮转失败", type: "error" }
  } finally {
    rotateLoading.value = false
  }
}

async function runDbCheck() {
  dbCheckLoading.value = true
  actionResult.value = null
  try {
    const result = await post<{ ok: boolean; errors: string[] }>("/api/runtime/check-db-integrity")
    actionResult.value = {
      message: result.ok && result.errors.length === 0
        ? "数据库完整性检查通过"
        : `发现 ${result.errors.length} 个问题`,
      type: result.ok ? "success" : "warning",
      detail: result.errors.length > 0 ? result.errors.join("; ") : undefined,
    }
    await refresh()
  } catch {
    actionResult.value = { message: "数据库检查失败", type: "error" }
  } finally {
    dbCheckLoading.value = false
  }
}

function fmtUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function fmtBytes(bytes: number): string {
  if (!bytes || bytes === 0) return "0 B"
  if (bytes < 1024) return bytes + " B"
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB"
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + " MB"
  return (bytes / (1024 * 1024 * 1024)).toFixed(1) + " GB"
}

function fmtTime(iso: string): string {
  if (!iso) return "-"
  try {
    const d = new Date(iso)
    return d.toLocaleString("zh-CN", {
      month: "2-digit", day: "2-digit",
      hour: "2-digit", minute: "2-digit", second: "2-digit",
    })
  } catch { return iso }
}

onMounted(() => {
  refresh()
})
</script>

<style scoped>
.lr-page {
  padding: 20px 24px;
  max-width: 900px;
  margin: 0 auto;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin: 0 0 16px 0;
}

.section-card {
  margin-bottom: 14px;
  border: 1px solid var(--ac-color-border-light);
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}

/* Status Grid */
.lr-status-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.lr-stat-card {
  padding: 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  text-align: center;
}

.lr-stat-label {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-bottom: 4px;
}

.lr-stat-value {
  font-size: 18px;
  font-weight: 700;
  color: var(--ac-color-text);
}

.lr-stat-value.lr-sm {
  font-size: 13px;
  font-weight: 500;
}

.lr-stat-value.lr-warn {
  color: #e6a23c;
}

/* Storage Grid */
.lr-storage-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14px;
}

.lr-storage-section {
  padding: 10px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
}

.lr-ss-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.lr-ss-sub {
  font-size: 11px;
  font-weight: 400;
  color: var(--ac-color-text-muted);
}

.lr-ss-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
}

.lr-ss-val {
  font-weight: 500;
  color: var(--ac-color-text);
}

.lr-ss-val.lr-sm {
  font-size: 11px;
  color: var(--ac-color-text-muted);
}

.lr-ss-val.lr-warn {
  color: #e6a23c;
}

.lr-empty {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  padding: 6px 0;
}

/* Log List */
.lr-log-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
  max-height: 140px;
  overflow-y: auto;
}

.lr-log-row {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  padding: 2px 0;
}

.lr-log-name {
  color: var(--ac-color-text-secondary);
  font-family: "Consolas", "Courier New", monospace;
}

.lr-log-size {
  color: var(--ac-color-text-muted);
}

/* Bridge */
.lr-bridge-status {
  display: flex;
  align-items: center;
  gap: 12px;
}

.lr-bridge-latency {
  font-size: 13px;
  color: var(--ac-color-text-secondary);
}

.lr-bridge-hint {
  margin-top: 10px;
  padding: 8px 12px;
  border-radius: var(--ac-radius-sm);
  background: #fef0f0;
  color: #f56c6c;
  font-size: 13px;
}

.lr-bridge-time {
  margin-top: 8px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
}

/* Actions */
.lr-action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
}

.lr-action-info {
  flex: 1;
}

.lr-action-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--ac-color-text);
}

.lr-action-desc {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-top: 2px;
}

.lr-action-result {
  margin-top: 12px;
}

.lr-action-detail {
  margin-top: 6px;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  padding: 6px 10px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  word-break: break-all;
}

@media (max-width: 600px) {
  .lr-status-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .lr-storage-grid {
    grid-template-columns: 1fr;
  }
  .lr-action-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}

/* Task list */
.lr-task-list {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lr-task-list-title {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text-secondary);
  margin-bottom: 4px;
}
.lr-task-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  font-size: 13px;
}
.lr-task-title {
  font-weight: 500;
  color: var(--ac-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  margin-right: 12px;
}
.lr-task-time {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  white-space: nowrap;
}

/* Config form */
.lr-config-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.lr-cfg-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.lr-cfg-label {
  font-size: 13px;
  color: var(--ac-color-text);
  min-width: 130px;
}
.lr-cfg-actions {
  margin-top: 4px;
}
</style>
