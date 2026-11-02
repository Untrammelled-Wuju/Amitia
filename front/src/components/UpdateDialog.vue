<template>
  <el-dialog
    data-amitia-ui="component.dialog"
    :model-value="visible"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :show-close="false"
    align-center
    width="850px"
    :class="['update-dialog', { 'is-complete': state === 'downloaded' || state === 'error' }]"
  >
    <template #header>
      <div v-if="state === 'available' || state === 'downloading'" class="update-header">
        <div class="update-brand">
          <img :src="logoUrl" alt="Amitia" />
        </div>
        <div class="update-heading">
          <h2>{{ title }}</h2>
          <p>{{ headingDescription }}</p>
        </div>
        <button
          class="dialog-close"
          type="button"
          aria-label="关闭更新弹窗"
          @click="handleDismiss"
        >
          <el-icon><Close /></el-icon>
        </button>
      </div>
    </template>

    <div v-if="state === 'available' || state === 'downloading'" class="dialog-content">
      <aside class="version-panel">
        <p class="panel-kicker">更新版本</p>
        <p class="new-version">
          <template v-for="(line, index) in latestVersionHeadline" :key="line">
            {{ line }}<br v-if="index < latestVersionHeadline.length - 1" />
          </template>
        </p>
        <div class="version-route">
          <div class="version-item">
            <span class="version-label">当前</span>
            <span class="version-value">{{ currentVersionText }}</span>
          </div>
          <el-icon class="version-arrow"><Right /></el-icon>
          <div class="version-item latest">
            <span class="version-label">最新</span>
            <span class="version-value">{{ latestVersionText }}</span>
          </div>
        </div>
        <div class="meta-list">
          <div class="meta-row">
            <span>发布日期</span>
            <strong>{{ releaseDateText }}</strong>
          </div>
          <div class="meta-row">
            <span>安装包</span>
            <strong>{{ releaseSizeText }}</strong>
          </div>
          <div class="meta-row">
            <span>更新通道</span>
            <strong>{{ channelLabel }}</strong>
          </div>
        </div>
      </aside>

      <article class="release-panel" tabindex="0" aria-label="完整更新日志">
        <div class="release-inner">
          <section class="release-intro-section">
            <h3>本次更新</h3>
            <p>{{ updateTitle }}</p>
          </section>

          <section v-if="highlights.length" class="highlight-panel" aria-labelledby="highlightTitle">
            <h3 id="highlightTitle" class="highlight-title">
              <span class="highlight-mark" aria-hidden="true">✦</span>
              本版亮点
            </h3>
            <div class="highlight-grid">
              <div v-for="(highlight, index) in highlights" :key="`${index}-${highlight}`" class="highlight-item">
                {{ highlight }}
              </div>
            </div>
          </section>

          <section class="release-notes-section" aria-labelledby="releaseNotesTitle">
            <h3 id="releaseNotesTitle">完整更新内容</h3>
            <div v-if="releaseNotes" class="release-markdown">
              <MarkdownContent :source="releaseNotes" />
            </div>
            <p v-else class="empty-notes">暂无详细更新内容。</p>
          </section>
        </div>
      </article>
    </div>

    <section v-else-if="state === 'downloaded'" class="download-complete">
      <div class="complete-icon">
        <el-icon><CircleCheckFilled /></el-icon>
      </div>
      <h2>新版本已下载完成</h2>
      <p>{{ latestVersionText }} 已准备好安装。点击“确定”后开始安装。</p>
      <el-button type="primary" class="complete-button" @click="handleRestartNow">确定</el-button>
    </section>

    <section v-else-if="state === 'error'" class="download-error">
      <div class="error-icon">
        <el-icon><WarningFilled /></el-icon>
      </div>
      <h2>更新失败</h2>
      <p>{{ errorMessage }}</p>
      <el-button class="complete-button" @click="handleDismiss">关闭</el-button>
    </section>

    <template #footer>
      <footer v-if="state === 'available'" class="dialog-footer">
        <div class="dialog-footer-note">
          <span>升级后首次启动可能需要一小段时间完成运行环境和数据库检查</span>
          <span>如遇到升级后不可用的情况，可在设置-问题反馈中提交问题，或添加QQ群聊193879020进行反馈</span>
        </div>
        <div class="footer-actions">
          <el-button text class="skip-button" @click="handleSkip">跳过此版本</el-button>
          <el-button class="remind-button" @click="handleRemindLater">稍后提醒</el-button>
          <el-button type="primary" class="download-button" @click="handleStartDownload">
            <el-icon><Download /></el-icon>
            立即下载
          </el-button>
        </div>
      </footer>

      <footer v-else-if="state === 'downloading'" class="dialog-footer">
        <div class="dialog-footer-note">
          <span>关闭弹窗不会中止下载，完成后会再次提示确认安装。</span>
          <span>如遇到升级后不可用的情况，可在设置-问题反馈中提交问题，或添加QQ群聊193879020进行反馈。</span>
        </div>
        <div class="download-progress" aria-live="polite">
          <div class="progress-meta">
            <span>正在下载更新</span>
            <strong>{{ downloadPercent }}%</strong>
          </div>
          <div class="progress-track">
            <div class="progress-fill" :style="{ width: `${downloadPercent}%` }"></div>
          </div>
          <div class="progress-detail">
            <span>{{ downloadSizeText }}</span>
            <span v-if="downloadSpeed">{{ downloadSpeed }}</span>
          </div>
        </div>
      </footer>

    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  Right,
  Close,
  Download,
  CircleCheckFilled,
  WarningFilled,
} from "@element-plus/icons-vue";
import MarkdownContent from "@/conversation/rendering/markdown/MarkdownContent.vue";
import { useBrandLogo } from "@/composables/useBrandLogo";
import { isDesktopShell } from "@/runtime/runtime-capabilities";

