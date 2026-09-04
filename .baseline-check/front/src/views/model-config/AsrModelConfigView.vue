<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="asr-config-view">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 14px">
      <template #title>语音识别配置与测试任务均通过当前后端执行；API Key 仅保存在当前部署后端。</template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="openCreate">新增 ASR 配置</el-button>
      <el-button :loading="loading" @click="loadAll">刷新</el-button>
    </div>

    <el-table :data="configs" v-loading="loading" border style="width: 100%">
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column prop="apiType" label="Provider" width="130" />
      <el-table-column prop="resourceId" label="模型 / Resource ID" min-width="180" />
      <el-table-column prop="baseUrl" label="Base URL" min-width="220" show-overflow-tooltip />
      <el-table-column label="API Key" width="110">
        <template #default="{ row }">
          <el-tag :type="row.hasApiKey ? 'success' : 'warning'">{{ row.hasApiKey ? '已配置' : '未配置' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="isActive(row) ? 'success' : 'info'">{{ isActive(row) ? '启用' : '未启用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="300" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" :loading="testingId === String(row.id)" @click="testConfig(row)">测试</el-button>
          <el-button v-if="!isActive(row)" size="small" type="primary" plain @click="activate(row)">启用</el-button>
          <el-button size="small" type="danger" plain @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-card class="recognition-card" shadow="never">
      <template #header><strong>音频识别测试</strong></template>
      <el-form label-width="100px">
        <el-form-item label="音频文件">
          <input ref="audioInput" type="file" accept="audio/*,.pcm" @change="onAudioSelected" />
          <span v-if="audioFile" class="file-name">{{ audioFile.name }}</span>
        </el-form-item>
        <el-form-item label="语言">
          <el-select v-model="language" style="width: 220px">
            <el-option label="自动识别" value="" />
            <el-option label="中文普通话" value="zh-CN" />
            <el-option label="英语" value="en-US" />
            <el-option label="日语" value="ja-JP" />
            <el-option label="韩语" value="ko-KR" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="recognizing" :disabled="!audioFile" @click="recognize">上传并识别</el-button>
        </el-form-item>
      </el-form>
      <el-descriptions v-if="taskId" :column="2" border style="margin-top: 12px">
        <el-descriptions-item label="任务 ID">{{ taskId }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ taskStatus || 'processing' }}</el-descriptions-item>
      </el-descriptions>
      <el-input v-if="resultText" v-model="resultText" type="textarea" :rows="6" readonly style="margin-top: 12px" />
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑 ASR 配置' : '新增 ASR 配置'" width="560px">
      <el-form label-width="120px">
        <el-form-item label="名称" required><el-input v-model="form.name" /></el-form-item>
        <el-form-item label="Provider" required>
          <el-select v-model="form.apiType" style="width: 100%" @change="applyProviderDefaults">
            <el-option v-for="p in providers" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="API Key" :required="!editingId">
          <el-input v-model="form.apiKey" type="password" show-password :placeholder="editingId ? '留空则保持现有 API Key' : '请输入 API Key'" />
        </el-form-item>
        <el-form-item label="Base URL"><el-input v-model="form.baseUrl" /></el-form-item>
        <el-form-item label="Resource ID"><el-input v-model="form.resourceId" /></el-form-item>
        <el-form-item label="创建后启用"><el-switch v-model="form.activateAfterSave" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus } from "@element-plus/icons-vue";
import { apiClient, useApi } from "@/composables/useApi";

type AsrProvider = { id: string; name: string; defaultBaseUrl?: string; defaultModel?: string };
type AsrConfig = { id: number | string; name: string; apiType: string; baseUrl?: string; resourceId?: string; isActive?: number | boolean; hasApiKey?: boolean };

const { get, post, put, del } = useApi();
const configs = ref<AsrConfig[]>([]);
const providers = ref<AsrProvider[]>([]);
const loading = ref(false);
const testingId = ref("");
const dialogVisible = ref(false);
const editingId = ref<number | string | null>(null);
const saving = ref(false);
const audioInput = ref<HTMLInputElement | null>(null);
const audioFile = ref<File | null>(null);
const language = ref("");
const recognizing = ref(false);
const taskId = ref("");
const taskStatus = ref("");
const resultText = ref("");
let pollTimer: number | null = null;

const form = reactive({
  name: "",
  apiType: "volcengine",
  apiKey: "",
  baseUrl: "https://openspeech.bytedance.com",
  resourceId: "volc.seedasr.auc",
  activateAfterSave: true,
});

function isActive(row: AsrConfig) {
  return row.isActive === 1 || row.isActive === true;
}

async function loadAll() {
  loading.value = true;
  try {
    const [configData, providerData] = await Promise.all([
      get<AsrConfig[]>("/api/asr/configs"),
      get<AsrProvider[]>("/api/asr/providers"),
    ]);
    configs.value = Array.isArray(configData) ? configData : [];
    providers.value = Array.isArray(providerData) ? providerData : [];
  } finally {
    loading.value = false;
  }
}

function resetForm() {
  Object.assign(form, {
    name: "",
    apiType: providers.value[0]?.id || "volcengine",
    apiKey: "",
    baseUrl: providers.value[0]?.defaultBaseUrl || "https://openspeech.bytedance.com",
    resourceId: providers.value[0]?.defaultModel || "volc.seedasr.auc",
    activateAfterSave: true,
  });
}

function openCreate() {
  editingId.value = null;
  resetForm();
  dialogVisible.value = true;
}

function openEdit(row: AsrConfig) {
  editingId.value = row.id;
  Object.assign(form, {
    name: row.name || "",
    apiType: row.apiType || "volcengine",
    apiKey: "",
    baseUrl: row.baseUrl || "",
    resourceId: row.resourceId || "",
    activateAfterSave: isActive(row),
  });
  dialogVisible.value = true;
}

function applyProviderDefaults(id: string) {
  const p = providers.value.find((item) => item.id === id);
  if (!p) return;
  form.baseUrl = p.defaultBaseUrl || form.baseUrl;
  form.resourceId = p.defaultModel || form.resourceId;
}

async function save() {
  if (!form.name.trim() || !form.apiType.trim()) {
    ElMessage.warning("请填写名称和 Provider");
    return;
  }
  saving.value = true;
  try {
    const payload: Record<string, unknown> = {
      name: form.name.trim(),
      apiType: form.apiType,
      baseUrl: form.baseUrl.trim(),
      resourceId: form.resourceId.trim(),
      isActive: form.activateAfterSave ? 1 : 0,
    };
    if (form.apiKey.trim()) payload.apiKey = form.apiKey.trim();
    if (editingId.value != null) {
      await put(`/api/asr/configs/${editingId.value}`, payload);
      if (form.activateAfterSave) await post(`/api/asr/configs/${editingId.value}/activate`);
    } else {
      const created = await post<any>("/api/asr/configs", payload);
      if (form.activateAfterSave && created?.id != null) await post(`/api/asr/configs/${created.id}/activate`);
    }
    dialogVisible.value = false;
    ElMessage.success("ASR 配置已保存");
    await loadAll();
  } finally {
    saving.value = false;
  }
}

async function activate(row: AsrConfig) {
  await post(`/api/asr/configs/${row.id}/activate`);
  ElMessage.success("ASR 配置已启用");
  await loadAll();
}

async function testConfig(row: AsrConfig) {
  testingId.value = String(row.id);
  try {
    await post(`/api/asr/configs/${row.id}/test`);
    ElMessage.success("ASR 连接测试通过");
  } finally {
    testingId.value = "";
  }
}

async function remove(row: AsrConfig) {
  await ElMessageBox.confirm(`确定删除 ASR 配置“${row.name}”吗？`, "删除配置", { type: "warning" });
  await del(`/api/asr/configs/${row.id}`);
  ElMessage.success("ASR 配置已删除");
  await loadAll();
}

function onAudioSelected(event: Event) {
  const target = event.target as HTMLInputElement;
  audioFile.value = target.files?.[0] || null;
}

async function recognize() {
  if (!audioFile.value) return;
  recognizing.value = true;
  taskId.value = "";
  taskStatus.value = "";
  resultText.value = "";
  stopPolling();
  try {
    const uploadForm = new FormData();
    uploadForm.append("audio", audioFile.value);
    const upload = await apiClient.post("/api/asr/upload", uploadForm, { headers: { "Content-Type": "multipart/form-data" } });
    const audioUrl = String(upload.data?.url || "");
    if (!audioUrl) throw new Error("后端未返回音频地址");

    const submitForm = new FormData();
    submitForm.append("audioUrl", audioUrl);
    if (language.value) submitForm.append("language", language.value);
    const submit = await apiClient.post("/api/asr/submit", submitForm, { headers: { "Content-Type": "multipart/form-data" } });
    taskId.value = String(submit.data?.taskId || "");
    if (!taskId.value) throw new Error("后端未返回 ASR 任务 ID");
    taskStatus.value = "processing";
    await pollResult();
    pollTimer = window.setInterval(pollResult, 2000);
  } catch (error: any) {
    ElMessage.error(error?.message || "ASR 识别失败");
    recognizing.value = false;
  }
}

async function pollResult() {
  if (!taskId.value) return;
  try {
    const data = await get<any>("/api/asr/query", { taskId: taskId.value });
    taskStatus.value = String(data?.status || taskStatus.value || "processing");
    if (data?.result) resultText.value = String(data.result);
    if (["success", "completed", "failed", "error"].includes(taskStatus.value.toLowerCase())) {
      stopPolling();
      recognizing.value = false;
      if (["success", "completed"].includes(taskStatus.value.toLowerCase())) ElMessage.success("ASR 识别完成");
    }
  } catch {
    stopPolling();
    recognizing.value = false;
  }
}

function stopPolling() {
  if (pollTimer != null) {
    window.clearInterval(pollTimer);
    pollTimer = null;
  }
}

onMounted(loadAll);
onBeforeUnmount(stopPolling);
</script>

<style scoped>
.asr-config-view { padding-bottom: 24px; }
.toolbar { display: flex; gap: 8px; margin-bottom: 14px; }
.recognition-card { margin-top: 18px; }
.file-name { margin-left: 10px; color: var(--el-text-color-secondary); font-size: 13px; }
</style>
