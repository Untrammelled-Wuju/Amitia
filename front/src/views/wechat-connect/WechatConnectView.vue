<template>
  <div class="wechat-page">
    <h2 class="page-title">微信连接</h2>
    <div v-if="!pageReady" style="text-align:center;padding:40px 0;color:#909399">
      <el-icon class="is-loading" :size="24"><Loading /></el-icon>
      <p style="margin-top:8px">检测连接状态...</p>
    </div>
    <template v-if="pageReady">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 14px">
      <template #title>
        扫码连接你的微信
      </template>
    </el-alert>

    <template v-if="!isConnected">
      <el-card shadow="never" class="section-card">
        <template #header><span class="card-header-title">扫码连接</span></template>

        <div class="login-layout">
          <div class="login-steps">
            <div class="qr-step-row">
              <div class="qr-step-num" :class="{ active: qrStep >= 0 }">1</div>
              <div class="qr-step-body">
                <span class="qr-step-title">生成二维码</span>
                <el-button
                  size="small"
                  type="primary"
                  :loading="qrLoading"
                  @click="startLogin"
                  :disabled="qrLoading || isConnected"
                >获取二维码</el-button>
              </div>
            </div>
            <div class="qr-step-row">
              <div class="qr-step-num" :class="{ active: qrStep >= 1 }">2</div>
              <div class="qr-step-body">
                <span class="qr-step-title">用微信扫码</span>
                <span v-if="qrStep >= 1 && !isConnected" class="qr-status">
                  <el-icon class="is-loading" v-if="scanning"><Loading /></el-icon>
                  {{ scanning ? '等待扫码中...' : '请扫描二维码' }}
                </span>
              </div>
            </div>
            <div class="qr-step-row">
              <div class="qr-step-num" :class="{ active: isConnected }">3</div>
              <div class="qr-step-body">
                <span class="qr-step-title">确认连接</span>
                <span v-if="isConnected" class="qr-done">
                  <el-icon><CircleCheckFilled /></el-icon> 已连接
                </span>
              </div>
            </div>
          </div>

          <div class="login-qr">
            <div class="qr-frame" v-if="qrCodeUrl">
              <img :src="qrCodeUrl" alt="二维码" />
            </div>
            <div class="qr-frame qr-empty" v-else>
              <el-icon :size="36"><Picture /></el-icon>
              <span>点击按钮获取二维码</span>
            </div>
          </div>
        </div>

        <div class="qr-tip" v-if="qrCodeUrl">
          打开微信，扫描二维码确认连接。
        </div>
      </el-card>
    </template>

    <template v-if="isConnected">
      <el-card shadow="never" class="section-card">
        <template #header>
          <div class="card-header-row">
            <span class="card-header-title">连接状态</span>
            <div class="header-actions">
              <el-button size="small" @click="refreshStatus" :loading="loading">刷新</el-button>
              <el-button size="small" type="success" @click="reconnectBot" :loading="reconnecting">重新连接</el-button>
              <el-button size="small" type="warning" @click="handleRescan" :loading="qrLoading">
                重新添加机器人
              </el-button>
            </div>
          </div>
        </template>
        <div class="status-main">
          <div class="status-row">
            <div class="status-indicator ok"></div>
            <span class="status-label">已连接</span>
          </div>
          <div class="status-detail-grid" v-if="detail">
            <div class="sd-item">
              <span class="sd-label">消息数</span>
              <span class="sd-value">{{ detail.messageCount || 0 }}</span>
            </div>
            <div class="sd-item" v-if="detail.accountId">
              <span class="sd-label">账号</span>
              <span class="sd-value">{{ detail.accountId.slice(0, 12) }}...</span>
            </div>
            <div class="sd-item">
              <span class="sd-label">模式</span>
              <span class="sd-value">OpenClaw</span>
            </div>
            <div class="sd-item" v-if="detail.startedAt">
              <span class="sd-label">连接时间</span>
              <span class="sd-value">{{ detail.startedAt }}</span>
            </div>
          </div>
        </div>
      </el-card>
    </template>

    <!-- Cloud Deployment Check -->
    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="card-header-row">
          <span class="card-header-title">云端部署检查</span>
          <el-button
            size="small"
            type="primary"
            :loading="cloudCheckLoading"
            @click="runCloudCheck"
          >运行检查</el-button>
        </div>
      </template>

      <div class="cloud-check-hint" v-if="!cloudReport">
        <el-icon :size="16"><InfoFilled /></el-icon>
        <span>点击"运行检查"验证桥接服务是否准备好云端部署</span>
      </div>

      <div class="cloud-check-hint" v-if="cloudReport">
        <el-icon :size="16"><InfoFilled /></el-icon>
        <span>云端桥接运行后，你的电脑无需保持开机。</span>
      </div>

      <div v-if="cloudReport" class="cloud-check-result">
        <div class="cloud-overall" :class="'cloud-' + cloudReport.overallStatus">
          <span class="cloud-overall-dot"></span>
          <span class="cloud-overall-label">总体：{{ cloudOverallLabel }}</span>
          <span class="cloud-overall-time">{{ formatTime(cloudReport.checkedAt) }}</span>
        </div>

        <div class="cloud-item-list">
          <div
            v-for="(item, idx) in cloudReport.items"
            :key="idx"
            class="cloud-item"
            :class="'cloud-item-' + item.status"
          >
            <div class="cloud-item-header">
              <span class="cloud-item-dot" :class="item.status"></span>
              <span class="cloud-item-name">{{ item.name }}</span>
              <span class="cloud-item-badge" :class="item.status">{{ statusBadge(item.status) }}</span>
            </div>
            <div class="cloud-item-msg">{{ item.message }}</div>
            <div class="cloud-item-suggestion" v-if="item.suggestion">
              <el-icon :size="12"><Warning /></el-icon>
              {{ item.suggestion }}
            </div>
          </div>
        </div>
      </div>
    </el-card>
  </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, inject } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"
