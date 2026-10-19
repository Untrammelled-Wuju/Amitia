<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="user-feedback-panel">
    <el-card shadow="never" class="section-card">
      <template #header><span>反馈上报地址</span></template>
      <div class="upload-target">
        <el-input v-model="uploadUrl" placeholder="https://example.com/api/v1/feedbacks">
          <template #prepend>上报到</template>
        </el-input>
        <el-button type="primary" :loading="savingTarget" @click="saveUploadUrl">保存</el-button>
        <el-button :disabled="uploadUrl === settings.defaultUploadUrl" @click="resetUploadUrl">恢复默认</el-button>
      </div>
      <p class="hint">反馈由本机 Core 服务转发到上述地址，离线时会自动排队并在网络恢复后重试。</p>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>报告问题</span></template>
      <el-form label-position="top">
        <el-form-item label="反馈类型">
          <el-radio-group v-model="form.category">
            <el-radio-button value="bug">功能异常</el-radio-button>
            <el-radio-button value="experience">体验问题</el-radio-button>
            <el-radio-button value="feature">功能建议</el-radio-button>
            <el-radio-button value="other">其他</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="form.title" maxlength="120" show-word-limit placeholder="一句话描述问题" />
        </el-form-item>
        <el-form-item label="详细描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="5"
            maxlength="8000"
            show-word-limit
            placeholder="补充操作步骤、预期结果与实际结果"
          />
        </el-form-item>
        <el-form-item label="联系方式（可选）">
          <el-input v-model="form.contact" maxlength="200" placeholder="邮箱或其它可联系到你的方式" />
        </el-form-item>
        <el-form-item label="截图（可选）">
          <div class="screenshot-row">
            <el-button plain @click="pickScreenshot">选择图片</el-button>
            <span v-if="screenshot" class="screenshot-name">
              {{ screenshot.name }}
              <el-button link type="primary" @click="removeScreenshot">移除</el-button>
            </span>
            <span v-else class="screenshot-hint">未选择图片，最多 {{ settings.maxScreenshotMB }} MB</span>
            <input
              ref="fileInputRef"
              class="screenshot-input"
              type="file"
              accept="image/png,image/jpeg,image/webp,image/gif"
              @change="handleScreenshotInput"
            />
          </div>
        </el-form-item>
        <el-form-item label="报错日志">
          <div class="log-consent">
            <el-switch v-model="form.includeLogs" />
            <span class="log-consent-text">
              提交时会一起上传本机与插件的运行日志（最近 {{ settings.logLineLimit }} 行运行日志与
              最多 {{ settings.errorLineLimit }} 条报错行），以及客户端捕获的异常堆栈。
            </span>
          </div>
        </el-form-item>
      </el-form>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="上传前会先做脱敏处理"
        description="API Key、Token、密码、邮箱、手机号与用户目录路径会被自动替换，脱敏结果可在下方预览中核对。"
      />

      <div class="form-actions">
        <el-button :loading="previewing" @click="openPreview">查看将要上传的信息</el-button>
        <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submit">提交反馈</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header>
        <div class="section-header">
          <span>我的提交记录</span>
          <el-button link type="primary" :loading="loadingTickets" @click="loadTickets">刷新</el-button>
        </div>
      </template>
      <el-empty v-if="!tickets.length" description="还没有提交过反馈" :image-size="60" />
      <el-table v-else :data="tickets" size="small">
        <el-table-column label="提交时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="200" show-overflow-tooltip />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">{{ categoryLabel(row.category) }}</template>
        </el-table-column>
        <el-table-column label="上传状态" min-width="180">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)" effect="plain" size="small">{{ statusLabel(row.status) }}</el-tag>
            <span v-if="row.status === 'failed'" class="status-detail" :title="row.lastError">{{ row.lastError }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button
              v-if="row.status !== 'uploaded'"
              link
              type="primary"
              :loading="retrying === row.id"
              @click="retry(row)"
            >
              重试
            </el-button>
            <span v-else class="status-detail">已送达</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="previewVisible" title="将要上传的信息" width="760px">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        :title="`上报地址：${previewUrl || uploadUrl || '未配置'}`"
        description="以下内容与真正提交时发送的数据一致。"
      />
      <pre class="preview-text">{{ previewText }}</pre>
      <el-collapse v-model="previewRawOpen">
        <el-collapse-item name="raw" title="查看原始 JSON">
          <pre class="preview-raw">{{ previewRaw }}</pre>
        </el-collapse-item>
      </el-collapse>
      <template #footer>
        <el-button @click="previewVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "@/composables/useApi";
import { clearClientErrors } from "@/runtime/client-error-tracker";
import { buildUserFeedbackPayload } from "@/runtime/userFeedbackPayload";

interface FeedbackSettings {
  uploadUrl: string;
  defaultUploadUrl: string;
  logLineLimit: number;
  errorLineLimit: number;
  maxScreenshotMB: number;
}

interface FeedbackTicket {
  id: string;
  category: string;
  title: string;
  status: string;
  lastError: string;
  createdAt: string;
  remoteId: string;
}

const { get, post, put } = useApi();

const settings = ref<FeedbackSettings>({
  uploadUrl: "",
  defaultUploadUrl: "",
  logLineLimit: 500,
  errorLineLimit: 500,
  maxScreenshotMB: 6,
});
const uploadUrl = ref("");
const savingTarget = ref(false);
const submitting = ref(false);
const previewing = ref(false);
const loadingTickets = ref(false);
const retrying = ref("");
const tickets = ref<FeedbackTicket[]>([]);
const appVersion = ref("");
const fileInputRef = ref<HTMLInputElement | null>(null);
const screenshot = ref<{ name: string; mediaType: string; dataBase64: string } | null>(null);

const previewVisible = ref(false);
const previewText = ref("");
const previewRaw = ref("");
const previewUrl = ref("");
const previewRawOpen = ref<string[]>([]);

const form = reactive({
  category: "bug",
  title: "",
  description: "",
  contact: "",
  includeLogs: true,
});

const canSubmit = computed(() => form.title.trim() !== "" && form.description.trim() !== "" && !submitting.value);

function categoryLabel(value: string) {
  return { bug: "功能异常", experience: "体验问题", feature: "功能建议", other: "其他" }[value] || value || "其他";
}

function statusLabel(value: string) {
  return { pending: "待上传", uploaded: "已上传", failed: "上传失败" }[value] || value;
}

function statusTagType(value: string) {
  if (value === "uploaded") return "success";
  if (value === "failed") return "danger";
  return "warning";
}

function formatTime(value: string) {
  if (!value) return "—";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

async function buildPayload() {
  return buildUserFeedbackPayload(
    {
      category: form.category,
      title: form.title,
      description: form.description,
      contact: form.contact,
      includeLogs: form.includeLogs,
      screenshot: screenshot.value,
    },
    { appVersion: appVersion.value, uploadUrl: uploadUrl.value },
  );
}

async function loadSettings() {
  try {
    const result = await get<FeedbackSettings>("/api/user-feedback/settings");
    if (result) {
      settings.value = { ...settings.value, ...result };
      if (!uploadUrl.value) uploadUrl.value = result.uploadUrl || result.defaultUploadUrl;
    }
  } catch {
    // Keep the editable field usable even when the Core is unreachable.
  }
}

async function loadVersion() {
  try {
    const result = await get<{ version?: string }>("/api/version");
    appVersion.value = String(result?.version || "");
  } catch {
    appVersion.value = "";
  }
}

async function loadTickets() {
  loadingTickets.value = true;
  try {
    const result = await get<{ tickets?: FeedbackTicket[] }>("/api/user-feedback/tickets", { limit: 20 });
    tickets.value = Array.isArray(result?.tickets) ? result.tickets : [];
  } catch {
    tickets.value = [];
  } finally {
    loadingTickets.value = false;
  }
}

async function saveUploadUrl() {
  savingTarget.value = true;
  try {
    const result = await put<FeedbackSettings>("/api/user-feedback/settings", { uploadUrl: uploadUrl.value.trim() });
    if (result) settings.value = { ...settings.value, ...result };
    uploadUrl.value = settings.value.uploadUrl;
    ElMessage.success("上报地址已保存");
  } catch {
    ElMessage.error("保存上报地址失败");
  } finally {
    savingTarget.value = false;
  }
}

function resetUploadUrl() {
  uploadUrl.value = settings.value.defaultUploadUrl;
}

function pickScreenshot() {
  fileInputRef.value?.click();
}

function removeScreenshot() {
  screenshot.value = null;
  if (fileInputRef.value) fileInputRef.value.value = "";
}

function handleScreenshotInput(event: Event) {
  const input = event.target as HTMLInputElement;
  const raw = input.files?.[0];
  if (!raw) return;
  const maxBytes = settings.value.maxScreenshotMB * 1024 * 1024;
  if (raw.size > maxBytes) {
    ElMessage.error(`截图不能超过 ${settings.value.maxScreenshotMB} MB`);
    screenshot.value = null;
    input.value = "";
    return;
  }
  const reader = new FileReader();
  reader.onload = () => {
    const result = String(reader.result || "");
    const comma = result.indexOf(",");
    screenshot.value = {
      name: raw.name || "screenshot",
      mediaType: raw.type || "image/png",
      dataBase64: comma >= 0 ? result.slice(comma + 1) : result,
    };
  };
  reader.onerror = () => {
    screenshot.value = null;
    input.value = "";
    ElMessage.error("读取截图失败");
  };
  reader.readAsDataURL(raw);
}

async function openPreview() {
  previewing.value = true;
  try {
    const payload = await buildPayload();
    const result = await post<{ text?: string; payload?: unknown; uploadUrl?: string }>(
      "/api/user-feedback/diagnostics/preview",
      payload,
    );
    previewText.value = String(result?.text || "");
    previewRaw.value = JSON.stringify(result?.payload ?? {}, null, 2);
    previewUrl.value = String(result?.uploadUrl || payload.uploadUrl || "");
    previewRawOpen.value = [];
    previewVisible.value = true;
  } catch {
    ElMessage.error("生成上传预览失败");
  } finally {
    previewing.value = false;
  }
}

async function submit() {
  if (!canSubmit.value) {
    ElMessage.warning("请填写标题与详细描述");
    return;
  }
  submitting.value = true;
  try {
    const payload = await buildPayload();
    await post("/api/user-feedback/tickets", payload);
    ElMessage.success("反馈已提交，正在上传");
    form.title = "";
    form.description = "";
    form.contact = "";
    removeScreenshot();
    clearClientErrors();
    await loadTickets();
    window.setTimeout(() => { void loadTickets(); }, 1500);
  } catch {
    ElMessage.error("提交失败，请稍后重试");
  } finally {
    submitting.value = false;
  }
}

async function retry(row: FeedbackTicket) {
  retrying.value = row.id;
  try {
    await post(`/api/user-feedback/tickets/${encodeURIComponent(row.id)}/retry`);
    await loadTickets();
  } catch {
    ElMessage.error("重试上传失败");
  } finally {
    retrying.value = "";
  }
}

onMounted(async () => {
  await Promise.all([loadSettings(), loadVersion()]);
  await loadTickets();
});
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.upload-target {
  display: flex;
  align-items: center;
  gap: 8px;
}

.hint {
  margin: 8px 0 0;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.log-consent {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.screenshot-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.screenshot-input {
  display: none;
}

.screenshot-name {
  color: var(--ac-color-text);
  font-size: 13px;
}

.screenshot-hint {
  color: var(--ac-color-text-muted);
  font-size: 12px;
}

.log-consent-text {
  color: var(--ac-color-text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}

.status-detail {
  margin-left: 8px;
  color: var(--ac-color-text-muted);
  font-size: 12px;
}

.preview-text,
.preview-raw {
  margin: 12px 0;
  padding: 12px;
  max-height: 320px;
  overflow: auto;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 6px;
  background: var(--ac-color-surface-muted, rgba(0, 0, 0, 0.02));
  font-family: "Cascadia Mono", Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
