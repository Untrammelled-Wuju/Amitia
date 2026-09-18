<template>
  <section class="pet-plugin-manager">
    <div class="section-head">
      <div>
        <h2>桌宠插件</h2>
        <p>管理桌宠扩展插件的安装、更新、启用与卸载</p>
      </div>
      <div class="section-actions">
        <el-button :icon="Plus" type="primary" @click="openInstallDialog()">安装插件</el-button>
        <el-button :icon="Refresh" :loading="loading" @click="loadPlugins">刷新</el-button>
      </div>
    </div>

    <section class="summary-grid">
      <div class="summary-card">
        <span>插件总数</span>
        <strong>{{ plugins.length }}</strong>
      </div>
      <div class="summary-card">
        <span>已启用</span>
        <strong>{{ enabledCount }}</strong>
      </div>
      <div class="summary-card">
        <span>运行端支持</span>
        <strong>桌面 / Android</strong>
      </div>
    </section>

    <el-card shadow="never" class="list-card">
      <el-empty
        v-if="!loading && plugins.length === 0"
        description="暂无已安装的桌宠插件"
        :image-size="80"
      >
        <el-button type="primary" :icon="Plus" @click="openInstallDialog()">安装插件</el-button>
      </el-empty>

      <div v-else v-loading="loading" class="plugin-list">
        <article v-for="plugin in plugins" :key="plugin.pluginId" class="plugin-item">
          <div class="plugin-mark">
            <el-icon><Box /></el-icon>
          </div>
          <div class="plugin-main">
            <div class="plugin-title">
              <strong>{{ plugin.name || plugin.pluginId }}</strong>
              <el-tag :type="plugin.enabled ? 'success' : 'info'" size="small">
                {{ plugin.enabled ? "已启用" : "已禁用" }}
              </el-tag>
            </div>
            <p>{{ plugin.description || "暂无描述" }}</p>
            <div class="plugin-meta">
              <span>v{{ plugin.version }}</span>
              <span>{{ plugin.extensionId }}</span>
              <span>{{ plugin.pluginId }}</span>
            </div>
          </div>
          <div class="plugin-actions">
            <el-button size="small" @click="openInstallDialog(plugin)">更新</el-button>
            <el-button
              size="small"
              :type="plugin.enabled ? 'warning' : 'success'"
              plain
              :loading="busy === plugin.extensionId"
              @click="togglePlugin(plugin)"
            >
              {{ plugin.enabled ? "禁用" : "启用" }}
            </el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :loading="busy === plugin.extensionId"
              @click="uninstallPlugin(plugin)"
            >
              卸载
            </el-button>
          </div>
        </article>
      </div>
    </el-card>

    <el-dialog
      v-model="installDialogVisible"
      :title="installMode === 'update' ? `更新 ${updateTarget?.name || '桌宠插件'}` : '安装桌宠插件'"
      width="680px"
      destroy-on-close
      @closed="resetInstallState"
    >
      <div
        class="package-drop-zone"
        :class="{ 'has-file': !!installFile }"
        @dragover.prevent
        @drop.prevent="onPackageDrop"
      >
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <template v-if="installFile">
          <strong>{{ installFile.name }}</strong>
          <span>{{ formatBytes(installFile.size) }} · {{ previewLoading ? `正在检查 ${uploadProgress}%` : "已选择" }}</span>
        </template>
        <template v-else>
          <strong>选择或拖入 .petx 桌宠插件</strong>
          <span>插件将安装到当前设备并支持桌面端与 Android 端</span>
        </template>
        <el-button :loading="previewLoading" @click="choosePackage">
          {{ installFile ? "重新选择" : "选择文件" }}
        </el-button>
        <input ref="packageInput" class="sr-only" type="file" accept=".petx" @change="onPackageFile" />
      </div>

      <el-progress
        v-if="previewLoading"
        class="upload-progress"
        :percentage="uploadProgress"
        :show-text="false"
        :stroke-width="4"
      />

      <div v-if="installPreview" class="package-preview">
        <div class="preview-head">
          <div>
            <span class="preview-kicker">安装预览</span>
            <h3>{{ installPreview.name }}</h3>
            <p>{{ installPreview.description || installPreview.id }}</p>
          </div>
          <el-tag :type="previewIsPetPlugin ? 'success' : 'danger'">
            {{ previewIsPetPlugin ? "桌宠插件" : "非桌宠插件" }}
          </el-tag>
        </div>

        <div class="preview-facts">
          <div><span>版本</span><strong>{{ installPreview.version }}</strong></div>
          <div><span>签名</span><strong>{{ signatureLabel(installPreview.signature?.status) }}</strong></div>
          <div><span>兼容性</span><strong>{{ installPreview.compatible ? "通过" : "不兼容" }}</strong></div>
          <div><span>目标</span><strong>{{ installPreview.managementTarget || "未知" }}</strong></div>
        </div>

        <el-alert
          v-if="!previewIsPetPlugin"
          title="该扩展不是桌宠插件，无法从桌宠中心安装。"
          type="error"
          show-icon
          :closable="false"
        />
        <el-alert
          v-else-if="installPreview.errors?.length"
          :title="installPreview.errors.join('；')"
          type="error"
          show-icon
          :closable="false"
        />
        <el-alert
          v-else-if="previewMatchesInstalledVersion"
          :title="`版本 ${installPreview.version} 已安装，无需重复安装。`"
          type="info"
          show-icon
          :closable="false"
        />

        <div v-if="installPreview.highRiskCapabilities?.length" class="preview-block">
          <span class="preview-label">高风险项</span>
          <div class="chip-row">
            <span
              v-for="capability in installPreview.highRiskCapabilities"
              :key="capability"
              class="permission-chip risk"
            >
              {{ capability }}
            </span>
          </div>
        </div>

        <el-checkbox v-if="needsInstallAcknowledgement" v-model="installAcknowledged" class="install-confirmation">
          我已查看签名、权限和风险信息，并确认继续{{ installMode === "update" ? "更新" : "安装" }}。
        </el-checkbox>
      </div>

      <template #footer>
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="installLoading"
          :disabled="!canInstallPreview"
          @click="commitPackageInstall"
        >
          {{ installMode === "update" ? "确认更新" : "确认安装" }}
        </el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Box, Plus, Refresh, UploadFilled } from "@element-plus/icons-vue";