type DialogState =
  | "idle"
  | "available"
  | "downloading"
  | "downloaded"
  | "error";

interface UpdatePresentation {
  currentVersion?: unknown;
  version?: unknown;
  releaseDate?: unknown;
  releaseNotes?: unknown;
  updateTitle?: unknown;
  highlights?: unknown;
  size?: unknown;
  channel?: unknown;
}

const state = ref<DialogState>("idle");
const currentVersion = ref("");
const latestVersion = ref("");
const releaseDate = ref("");
const releaseNotes = ref("");
const updateTitle = ref("查看本次版本的主要变化。");
const highlights = ref<string[]>([]);
const releaseSize = ref(0);
const channel = ref("");
const downloadPercent = ref(0);
const downloadedBytes = ref(0);
const totalBytes = ref(0);
const downloadSpeed = ref("");
const errorMessage = ref("");
const { logoUrl } = useBrandLogo();
const cleanupCallbacks: Array<() => void> = [];

const visible = computed(() => state.value !== "idle");
const currentVersionText = computed(() => formatVersion(currentVersion.value));
const latestVersionText = computed(() => formatVersion(latestVersion.value));
const latestVersionHeadline = computed(() => formatHeadlineVersion(latestVersion.value));
const releaseDateText = computed(() => formatDate(releaseDate.value));
const releaseSizeText = computed(() => formatFileSize(releaseSize.value));
const channelLabel = computed(() => (channel.value === "beta" ? "测试版" : "正式版"));
const headingDescription = computed(() =>
  state.value === "downloading"
    ? "下载将在后台继续，完成后会提示确认安装"
    : "Amitia 可以更新了！",
);
const downloadSizeText = computed(() => {
  const downloaded = formatFileSize(downloadedBytes.value);
  const total = formatFileSize(totalBytes.value || releaseSize.value);
  return `${downloaded} / ${total}`;
});
const title = computed(() => {
  switch (state.value) {
    case "available":
      return "发现新版本";
    case "downloading":
      return "正在下载更新";
    case "downloaded":
      return "新版本已下载完成";
    case "error":
      return "更新失败";
    default:
      return "";
  }
});

