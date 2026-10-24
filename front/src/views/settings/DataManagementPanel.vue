<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="data-management-panel">
    <el-card shadow="never" class="section-card">
      <template #header><span>存储概览</span></template>
      <div v-if="storageLoading" class="storage-loading"><el-icon class="is-loading" size="20"><Loading /></el-icon></div>
      <el-descriptions v-else :column="2" border size="small">
        <el-descriptions-item label="数据库大小">{{ storageInfo.dbSize || "—" }}</el-descriptions-item>
        <el-descriptions-item label="消息总数">{{ storageInfo.messageCount ?? "—" }}</el-descriptions-item>
        <el-descriptions-item label="对话数">{{ storageInfo.conversationCount ?? "—" }}</el-descriptions-item>
        <el-descriptions-item label="记忆数">{{ storageInfo.memoryCount ?? "—" }}</el-descriptions-item>
      </el-descriptions>
      <div class="data-actions">
        <el-button @click="goStorage"><el-icon><FolderOpened /></el-icon>备份与灾难恢复</el-button>
        <el-button @click="goStorage"><el-icon><Delete /></el-icon>存储清理</el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>用户数据导出</span></template>
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item label="导出范围">
            <el-select v-model="exportForm.scope">
              <el-option label="全部数据" value="all" />
              <el-option label="角色及关联数据" value="character" />
              <el-option label="设置与集成" value="settings" />
              <el-option label="记忆" value="memory" />
            </el-select>
          </el-form-item>
          <el-form-item label="格式">
            <el-select v-model="exportForm.profile">
              <el-option label="可迁移格式" value="portable" />
              <el-option label="完整格式" value="full" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="exportForm.scope === 'character'" label="角色">
            <el-select v-model="exportForm.characterId" filterable placeholder="选择角色">
              <el-option v-for="character in characters" :key="character.id" :label="character.name" :value="character.id" />
            </el-select>
          </el-form-item>
        </div>
        <el-checkbox v-model="exportEncrypted">使用口令加密导出包</el-checkbox>
        <div v-if="exportEncrypted" class="passphrase-grid">
          <el-input v-model="exportPassphrase" type="password" show-password autocomplete="new-password" placeholder="设置导出口令" />
          <el-input v-model="exportPassphraseConfirm" type="password" show-password autocomplete="new-password" placeholder="再次输入口令" />
        </div>
      </el-form>
      <div class="data-actions">
        <el-button :disabled="!exportReady" :loading="exportPreviewing" @click="openExportPreview">预检导出</el-button>
        <el-button type="primary" :disabled="!exportReady" :loading="exporting" @click="runExport">导出</el-button>
      </div>
      <p v-if="lastExport" class="result-text">
        已导出 {{ lastExport.file }}，{{ formatBytes(lastExport.size) }}，{{ lastExport.components.length }} 个组件。
      </p>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>导入用户数据</span></template>
      <p class="page-summary">文件先进入隔离区并完成校验、解密和冲突预检，确认后才会写入。</p>
      <div class="passphrase-grid">
        <el-input v-model="importPassphrase" type="password" show-password autocomplete="off" placeholder="加密归档口令，未加密可留空" />
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="inspectImportFile" accept=".amitia-backup,.enc">
          <el-button :loading="importInspecting"><el-icon><Upload /></el-icon>选择并预检</el-button>
        </el-upload>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span>配置迁移</span></template>
      <p class="page-summary">配置迁移只同步应用设置，不包含角色、聊天和记忆。</p>
      <div class="data-actions">
        <el-button :loading="configBusy" @click="exportConfig"><el-icon><Download /></el-icon>导出配置</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleConfigImport" accept=".json">
          <el-button :loading="configBusy"><el-icon><Upload /></el-icon>导入配置</el-button>
        </el-upload>
      </div>
    </el-card>

    <el-dialog v-model="exportPreviewVisible" title="确认导出内容" width="720px">
      <div v-if="exportPreview" class="preview-summary">
        <p>
          范围 {{ scopeLabel(exportPreview.scope) }}，格式 {{ exportPreview.profile }}，
          预计 {{ formatBytes(exportPreview.estimatedSize) }}，{{ exportPreview.encrypted ? "已启用加密" : "未加密" }}。
        </p>
        <el-table :data="exportPreview.components" max-height="420">
          <el-table-column prop="logicalName" label="组件" min-width="220" />
          <el-table-column prop="itemCount" label="记录数" width="110" />
          <el-table-column label="标记" width="160">
            <template #default="{ row }">
              <el-tag v-if="row.sensitive" type="warning" size="small">敏感</el-tag>
              <el-tag v-if="row.rebuildable" type="info" size="small">可重建</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="exportPreviewVisible = false">取消</el-button>
        <el-button type="primary" :loading="exporting" @click="runExport">确认导出</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="确认导入" width="760px" :close-on-click-modal="false" @closed="discardImportTicket">
      <template v-if="importPreview">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="备份 ID">{{ importPreview.backupId }}</el-descriptions-item>
          <el-descriptions-item label="应用版本">{{ importPreview.preview.manifest.appVersion }}</el-descriptions-item>
          <el-descriptions-item label="来源平台">{{ importPreview.preview.manifest.sourcePlatform }}</el-descriptions-item>
          <el-descriptions-item label="范围">{{ scopeLabel(importPreview.preview.manifest.scope) }}</el-descriptions-item>
        </el-descriptions>
        <el-table :data="importPreview.preview.components" max-height="320" class="import-table">
          <el-table-column prop="logicalName" label="组件" min-width="220" />
          <el-table-column prop="itemCount" label="记录数" width="100" />
          <el-table-column label="冲突" width="90">
            <template #default="{ row }">{{ row.collisions?.length || 0 }}</template>
          </el-table-column>
          <el-table-column label="警告" min-width="180">
            <template #default="{ row }">{{ row.warnings?.join("；") || "—" }}</template>
          </el-table-column>
        </el-table>
        <el-form label-position="top" class="import-options">
          <el-form-item label="冲突处理">
            <el-radio-group v-model="importPolicy">
              <el-radio value="duplicate">保留双方并生成新标识</el-radio>
              <el-radio value="skip">跳过已有数据</el-radio>
              <el-radio value="replace">覆盖同一标识数据</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-checkbox v-model="importActivate">导入后启用角色和配置</el-checkbox>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="importCommitting" @click="commitImport">确认导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { apiClient } from "@/composables/useApi";
