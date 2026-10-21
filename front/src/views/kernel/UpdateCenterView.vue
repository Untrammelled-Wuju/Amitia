<template>
  <div class="update-center">
    <ExtensionPageHeader
      title="扩展更新中心"
      description="使用 Kernel 包事务完成检查、下载、预览、确认、更新和真实回滚"
      parent-title="扩展内核中心"
      parent-path="/kernel"
    >
      <template #actions>
        <el-button :icon="Refresh" :loading="extLoading" @click="loadExtensions">刷新扩展</el-button>
      </template>
    </ExtensionPageHeader>

    <div class="update-toolbar">
      <el-select
        v-model="selectedExtension"
        placeholder="选择扩展"
        filterable
        clearable
        style="width: 380px"
        @change="onExtensionChange"
      >
        <el-option
          v-for="ext in extensions"
          :key="ext.extensionId"
          :label="`${ext.extensionId} · v${ext.version}`"
          :value="ext.extensionId"
        />
      </el-select>
      <el-button type="primary" :loading="checkLoading" :disabled="!selectedExtension" @click="doCheckUpdate">
        检查更新
      </el-button>
      <el-button :loading="versionsLoading" :disabled="!selectedExtension" @click="loadVersions">
        版本历史
      </el-button>
    </div>

    <el-card v-if="updateMeta" class="update-info-card">
      <template #header>
        <div class="card-header">
          <span>可用更新</span>
          <el-tag type="success" size="small">已通过签名索引校验</el-tag>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="目标版本">{{ updateMeta.version }}</el-descriptions-item>
        <el-descriptions-item label="发布通道">{{ updateMeta.releaseChannel || "-" }}</el-descriptions-item>
        <el-descriptions-item label="发布者">{{ updateMeta.publisherId }}</el-descriptions-item>
        <el-descriptions-item label="包大小">{{ formatSize(updateMeta.packageSize) }}</el-descriptions-item>
        <el-descriptions-item label="发布时间">{{ formatDate(updateMeta.publishedAt) }}</el-descriptions-item>
        <el-descriptions-item label="包格式版本">{{ updateMeta.manifestVersion }}</el-descriptions-item>
        <el-descriptions-item label="SHA-256" :span="2">{{ updateMeta.packageSha256 }}</el-descriptions-item>
        <el-descriptions-item label="宿主版本范围" :span="2">
          {{ updateMeta.minimumHostVersion || "-" }} 至 {{ updateMeta.maximumHostVersion || "-" }}
        </el-descriptions-item>
      </el-descriptions>
      <div class="action-buttons">
        <el-button type="primary" :loading="downloadLoading" :disabled="!!preview" @click="doDownload">
          下载并预览
        </el-button>
        <el-button
          v-if="download && isDownloadActive"
          type="warning"
          :loading="downloadLoading"
          @click="doCancelDownload"
        >
          取消下载
        </el-button>
        <el-button
          v-if="download && download.status === 'failed'"
          :loading="downloadLoading"
          @click="doRetryDownload"
        >
          重试下载
        </el-button>
      </div>
      <el-progress
        v-if="download"
        class="download-progress"
        :percentage="downloadPercent"
        :status="download.status === 'failed' ? 'exception' : undefined"
        :stroke-width="6"
      />
      <div v-if="download" class="download-state">
        {{ download.status }} · {{ formatSize(download.downloadedSize) }} / {{ formatSize(download.expectedSize) }}
        <span v-if="download.error"> · {{ download.error }}</span>
      </div>
    </el-card>

    <el-card v-if="preview" class="update-info-card">
      <template #header>
        <div class="card-header">
          <span>Kernel 包预览</span>
          <el-tag :type="preview.compatible ? 'success' : 'danger'" size="small">
            {{ preview.compatible ? "可更新" : "不可更新" }}
          </el-tag>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="扩展">{{ preview.name }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ preview.version }}</el-descriptions-item>
        <el-descriptions-item label="当前版本">{{ preview.currentVersion || "-" }}</el-descriptions-item>
        <el-descriptions-item label="签名">{{ preview.signature?.status || "未提供" }}</el-descriptions-item>
        <el-descriptions-item label="兼容性">{{ preview.compatible ? "通过" : "失败" }}</el-descriptions-item>
        <el-descriptions-item label="管理目标">{{ preview.managementTarget || "扩展中心" }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="preview.errors?.length" class="preview-errors">
        {{ preview.errors.join("；") }}
      </div>
      <div v-if="preview.newPermissions?.length" class="preview-section">
        <h4>新增权限</h4>
        <el-tag v-for="permission in preview.newPermissions" :key="permission.name" type="warning" class="preview-tag">
          {{ permission.name }}
        </el-tag>
      </div>
      <div v-if="preview.capabilityConfirmations?.length" class="preview-section">
        <h4>必需确认</h4>
        <el-tag v-for="item in preview.capabilityConfirmations" :key="item" class="preview-tag">{{ item }}</el-tag>
      </div>
      <el-checkbox v-if="preview.compatible" v-model="confirmed">
        我已检查签名、权限、迁移和回滚信息，确认执行真实更新
      </el-checkbox>
      <div class="action-buttons">
        <el-button
          type="success"
          :loading="installLoading"
          :disabled="!preview.compatible || !confirmed"
          @click="doInstall"
        >
          确认更新
        </el-button>
      </div>
    </el-card>

    <el-card v-if="operation" class="current-op-card">
      <template #header>
        <div class="card-header">
          <span>Kernel 更新操作</span>
          <el-tag :type="statusTagType(operation.status)" size="small">{{ operation.status }}</el-tag>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="操作ID">{{ operation.id }}</el-descriptions-item>
        <el-descriptions-item label="扩展ID">{{ operation.extensionId }}</el-descriptions-item>
        <el-descriptions-item label="目标版本">{{ operation.targetVersion || "-" }}</el-descriptions-item>
        <el-descriptions-item label="当前步骤">{{ operation.currentStep || "-" }}</el-descriptions-item>
        <el-descriptions-item label="TraceID" :span="2">{{ operation.traceId }}</el-descriptions-item>
        <el-descriptions-item v-if="operation.errorCode" label="错误码" :span="2">
          {{ operation.errorCode }}
        </el-descriptions-item>
      </el-descriptions>
      <el-timeline v-if="operation.steps?.length" class="operation-steps">
        <el-timeline-item
          v-for="step in operation.steps"
          :key="`${step.order}-${step.name}`"
          :timestamp="formatDate(step.startedAt || step.completedAt)"
          :type="stepTimelineType(step.status)"
        >
          <div class="step-item">
            <span>{{ step.name }}</span>
            <el-tag :type="statusTagType(step.status)" size="small">{{ step.status }}</el-tag>
          </div>
          <div v-if="step.errorCode" class="step-error">{{ step.errorCode }}</div>
        </el-timeline-item>
      </el-timeline>
    </el-card>

    <el-card class="history-card">
      <template #header>
        <div class="card-header">
          <span>不可变版本历史</span>
          <el-button size="small" :loading="versionsLoading" :disabled="!selectedExtension" @click="loadVersions">
            刷新
          </el-button>
        </div>
      </template>
      <el-table :data="versions" size="small" border v-loading="versionsLoading">
        <el-table-column prop="version" label="版本" width="120" />
        <el-table-column label="状态" width="130">
          <template #default="{ row }">
            <el-tag :type="row.active ? 'success' : 'info'" size="small">
              {{ row.active ? "当前" : row.versionState }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="generationId" label="Generation" min-width="180" show-overflow-tooltip />
        <el-table-column prop="installedAt" label="安装时间" width="190">
          <template #default="{ row }">{{ formatDate(row.installedAt) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120">
          <template #default="{ row }">
            <el-button
              size="small"
              type="warning"
              :loading="rollbackLoading"
              :disabled="row.active || ['removed', 'blocked', 'corrupted'].includes(String(row.versionState || ''))"
              @click="doRollbackVersion(row)"
            >
              回滚
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!versionsLoading && versions.length === 0" description="暂无版本历史" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import {
  cancelExtensionUpdateDownload,
  checkExtensionUpdate,
  confirmPackageRollback,
  fetchPackageVersions,
  getExtensionUpdateDownload,
  getPackageOperation,
  installExtensionPackage,
  previewExtensionUpdate,
  previewPackageRollback,
  retryExtensionUpdateDownload,
  rollbackExtensionPackage,
  startExtensionUpdateDownload,
} from "@/views/extensions/api";
import type {
  ExtensionUpdateDownload,
  ExtensionUpdateMetadata,
  PackageImportPreview,
  PackageOperation,
  PackageVersion,
} from "@/views/extensions/types";
import { listExtensions, type KernelExtension } from "./api";

const extLoading = ref(false);
const checkLoading = ref(false);
const downloadLoading = ref(false);
const installLoading = ref(false);
const versionsLoading = ref(false);
const rollbackLoading = ref(false);

const extensions = ref<KernelExtension[]>([]);
const selectedExtension = ref("");
const updateMeta = ref<ExtensionUpdateMetadata | null>(null);
const download = ref<ExtensionUpdateDownload | null>(null);
const preview = ref<PackageImportPreview | null>(null);
const confirmed = ref(false);
const operation = ref<PackageOperation | null>(null);
const versions = ref<PackageVersion[]>([]);

const isDownloadActive = computed(() =>
  download.value ? ["pending", "downloading", "paused"].includes(download.value.status) : false
);
const downloadPercent = computed(() => {
  const item = download.value;
  if (!item || item.expectedSize <= 0) return item?.status === "completed" ? 100 : 0;
  return Math.min(100, Math.round((item.downloadedSize / item.expectedSize) * 100));
});

function statusTagType(status: string): "success" | "info" | "warning" | "danger" {
  if (["completed", "success"].includes(status)) return "success";
  if (["failed", "cancelled", "requires_recovery"].includes(status)) return "danger";
  if (["created", "running", "pending", "downloading"].includes(status)) return "warning";
  return "info";
}

function stepTimelineType(status: string): "primary" | "success" | "warning" | "danger" {
  if (["completed", "success"].includes(status)) return "success";
  if (["failed", "requires_recovery"].includes(status)) return "danger";
  if (["running", "pending"].includes(status)) return "warning";
  return "primary";
}

function formatDate(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

function formatSize(bytes: number): string {
  if (!bytes && bytes !== 0) return "-";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function resetUpdateState() {
  updateMeta.value = null;
  download.value = null;
  preview.value = null;
  confirmed.value = false;
  operation.value = null;
}

async function loadExtensions() {
  extLoading.value = true;
  try {
    const data = await listExtensions();
    extensions.value = data.extensions || [];
    if (!selectedExtension.value && extensions.value.length) {
      selectedExtension.value = extensions.value[0]?.extensionId || "";
      await loadVersions();
    }
  } catch (e: any) {
    ElMessage.error(`加载扩展失败: ${e?.message || e}`);
  } finally {
    extLoading.value = false;
  }
}

async function onExtensionChange() {
  resetUpdateState();
  await loadVersions();
}

async function doCheckUpdate() {
  if (!selectedExtension.value) return;
  checkLoading.value = true;
  resetUpdateState();
  try {
    const result = await checkExtensionUpdate(selectedExtension.value);
    if (!result.available || !result.update) {
      ElMessage.info("当前已是最新版本");
      return;
    }
    updateMeta.value = result.update;
    ElMessage.success(`发现可用更新 v${result.update.version}`);
  } catch (e: any) {
    ElMessage.error(`检查更新失败: ${e?.message || e}`);
  } finally {
    checkLoading.value = false;
  }
}

async function doDownload() {
  if (!selectedExtension.value || !updateMeta.value) return;
  downloadLoading.value = true;
  try {
    download.value = await startExtensionUpdateDownload(selectedExtension.value, updateMeta.value.version);
    await waitForDownload();
    const result = await previewExtensionUpdate(
      selectedExtension.value,
      download.value.downloadId,
      "global",
      "",
    );
    preview.value = result.preview;
    ElMessage.success("下载完成，已进入 Kernel 包预览");
  } catch (e: any) {
    ElMessage.error(`下载或预览失败: ${e?.message || e}`);
  } finally {
    downloadLoading.value = false;
  }
}

async function waitForDownload() {
  if (!download.value) return;
  const downloadId = download.value.downloadId;
  for (let attempt = 0; attempt < 720; attempt += 1) {
    const payload = await getExtensionUpdateDownload(downloadId);
    download.value = payload.download;
    if (payload.download.status === "completed") return;
    if (["failed", "cancelled"].includes(payload.download.status)) {
      throw new Error(payload.download.error || `下载状态 ${payload.download.status}`);
    }
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("扩展包下载等待超时");
}

async function doCancelDownload() {
  if (!download.value) return;
  downloadLoading.value = true;
  try {
    await cancelExtensionUpdateDownload(download.value.downloadId);
    download.value = { ...download.value, status: "cancelled" };
  } catch (e: any) {
    ElMessage.error(`取消下载失败: ${e?.message || e}`);
  } finally {
    downloadLoading.value = false;
  }
}

async function doRetryDownload() {
  if (!download.value) return;
  downloadLoading.value = true;
  try {
    download.value = await retryExtensionUpdateDownload(download.value.downloadId);
    await waitForDownload();
    const result = await previewExtensionUpdate(
      selectedExtension.value,
      download.value.downloadId,
      "global",
      "",
    );
    preview.value = result.preview;
  } catch (e: any) {
    ElMessage.error(`重试下载失败: ${e?.message || e}`);
  } finally {
    downloadLoading.value = false;
  }
}

async function doInstall() {
  if (!selectedExtension.value || !preview.value || !confirmed.value) return;
  installLoading.value = true;
  try {
    const result = await installExtensionPackage(
      preview.value,
      {
        unsigned: true,
        scripts: true,
        capabilities: [...(preview.value.highRiskCapabilities || [])],
        versionChange: true,
        signerChange: true,
        configMigration: true,
      },
      selectedExtension.value,
    );
    operation.value = await waitForOperation(result.operationId);
    ElMessage.success(`扩展已更新到 v${result.version}`);
    await loadExtensions();
    await loadVersions();
  } catch (e: any) {
    ElMessage.error(`更新失败: ${e?.message || e}`);
  } finally {
    installLoading.value = false;
  }
}

async function waitForOperation(operationId: string) {
  let latest: PackageOperation | null = null;
  for (let attempt = 0; attempt < 240; attempt += 1) {
    latest = await getPackageOperation(operationId);
    operation.value = latest;
    if (latest.status === "completed") return latest;
    if (["failed", "requires_recovery"].includes(latest.status)) {
      throw new Error(latest.errorCode || `操作状态 ${latest.status}`);
    }
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("Kernel 更新操作等待超时");
}

async function loadVersions() {
  if (!selectedExtension.value) return;
  versionsLoading.value = true;
  try {
    versions.value = await fetchPackageVersions(selectedExtension.value, "global", "");
  } catch (e: any) {
    ElMessage.error(`加载版本历史失败: ${e?.message || e}`);
  } finally {
    versionsLoading.value = false;
  }
}

async function doRollbackVersion(row: PackageVersion) {
  if (!selectedExtension.value) return;
  rollbackLoading.value = true;
  try {
    const rollbackPreview = await previewPackageRollback(
      selectedExtension.value,
      row.version,
      "global",
      "",
    );
    if (!rollbackPreview.installable) throw new Error("该版本当前不可回滚");
    if (rollbackPreview.manualRequired) {
      throw new Error(rollbackPreview.manualReason || "该回滚需要人工恢复");
    }
    const dependentText = rollbackPreview.dependents.length
      ? `\n依赖项目：${rollbackPreview.dependents.join("、")}`
      : "";
    await ElMessageBox.confirm(
      `将从 v${rollbackPreview.currentVersion} 回滚到 v${rollbackPreview.targetVersion}，并恢复文件、配置、权限和迁移状态。${dependentText}`,
      "真实回滚确认",
      { type: "warning", confirmButtonText: "确认回滚" },
    );
    const confirmations = Object.fromEntries(
      rollbackPreview.requiredConfirmations.map((item) => [item, true]),
    );
    const confirmation = await confirmPackageRollback(
      selectedExtension.value,
      row.version,
      rollbackPreview,
      confirmations,
      "global",
      "",
    );
    const result = await rollbackExtensionPackage(
      selectedExtension.value,
      row.version,
      "global",
      "",
      confirmation.confirmationToken,
    );
    operation.value = await waitForOperation(result.operationId);
    ElMessage.success(`已回滚到 v${row.version}`);
    await loadExtensions();
    await loadVersions();
  } catch (e: any) {
    if (e !== "cancel" && e !== "close") {
      ElMessage.error(`回滚失败: ${e?.message || e}`);
    }
  } finally {
    rollbackLoading.value = false;
  }
}

onMounted(loadExtensions);
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
.history-card {
  margin-bottom: 20px;
}

.card-header,
.step-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.action-buttons {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}

.download-progress {
  margin-top: 16px;
}

.download-state,
.preview-errors,
.step-error {
  margin-top: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.preview-errors,
.step-error {
  color: var(--el-color-danger);
}

.preview-section {
  margin: 16px 0;
}

.preview-tag {
  margin: 0 8px 8px 0;
}

.operation-steps {
  margin-top: 20px;
}
</style>
