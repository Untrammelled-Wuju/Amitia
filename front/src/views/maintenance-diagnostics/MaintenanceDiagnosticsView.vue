<template>
  <div class="maintenance-page">
    <div class="mp-header">
      <h2 class="page-title">维护与诊断</h2>
      <span class="mp-subtitle">单用户运维工具 - 仅服务部署者本人</span>
    </div>
    <!-- Alert: high-risk operations -->
    <el-alert
      v-if="showRestartWarning"
      title="高风险操作警告"
      type="warning"
      :closable="true"
      show-icon
      class="mp-alert"
      @close="showRestartWarning = false"
    >
      <template #default>
        <p>重启 Bridge 和重载配置是高风险操作，可能影响正在进行的对话。</p>
        <p>所有操作都会记录到审计日志中。</p>
      </template>
    </el-alert>
    <!-- Section 1: One-Click Diagnostics -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header">
          <span class="section-title">
            <el-icon><Monitor /></el-icon> 一键诊断
          </span>
          <div class="section-actions">
            <el-button
              size="small"
              type="primary"
              :loading="diagLoading"
              @click="runDiagnose"
            >
              {{ diagLoading ? '诊断中...' : '开始诊断' }}
            </el-button>
            <el-button
              size="small"
              :loading="exportLoading"
              :disabled="!lastDiagResult"
              @click="exportDiagnostic"
            >
              {{ exportLoading ? '导出中...' : '导出诊断包' }}
            </el-button>
          </div>
        </div>
      </template>
      <!-- Diagnostic Results -->
      <div v-if="diagResult" class="diag-result">
        <div class="dr-overall" :class="diagResult.overallStatus">
          <el-tag
            :type="overallTagType"
            size="large"
          >
            {{ overallLabel }}
          </el-tag>
          <span class="dr-time">{{ formatTime(diagResult.timestamp) }}</span>
        </div>
        <div class="dr-summary">
          <span class="drs-item ok"><el-icon><CircleCheck /></el-icon> 正常: {{ diagResult.summary.ok }}</span>
          <span class="drs-item warn" v-if="diagResult.summary.warn"><el-icon><WarningFilled /></el-icon> 警告: {{ diagResult.summary.warn }}</span>
          <span class="drs-item error" v-if="diagResult.summary.error"><el-icon><CircleCloseFilled /></el-icon> 错误: {{ diagResult.summary.error }}</span>
        </div>
        <div class="dr-list">
          <div
            v-for="item in diagResult.items"
            :key="item.name"
            class="dr-item"
            :class="item.status"
          >
            <span class="dri-icon">
              <el-icon v-if="item.status === 'ok'"><CircleCheck /></el-icon>
              <el-icon v-else-if="item.status === 'warn'"><WarningFilled /></el-icon>
              <el-icon v-else-if="item.status === 'error'"><CircleCloseFilled /></el-icon>
              <el-icon v-else><QuestionFilled /></el-icon>
            </span>
            <div class="dri-body">
              <div class="dri-name">{{ item.name }}</div>
              <div class="dri-msg">{{ item.message }}</div>
              <div v-if="item.details" class="dri-details">{{ item.details }}</div>
              <div v-if="item.status !== 'ok' && item.suggestion" class="dri-suggestion">
                <el-icon><InfoFilled /></el-icon> {{ item.suggestion }}
              </div>
            </div>
          </div>
        </div>
      </div>
      <div v-else class="empty-hint">
        <el-icon><InfoFilled /></el-icon>
        点击"开始诊断"检查系统各组件运行状态
      </div>
    </el-card>
    <!-- Section 2: Service Status -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header">
          <span class="section-title">
            <el-icon><Odometer /></el-icon> 服务状态
          </span>
          <el-button size="small" @click="fetchStatus" :loading="statusLoading">刷新</el-button>
        </div>
      </template>
      <div v-if="statusData" class="status-simple">
        <div class="ss-overall">
          <span class="ss-label">系统状态</span>
          <el-tag :type="statusData.status === 'healthy' ? 'success' : 'warning'" size="large">
            {{ statusData.status === 'healthy' ? '健康' : '部分异常' }}
          </el-tag>
          <span class="ss-time" v-if="statusData.lastCheck">上次检查: {{ statusData.lastCheck }}</span>
        </div>
        <div v-if="statusData.issues && statusData.issues.length > 0" class="ss-issues">
          <div class="ss-issues-title">
            <el-icon><WarningFilled /></el-icon> 发现问题 ({{ statusData.issues.length }})
          </div>
          <div v-for="(issue, idx) in statusData.issues" :key="idx" class="ss-issue-item">
            <span class="ssi-type">{{ issue.type }}</span>
            <span class="ssi-msg">{{ issue.msg }}</span>
          </div>
        </div>
        <div v-else class="ss-no-issues">
          <el-icon><CircleCheck /></el-icon> 未发现问题
        </div>
      </div>
    </el-card>
    <!-- Section 3: Operations -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <span class="section-title">
          <el-icon><Setting /></el-icon> 维护操作
        </span>
      </template>
      <div class="ops-grid">
        <!-- Restart Bridge -->
        <div class="op-card">
          <div class="op-info">
            <div class="op-title">重启 Bridge</div>
            <div class="op-desc">仅重启微信 Bridge 进程，不影响 Core 服务。适用于 Bridge 断连或不响应的情况。</div>
            <div class="op-risk high">高风险操作</div>
          </div>
          <div class="op-action">
            <el-popconfirm
              title="确定要重启 Bridge 吗？正在进行的微信对话可能会中断。"
              confirm-button-text="确认重启"
              cancel-button-text="取消"
              @confirm="restartBridge"
            >
              <template #reference>
                <el-button type="warning" size="small" :loading="bridgeRestartLoading">
                  {{ bridgeRestartLoading ? '重启中...' : '重启 Bridge' }}
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
        <!-- Reload Config -->
        <div class="op-card">
          <div class="op-info">
            <div class="op-title">重新加载配置</div>
            <div class="op-desc">重新读取 config.yaml 配置文件。部分配置需要重启 Core 服务才能完全生效。</div>
            <div class="op-risk high">高风险操作</div>
          </div>
          <div class="op-action">
            <el-popconfirm
              title="确定要重新加载配置吗？部分更改可能需要重启服务。"
              confirm-button-text="确认重载"
              cancel-button-text="取消"
              @confirm="reloadConfig"
            >
              <template #reference>
                <el-button type="warning" size="small" :loading="configReloadLoading">
                  {{ configReloadLoading ? '重载中...' : '重载配置' }}
                </el-button>
              </template>
            </el-popconfirm>
          </div>
        </div>
      </div>
    </el-card>
    <!-- Section 4: Export History -->
    <el-card shadow="never" class="section-card" v-if="exportHistory.length > 0">
      <template #header>
        <span class="section-title">
          <el-icon><FolderOpened /></el-icon> 导出记录
        </span>
      </template>
      <div class="export-list">
        <div v-for="(item, idx) in exportHistory" :key="idx" class="export-row">
          <span class="er-file">{{ item.file }}</span>
          <span class="er-time">{{ formatTime(item.timestamp) }}</span>
        </div>
      </div>
    </el-card>
  </div>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  Monitor, Odometer, Setting, FolderOpened,
  CircleCheck, CircleCloseFilled, WarningFilled, QuestionFilled, InfoFilled,
} from "@element-plus/icons-vue"
import { apiClient } from "../../composables/useApi"
interface DiagItem {
  name: string
  status: "ok" | "warn" | "error" | "unknown"
  message: string
  details?: string
  suggestion?: string
}
interface DiagResult {
  timestamp: string
  overallStatus: "healthy" | "degraded" | "unhealthy"
  items: DiagItem[]
  summary: { ok: number; warn: number; error: number; unknown: number }
}
interface StatusData {
  status: string
  issues: Array<{ type: string; msg: string }>
  lastCheck: string
}
interface ExportRecord {
  file: string
  timestamp: string
}
const diagLoading = ref(false)
const exportLoading = ref(false)
const statusLoading = ref(false)
const bridgeRestartLoading = ref(false)
const configReloadLoading = ref(false)
const showRestartWarning = ref(true)
const diagResult = ref<DiagResult | null>(null)
const statusData = ref<StatusData | null>(null)
const exportHistory = ref<ExportRecord[]>([])
const lastDiagResult = computed(() => diagResult.value)
const overallTagType = computed(() => {
  if (!diagResult.value) return "info"
  if (diagResult.value.overallStatus === "healthy") return "success"
  if (diagResult.value.overallStatus === "degraded") return "warning"
  return "danger"
})
const overallLabel = computed(() => {
  if (!diagResult.value) return "未诊断"
  if (diagResult.value.overallStatus === "healthy") return "系统健康"
  if (diagResult.value.overallStatus === "degraded") return "部分异常"
  return "存在错误"
})
async function apiPost(url: string, data?: any) {
  const res = await apiClient.post(url, data || {})
  return res.data?.data ?? res.data
}
async function apiGet(url: string) {
  const res = await apiClient.get(url)
  return res.data?.data ?? res.data
}
async function runDiagnose() {
  diagLoading.value = true
  try {
    const data = await apiPost("/api/maintenance/diagnose")
    const checks = data?.diagnosis?.checks || []
    const passedCount = checks.filter((c: any) => c.pass).length
    diagResult.value = {
      overallStatus: data?.diagnosis?.passed ? "healthy" : passedCount > 0 ? "degraded" : "unhealthy",
      items: checks.map((c: any) => ({ name: c.name, status: c.pass ? "ok" : "error", message: c.pass ? "正常" : (c.error || "异常") })),
      summary: { ok: passedCount, warn: 0, error: checks.length - passedCount, unknown: 0 },
      timestamp: new Date().toISOString()
    }
    ElMessage.success("诊断完成")
  } catch (e: any) {
    ElMessage.error("诊断失败: " + (e.response?.data?.message || e.message))
  } finally {
    diagLoading.value = false
  }
}
async function exportDiagnostic() {
  exportLoading.value = true
  try {
    const data = await apiPost("/api/maintenance/export-diagnostic")
    exportHistory.value.unshift({ file: data.file, timestamp: new Date().toISOString() })
    if (exportHistory.value.length > 10) exportHistory.value.pop()
    ElMessage.success("诊断包已导出: " + data.file)
  } catch (e: any) {
    ElMessage.error("导出失败: " + (e.response?.data?.message || e.message))
  } finally {
    exportLoading.value = false
  }
}
async function fetchStatus() {
  statusLoading.value = true
  try {
    statusData.value = await apiGet("/api/maintenance/status")
  } catch (e: any) {
    // silent
  } finally {
    statusLoading.value = false
  }
}
async function restartBridge() {
  bridgeRestartLoading.value = true
  try {
    const data = await apiPost("/api/maintenance/restart-bridge", {
      confirmToken: "restart-bridge-confirm",
    })
    ElMessage.success("Bridge 重启指令已发送")
  } catch (e: any) {
    ElMessage.error("重启失败: " + (e.response?.data?.message || e.message))
  } finally {
    bridgeRestartLoading.value = false
  }
}
async function reloadConfig() {
  configReloadLoading.value = true
  try {
    const data = await apiPost("/api/maintenance/reload-config", {
      confirmToken: "reload-config-confirm",
    })
    ElMessage.success("配置已重载")
  } catch (e: any) {
    ElMessage.error("重载失败: " + (e.response?.data?.message || e.message))
  } finally {
    configReloadLoading.value = false
  }
}
function formatTime(iso: string): string {
  if (!iso) return "-"
  try {
    return new Date(iso).toLocaleString("zh-CN")
  } catch {
    return iso
  }
}
onMounted(() => {
  fetchStatus()
})
</script>
<style scoped>
.maintenance-page {
  padding: 0 0 24px 0;
}
.mp-header {
  margin-bottom: 16px;
}
.page-title {
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
  margin: 0 0 4px 0;
}
.mp-subtitle {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
.mp-alert {
  margin-bottom: 12px;
}
.section-card {
  margin-bottom: 12px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-actions {
  display: flex;
  gap: 8px;
}
.empty-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  padding: 16px 0;
}
/* Diagnostic results */
.diag-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.dr-overall {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dr-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.dr-summary {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  font-size: var(--ac-font-size-sm);
}
.drs-item {
  display: flex;
  align-items: center;
  gap: 4px;
}
.drs-item.ok { color: var(--ac-color-success); }
.drs-item.warn { color: var(--ac-color-warning); }
.drs-item.error { color: var(--ac-color-error, #f56c6c); }
.dr-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.dr-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  border: 1px solid var(--ac-color-border-light);
}
.dr-item.ok { border-left: 3px solid var(--ac-color-success); }
.dr-item.warn { border-left: 3px solid var(--ac-color-warning); background: #fef7e0; }
.dr-item.error { border-left: 3px solid var(--ac-color-error, #f56c6c); background: #fef0f0; }
.dri-icon { flex-shrink: 0; margin-top: 1px; }
.dri-icon .el-icon { font-size: 18px; }
.dr-item.ok .dri-icon { color: var(--ac-color-success); }
.dr-item.warn .dri-icon { color: var(--ac-color-warning); }
.dr-item.error .dri-icon { color: var(--ac-color-error, #f56c6c); }
.dri-body { flex: 1; min-width: 0; }
.dri-name { font-size: var(--ac-font-size-sm); font-weight: 600; }
.dri-msg { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); }
.dri-details { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 2px; word-break: break-all; }
.dri-suggestion {
  font-size: 12px;
  color: #e6a23c;
  margin-top: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  background: #fef7e0;
  display: flex;
  align-items: flex-start;
  gap: 4px;
  line-height: 1.4;
}
/* Status simple */
.status-simple {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.ss-overall {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.ss-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text);
}
.ss-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-left: auto;
}
.ss-issues {
  padding: 10px 12px;
  border-radius: var(--ac-radius-md);
  background: #fef7e0;
  border: 1px solid #f5dab1;
}
.ss-issues-title {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: #e6a23c;
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 6px;
}
.ss-issue-item {
  display: flex;
  gap: 8px;
  padding: 4px 0;
  font-size: 13px;
}
.ssi-type {
  font-weight: 600;
  color: var(--ac-color-text);
  text-transform: uppercase;
  font-size: 11px;
  padding: 0 4px;
  border-radius: 2px;
  background: #f5dab1;
  white-space: nowrap;
}
.ssi-msg {
  color: var(--ac-color-text-secondary);
}
.ss-no-issues {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-success);
}

/* Operations */
.ops-grid {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.op-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.op-info {
  flex: 1;
  min-width: 0;
}
.op-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.op-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin-top: 4px;
}
.op-risk {
  font-size: 11px;
  margin-top: 4px;
  padding: 1px 6px;
  border-radius: 3px;
  display: inline-block;
}
.op-risk.high {
  color: #f56c6c;
  background: #fef0f0;
  border: 1px solid #fde2e2;
}
.op-action {
  flex-shrink: 0;
  margin-left: 16px;
}
/* Export history */
.export-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.export-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 10px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  font-size: 13px;
}
.er-file {
  flex: 1;
  font-weight: 500;
  color: var(--ac-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.er-time {
  color: var(--ac-color-text-muted);
  white-space: nowrap;
}
@media (max-width: 600px) {
  .status-simple {
    grid-template-columns: 1fr 1fr;
  }
  .op-card {
    flex-direction: column;
    align-items: flex-start;
    gap: 10px;
  }
  .op-action {
    margin-left: 0;
  }
}
</style>