import { useApi } from "../../../composables/useApi";
import {
  installExtensionPackage,
  previewExtensionPackage,
  type PackageImportPreview,
  type PackageOperationResult,
} from "../../extensions/api";

interface PetPlugin {
  extensionId: string;
  pluginId: string;
  name: string;
  description: string;
  version: string;
  enabled: boolean;
  installState: string;
  managementTarget: string;
}

interface PetPluginList {
  plugins: PetPlugin[];
  total: number;
  page: number;
  pageSize: number;
}

interface PackageOperationView {
  status: string;
  errorCode?: string;
}

const { get, post, del } = useApi();
const plugins = ref<PetPlugin[]>([]);
const loading = ref(false);
const busy = ref("");
const enabledCount = computed(() => plugins.value.filter((item) => item.enabled).length);

const installDialogVisible = ref(false);
const installMode = ref<"install" | "update">("install");
const updateTarget = ref<PetPlugin | null>(null);
const installFile = ref<File | null>(null);
const installPreview = ref<PackageImportPreview | null>(null);
const previewLoading = ref(false);
const installLoading = ref(false);
const uploadProgress = ref(0);
const installAcknowledged = ref(false);
const packageInput = ref<HTMLInputElement | null>(null);

const previewIsPetPlugin = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  return preview.managementTarget === "pet_center" || preview.contributionKinds?.includes("petx") === true;
});

const previewMatchesInstalledVersion = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  const current = plugins.value.find((item) => item.extensionId === preview.id);
  return current?.version === preview.version;
});

const needsInstallAcknowledgement = computed(() => {
  const preview = installPreview.value;
  if (!preview) return false;
  return preview.signature?.status === "unsigned"
    || preview.scripts > 0
    || (preview.highRiskCapabilities?.length || 0) > 0
    || (preview.capabilityConfirmations?.length || 0) > 0
    || (preview.warnings?.length || 0) > 0;
});

const canInstallPreview = computed(() => {
  const preview = installPreview.value;
  if (!preview || previewLoading.value || installLoading.value || !previewIsPetPlugin.value) return false;
  if (!preview.compatible || (preview.errors?.length || 0) > 0) return false;
  if (previewMatchesInstalledVersion.value) return false;
  if (installMode.value === "update" && updateTarget.value && preview.id !== updateTarget.value.extensionId) return false;
  return !needsInstallAcknowledgement.value || installAcknowledged.value;
});

onMounted(loadPlugins);

