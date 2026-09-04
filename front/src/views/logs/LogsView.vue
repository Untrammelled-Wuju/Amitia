<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="logs-page">
    <div class="page-header">
      <div>
        <h2 class="page-title">系统运行日志</h2>
        <p>读取当前 Business Core 的运行日志；云端模式下这里显示 Cloud Core 日志。</p>
      </div>
      <el-button :loading="loading" @click="refreshCurrent">刷新</el-button>
    </div>

    <el-tabs v-model="activeTab" @tab-change="refreshCurrent">
      <el-tab-pane label="最近日志" name="recent">
        <div class="log-toolbar">
          <span class="log-count">{{ recentLogs.length }} 条</span>
          <el-button
            size="small"
            type="danger"
            plain
            :disabled="logFiles.length === 0"
            @click="clearAllLogs"
          >清除日志文件</el-button>
        </div>
        <el-table :data="recentLogs" stripe size="small" max-height="560" v-loading="loading">
          <el-table-column prop="file" label="文件" width="190" show-overflow-tooltip />
          <el-table-column prop="line" label="内容" min-width="520" show-overflow-tooltip />
          <el-table-column prop="time" label="读取时间" width="170" />
        </el-table>
        <el-empty v-if="!loading && recentLogs.length === 0" description="暂无运行日志" :image-size="48" />
      </el-tab-pane>

      <el-tab-pane label="日志文件" name="files">
        <div class="files-layout" v-loading="loading">
          <div class="file-list">
            <div class="file-toolbar">
              <span class="log-count">{{ logFiles.length }} 个文件</span>
              <el-button
                size="small"
                type="danger"
                plain
                :disabled="logFiles.length === 0"
                @click="clearAllLogs"
              >清除所有</el-button>
            </div>
            <button
              v-for="file in logFiles"
              :key="file.name"
              type="button"
              class="file-item"
              :class="{ active: selectedFile === file.name }"
              @click="viewFile(file.name)"
            >
              <span class="fi-main">
                <span class="fi-name">{{ file.name }}</span>
                <span class="fi-time">{{ file.modTime || "" }}</span>
              </span>
              <span class="fi-size">{{ formatSize(file.size) }}</span>
            </button>
            <el-empty v-if="!loading && logFiles.length === 0" description="暂无日志文件" :image-size="40" />
          </div>
          <div v-if="selectedFile" class="file-content">
            <div class="fc-header">
              <span>{{ selectedFile }}</span>
              <span class="fc-lines">{{ fileLineCount }} 行</span>
            </div>
            <pre class="fc-body">{{ fileContent }}</pre>
          </div>
          <div v-else class="file-content empty">
            <el-empty description="选择左侧文件查看" :image-size="50" />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useApi } from "@/composables/useApi";

type RecentLog = { file?: string; line?: string; time?: string };
type LogFile = { name: string; size: number; modTime?: string };

const { get, del } = useApi();
const activeTab = ref("recent");
const loading = ref(false);
const recentLogs = ref<RecentLog[]>([]);
const logFiles = ref<LogFile[]>([]);
const selectedFile = ref("");
const fileContent = ref("");
const fileLineCount = ref(0);

async function fetchRecentLogs() {
  const result = await get<{ logs?: RecentLog[] }>("/api/logs/recent", { limit: 100 });
  recentLogs.value = Array.isArray(result?.logs) ? result.logs : [];
}

async function fetchLogFiles() {
  const result = await get<{ files?: LogFile[] }>("/api/logs/files");
  logFiles.value = Array.isArray(result?.files)
    ? result.files.filter((file) => String(file?.name || "").toLowerCase().endsWith(".log"))
    : [];
  if (selectedFile.value && !logFiles.value.some((file) => file.name === selectedFile.value)) {
    selectedFile.value = "";
    fileContent.value = "";
    fileLineCount.value = 0;
  }
}

async function refreshCurrent() {
  loading.value = true;
  try {
    if (activeTab.value === "recent") {
      await Promise.all([fetchRecentLogs(), fetchLogFiles()]);
    } else {
      await fetchLogFiles();
      if (selectedFile.value) await viewFile(selectedFile.value, false);
    }
  } catch (e: any) {
    ElMessage.error(e?.message || "日志加载失败");
  } finally {
    loading.value = false;
  }
}

async function clearAllLogs() {
  try {
    await ElMessageBox.confirm("确定删除当前 Core 的所有 .log 运行日志文件？", "清除系统日志", { type: "warning" });
    await del("/api/logs");
    recentLogs.value = [];
    logFiles.value = [];
    selectedFile.value = "";
    fileContent.value = "";
    fileLineCount.value = 0;
    ElMessage.success("系统日志已清除");
  } catch (e: any) {
    if (e === "cancel" || e === "close") return;
    ElMessage.error(e?.message || "清除失败");
  }
}

async function viewFile(name: string, showLoading = true) {
  selectedFile.value = name;
  if (showLoading) loading.value = true;
  try {
    const content = await get<string>(`/api/logs/files/${encodeURIComponent(name)}`);
    fileContent.value = typeof content === "string" ? content : String(content ?? "");
    fileLineCount.value = fileContent.value ? fileContent.value.split(/\r?\n/).length : 0;
  } catch (e: any) {
    fileContent.value = "";
    fileLineCount.value = 0;
    ElMessage.error(e?.message || "日志文件读取失败");
  } finally {
    if (showLoading) loading.value = false;
  }
}

function formatSize(bytes: number): string {
  const value = Number(bytes) || 0;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

onMounted(refreshCurrent);
</script>

<style scoped>
.logs-page {
  min-height: 320px;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}
.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.page-header p {
  margin: 5px 0 0;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}
.log-toolbar,
.file-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}
.log-count {
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}
.files-layout {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: 14px;
  min-height: 440px;
}
.file-list {
  overflow-y: auto;
  border-right: 1px solid var(--ac-color-border-light);
  padding-right: 10px;
}
.file-item {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px;
  border: 0;
  border-radius: var(--ac-radius-sm);
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}
.file-item:hover,
.file-item.active {
  background: var(--ac-color-surface-hover);
}
.fi-main {
  min-width: 0;
}
.fi-name,
.fi-time {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.fi-name {
  font-size: var(--ac-font-size-sm);
}
.fi-time,
.fi-size {
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}
.fi-size {
  flex-shrink: 0;
}
.file-content {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
}
.file-content.empty {
  display: grid;
  place-items: center;
}
.fc-header {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
  font-size: var(--ac-font-size-sm);
}
.fc-lines {
  color: var(--ac-color-text-muted);
}
.fc-body {
  height: 410px;
  overflow: auto;
  margin: 0;
  padding: 12px;
  background: var(--ac-color-surface-secondary);
  font: 12px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
@media (max-width: 800px) {
  .files-layout {
    grid-template-columns: 1fr;
  }
  .file-list {
    max-height: 220px;
    border-right: 0;
    border-bottom: 1px solid var(--ac-color-border-light);
    padding: 0 0 10px;
  }
}
</style>
