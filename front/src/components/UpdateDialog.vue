<template>
  <el-dialog
    :model-value="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    align-center
    :show-close="state === 'downloaded' || state === 'error'"
    width="420px"
    class="update-dialog"
    @close="handleClose"
  >
    <template #header>
      <span class="update-dialog-title">{{ title }}</span>
    </template>

    <div class="update-dialog-body">
      <div v-if="state === 'available'" class="update-info">
        <div class="update-versions">
          <span class="version-old">v{{ currentVersion }}</span>
          <el-icon class="version-arrow"><Right /></el-icon>
          <span class="version-new">v{{ latestVersion }}</span>
        </div>
        <div v-if="releaseNotes" class="update-notes">
          <div class="update-notes-label">更新内容</div>
          <div class="update-notes-text">{{ releaseNotes }}</div>
        </div>
      </div>

      <div v-else-if="state === 'downloading'" class="update-downloading">
        <el-progress :percentage="downloadPercent" :stroke-width="8" :show-text="true" />
        <div class="download-progress-meta" aria-live="polite">
          <span class="download-size">{{ downloadSizeText }}</span>
          <span v-if="downloadSpeed" class="download-speed">{{ downloadSpeed }}</span>
        </div>
      </div>

      <div v-else-if="state === 'downloaded'" class="update-downloaded">
        <el-icon class="update-success-icon"><CircleCheckFilled /></el-icon>
        <span>v{{ latestVersion }} 已下载完成，是否立即重启安装？</span>
      </div>

      <div v-else-if="state === 'error'" class="update-error">
        <el-icon class="update-error-icon"><WarningFilled /></el-icon>
        <span>{{ errorMessage }}</span>
      </div>
    </div>

    <template #footer>
      <div class="update-dialog-footer">
        <template v-if="state === 'available'">
          <el-button @click="handleSkip">稍后提醒</el-button>
          <el-button type="default" @click="handleOpenGitee">Gitee 备用下载</el-button>
          <el-button type="primary" @click="handleStartDownload">立即下载</el-button>
        </template>
        <template v-else-if="state === 'downloaded'">
          <el-button @click="handleRestartLater">稍后处理</el-button>
          <el-button type="primary" @click="handleRestartNow">立即重启</el-button>
        </template>
        <template v-else-if="state === 'error'">
          <el-button @click="handleSkip">关闭</el-button>
        </template>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue"
import { Right, CircleCheckFilled, WarningFilled } from "@element-plus/icons-vue"
import { isDesktopShell } from "@/runtime/runtime-capabilities"

type DialogState = "idle" | "available" | "downloading" | "downloaded" | "error"

const state = ref<DialogState>("idle")
const currentVersion = ref("")
const latestVersion = ref("")
const releaseNotes = ref("")
const downloadPercent = ref(0)
const downloadedBytes = ref(0)
const totalBytes = ref(0)
const downloadSpeed = ref("")
const errorMessage = ref("")

const visible = computed(() => state.value !== "idle")
const downloadSizeText = computed(() => {
  const downloaded = formatFileSize(downloadedBytes.value)
  const total = totalBytes.value > 0 ? formatFileSize(totalBytes.value) : "--"
  return `${downloaded} / ${total}`
})

const title = computed(() => {
  switch (state.value) {
    case "available": return "发现新版本"
    case "downloading": return "正在下载更新"
    case "downloaded": return "更新已下载"
    case "error": return "更新失败"
    default: return ""
  }
})

function handleClose() {
  if (state.value === "downloaded" || state.value === "error") {
    state.value = "idle"
  }
}

async function handleStartDownload() {
  if (!window.amitiaDesktop) return
  state.value = "downloading"
  downloadPercent.value = 0
  downloadedBytes.value = 0
  totalBytes.value = 0
  downloadSpeed.value = ""
  await window.amitiaDesktop.startDownload()
}

async function handleSkip() {
  if (!window.amitiaDesktop) return
  state.value = "idle"
  await window.amitiaDesktop.skipVersion()
}

async function handleOpenGitee() {
  if (!window.amitiaDesktop) return
  state.value = "idle"
  await window.amitiaDesktop.skipVersion()
  await window.amitiaDesktop.openGiteeRelease()
}

async function handleRestartNow() {
  if (!window.amitiaDesktop) return
  await window.amitiaDesktop.restartNow()
}

async function handleRestartLater() {
  if (!window.amitiaDesktop) return
  state.value = "idle"
  await window.amitiaDesktop.restartLater()
}

function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  if (bytes < 1024) return `${Math.round(bytes)} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatSpeed(bytes: number): string {
  if (bytes < 1024) return `${bytes} B/s`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB/s`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB/s`
}

onMounted(() => {
  if (!isDesktopShell()) return
  const api = window.amitiaDesktop!

  api.onUpdateAvailable((_event, data: any) => {
    currentVersion.value = data.currentVersion || ""
    latestVersion.value = data.version || ""
    releaseNotes.value = data.releaseNotes ? String(data.releaseNotes) : ""
    state.value = "available"
  })

  api.onUpdateDownloadProgress((_event, data: any) => {
    const percent = Number(data?.percent)
    const transferred = Number(data?.transferred)
    const total = Number(data?.total)
    const bytesPerSecond = Number(data?.bytesPerSecond)
    downloadPercent.value = Number.isFinite(percent) ? Math.min(100, Math.max(0, Math.round(percent))) : 0
    downloadedBytes.value = Number.isFinite(transferred) && transferred > 0 ? transferred : 0
    totalBytes.value = Number.isFinite(total) && total > 0 ? total : 0
    downloadSpeed.value = Number.isFinite(bytesPerSecond) && bytesPerSecond > 0 ? formatSpeed(bytesPerSecond) : ""
  })

  api.onUpdateDownloaded((_event, data: any) => {
    latestVersion.value = data.version || latestVersion.value
    state.value = "downloaded"
  })

  api.onUpdateError((_event, data: any) => {
    errorMessage.value = data?.message || "更新过程发生错误"
    state.value = "error"
  })

  api.onUpdateNotAvailable(() => {
    state.value = "idle"
  })
})
</script>

<style scoped>
.update-dialog :deep(.el-dialog__header) {
  padding: 20px 24px 0;
  margin: 0;
}

.update-dialog-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--console-text);
}

.update-dialog-body {
  padding: 16px 0;
  min-height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.update-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  width: 100%;
}

.update-versions {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 18px;
  font-weight: 500;
}

.version-old {
  color: var(--console-text-muted);
}

.version-arrow {
  color: var(--console-text-muted);
  font-size: 14px;
}

.version-new {
  color: var(--el-color-primary);
  font-weight: 600;
}

.update-notes {
  width: 100%;
  max-height: 120px;
  overflow-y: auto;
  background: var(--console-bg-subtle);
  border-radius: 6px;
  padding: 10px 14px;
}

.update-notes-label {
  font-size: 12px;
  color: var(--console-text-muted);
  margin-bottom: 6px;
}

.update-notes-text {
  font-size: 13px;
  color: var(--console-text);
  white-space: pre-wrap;
  word-break: break-word;
}

.update-downloading {
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.update-downloading :deep(.el-progress) {
  width: 100%;
}

.download-progress-meta {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
  color: var(--console-text-muted);
  font-variant-numeric: tabular-nums;
}

.update-downloaded {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  color: var(--console-text);
}

.update-success-icon {
  font-size: 28px;
  color: var(--el-color-success);
  flex-shrink: 0;
}

.update-error {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  color: var(--console-text);
}

.update-error-icon {
  font-size: 28px;
  color: var(--el-color-danger);
  flex-shrink: 0;
}

.update-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