import { ElMessage, ElMessageBox } from "element-plus";
import { Delete, Download, FolderOpened, Loading, Upload } from "@element-plus/icons-vue";
import {
  cancelDataPortabilityImport,
  commitDataPortabilityImport,
  downloadDataPortabilityExport,
  exportDataPortability,
  inspectDataPortabilityImport,
  previewDataPortabilityExport,
  type DataPortabilityCollisionPolicy,
  type DataPortabilityExportPreview,
  type DataPortabilityExportResult,
  type DataPortabilityImportPreview,
  type DataPortabilityProfile,
  type DataPortabilityScope,
} from "@/runtime/data-portability";

const router = useRouter();
const storageLoading = ref(false);
const storageInfo = ref<any>({});
const characters = ref<any[]>([]);
const exportForm = reactive<{ scope: DataPortabilityScope; profile: DataPortabilityProfile; characterId: string }>({
  scope: "all",
  profile: "portable",
  characterId: "",
});
const exportEncrypted = ref(false);
const exportPassphrase = ref("");
const exportPassphraseConfirm = ref("");
const exportPreviewing = ref(false);
const exporting = ref(false);
const exportPreviewVisible = ref(false);
const exportPreview = ref<DataPortabilityExportPreview | null>(null);
const lastExport = ref<DataPortabilityExportResult | null>(null);
const importPassphrase = ref("");
const importInspecting = ref(false);
const importCommitting = ref(false);
const importDialogVisible = ref(false);
const importPreview = ref<DataPortabilityImportPreview | null>(null);
const importPolicy = ref<DataPortabilityCollisionPolicy>("duplicate");
const importActivate = ref(false);
const configBusy = ref(false);

const exportReady = computed(() => {
  if (exportForm.scope === "character" && !exportForm.characterId) return false;
  if (!exportEncrypted.value) return true;
  return exportPassphrase.value.length >= 8 && exportPassphrase.value === exportPassphraseConfirm.value;
});

function goStorage() {
  router.push("/storage");
}

