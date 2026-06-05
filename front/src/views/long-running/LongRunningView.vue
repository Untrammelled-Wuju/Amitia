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
          <div class="lr-stat-label">运行时长</div>
          <div class="lr-stat-value">{{ fmtUptime(status?.uptime ?? 0) }}</div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">内存占用 (RSS)</div>
          <div class="lr-stat-value">{{ status?.memory?.rssMB ?? 0 }} MB</div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">堆内存</div>
          <div class="lr-stat-value">{{ status?.memory?.heapUsedMB ?? 0 }} MB</div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">SSE 连接数</div>
          <div class="lr-stat-value">{{ status?.activeSseConnections ?? 0 }}</div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">无活动时间</div>
          <div class="lr-stat-value" :class="{ 'lr-warn': status.inactivityMinutes > 30 }">
            {{ status.inactivityMinutes }} 分钟
          </div>
        </div>
        <div class="lr-stat-card">
          <div class="lr-stat-label">最后清理</div>
          <div class="lr-stat-value lr-sm">{{ status.lastCleanup ? fmtTime(status.lastCleanup) : '未执行' }}</div>
        </div>
      </div>
    </el-card>

    <!-- Storage Status -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">存储状态</span></template>

      <div v-if="status" class="lr-storage-grid">
        <!-- Database -->
        <div class="lr-storage-section">
          <div class="lr-ss-title">数据库</div>
          <div class="lr-ss-row">
            <span>主文件</span>
            <span class="lr-ss-val">{{ fmtBytes(status.database.sizeBytes) }}</span>
          </div>
          <div class="lr-ss-row">
            <span>WAL 文件</span>
            <span class="lr-ss-val">{{ fmtBytes(status.database.walSizeBytes) }}</span>
          </div>
          <div class="lr-ss-row">
            <span>完整性检查</span>
            <span class="lr-ss-val">
              <el-tag v-if="status.database.integrityOk === true" type="success" size="small">正常</el-tag>
              <el-tag v-else-if="status.database.integrityOk === false" type="danger" size="small">异常</el-tag>
              <el-tag v-else type="info" size="small">未检查</el-tag>
            </span>
          </div>
          <div class="lr-ss-row" v-if="status.database.lastIntegrityCheck">
            <span>上次检查</span>
            <span class="lr-ss-val lr-sm">{{ fmtTime(status.database.lastIntegrityCheck) }}</span>
          </div>
        </div>

        <!-- Logs -->
        <div class="lr-storage-section">
          <div class="lr-ss-title">
            日志文件
            <span class="lr-ss-sub">{{ fmtBytes(status.logs.totalSizeBytes) }}</span>
          </div>
          <div class="lr-log-list" v-if="status.logs.files.length > 0">
            <div v-for="f in status.logs.files" :key="f.name" class="lr-log-row">
              <span class="lr-log-name">{{ f.name }}</span>
              <span class="lr-log-size">{{ fmtBytes(f.sizeBytes) }}</span>
            </div>
          </div>
          <div v-else class="lr-empty">无日志文件</div>
        </div>

        <!-- Temp Files -->
        <div class="lr-storage-section">
          <div class="lr-ss-title">
            临时文件
            <span class="lr-ss-sub">{{ status.tempFiles.count }} 个 ({{ fmtBytes(status.tempFiles.totalSizeBytes) }})</span>
          </div>
        </div>

        <!-- Backup -->
        <div class="lr-storage-section">
          <div class="lr-ss-title">最近备份</div>
          <div v-if="status.lastBackup.time" class="lr-ss-row">
            <span>时间</span>
            <span class="lr-ss-val">{{ fmtTime(status.lastBackup.time) }}</span>
          </div>
          <div v-if="status.lastBackup.ageHours !== null" class="lr-ss-row">
            <span>距今</span>
            <span class="lr-ss-val" :class="{ 'lr-warn': status.lastBackup.ageHours > 168 }">
              {{ status.lastBackup.ageHours }} 小时
            </span>
          </div>
          <div v-else class="lr-empty">无备份记录</div>
        </div>
      </div>
    </el-card>

    <!-- Bridge Heartbeat -->
    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">Bridge 心跳</span></template>
      <div v-if="status" class="lr-bridge">
        <div class="lr-bridge-status">
          <el-tag v-if="status.bridgeHeartbeat.ok" type="success" size="large">
            <el-icon><CircleCheck /></el-icon> 正常
          </el-tag>
          <el-tag v-else type="danger" size="large">
            <el-icon><CircleClose /></el-icon> {{ status.bridgeHeartbeat.lastCheck ? '异常' : '未检测' }}
          </el-tag>
          <span v-if="status.bridgeHeartbeat.latencyMs" class="lr-bridge-latency">
            延迟: {{ status.bridgeHeartbeat.latencyMs }}ms
          </span>
        </div>
        <div v-if="!status.bridgeHeartbeat.ok" class="lr-bridge-hint">
          Bridge 心跳异常，请检查微信桥服务是否正常运行。
        </div>
        <div v-if="status.bridgeHeartbeat.lastCheck" class="lr-bridge-time">
          上次检测: {{ fmtTime(status.bridgeHeartbeat.lastCheck) }}
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
      <el-form label-width="180px" label-position="left" size="small">
        <el-form-item label="临时文件清理">
          <el-switch v-model="config.tempCleanupEnabled" @change="saveConfig" />
        </el-form-item>
        <el-form-item label="临时文件保留天数">
          <el-input-number v-model="config.tempRetentionDays" :min="1" :max="90" @change="saveConfig" size="small" />
        </el-form-item>
        <el-form-item label="日志轮转">
          <el-switch v-model="config.logRotateEnabled" @change="saveConfig" />
        </el-form-item>
        <el-form-item label="日志最大大小 (MB)">
          <el-input-number v-model="config.logMaxSizeMb" :min="1" :max="1000" @change="saveConfig" size="small" />
        </el-form-item>
        <el-form-item label="数据库完整性检查">
          <el-switch v-model="config.dbIntegrityCheckEnabled" @change="saveConfig" />
        </el-form-item>
        <el-form-item label="备份提醒">
          <el-switch v-model="config.backupReminderEnabled" @change="saveConfig" />
        </el-form-item>
        <el-form-item label="调度器错误隔离">
          <el-switch v-model="config.schedulerErrorIsolation" @change="saveConfig" />
        </el-form-item>
        <el-form-item label="无活动超时 (分钟)">
          <el-input-number v-model="config.inactivityTimeoutMinutes" :min="5" :max="1440" @change="saveConfig" size="small" />
        </el-form-item>
        <el-form-item label="模型请求超时 (ms)">
          <el-input-number v-model="config.modelRequestTimeoutMs" :min="10000" :max="600000" :step="10000" @change="saveConfig" size="small" />
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { CircleCheck, CircleClose } from "@element-plus/icons-vue"
import { get, post, put } from "../../composables/request.js"