function formatVersion(value: string): string {
  const normalized = String(value ?? "").trim();
  if (!normalized) return "—";
  return normalized.startsWith("v") ? normalized : `v${normalized}`;
}

function formatHeadlineVersion(value: string): string[] {
  const normalized = String(value ?? "").trim();
  if (!normalized) return ["—"];
  const separator = normalized.indexOf("-");
  if (separator < 0) return [normalized];
  return [
    normalized.slice(0, separator),
    normalized.slice(separator + 1),
  ];
}

function normalizeReleaseNotes(value: unknown): string {
  if (typeof value === "string") return value;
  if (Array.isArray(value)) {
    return value
      .map((item) => {
        if (typeof item === "string") return item;
        if (item && typeof item === "object" && "note" in item) {
          return String((item as { note?: unknown }).note ?? "");
        }
        return "";
      })
      .filter(Boolean)
      .join("\n\n");
  }
  return value == null ? "" : String(value);
}

function normalizeHighlights(value: unknown): string[] {
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => String(item ?? "").trim())
    .filter(Boolean)
    .slice(0, 8);
}

function applyUpdateData(data: UpdatePresentation) {
  currentVersion.value = String(data.currentVersion ?? "");
  latestVersion.value = String(data.version ?? "");
  releaseDate.value = String(data.releaseDate ?? "");
  releaseNotes.value = normalizeReleaseNotes(data.releaseNotes);
  updateTitle.value =
    String(data.updateTitle ?? "").trim() || "查看本次版本的主要变化。";
  highlights.value = normalizeHighlights(data.highlights);
  releaseSize.value = Math.max(0, Number(data.size) || 0);
  channel.value = String(data.channel ?? "");
}

async function handleDismiss() {
  if (state.value === "downloaded") return;
  state.value = "idle";
  if (!window.amitiaDesktop) return;
  try {
    await window.amitiaDesktop.cancelAndEnter();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
    state.value = "error";
  }
}

async function handleStartDownload() {
  if (!window.amitiaDesktop) return;
  state.value = "downloading";
  downloadPercent.value = 0;
  downloadedBytes.value = 0;
  totalBytes.value = releaseSize.value;
  downloadSpeed.value = "";
  try {
    await window.amitiaDesktop.startDownload();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
    state.value = "error";
  }
}

async function handleSkip() {
  if (!window.amitiaDesktop) return;
  state.value = "idle";
  try {
    await window.amitiaDesktop.skipVersion();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
    state.value = "error";
  }
}

async function handleRemindLater() {
  if (!window.amitiaDesktop) return;
  state.value = "idle";
  try {
    await window.amitiaDesktop.cancelAndEnter();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
    state.value = "error";
  }
}

async function handleRestartNow() {
  if (!window.amitiaDesktop) return;
  try {
    await window.amitiaDesktop.restartNow();
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : String(error);
    state.value = "error";
  }
}