function formatBytes(value?: number) {
  const bytes = Number(value || 0);
  if (bytes >= 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function scopeLabel(scope: string) {
  return ({ all: "全部数据", character: "角色及关联数据", settings: "设置与集成", memory: "记忆" } as Record<string, string>)[scope] || scope;
}

async function loadStorageInfo() {
  storageLoading.value = true;
  try {
    const { data } = await apiClient.get("/api/storage/info");
    storageInfo.value = data ?? {};
  } finally {
    storageLoading.value = false;
  }
}

async function loadCharacters() {
  try {
    const { data } = await apiClient.get("/api/characters");
    characters.value = Array.isArray(data) ? data : [];
  } catch {
    characters.value = [];
  }
}

function exportPayload() {
  return {
    scope: exportForm.scope,
    profile: exportForm.profile,
    characterId: exportForm.scope === "character" ? exportForm.characterId : "",
    passphrase: exportEncrypted.value ? exportPassphrase.value : "",
  };
}

async function openExportPreview() {
  exportPreviewing.value = true;
  try {
    exportPreview.value = await previewDataPortabilityExport(exportPayload());
    exportPreviewVisible.value = true;
  } catch (error: any) {
    ElMessage.error(error?.message || "导出预检失败");
  } finally {
    exportPreviewing.value = false;
  }
}

async function runExport() {
  exporting.value = true;
  try {
    const result = await exportDataPortability(exportPayload());
    const blob = await downloadDataPortabilityExport(result.file);
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = result.file;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
    lastExport.value = result;
    exportPreviewVisible.value = false;
    ElMessage.success("数据导出完成");
  } catch (error: any) {
    ElMessage.error(error?.message || "导出失败");
  } finally {
    exporting.value = false;
  }
}

async function inspectImportFile(file: any) {
  if (!file?.raw) return;
  importInspecting.value = true;
  try {
    importPreview.value = await inspectDataPortabilityImport(file.raw, importPassphrase.value);
    importPolicy.value = "duplicate";
    importActivate.value = false;
    importDialogVisible.value = true;
  } catch (error: any) {
    ElMessage.error(error?.message || "导入预检失败");
  } finally {
    importInspecting.value = false;
  }
}

async function commitImport() {
  if (!importPreview.value) return;
  importCommitting.value = true;
  try {
    const result = await commitDataPortabilityImport(importPreview.value.ticketId, importPolicy.value, importActivate.value);
    ElMessage.success(`已导入 ${result.totalImported} 条记录，已生成恢复前备份`);
    importDialogVisible.value = false;
    importPreview.value = null;
    await loadStorageInfo();
  } catch (error: any) {
    ElMessage.error(error?.message || "导入失败");
  } finally {
    importCommitting.value = false;
  }
}

async function discardImportTicket() {
  const ticketId = importPreview.value?.ticketId;
  importPreview.value = null;
  if (!ticketId) return;
  try {
    await cancelDataPortabilityImport(ticketId);
  } catch {}
}

async function exportConfig() {
  configBusy.value = true;
  try {
    const { data } = await apiClient.post("/api/config/export");
    if (!data?.exported) throw new Error(data?.error || "配置导出失败");
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "amitia-config.json";
    anchor.click();
    URL.revokeObjectURL(url);
    ElMessage.success("配置已导出");
  } catch (error: any) {
    ElMessage.error(error?.message || "配置导出失败");
  } finally {
    configBusy.value = false;
  }
}

async function handleConfigImport(file: any) {
  configBusy.value = true;
  try {
    const raw = await file.raw.text();
    const previewResponse = await apiClient.post("/api/config/import/preview", { raw });
    const preview = previewResponse.data;
    if (!preview?.valid) throw new Error(preview?.error || "配置文件无效");
    await ElMessageBox.confirm(
      `共 ${preview.itemCount ?? 0} 项配置；新增 ${preview.newCount ?? 0} 项，修改 ${preview.changed ?? 0} 项，未变化 ${preview.unchanged ?? 0} 项。继续后会覆盖同名设置。`,
      "确认导入配置",
      { type: "warning", confirmButtonText: "导入", cancelButtonText: "取消" },
    );
    const result = await apiClient.post("/api/config/import/confirm", { raw });
    if (!result.data?.imported) throw new Error(result.data?.error || "配置导入失败");
    ElMessage.success(`已导入 ${result.data.importedCount ?? 0} 项配置`);
  } catch (error: any) {
    if (error === "cancel" || error === "close") return;
    ElMessage.error(error?.message || "配置导入失败");
  } finally {
    configBusy.value = false;
  }
}

onMounted(() => {
  void loadStorageInfo();
  void loadCharacters();
});
</script>

<style scoped>
.section-card { margin-bottom: 12px; }
.data-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 16px; }
.form-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.form-grid :deep(.el-select) { width: 100%; }
.passphrase-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; margin-top: 12px; }
.storage-loading { padding: 16px; text-align: center; }
.page-summary { color: var(--text-secondary); font-size: var(--ac-font-size-sm); margin: 8px 0 16px; }
.result-text { margin-top: 12px; color: var(--el-color-success); }
.preview-summary p { margin: 0 0 12px; }
.import-table { margin: 14px 0; }
.import-options { margin-top: 8px; }
@media (max-width: 760px) {
  .form-grid, .passphrase-grid { grid-template-columns: 1fr; }
}
</style>
