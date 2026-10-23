<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage, ElMessageBox } from "element-plus"

import { errorMessage } from "@/api/client"
import {
  FEEDBACK_EXPORT_URL,
  feedbackScreenshotURL,
  fetchFeedback,
  fetchFeedbacks,
  updateFeedbackResolved,
} from "@/api/feedbacks"
import type { UserFeedbackDetail, UserFeedbackSummary } from "@/api/feedbacks"

const loading = ref(false)
const items = ref<UserFeedbackSummary[]>([])
const total = ref(0)
const query = reactive({
  page: 1,
  pageSize: 20,
  category: "",
  platform: "",
  keyword: "",
  resolved: "",
})

const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<UserFeedbackDetail | null>(null)
const detailTab = ref("summary")
const resolvingId = ref(0)

const categoryOptions = [
  { value: "", label: "全部类型" },
  { value: "bug", label: "功能异常" },
  { value: "experience", label: "体验问题" },
  { value: "feature", label: "功能建议" },
  { value: "other", label: "其他" },
]

const platformOptions = [
  { value: "", label: "全部平台" },
  { value: "desktop", label: "桌面端" },
  { value: "web", label: "网页端" },
  { value: "android", label: "Android" },
  { value: "ios", label: "iOS" },
]

const resolvedOptions = [
  { value: "", label: "全部状态" },
  { value: "false", label: "未解决" },
  { value: "true", label: "已解决" },
]

const payload = computed(() => detail.value?.payload ?? {})
const errorLines = computed<any[]>(() => (Array.isArray(payload.value.errorLines) ? payload.value.errorLines : []))
const recentLines = computed<any[]>(() => (Array.isArray(payload.value.recentLines) ? payload.value.recentLines : []))
const clientErrors = computed<any[]>(() => (Array.isArray(payload.value.clientErrors) ? payload.value.clientErrors : []))
const logFiles = computed<string[]>(() => (Array.isArray(payload.value.logFiles) ? payload.value.logFiles : []))
const hasScreenshot = computed(() => detail.value?.hasScreenshot === true)
const screenshotURL = computed(() => (detail.value ? feedbackScreenshotURL(detail.value.id) : ""))

function categoryLabel(value: string) {
  return categoryOptions.find((option) => option.value === value)?.label || value || "其他"
}

function platformLabel(value: string) {
  return platformOptions.find((option) => option.value === value)?.label || value || "-"
}

function formatTime(value: string) {
  if (!value) return "-"
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(parsed)
}