interface LongRunningStatus {
  tempFiles: { count: number; totalSizeBytes: number }
  logs: { files: { name: string; sizeBytes: number; lastModified: string }[]; totalSizeBytes: number }
  database: { path: string; sizeBytes: number; walSizeBytes: number; lastIntegrityCheck: string | null; integrityOk: boolean | null }
  lastBackup: { time: string | null; ageHours: number | null }
  bridgeHeartbeat: { ok: boolean; lastCheck: string | null; latencyMs: number | null }
  uptime: number
  memory: { rssMB: number }
  activeSseConnections: number
  inactivityMinutes: number
  lastCleanup: string | null
  lastLogRotate: string | null
}

interface LongRunningConfig {
  tempCleanupEnabled: boolean
  tempRetentionDays: number
  logRotateEnabled: boolean
  logMaxSizeMb: number
  dbIntegrityCheckEnabled: boolean
  backupReminderEnabled: boolean
  schedulerErrorIsolation: boolean
  inactivityTimeoutMinutes: number
  modelRequestTimeoutMs: number
  bridgeHeartbeatIntervalMs: number
  sseConnectionMaxAgeMs: number
  cleanupIntervalMinutes: number
  logRotateIntervalMinutes: number
  dbIntegrityCheckIntervalHours: number
  backupReminderIntervalHours: number
}

const status = ref<LongRunningStatus | null>(null)
const config = ref<LongRunningConfig>({
  tempCleanupEnabled: true,
  tempRetentionDays: 7,
  logRotateEnabled: true,
  logMaxSizeMb: 50,
  dbIntegrityCheckEnabled: true,
  backupReminderEnabled: true,
  schedulerErrorIsolation: true,
  inactivityTimeoutMinutes: 30,
  modelRequestTimeoutMs: 120000,
  bridgeHeartbeatIntervalMs: 30000,
  sseConnectionMaxAgeMs: 600000,
  cleanupIntervalMinutes: 60,
  logRotateIntervalMinutes: 30,
  dbIntegrityCheckIntervalHours: 24,
  backupReminderIntervalHours: 168,
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
      tempCleanupEnabled: config.value.tempCleanupEnabled,
      tempRetentionDays: config.value.tempRetentionDays,
      logRotateEnabled: config.value.logRotateEnabled,
      logMaxSizeMb: config.value.logMaxSizeMb,
      dbIntegrityCheckEnabled: config.value.dbIntegrityCheckEnabled,
      backupReminderEnabled: config.value.backupReminderEnabled,
      schedulerErrorIsolation: config.value.schedulerErrorIsolation,
      inactivityTimeoutMinutes: config.value.inactivityTimeoutMinutes,
      modelRequestTimeoutMs: config.value.modelRequestTimeoutMs,
    })
    ElMessage.success("配置已保存")
  } catch {
    // Error handled by interceptor
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
</style>