async function loadPlugins() {
  loading.value = true;
  try {
    const result = await get<PetPluginList>("/api/extensions/pet/plugins", {
      page: 1,
      pageSize: 100,
    });
    plugins.value = result?.plugins ?? [];
  } catch (error: any) {
    ElMessage.error(error?.message || "桌宠插件加载失败");
  } finally {
    loading.value = false;
  }
}

function openInstallDialog(plugin?: PetPlugin) {
  resetInstallState();
  installMode.value = plugin ? "update" : "install";
  updateTarget.value = plugin || null;
  installDialogVisible.value = true;
}

function resetInstallState() {
  installFile.value = null;
  installPreview.value = null;
  previewLoading.value = false;
  installLoading.value = false;
  uploadProgress.value = 0;
  installAcknowledged.value = false;
  updateTarget.value = null;
  installMode.value = "install";
}

async function choosePackage() {
  const desktop = window.amitiaDesktop;
  if (!desktop?.selectExtensionPackage) {
    packageInput.value?.click();
    return;
  }
  try {
    const selected = await desktop.selectExtensionPackage();
    if (!selected) return;
    const bytes = Uint8Array.from(atob(selected.base64), (character) => character.charCodeAt(0));
    await setPackageFile(new File([bytes], selected.name, { type: "application/zip" }));
  } catch (error: any) {
    ElMessage.error(error?.message || "选择插件包失败");
  }
}

async function onPackageFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) await setPackageFile(file);
}

async function onPackageDrop(event: DragEvent) {
  const file = event.dataTransfer?.files?.[0];
  if (file) await setPackageFile(file);
}

async function setPackageFile(file: File) {
  if (!/\.petx$/i.test(file.name)) {
    ElMessage.warning("请选择 .petx 桌宠插件包");
    return;
  }
  installFile.value = file;
  installPreview.value = null;
  installAcknowledged.value = false;
  await buildPackagePreview();
}

async function buildPackagePreview() {
  if (!installFile.value) return;
  previewLoading.value = true;
  uploadProgress.value = 0;
  try {
    const preview = await previewExtensionPackage(
      installFile.value,
      "global",
      "",
      updateTarget.value?.extensionId || "",
      (percent) => {
        uploadProgress.value = percent;
      },
      "pet-center",
    );
    installPreview.value = preview;
    if (preview.currentVersion) installMode.value = "update";
    if (!previewIsPetPlugin.value) {
      ElMessage.error("该扩展包不是桌宠插件，已阻止安装");
      return;
    }
    if (updateTarget.value && preview.id !== updateTarget.value.extensionId) {
      ElMessage.error(`更新包 ID 不匹配：需要 ${updateTarget.value.extensionId}，实际为 ${preview.id}`);
    }
  } catch (error: any) {
    installPreview.value = null;
    ElMessage.error(error?.message || "插件包预览失败");
  } finally {
    previewLoading.value = false;
    uploadProgress.value = installPreview.value ? 100 : uploadProgress.value;
  }
}

async function commitPackageInstall() {
  const preview = installPreview.value;
  if (!preview || !canInstallPreview.value) return;
  installLoading.value = true;
  try {
    const acknowledged = installAcknowledged.value || !needsInstallAcknowledgement.value;
    const result = await installExtensionPackage(
      preview,
      {
        unsigned: acknowledged,
        scripts: acknowledged,
        capabilities: acknowledged ? [...(preview.highRiskCapabilities || [])] : [],
        versionChange: acknowledged,
        signerChange: acknowledged,
        configMigration: acknowledged,
      },
      installMode.value === "update" ? updateTarget.value?.extensionId || preview.id : "",
      "pet-center",
    );
    await waitForPackageOperation(result.operationId);
    ElMessage.success(installMode.value === "update" ? "桌宠插件已更新" : "桌宠插件已安装");
    installDialogVisible.value = false;
    await loadPlugins();
  } catch (error: any) {
    ElMessage.error(error?.message || (installMode.value === "update" ? "更新失败" : "安装失败"));
  } finally {
    installLoading.value = false;
  }
}