function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "—";
  if (bytes < 1024) return `${Math.round(bytes)} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatDate(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function formatSpeed(bytes: number): string {
  if (bytes < 1024) return `${bytes} B/s`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB/s`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB/s`;
}

onMounted(() => {
  if (!isDesktopShell() || !window.amitiaDesktop) return;
  const api = window.amitiaDesktop;

  cleanupCallbacks.push(
    api.onUpdateAvailable((_event, data) => {
      applyUpdateData((data ?? {}) as UpdatePresentation);
      state.value = "available";
    }),
    api.onUpdateDownloadProgress((_event, data: any) => {
      const percent = Number(data?.percent);
      const transferred = Number(data?.transferred);
      const total = Number(data?.total);
      const bytesPerSecond = Number(data?.bytesPerSecond);
      downloadPercent.value = Number.isFinite(percent)
        ? Math.min(100, Math.max(0, Math.round(percent)))
        : 0;
      downloadedBytes.value =
        Number.isFinite(transferred) && transferred > 0 ? transferred : 0;
      totalBytes.value =
        Number.isFinite(total) && total > 0 ? total : releaseSize.value;
      downloadSpeed.value =
        Number.isFinite(bytesPerSecond) && bytesPerSecond > 0
          ? formatSpeed(bytesPerSecond)
          : "";
    }),
    api.onUpdateDownloaded((_event, data: any) => {
      latestVersion.value = data.version || latestVersion.value;
      state.value = "downloaded";
    }),
    api.onUpdateError((_event, data: any) => {
      errorMessage.value = data?.message || "更新过程发生错误";
      state.value = "error";
    }),
    api.onUpdateNotAvailable(() => {
      state.value = "idle";
    }),
  );

  void api.getPendingUpdate().then((pending) => {
    if (!pending?.version) return;
    applyUpdateData(pending);
    state.value = "available";
  });
});

onUnmounted(() => {
  cleanupCallbacks.splice(0).forEach((cleanup) => cleanup());
});
</script>

<style scoped>
:global(.el-dialog.update-dialog) {
  display: grid;
  grid-template-rows: 76px minmax(0, 1fr) 76px;
  width: min(850px, calc(100vw - 48px));
  max-width: calc(100vw - 48px);
  height: min(690px, calc(100vh - 48px));
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--console-border-soft);
  border-radius: 20px;
  background: var(--console-card);
  box-shadow: none;
  animation: update-dialog-in 0.34s cubic-bezier(0.2, 0.8, 0.2, 1) both;
}

:global(.el-dialog.update-dialog.is-complete) {
  display: block;
  width: min(430px, calc(100vw - 40px));
  height: auto;
}

@keyframes update-dialog-in {
  from {
    opacity: 0;
    transform: translateY(14px) scale(0.985);
  }

  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

:global(.el-dialog.update-dialog .el-dialog__header) {
  min-height: 76px;
  padding: 0;
}

:global(.el-dialog.update-dialog .el-dialog__body) {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  padding: 0;
}

:global(.el-dialog.update-dialog .el-dialog__footer) {
  min-height: 76px;
  padding: 0;
}

:global(.el-dialog.update-dialog.is-complete .el-dialog__header),
:global(.el-dialog.update-dialog.is-complete .el-dialog__footer) {
  display: none;
}

:global(.el-dialog.update-dialog.is-complete .el-dialog__body) {
  min-height: 0;
}

.update-header {
  display: flex;
  align-items: center;
  gap: 14px;
  height: 100%;
  min-height: 76px;
  padding: 0 22px 0 24px;
  border-bottom: 1px solid var(--console-border-soft);
}

.update-brand {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  flex: 0 0 auto;
  overflow: hidden;
  border: 1px solid var(--console-border-soft);
  border-radius: 12px;
  background: var(--console-bg-subtle);
}

.update-brand img {
  width: 34px;
  height: 34px;
  object-fit: contain;
}

.update-heading {
  min-width: 0;
  flex: 1;
}

.update-heading h2 {
  margin: 0;
  color: var(--console-text);
  font-size: 19px;
  font-weight: 680;
  letter-spacing: -0.015em;
}

.update-heading p {
  margin: 3px 0 0;
  overflow: hidden;
  color: var(--console-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dialog-close {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  border: 0;
  border-radius: 10px;
  color: var(--console-text-muted);
  background: transparent;
  cursor: pointer;
  transition:
    color 0.18s ease,
    background-color 0.18s ease;
}

.dialog-close:hover {
  color: var(--console-text);
  background: var(--console-bg-subtle);
}

.dialog-close :deep(.el-icon) {
  font-size: 18px;
}

.dialog-content {
  display: grid;
  grid-template-columns: 218px minmax(0, 1fr);
  height: 100%;
  min-height: 0;
}

.version-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 24px 22px;
  border-right: 1px solid var(--console-border-soft);
  background: var(--console-card);
}

.panel-kicker {
  margin: 0 0 8px;
  color: var(--console-text-muted);
  font-size: 10px;
  font-weight: 650;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.new-version {
  margin: 0;
  color: var(--console-text);
  font-size: 24px;
  font-weight: 720;
  letter-spacing: -0.035em;
  line-height: 1.15;
}

.version-route {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
  margin-top: 22px;
  color: var(--console-text-secondary);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.version-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}

.version-label {
  color: var(--console-text-muted);
  font-size: 9px;
  line-height: 1.3;
}

.version-value {
  color: var(--console-text-secondary);
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
}

.version-item.latest .version-value {
  color: var(--console-text-secondary);
  font-weight: 650;
}

.version-arrow {
  align-self: flex-start;
  color: var(--console-text-muted);
  font-size: 12px;
  transform: rotate(90deg);
}

.meta-list {
  display: grid;
  gap: 0;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid var(--console-border);
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 38px;
  border-bottom: 1px solid var(--console-border);
  font-size: 12px;
}

.meta-row span {
  color: var(--console-text-muted);
}

.meta-row strong {
  color: var(--console-text-secondary);
  font-weight: 600;
  text-align: right;
}

.release-panel {
  min-width: 0;
  height: 100%;
  overflow: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: var(--console-border) transparent;
}

.release-panel::-webkit-scrollbar {
  width: 6px;
}

.release-panel::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: var(--console-border);
}

.release-inner {
  padding: 24px 28px 30px;
}

.release-intro-section h3 {
  margin: 0 0 8px;
  color: var(--console-text);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.release-notes-section > h3 {
  margin: 0;
  color: var(--console-text);
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.release-intro-section p {
  margin: 0;
  color: var(--console-text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.highlight-panel {
  margin-top: 20px;
  padding: 15px 16px;
  border: 1px solid var(--ac-color-primary-border);
  border-radius: 14px;
  background: var(--ac-color-primary-bg);
}

.highlight-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 10px;
  color: var(--el-color-primary);
  font-size: 12px;
  font-weight: 700;
}

.highlight-mark {
  font-size: 13px;
}

.highlight-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px 16px;
}

.highlight-item {
  display: grid;
  grid-template-columns: 6px minmax(0, 1fr);
  gap: 8px;
  color: var(--console-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.highlight-item::before {
  width: 5px;
  height: 5px;
  margin-top: 7px;
  border-radius: 50%;
  background: var(--el-color-primary);
  content: "";
}

.release-notes-section {
  margin-top: 26px;
}

.release-markdown {
  --amrp-text: var(--console-text);
  --amrp-muted: var(--console-text-muted);
  --amrp-line: var(--console-border);
  --amrp-surface: var(--console-card);
  --amrp-soft: var(--console-bg-subtle);
  --amrp-accent: var(--el-color-primary);
  --amrp-inline-code: var(--console-code-bg);
  color: var(--console-text);
  font-size: 13px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.release-markdown :deep(.amrp-markdown-segment p) {
  max-width: none;
  margin: 0 0 11px;
  color: var(--console-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.release-markdown :deep(.amrp-markdown-segment h1),
.release-markdown :deep(.amrp-markdown-segment h2),
.release-markdown :deep(.amrp-markdown-segment h3),
.release-markdown :deep(.amrp-markdown-segment h4) {
  max-width: none;
  color: var(--console-text);
  line-height: 1.4;
}

.release-markdown :deep(.amrp-markdown-segment h2) {
  margin: 0 0 11px;
  font-size: 14px;
  font-weight: 680;
  letter-spacing: -0.005em;
}

.release-markdown :deep(.amrp-markdown-segment h2:not(:first-child)) {
  margin-top: 26px;
  padding-top: 22px;
  border-top: 1px solid var(--console-border-soft);
}

.release-markdown :deep(.amrp-markdown-segment h3) {
  margin: 20px 0 9px;
  font-size: 14px;
  font-weight: 680;
}

.release-markdown :deep(.amrp-markdown-segment ul),
.release-markdown :deep(.amrp-markdown-segment ol) {
  display: grid;
  max-width: none;
  gap: 9px;
  margin: 0 0 13px;
  padding: 0;
  list-style: none;
}

.release-markdown :deep(.amrp-markdown-segment li) {
  position: relative;
  margin: 0;
  padding-left: 17px;
  color: var(--console-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.release-markdown :deep(.amrp-markdown-segment li::before) {
  position: absolute;
  top: 8px;
  left: 1px;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--console-text-muted);
  content: "";
}

.release-markdown :deep(.amrp-markdown-segment strong) {
  color: var(--console-text);
  font-weight: 650;
}

.empty-notes {
  margin: 0;
  color: var(--console-text-muted);
  font-size: 13px;
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  height: 76px;
  padding: 14px 20px;
  border-top: 1px solid var(--console-border-soft);
  background: color-mix(in srgb, var(--console-card) 94%, var(--console-bg-subtle));
}

.dialog-footer-note {
  display: grid;
  justify-items: start;
  gap: 1px;
  color: var(--console-text-muted);
  font-size: 11px;
  line-height: 1.45;
  text-align: left;
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.footer-actions :deep(.el-button) {
  min-height: 38px;
  margin-left: 0;
  padding: 0 15px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 650;
}

.footer-actions :deep(.skip-button) {
  padding: 0 8px;
  color: var(--console-text-secondary);
}

.footer-actions :deep(.skip-button:hover) {
  color: var(--console-text);
  background: var(--console-bg-subtle);
}

.footer-actions :deep(.remind-button) {
  border-color: var(--console-border);
  color: var(--console-text);
  background: var(--console-card);
}

.footer-actions :deep(.remind-button:hover) {
  border-color: var(--console-text-muted);
  background: var(--console-bg-subtle);
}

.download-button {
  box-shadow: 0 3px 10px color-mix(in srgb, var(--el-color-primary) 10%, transparent);
}

.download-button :deep(.el-icon) {
  font-size: 16px;
}

.download-progress {
  display: grid;
  width: min(330px, 100%);
  gap: 6px;
}

.progress-meta,
.progress-detail {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-variant-numeric: tabular-nums;
}

.progress-meta {
  color: var(--console-text);
  font-size: 11px;
  font-weight: 650;
}

.progress-meta strong {
  color: var(--el-color-primary);
}

.progress-track {
  height: 7px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--ac-color-primary-bg);
}

.progress-fill {
  width: 0;
  height: 100%;
  border-radius: inherit;
  background: var(--el-color-primary);
  transition: width 0.12s linear;
}

.progress-detail {
  color: var(--console-text-muted);
  font-size: 10px;
}

.download-complete,
.download-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 30px 28px 26px;
  text-align: center;
}

.complete-icon,
.error-icon {
  display: grid;
  place-items: center;
  width: 50px;
  height: 50px;
  border-radius: 16px;
  font-size: 25px;
}

.complete-icon {
  color: var(--el-color-success);
  background: var(--el-color-success-light-9);
}

.error-icon {
  color: var(--el-color-danger);
  background: var(--el-color-danger-light-9);
}

.download-complete h2,
.download-error h2 {
  margin: 16px 0 0;
  color: var(--console-text);
  font-size: 19px;
  font-weight: 720;
  letter-spacing: -0.02em;
}

.download-complete p,
.download-error p {
  max-width: 374px;
  margin: 8px 0 22px;
  color: var(--console-text-secondary);
  font-size: 13px;
  line-height: 1.65;
  word-break: break-word;
}

.complete-button {
  min-width: 104px;
  min-height: 38px;
  padding: 0 15px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 650;
}

@media (max-width: 760px) {
  :global(.el-dialog.update-dialog) {
    max-width: calc(100vw - 24px);
  }

  .dialog-content {
    display: block;
    max-height: calc(100vh - 190px);
    overflow: auto;
  }

  .version-panel {
    border-right: 0;
    border-bottom: 1px solid var(--console-border-soft);
  }

  .meta-list {
    margin-top: 20px;
  }

  .release-panel {
    max-height: none;
    overflow: visible;
  }

  .highlight-grid {
    grid-template-columns: 1fr;
  }

  .dialog-footer {
    align-items: stretch;
    flex-direction: column;
  }

  .footer-actions {
    justify-content: flex-end;
  }
}
</style>
