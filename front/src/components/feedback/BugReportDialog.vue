<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="visible"
    title="提交报错信息"
    width="min(640px, 92vw)"
    class="bug-report-dialog"
    append-to-body
    :close-on-click-modal="false"
    @closed="resetForm"
  >
    <div class="bug-report">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="请描述遇到的功能异常"
        description="补充操作步骤、预期结果与实际结果，能帮助我们更快定位问题。提交内容会通过本机 Core 服务转发，离线时自动排队重试。"
      />

      <div class="report-meta">
        <span class="meta-label">反馈类型</span>
        <el-tag type="danger" effect="plain">功能异常</el-tag>
        <span v-if="pagePath" class="meta-path" :title="pagePath">当前页面：{{ pagePath }}</span>
      </div>

      <el-form label-position="top" class="report-form">
        <el-form-item label="标题" required>
          <el-input
            v-model="form.title"
            maxlength="120"
            show-word-limit
            placeholder="一句话描述问题，例如：打开设置页后页面空白"
          />
        </el-form-item>
        <el-form-item label="详细描述" required>
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="5"
            maxlength="8000"
            show-word-limit
            placeholder="请补充操作步骤、预期结果与实际结果"
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
        <el-form-item label="上传报错日志文件">
          <div class="log-consent">
            <el-switch v-model="form.includeLogs" />
            <span class="log-consent-text">
              提交时会一起上传本机与插件的运行日志（最近 {{ settings.logLineLimit }} 行运行日志与
              最多 {{ settings.errorLineLimit }} 条报错行），以及客户端捕获的异常堆栈。
            </span>
          </div>
        </el-form-item>
      </el-form>

      <p class="redaction-hint">
        上传前会自动脱敏：API Key、Token、密码、邮箱、手机号与用户目录路径会被替换，可在“查看将要上传的信息”中核对。
      </p>
    </div>

    <template #footer>
      <div class="report-actions">
        <el-button :loading="previewing" @click="openPreview">查看将要上传的信息</el-button>
        <el-button @click="visible = false">取消</el-button>
        <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submit">
          提交报错信息
        </el-button>
      </div>
    </template>

    <el-dialog v-model="previewVisible" title="将要上传的信息" width="min(760px, 94vw)" append-to-body>
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
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
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

const props = defineProps<{
  modelValue: boolean;
  pagePath?: string;
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void;
}>();

const { get, post } = useApi();

const settings = ref<FeedbackSettings>({
  uploadUrl: "",
  defaultUploadUrl: "",
  logLineLimit: 500,
  errorLineLimit: 500,
  maxScreenshotMB: 6,
});
const uploadUrl = ref("");
const appVersion = ref("");
const submitting = ref(false);
const previewing = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);
const screenshot = ref<{ name: string; mediaType: string; dataBase64: string } | null>(null);
const previewVisible = ref(false);
const previewText = ref("");
const previewRaw = ref("");
const previewUrl = ref("");
const previewRawOpen = ref<string[]>([]);

const form = reactive({
  title: "",
  description: "",
  contact: "",
  includeLogs: true,
});

const visible = computed({
  get: () => props.modelValue,
  set: (value: boolean) => emit("update:modelValue", value),
});

const canSubmit = computed(() => form.title.trim() !== "" && form.description.trim() !== "" && !submitting.value);

async function loadSettings() {
  try {
    const result = await get<FeedbackSettings>("/api/user-feedback/settings");
    if (result) {
      settings.value = { ...settings.value, ...result };
      uploadUrl.value = result.uploadUrl || result.defaultUploadUrl;
    }
  } catch {}
}

async function loadVersion() {
  try {
    const result = await get<{ version?: string }>("/api/version");
    appVersion.value = String(result?.version || "");
  } catch {
    appVersion.value = "";
  }
}

async function initialize() {
  await Promise.all([loadSettings(), loadVersion()]);
}

async function buildPayload() {
  return buildUserFeedbackPayload(
    {
      category: "bug",
      title: form.title,
      description: form.description,
      contact: form.contact,
      includeLogs: form.includeLogs,
      screenshot: screenshot.value,
    },
    { appVersion: appVersion.value, uploadUrl: uploadUrl.value },
  );
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
    removeScreenshot();
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
    removeScreenshot();
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
    ElMessage.success("报错信息已提交，正在上传");
    clearClientErrors();
    visible.value = false;
  } catch {
    ElMessage.error("提交失败，请稍后重试");
  } finally {
    submitting.value = false;
  }
}

function resetForm() {
  form.title = "";
  form.description = "";
  form.contact = "";
  form.includeLogs = true;
  removeScreenshot();
  previewVisible.value = false;
  previewText.value = "";
  previewRaw.value = "";
  previewUrl.value = "";
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) void initialize();
  },
  { immediate: true },
);
</script>

<style scoped>
:global(.bug-report-dialog .el-dialog__body) {
  max-height: 68vh;
  overflow-y: auto;
}

.bug-report {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.report-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.meta-label {
  color: var(--el-text-color-regular);
  font-size: 13px;
}

.meta-path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.report-form {
  margin-bottom: -18px;
}

.screenshot-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.screenshot-input {
  display: none;
}

.screenshot-name {
  color: var(--el-text-color-primary);
  font-size: 13px;
}

.screenshot-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.log-consent {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.log-consent-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.redaction-hint {
  margin: 0;
  padding: 10px 12px;
  border-radius: 6px;
  background: var(--el-fill-color-lighter);
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.report-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.preview-text,
.preview-raw {
  margin: 12px 0;
  padding: 12px;
  max-height: 320px;
  overflow: auto;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  background: var(--el-fill-color-lighter);
  font-family: "Cascadia Mono", Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
