<template>
  <div class="update-center">
    <ExtensionPageHeader
      title="扩展更新中心"
      description="Extension Update Center — 检查、下载、安装、回滚扩展更新"
      parent-title="扩展内核中心"
      parent-path="/kernel"
    >
      <template #actions>
        <el-button @click="loadExtensions" :icon="Refresh" :loading="extLoading">刷新扩展</el-button>
      </template>
    </ExtensionPageHeader>

    <div class="update-toolbar">
      <el-select
        v-model="selectedExtension"
        placeholder="选择扩展..."
        filterable
        clearable
        style="width: 360px"
      >
        <el-option
          v-for="ext in extensions"
          :key="ext.extensionId"
          :label="ext.extensionId"
          :value="ext.extensionId"
        />
      </el-select>
      <el-button
        type="primary"
        :loading="checkLoading"
        :disabled="!selectedExtension"
        @click="doCheckUpdate"
      >检查更新</el-button>
    </div>

    <el-card v-if="updateMeta" class="update-info-card">
      <template #header>
        <div class="card-header">
          <span>可用更新</span>
          <el-tag type="success" size="small">有可用更新</el-tag>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="版本">{{ updateMeta.version }}</el-descriptions-item>
        <el-descriptions-item label="发布通道">{{ updateMeta.releaseChannel || '-' }}</el-descriptions-item>
        <el-descriptions-item label="发布者">{{ updateMeta.publisherId }}</el-descriptions-item>
        <el-descriptions-item label="包大小">{{ formatSize(updateMeta.packageSize) }}</el-descriptions-item>
        <el-descriptions-item label="发布时间">{{ formatDate(updateMeta.publishedAt) }}</el-descriptions-item>
        <el-descriptions-item label="Manifest版本">{{ updateMeta.manifestVersion }}</el-descriptions-item>
        <el-descriptions-item label="包SHA256" :span="2">{{ updateMeta.packageSha256 }}</el-descriptions-item>
        <el-descriptions-item label="最低宿主版本">{{ updateMeta.minimumHostVersion || '-' }}</el-descriptions-item>
        <el-descriptions-item label="最高宿主版本">{{ updateMeta.maximumHostVersion || '-' }}</el-descriptions-item>
        <el-descriptions-item label="支持平台" :span="2">{{ updateMeta.supportedPlatforms?.join(', ') || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="action-buttons">
        <el-button
          type="primary"
          :loading="actionLoading"
          :disabled="!selectedExtension"
          @click="doDownload"
        >下载</el-button>
        <el-button
          type="success"
          :loading="actionLoading"
          :disabled="!selectedExtension || !currentOperationId"
          @click="doInstall"
        >安装</el-button>
        <el-button
          type="warning"
          :loading="actionLoading"
          :disabled="!selectedExtension || !currentOperationId"
          @click="doCancel"
        >取消</el-button>
        <el-button
          :loading="actionLoading"
          :disabled="!selectedExtension || !currentOperationId"
          @click="doRetry"
        >重试</el-button>
        <el-button
          type="danger"
          :loading="actionLoading"
          :disabled="!selectedExtension || !currentOperationId"
          @click="doRollback"
        >回滚</el-button>
      </div>
    </el-card>

    <el-card v-if="currentOperation" class="current-op-card">
      <template #header>
        <div class="card-header">
          <span>当前操作</span>
          <el-tag :type="opStatusTagType(currentOperation.status)" size="small">{{ currentOperation.status }}</el-tag>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="操作ID">{{ currentOperation.operationId }}</el-descriptions-item>
        <el-descriptions-item label="扩展ID">{{ currentOperation.extensionId }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ currentOperation.version || '-' }}</el-descriptions-item>
        <el-descriptions-item label="状态">{{ currentOperation.status }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ formatDate(currentOperation.createdAt) }}</el-descriptions-item>
        <el-descriptions-item label="更新时间">{{ formatDate(currentOperation.updatedAt) }}</el-descriptions-item>
        <el-descriptions-item v-if="currentOperation.error" label="错误" :span="2">{{ currentOperation.error }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="history-card">
      <template #header>
        <div class="card-header">
          <span>更新历史</span>
          <el-button size="small" @click="loadStepsForCurrent" :disabled="!currentOperationId">查看步骤</el-button>
        </div>
      </template>
      <el-table
        :data="history"
        border
        size="small"
        v-loading="actionLoading"
        highlight-current-row
        @current-change="onHistorySelect"
      >
        <el-table-column prop="operationId" label="操作ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="extensionId" label="扩展ID" min-width="160" show-overflow-tooltip />
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="opStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column label="创建时间" width="180">
          <template #default="{ row }">{{ formatDate(row.createdAt) }}</template>
        </el-table-column>
        <el-table-column prop="error" label="错误" min-width="160" show-overflow-tooltip />
      </el-table>
      <el-empty v-if="history.length === 0" description="暂无更新操作记录" />
    </el-card>

    <el-card v-if="steps.length > 0" class="steps-card">
      <template #header>
        <div class="card-header">
          <span>操作步骤</span>
          <el-button size="small" @click="loadStepsForCurrent" :loading="stepsLoading">刷新步骤</el-button>
        </div>
      </template>
      <el-timeline>
        <el-timeline-item
          v-for="step in steps"
          :key="step.stepId"
          :timestamp="formatDate(step.startedAt || step.endedAt)"
          :type="stepTimelineType(step.status)"
          :hollow="step.status === 'pending'"
        >
          <div class="step-item">
            <span class="step-name">{{ step.name }}</span>
            <el-tag :type="stepTagType(step.status)" size="small">{{ step.status }}</el-tag>
          </div>
          <div class="step-id">stepId: {{ step.stepId }}</div>
          <div v-if="step.error" class="step-error">{{ step.error }}</div>
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import {
  checkUpdates,
  downloadUpdate,
  installUpdate,
  cancelUpdate,
  retryUpdate,
  rollbackUpdate,
  getUpdateOperation,
  getUpdateOperationSteps,
} from "@/api/desktop";
import type {
  ExtensionUpdateMeta,
  UpdateOperationInfo,
  UpdateOperationStepInfo,
} from "@/api/desktop";
import { listExtensions, type KernelExtension } from "./api";

const extLoading = ref(false);
const checkLoading = ref(false);
const actionLoading = ref(false);
const stepsLoading = ref(false);

const extensions = ref<KernelExtension[]>([]);
const selectedExtension = ref("");
const updateMeta = ref<ExtensionUpdateMeta | null>(null);
const currentOperation = ref<UpdateOperationInfo | null>(null);
const currentOperationId = ref("");
const history = ref<UpdateOperationInfo[]>([]);
const steps = ref<UpdateOperationStepInfo[]>([]);

function opStatusTagType(status: string): "success" | "info" | "warning" | "danger" {
  if (status === "completed" || status === "installed" || status === "success") return "success";
  if (status === "pending" || status === "downloading" || status === "installing" || status === "queued") return "warning";
  if (status === "failed" || status === "cancelled") return "danger";
  return "info";
}

function stepTagType(status: string): "success" | "info" | "warning" | "danger" {
  if (status === "completed" || status === "success") return "success";
  if (status === "running" || status === "pending") return "warning";
  if (status === "failed" || status === "skipped") return "danger";
  return "info";
}

function stepTimelineType(status: string): "primary" | "success" | "warning" | "danger" {
  if (status === "completed" || status === "success") return "success";
  if (status === "running" || status === "pending") return "warning";
  if (status === "failed" || status === "skipped") return "danger";
  return "primary";
}

function formatDate(s?: string): string {
  if (!s) return "";
  try {
    return new Date(s).toLocaleString("zh-CN");
  } catch {
    return s;
  }
}

function formatSize(bytes: number): string {
  if (!bytes && bytes !== 0) return "-";
  if (bytes < 1024) return bytes + " B";
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + " MB";
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + " GB";
}

async function loadExtensions() {
  extLoading.value = true;
  try {
    const data = await listExtensions();
    extensions.value = data.extensions || [];
  } catch (e: any) {
    ElMessage.error("加载扩展列表失败: " + (e?.message || e));
  } finally {
    extLoading.value = false;
  }
}

async function doCheckUpdate() {
  if (!selectedExtension.value) {
    ElMessage.warning("请先选择扩展");
    return;
  }
  checkLoading.value = true;
  try {
    const result = await checkUpdates(selectedExtension.value);
    if (result.available && result.update) {
      updateMeta.value = result.update;
      ElMessage.success("发现可用更新: " + result.update.version);
    } else {
      updateMeta.value = null;
      ElMessage.info("当前已是最新版本");
    }
  } catch (e: any) {
    ElMessage.error("检查更新失败: " + (e?.message || e));
  } finally {
    checkLoading.value = false;
  }
}

function pushHistory(op: UpdateOperationInfo) {
  currentOperation.value = op;
  currentOperationId.value = op.operationId;
  const idx = history.value.findIndex((h: UpdateOperationInfo) => h.operationId === op.operationId);
  if (idx >= 0) {
    history.value[idx] = op;
  } else {
    history.value.unshift(op);
  }
}

async function doDownload() {
  if (!selectedExtension.value || !updateMeta.value) {
    ElMessage.warning("无可用更新");
    return;
  }
  actionLoading.value = true;
  try {
    const op = await downloadUpdate(selectedExtension.value, updateMeta.value.version);
    pushHistory(op);
    ElMessage.success("下载已开始: " + op.operationId);
  } catch (e: any) {
    ElMessage.error("下载失败: " + (e?.message || e));
  } finally {
    actionLoading.value = false;
  }
}

async function doInstall() {
  if (!selectedExtension.value || !currentOperationId.value) {
    ElMessage.warning("无可用操作");
    return;
  }
  actionLoading.value = true;
  try {
    const op = await installUpdate(selectedExtension.value, currentOperationId.value);
    pushHistory(op);
    ElMessage.success("安装已触发");
  } catch (e: any) {
    ElMessage.error("安装失败: " + (e?.message || e));
  } finally {
    actionLoading.value = false;
  }
}

async function doCancel() {
  if (!selectedExtension.value || !currentOperationId.value) return;
  actionLoading.value = true;
  try {
    const op = await cancelUpdate(selectedExtension.value, currentOperationId.value);
    pushHistory(op);
    ElMessage.success("操作已取消");
  } catch (e: any) {
    ElMessage.error("取消失败: " + (e?.message || e));
  } finally {
    actionLoading.value = false;
  }
}

async function doRetry() {
  if (!selectedExtension.value || !currentOperationId.value) return;
  actionLoading.value = true;
  try {
    const op = await retryUpdate(selectedExtension.value, currentOperationId.value);
    pushHistory(op);
    ElMessage.success("已重试");
  } catch (e: any) {
    ElMessage.error("重试失败: " + (e?.message || e));
  } finally {
    actionLoading.value = false;
  }
}

async function doRollback() {
  if (!selectedExtension.value || !currentOperationId.value) return;
  try {
    await ElMessageBox.confirm("确定要回滚此更新操作吗？", "回滚确认", { type: "warning" });
  } catch (e: any) {
    return;
  }
  actionLoading.value = true;
  try {
    const op = await rollbackUpdate(selectedExtension.value, currentOperationId.value);
    pushHistory(op);
    ElMessage.success("已回滚");
  } catch (e: any) {
    ElMessage.error("回滚失败: " + (e?.message || e));
  } finally {
    actionLoading.value = false;
  }
}

function onHistorySelect(row: UpdateOperationInfo | null) {
  if (row) {
    currentOperation.value = row;
    currentOperationId.value = row.operationId;
    steps.value = [];
  }
}

async function loadStepsForCurrent() {
  if (!currentOperationId.value) {
    ElMessage.warning("请先选择操作记录");
    return;
  }
  stepsLoading.value = true;
  try {
    const op = await getUpdateOperation(currentOperationId.value);
    currentOperation.value = op;
    const s = await getUpdateOperationSteps(currentOperationId.value);
    steps.value = Array.isArray(s) ? s : [];
  } catch (e: any) {
    ElMessage.error("加载步骤失败: " + (e?.message || e));
  } finally {
    stepsLoading.value = false;
  }
}

onMounted(async () => {
  await loadExtensions();
});
</script>

<style scoped>
.update-center {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.update-toolbar {
  display: flex;
  gap: 12px;
  margin: 20px 0;
  align-items: center;
}

.update-info-card,
.current-op-card,
.history-card,
.steps-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.action-buttons {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}

.step-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.step-name {
  font-weight: 600;
}

.step-id {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}

.step-error {
  font-size: 12px;
  color: var(--el-color-danger);
  margin-top: 4px;
}
</style>