async function waitForPackageOperation(operationId?: string) {
  if (!operationId) return;
  for (let attempt = 0; attempt < 120; attempt += 1) {
    const operation = await get<PackageOperationView>(
      `/api/extensions/packages/operations/${encodeURIComponent(operationId)}`,
      undefined,
      { headers: { "X-Amitia-Management-Target": "pet-center" } },
    );
    const status = String(operation?.status || "").toLowerCase();
    if (status === "completed") return;
    if (status === "failed" || status === "requires_recovery") {
      throw new Error(operation?.errorCode || "插件包操作失败");
    }
    await new Promise((resolve) => window.setTimeout(resolve, 500));
  }
  throw new Error("插件包操作等待超时，请刷新页面检查最终状态");
}

async function togglePlugin(plugin: PetPlugin) {
  busy.value = plugin.extensionId;
  try {
    await post(
      `/api/extensions/pet/plugins/${encodeURIComponent(plugin.extensionId)}/${plugin.enabled ? "disable" : "enable"}`,
    );
    ElMessage.success(plugin.enabled ? "桌宠插件已禁用" : "桌宠插件已启用");
    await loadPlugins();
  } catch (error: any) {
    ElMessage.error(error?.message || "桌宠插件状态切换失败");
  } finally {
    busy.value = "";
  }
}

async function uninstallPlugin(plugin: PetPlugin) {
  try {
    await ElMessageBox.confirm(
      `确定卸载「${plugin.name || plugin.pluginId}」吗？相关桌宠能力将立即移除。`,
      "确认卸载桌宠插件",
      {
        confirmButtonText: "确认卸载",
        cancelButtonText: "取消",
        type: "warning",
      },
    );
  } catch {
    return;
  }
  busy.value = plugin.extensionId;
  try {
    await del(`/api/extensions/pet/plugins/${encodeURIComponent(plugin.extensionId)}`);
    ElMessage.success("桌宠插件已卸载");
    await loadPlugins();
  } catch (error: any) {
    ElMessage.error(error?.message || "桌宠插件卸载失败");
  } finally {
    busy.value = "";
  }
}

function signatureLabel(status?: string) {
  const labels: Record<string, string> = {
    "valid-trusted": "可信签名",
    "valid-untrusted": "有效 / 未信任",
    unsigned: "未签名",
    invalid: "签名无效",
  };
  return labels[String(status || "").toLowerCase()] || status || "未知";
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
</script>

<style scoped>
.pet-plugin-manager {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-head h2 {
  margin: 0;
  color: var(--el-text-color-primary);
  font-size: 18px;
}

.section-head p {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.section-actions {
  display: flex;
  gap: 8px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.summary-card {
  display: flex;
  min-height: 84px;
  flex-direction: column;
  justify-content: space-between;
  padding: 16px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--el-bg-color);
}

.summary-card span {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.summary-card strong {
  color: var(--el-text-color-primary);
  font-size: 22px;
}

.list-card {
  border-radius: 8px;
}

.plugin-list {
  display: flex;
  min-height: 120px;
  flex-direction: column;
}

.plugin-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 4px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.plugin-item:last-child {
  border-bottom: 0;
}

.plugin-mark {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border-radius: 8px;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-size: 20px;
}

.plugin-main {
  min-width: 0;
  flex: 1;
}

.plugin-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plugin-title strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.plugin-main p {
  margin: 6px 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.plugin-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}

.plugin-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
}

.package-drop-zone {
  display: flex;
  min-height: 170px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 24px;
  border: 1px dashed var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-ultralight);
  text-align: center;
}

.package-drop-zone.has-file {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
}

.package-drop-zone span {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.upload-icon {
  color: var(--el-color-primary);
  font-size: 34px;
}

.upload-progress {
  margin-top: 12px;
}

.package-preview {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 18px;
}

.preview-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.preview-head h3 {
  margin: 4px 0;
}

.preview-head p {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.preview-kicker,
.preview-label {
  color: var(--el-color-primary);
  font-size: 12px;
}

.preview-facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.preview-facts div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
  padding: 10px;
  border-radius: 6px;
  background: var(--el-fill-color-lighter);
}

.preview-facts span {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.preview-facts strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.permission-chip {
  padding: 4px 8px;
  border-radius: 4px;
  background: var(--el-fill-color);
  font-size: 12px;
}

.permission-chip.risk {
  background: var(--el-color-danger-light-9);
  color: var(--el-color-danger);
}

.install-confirmation {
  align-self: flex-start;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
}

@media (max-width: 900px) {
  .section-head {
    flex-direction: column;
  }

  .summary-grid,
  .preview-facts {
    grid-template-columns: 1fr;
  }

  .plugin-item {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .plugin-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