import { CircleCheckFilled, Picture, Loading, InfoFilled, Warning } from "@element-plus/icons-vue"
import { useApi } from "../../composables/useApi"

const { get, post } = useApi()

const refreshHealth = inject<() => Promise<void>>("refreshHealth")

const detail = ref<any>(null)
const loading = ref(false)
const qrLoading = ref(false)
const qrCodeUrl = ref("")
const qrStep = ref(0)
const scanning = ref(false)
const loginError = ref("")

// Page load state
const pageReady = ref(false)

// Cloud check state
const reconnecting = ref(false)
const cloudCheckLoading = ref(false)
const cloudReport = ref<any>(null)

const isConnected = computed(() => detail.value?.status === "connected")

const cloudOverallLabel = computed(() => {
  const s = cloudReport.value?.overallStatus
  const map: Record<string, string> = { ok: "全部正常", warning: "需要注意", error: "发现问题", unknown: "未检查" }
  return map[s] || s || "未知"
})

function statusBadge(status: string): string {
  const map: Record<string, string> = { ok: "OK", warning: "WARN", error: "ERR", unknown: "N/A" }
  return map[status] || status.toUpperCase()
}

function formatTime(iso: string): string {
  if (!iso) return ""
  try {
    const d = new Date(iso)
    return d.toLocaleString()
  } catch { return iso }
}

async function runCloudCheck() {
  cloudCheckLoading.value = true
  try {
    const resp = await post<any>("/api/wechat/cloud-check/run")
    cloudReport.value = resp?.data || resp
    const msg = cloudReport.value?.overallStatus === "ok" ? "所有检查通过" : "发现问题"
    ElMessage({ type: cloudReport.value?.overallStatus === "ok" ? "success" : "warning", message: msg })
  } catch (err: any) {
    ElMessage.error(err?.message || "云端检查失败")
  } finally {
    cloudCheckLoading.value = false
  }
}

async function refreshStatus() {
  loading.value = true
  try {
    const resp = await get<any>("/api/wechat/status")
    detail.value = resp?.data || resp
  } catch (err: any) {
    if (err?.message && !err.message.includes("404")) {
      console.warn("WeChat status fetch failed:", err.message)
    }
  } finally {
    loading.value = false
  }

  // Also load cloud check
  try {
    const cloudResp = await get<any>("/api/wechat/cloud-check")
    if (cloudResp?.data?.items) {
      cloudReport.value = cloudResp.data
    }
  } catch { /* ignore */ }
  pageReady.value = true
}