function formatSize(value: number) {
  if (!value) return "-"
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(2)} MB`
}

function lineText(line: any) {
  if (typeof line === "string") return line
  const file = line?.file ? `[${line.file}] ` : ""
  return `${file}${line?.line ?? ""}`
}

async function load() {
  loading.value = true
  try {
    const result = await fetchFeedbacks({
      page: query.page,
      pageSize: query.pageSize,
      category: query.category,
      platform: query.platform,
      keyword: query.keyword.trim(),
      resolved: query.resolved,
    })
    items.value = result?.items ?? []
    total.value = result?.total ?? 0
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

function search() {
  query.page = 1
  void load()
}

function resetFilters() {
  query.category = ""
  query.platform = ""
  query.keyword = ""
  query.resolved = ""
  search()
}

function exportCsv() {
  window.open(FEEDBACK_EXPORT_URL, "_blank")
}

async function openDetail(row: UserFeedbackSummary) {
  detailVisible.value = true
  detailLoading.value = true
  detailTab.value = "summary"
  detail.value = null
  try {
    detail.value = await fetchFeedback(row.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    detailLoading.value = false
  }
}

async function changeResolved(id: number, title: string, next: boolean) {
  try {
    await ElMessageBox.confirm(
      next ? `确认将“${title}”标记为已解决吗？` : `确认将“${title}”恢复为未解决吗？`,
      next ? "标记已解决" : "取消解决",
      { type: "warning", confirmButtonText: "确认", cancelButtonText: "取消" },
    )
  } catch {
    return
  }
  resolvingId.value = id
  try {
    const result = await updateFeedbackResolved(id, next)
    const row = items.value.find((item) => item.id === id)
    if (row) {
      row.resolved = result.resolved
      row.resolvedAt = result.resolvedAt
      row.resolvedBy = result.resolvedBy
    }
    if (detail.value?.id === id) {
      detail.value.resolved = result.resolved
      detail.value.resolvedAt = result.resolvedAt
      detail.value.resolvedBy = result.resolvedBy
    }
    if (query.resolved !== "") {
      void load()
    }
    ElMessage.success(result.resolved ? "已标记为已解决" : "已恢复为未解决")
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    resolvingId.value = 0
  }
}

onMounted(load)
</script>

<template>
  <section class="panel full-panel" v-loading="loading">
    <div class="panel-heading">
      <div>
        <h2>用户反馈</h2>
        <p>用户端在设置中提交的问题反馈，包含自动采集的运行环境与报错日志。</p>
      </div>
      <el-button @click="exportCsv">
        <el-icon><Download /></el-icon>
        导出 CSV
      </el-button>
    </div>

    <div class="filter-row">
      <el-select v-model="query.category" style="width: 150px" @change="search">
        <el-option v-for="option in categoryOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-select v-model="query.platform" style="width: 140px" @change="search">
        <el-option v-for="option in platformOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-select v-model="query.resolved" style="width: 130px" @change="search">
        <el-option v-for="option in resolvedOptions" :key="option.value" :label="option.label" :value="option.value" />
      </el-select>
      <el-input
        v-model="query.keyword"
        placeholder="搜索标题、联系方式或设备标识"
        style="width: 280px"
        clearable
        @keyup.enter="search"
      />
      <el-button type="primary" @click="search">
        <el-icon><Search /></el-icon>
        查询
      </el-button>
      <el-button @click="resetFilters">重置</el-button>
      <el-button @click="load">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>

    <el-table :data="items" size="large" @row-click="openDetail">
      <el-table-column label="收到时间" min-width="170">
        <template #default="{ row }">{{ formatTime(row.receivedAt) }}</template>
      </el-table-column>
      <el-table-column label="类型" width="110">
        <template #default="{ row }">
          <el-tag effect="plain" size="small">{{ categoryLabel(row.category) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
      <el-table-column label="来源" width="120">
        <template #default="{ row }">{{ platformLabel(row.platform) }}</template>
      </el-table-column>
      <el-table-column label="版本" min-width="150">
        <template #default="{ row }">
          <div class="version-cell">
            <span>应用 {{ row.appVersion || "-" }}</span>
            <span>Core {{ row.coreVersion || "-" }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="报错日志" width="120">
        <template #default="{ row }">{{ row.errorLineCount }} / {{ row.logLineCount }}</template>
      </el-table-column>
      <el-table-column label="截图" width="80">
        <template #default="{ row }">
          <el-tag v-if="row.hasScreenshot" type="success" effect="plain" size="small">有</el-tag>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.resolved" type="success" effect="plain" size="small">已解决</el-tag>
          <el-tag v-else type="info" effect="plain" size="small">未解决</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180">
        <template #default="{ row }">
          <el-button link type="primary" @click.stop="openDetail(row)">查看</el-button>
          <el-button
            link
            :type="row.resolved ? 'info' : 'success'"
            :loading="resolvingId === row.id"
            @click.stop="changeResolved(row.id, row.title, !row.resolved)"
          >
            {{ row.resolved ? "取消解决" : "标记解决" }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        layout="total, prev, pager, next"
        :total="total"
        :current-page="query.page"
        :page-size="query.pageSize"
        @current-change="(page: number) => { query.page = page; load() }"
      />
    </div>

    <el-drawer v-model="detailVisible" title="反馈详情" size="720px">
      <div v-loading="detailLoading" class="detail-body">
        <template v-if="detail">
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="标题" :span="2">{{ payload.title || "-" }}</el-descriptions-item>
            <el-descriptions-item label="类型">{{ categoryLabel(payload.category) }}</el-descriptions-item>
            <el-descriptions-item label="来源">{{ platformLabel(payload.platform) }}</el-descriptions-item>
            <el-descriptions-item label="应用版本">{{ payload.appVersion || "-" }}</el-descriptions-item>
            <el-descriptions-item label="Core 版本">{{ payload.coreVersion || "-" }}</el-descriptions-item>
            <el-descriptions-item label="运行模式">{{ payload.runtimeMode || "-" }}</el-descriptions-item>
            <el-descriptions-item label="客户端">{{ payload.clientType || "-" }}</el-descriptions-item>
            <el-descriptions-item label="联系方式">{{ payload.contact || "-" }}</el-descriptions-item>
            <el-descriptions-item label="设备标识">{{ payload.deviceHash || "-" }}</el-descriptions-item>
            <el-descriptions-item label="语言 / 时区">
              {{ payload.locale || "-" }} / {{ payload.timezone || "-" }}
            </el-descriptions-item>
            <el-descriptions-item label="脱敏处数">{{ payload.redactedCount ?? 0 }}</el-descriptions-item>
            <el-descriptions-item label="上报时间">{{ formatTime(detail.receivedAt) }}</el-descriptions-item>
            <el-descriptions-item label="处理状态">
              <el-tag v-if="detail.resolved" type="success" effect="plain" size="small">已解决</el-tag>
              <el-tag v-else type="info" effect="plain" size="small">未解决</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="解决时间">{{ detail.resolvedAt ? formatTime(detail.resolvedAt) : "-" }}</el-descriptions-item>
            <el-descriptions-item label="解决人">{{ detail.resolvedBy || "-" }}</el-descriptions-item>
            <el-descriptions-item label="系统信息" :span="2">{{ payload.osInfo || "-" }}</el-descriptions-item>
            <el-descriptions-item label="问题描述" :span="2">
              <pre class="field-text">{{ payload.description || "-" }}</pre>
            </el-descriptions-item>
          </el-descriptions>

          <div class="detail-actions">
            <el-button
              :type="detail.resolved ? 'info' : 'success'"
              :loading="resolvingId === detail.id"
              @click="changeResolved(detail.id, payload.title || detail.ticketId, !detail.resolved)"
            >
              {{ detail.resolved ? "取消解决" : "标记为已解决" }}
            </el-button>
          </div>

          <el-tabs v-model="detailTab" class="detail-tabs">
            <el-tab-pane :label="`报错日志 (${errorLines.length})`" name="errors">
              <pre v-if="errorLines.length" class="log-block">{{ errorLines.map(lineText).join("\n") }}</pre>
              <el-empty v-else description="没有采集到报错行" :image-size="50" />
            </el-tab-pane>
            <el-tab-pane :label="`客户端异常 (${clientErrors.length})`" name="client">
              <div v-if="clientErrors.length" class="client-error-list">
                <div v-for="(item, index) in clientErrors" :key="index" class="client-error">
                  <div class="client-error-head">
                    <span>{{ item.at || "-" }}</span>
                    <span>{{ item.source || "-" }}</span>
                  </div>
                  <div class="client-error-message">{{ item.message }}</div>
                  <pre v-if="item.stack" class="log-block">{{ item.stack }}</pre>
                </div>
              </div>
              <el-empty v-else description="没有采集到客户端异常" :image-size="50" />
            </el-tab-pane>
            <el-tab-pane :label="`运行日志 (${recentLines.length})`" name="recent">
              <div class="log-files">
                <span class="log-files-label">日志文件</span>
                <el-tag v-for="file in logFiles" :key="file" size="small" effect="plain">{{ file }}</el-tag>
                <span v-if="!logFiles.length" class="log-files-empty">无</span>
              </div>
              <pre v-if="recentLines.length" class="log-block">{{ recentLines.map(lineText).join("\n") }}</pre>
              <el-empty v-else description="用户未附带运行日志" :image-size="50" />
            </el-tab-pane>
            <el-tab-pane label="截图" name="screenshot">
              <img v-if="hasScreenshot" class="screenshot" :src="screenshotURL" :alt="detail.screenshotName" />
              <div v-if="hasScreenshot" class="screenshot-meta">
                {{ detail.screenshotName }}（{{ formatSize(detail.screenshotSize) }}）
              </div>
              <el-empty v-else description="用户未上传截图" :image-size="50" />
            </el-tab-pane>
            <el-tab-pane label="原始数据" name="raw">
              <pre class="log-block">{{ JSON.stringify(payload, null, 2) }}</pre>
            </el-tab-pane>
          </el-tabs>
        </template>
        <el-empty v-else-if="!detailLoading" description="未加载到反馈详情" :image-size="60" />
      </div>
    </el-drawer>
  </section>
</template>

<style scoped>
.filter-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.version-cell {
  display: grid;
  gap: 2px;
  font-size: 12px;
  color: var(--Text_SecondaryColor, #666);
}

.pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.detail-body {
  min-height: 240px;
}

.detail-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.field-text {
  margin: 0;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.detail-tabs {
  margin-top: 16px;
}

.log-block {
  margin: 0;
  padding: 12px;
  max-height: 420px;
  overflow: auto;
  border: 1px solid var(--Main_BoxBorderColor, #e4e7ed);
  border-radius: 6px;
  background: var(--Main_Bg_BoxColor, #fafafa);
  font-family: "Cascadia Mono", Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.client-error-list {
  display: grid;
  gap: 12px;
}

.client-error-head {
  display: flex;
  gap: 12px;
  font-size: 12px;
  color: var(--Text_SecondaryColor, #909399);
}

.client-error-message {
  margin: 4px 0 6px;
  font-size: 13px;
  word-break: break-word;
}

.log-files {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.log-files-label {
  font-size: 12px;
  color: var(--Text_SecondaryColor, #909399);
}

.log-files-empty {
  font-size: 12px;
  color: var(--Text_SecondaryColor, #909399);
}

.screenshot {
  max-width: 100%;
  border: 1px solid var(--Main_BoxBorderColor, #e4e7ed);
  border-radius: 6px;
}

.screenshot-meta {
  margin-top: 8px;
  font-size: 12px;
  color: var(--Text_SecondaryColor, #909399);
}
</style>