async function startLogin() {
  stopPolling()
  qrStep.value = 0
  scanning.value = false
  qrCodeUrl.value = ""
  qrLoading.value = true
  loginError.value = ""
  try {
    const resp = await get<any>("/api/wechat/login/start")
    if (resp?.data?.status === "connected" || resp?.status === "connected") {
      await refreshStatus()
      ElMessage.success("已连接微信")
      refreshHealth?.()
      return
    }
    const imgUrl = resp?.data?.qrImageUrl || resp?.qrImageUrl || resp?.data?.qrCodeUrl || resp?.qrCodeUrl
    if (imgUrl) {
      qrCodeUrl.value = imgUrl
      qrStep.value = 1
      scanning.value = true
      ElMessage.success("二维码已生成，请用微信扫码")
      startPolling()
    } else {
      const msg = resp?.message || resp?.data?.message || "获取二维码失败"
      loginError.value = msg
      ElMessage.warning(msg)
    }
  } catch (err: any) {
    loginError.value = err.message || "获取二维码失败"
    ElMessage.error(loginError.value)
  } finally {
    qrLoading.value = false
  }
}


async function handleRescan() {
  try {
    await ElMessageBox.confirm(
      "重新添加将断开当前连接并生成新的二维码，确定要继续吗？",
      "确认操作",
      {
        confirmButtonText: "确定",
        cancelButtonText: "取消",
        type: "warning",
      }
    )
    await startRescan()
  } catch {
    // 用户取消
  }
}

async function startRescan() {
  stopPolling()
  qrLoading.value = true
  loginError.value = ""
  qrCodeUrl.value = ""
  qrStep.value = 0
  scanning.value = false
  try {
    const resp = await post<any>("/api/wechat/login/rescan")
    const imgUrl = resp?.data?.qrImageUrl || resp?.qrImageUrl || resp?.data?.qrCodeUrl || resp?.qrCodeUrl
    if (imgUrl) {
      detail.value = { status: "waiting_scan" }
      qrCodeUrl.value = imgUrl
      qrStep.value = 1
      scanning.value = true
      ElMessage.success("已生成新机器人的二维码，请用微信扫码添加")
      startPolling()
    } else {
      const msg = resp?.message || resp?.data?.message || "获取二维码失败"
      loginError.value = msg
      ElMessage.warning(msg)
      await refreshStatus()
    }
  } catch (err: any) {
    loginError.value = err.message || "获取二维码失败"
    ElMessage.error(loginError.value)
    await refreshStatus()
  } finally {
    qrLoading.value = false
  }
}

async function reconnectBot() {
  reconnecting.value = true
  try {
    await post<any>("/api/wechat/login/reconnect")
    await refreshStatus()
    ElMessage.success("已重新连接")
      refreshHealth?.()
  } catch (err: any) {
    ElMessage.error(err?.message || "重新连接失败")
  } finally {
    reconnecting.value = false
  }
}

let pollTimer: ReturnType<typeof setInterval> | null = null

function startPolling() {
  stopPolling()
  const startTime = Date.now()
  const maxWait = 130000
  pollTimer = setInterval(async () => {
    if (Date.now() - startTime > maxWait) {
      stopPolling()
      scanning.value = false
      qrStep.value = 0
      ElMessage.warning("扫码超时，请重新获取二维码")
      return
    }
    try {
      await refreshStatus()
      if (detail.value?.status === "connected") {
        stopPolling()
        qrStep.value = 3
        scanning.value = false
        ElMessage.success("已连接微信！")
          refreshHealth?.()
        }
    } catch { /* keep polling */ }
  }, 2000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

let statusTimer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  refreshStatus()
  statusTimer = setInterval(() => {
    refreshStatus()
  }, 10000)
})

onUnmounted(() => {
  if (statusTimer) { clearInterval(statusTimer); statusTimer = null }
})
</script>

<style scoped>
.wechat-page {
  max-width: 640px;
  margin: 0 auto;
  padding: 20px 16px;
}
.page-title { font-size: 20px; font-weight: 600; margin-bottom: 14px; color: var(--ac-color-text); }
.section-card { margin-bottom: 12px; }
.card-header-row { display: flex; align-items: center; justify-content: space-between; }
.header-actions { display: flex; align-items: center; gap: 8px; }
.card-header-title { font-weight: 600; font-size: var(--ac-font-size-sm); }

.login-layout {
  display: flex;
  gap: 28px;
  align-items: flex-start;
}
@media (max-width: 560px) {
  .login-layout { flex-direction: column-reverse; align-items: center; }
}

.login-steps { flex: 1; min-width: 0; }
.login-qr { flex-shrink: 0; }

.qr-frame {
  width: 200px;
  height: 200px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  overflow: hidden;
  background: #fff;
}
.qr-frame img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  display: block;
}
.qr-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #909399;
  font-size: 12px;
}

.qr-tip {
  margin-top: 14px;
  font-size: 12px;
  color: #909399;
  text-align: center;
}

.qr-step-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #ebeef5;
}
.qr-step-row:last-child { border-bottom: none; }
.qr-step-num { color: #4e5969; 
  width: 24px; height: 24px; border-radius: 50%; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 600;
  background: #f5f7fa; color: #909399;
  border: 2px solid #dcdfe6;
}
.qr-step-num.active { background: #5a9e6f; color: #fff; border-color: #5a9e6f; }
.qr-step-body { flex: 1; display: flex; align-items: center; gap: 10px; }
.qr-step-title { font-size: 13px; font-weight: 600; color: var(--ac-color-text); }
.qr-status { font-size: 12px; color: #4e5969; display: flex; align-items: center; gap: 4px; }
.qr-done { font-size: 12px; color: #5a9e6f; font-weight: 600; display: flex; align-items: center; gap: 4px; }

.status-main { display: flex; flex-direction: column; gap: 12px; }
.status-row { display: flex; align-items: center; gap: 10px; }
.status-indicator { width: 12px; height: 12px; border-radius: 50%; flex-shrink: 0; }
.status-indicator.ok { background: #5a9e6f; }
.status-label { font-size: 16px; font-weight: 600; color: var(--ac-color-text); }
.status-detail-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 8px; }
.sd-item { padding: 8px 12px; background: var(--ac-color-bg-secondary); border-radius: 4px; }
.sd-label { font-size: 11px; color: var(--ac-color-text-muted); display: block; }
.sd-value { font-size: 14px; font-weight: 600; color: var(--ac-color-text); }

/* Cloud check section */
.cloud-check-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: 4px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin-bottom: 12px;
}
.cloud-check-result {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.cloud-overall {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 600;
}
.cloud-ok { background: #f0f9eb; color: #5a9e6f; }
.cloud-warning { background: #fdf6ec; color: #e6a23c; }
.cloud-error { background: #fef0f0; color: #f56c6c; }
.cloud-unknown { background: #f5f7fa; color: #909399; }
.cloud-overall-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.cloud-ok .cloud-overall-dot { background: #5a9e6f; }
.cloud-warning .cloud-overall-dot { background: #e6a23c; }
.cloud-error .cloud-overall-dot { background: #f56c6c; }
.cloud-unknown .cloud-overall-dot { background: #909399; }
.cloud-overall-label { flex: 1; }
.cloud-overall-time { font-size: 11px; font-weight: 400; color: var(--ac-color-text-muted); }

.cloud-item-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 400px;
  overflow-y: auto;
}
.cloud-item {
  padding: 8px 12px;
  border-radius: 4px;
  border: 0.5px solid transparent;
}
.cloud-item-ok { background: var(--ac-color-bg); border-color: #ebeef5; }
.cloud-item-warning { background: var(--ac-color-bg); border-color: #faecd8; }
.cloud-item-error { background: var(--ac-color-bg); border-color: #fde2e2; }
.cloud-item-unknown { background: var(--ac-color-bg); border-color: #e4e7ed; }
.cloud-item-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}
.cloud-item-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.cloud-item-dot.ok { background: #5a9e6f; }
.cloud-item-dot.warning { background: #e6a23c; }
.cloud-item-dot.error { background: #f56c6c; }
.cloud-item-dot.unknown { background: #909399; }
.cloud-item-name { font-size: 13px; font-weight: 600; color: var(--ac-color-text); }
.cloud-item-badge {
  margin-left: auto;
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: 600;
  text-transform: uppercase;
}
.cloud-item-badge.ok { background: #e1f3d8; color: #5a9e6f; }
.cloud-item-badge.warning { background: #faecd8; color: #e6a23c; }
.cloud-item-badge.error { background: #fde2e2; color: #f56c6c; }
.cloud-item-badge.unknown { background: #e4e7ed; color: #909399; }
.cloud-item-msg { font-size: 12px; color: var(--ac-color-text-muted); margin-bottom: 2px; }
.cloud-item-suggestion {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  color: #e6a23c;
  padding: 4px 8px;
  background: rgba(230, 162, 60, 0.08);
  border-radius: 3px;
}
</style>