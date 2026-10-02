<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="data-management-panel">
    <h2>数据管理</h2>
    <p class="page-summary">备份、导入导出与存储清理</p>
    <el-card shadow="never" class="section-card">
      <template #header><span>存储与备份</span></template>
      <div v-if="storageLoading" class="storage-loading"><el-icon class="is-loading" size="20"><Loading /></el-icon></div>
      <el-descriptions v-else :column="2" border size="small">
        <el-descriptions-item label="数据库大小">{{ storageInfo.dbSize || '—' }}</el-descriptions-item>
        <el-descriptions-item label="消息总数">{{ storageInfo.messageCount ?? '—' }}</el-descriptions-item>
        <el-descriptions-item label="对话数">{{ storageInfo.conversationCount ?? '—' }}</el-descriptions-item>
        <el-descriptions-item label="记忆数">{{ storageInfo.memoryCount ?? '—' }}</el-descriptions-item>
        <el-descriptions-item label="备份数量">{{ backupList.length }}</el-descriptions-item>
      </el-descriptions>
      <div class="data-actions">
        <el-button :loading="backupCreating" @click="createBackup"><el-icon><FolderAdd /></el-icon>创建备份</el-button>
        <el-button @click="goStorage"><el-icon><Delete /></el-icon>存储清理</el-button>
      </div>
    </el-card>
    <el-card shadow="never" class="section-card">
      <template #header><span>数据导入导出</span></template>
      <el-form label-position="top">
        <el-form-item label="导出角色数据">
          <div class="data-actions role-export">
            <el-select v-model="exportCharId" placeholder="选择角色" clearable filterable class="character-select">
              <el-option v-for="character in exportCharacters" :key="character.id" :label="character.name" :value="character.id" />
            </el-select>
            <el-button :disabled="!exportCharId" :loading="exportingAmitia" @click="exportAmitiaData('character')"><el-icon><User /></el-icon>导出角色</el-button>
          </div>
        </el-form-item>
      </el-form>
      <div class="data-actions">
        <el-button :loading="exportingAmitia" @click="exportAmitiaData('all')"><el-icon><Download /></el-icon>导出所有数据</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleImportFile" accept=".amitia,.zip,.tar,.gz">
          <el-button :loading="importingAmitia"><el-icon><Upload /></el-icon>导入数据</el-button>
        </el-upload>
      </div>
      <p v-if="importResult" role="status" class="page-summary">{{ importResult }}</p>
    </el-card>
    <el-card shadow="never" class="section-card">
      <template #header><span>配置迁移</span></template>
      <p class="page-summary">导入或导出应用配置。导入前会预览变更并确认覆盖。</p>
      <div class="data-actions">
        <el-button :loading="configBusy" @click="exportConfig"><el-icon><Download /></el-icon>导出配置</el-button>
        <el-upload :auto-upload="false" :show-file-list="false" :on-change="handleConfigImport" accept=".json">
          <el-button :loading="configBusy"><el-icon><Upload /></el-icon>导入配置</el-button>
        </el-upload>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { apiClient } from "@/composables/useApi";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Loading,
  FolderAdd,
  Delete,
  Download,
  User,
  Upload,
} from "@element-plus/icons-vue";
const router = useRouter();
const storageLoading = ref(false);
const storageInfo = ref<any>({});
const backupList = ref<any[]>([]);
const backupCreating = ref(false);

const exportCharId = ref("");
const exportCharacters = ref<any[]>([]);
const exportingAmitia = ref(false);

const importingAmitia = ref(false);
const importResult = ref("");
const configBusy = ref(false);

function goStorage() {
  router.push("/storage");
}

async function downloadExportFile(file: string) {
  const name = String(file || "").trim();
  if (!name) throw new Error("后端未返回导出文件");
  const res = await apiClient.get(
    "/api/storage/export-download/" + encodeURIComponent(name),
    { responseType: "blob" },
  );
  const blob = res.data instanceof Blob ? res.data : new Blob([res.data]);
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

async function loadStorageInfo() {
  storageLoading.value = true;
  try {
    const { data } = await apiClient.get("/api/storage/info");
    storageInfo.value = data ?? {};
  } catch {}
  try {
    const { data } = await apiClient.get("/api/storage/backups");
    const payload = data || {};
    backupList.value = Array.isArray(payload.backups) ? payload.backups : [];
  } catch {}
  storageLoading.value = false;
}

async function createBackup() {
  backupCreating.value = true;
  try {
    await apiClient.post("/api/storage/backups");
    ElMessage.success("备份创建成功");
    await loadStorageInfo();
  } catch (err: any) {
    ElMessage.error(
      "创建备份失败: " + (err?.response?.data?.msg || err.message),
    );
  } finally {
    backupCreating.value = false;
  }
}

async function exportAmitiaData(scope: string) {
  exportingAmitia.value = true;
  try {
    const res = await apiClient.post(
      "/api/storage/export-amitia",
      {
        scope,
        characterId: scope === "character" ? exportCharId.value : "",
      },
    );
    const data = res.data;
    if (!data?.exported || !data?.file) throw new Error(data?.error || "后端未返回导出文件");
    await downloadExportFile(data.file);
    ElMessage.success(data?.message || "导出成功");
  } catch (err: any) {
    ElMessage.error("导出失败: " + (err?.response?.data?.msg || err.message));
  } finally {
    exportingAmitia.value = false;
  }
}

async function loadExportCharacters() {
  try {
    const { data } = await apiClient.get("/api/characters");
    if (Array.isArray(data)) exportCharacters.value = data;
  } catch {}
}

async function exportConfig() {
  configBusy.value = true;
  try {
    const res = await apiClient.post("/api/config/export");
    const data = res.data;
    if (!data?.exported) throw new Error(data?.error || "配置导出失败");
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "amitia-config.json";
    a.click();
    URL.revokeObjectURL(url);
    ElMessage.success("配置已导出");
  } catch (err: any) {
    ElMessage.error("配置导出失败: " + (err?.response?.data?.msg || err.message));
  } finally {
    configBusy.value = false;
  }
}

async function handleConfigImport(file: any) {
  configBusy.value = true;
  try {
    const raw = await file.raw.text();
    const previewRes = await apiClient.post("/api/config/import/preview", { raw });
    const preview = previewRes.data;
    if (!preview?.valid) throw new Error(preview?.error || "配置文件无效");
    await ElMessageBox.confirm(
      `共 ${preview.itemCount ?? 0} 项配置；新增 ${preview.newCount ?? 0} 项，修改 ${preview.changed ?? 0} 项，未变化 ${preview.unchanged ?? 0} 项。继续后会覆盖同名设置。`,
      "确认导入配置",
      { type: "warning", confirmButtonText: "导入", cancelButtonText: "取消" },
    );
    const confirmRes = await apiClient.post("/api/config/import/confirm", { raw });
    const result = confirmRes.data;
    if (!result?.imported) throw new Error(result?.error || "配置导入失败");
    ElMessage.success(`已导入 ${result.importedCount ?? 0} 项配置`);
  } catch (err: any) {
    if (err === "cancel" || err === "close") return;
    ElMessage.error("配置导入失败: " + (err?.response?.data?.msg || err.message || err));
  } finally {
    configBusy.value = false;
  }
}

async function handleImportFile(file: any) {
  importingAmitia.value = true;
  importResult.value = "";
  try {
    const formData = new FormData();
    formData.append("file", file.raw);
    const res = await apiClient.post(
      "/api/storage/import-amitia",
      formData,
    );
    const data = res.data;
    if (data?.imported) {
      const stats = data.stats || {};
      const tableCount = Object.keys(stats).length;
      ElMessage.success(
        "导入成功，共 " +
          data.totalImported +
          " 条记录，" +
          tableCount +
          " 张表",
      );
      importResult.value =
        "导入完成：共 " +
        data.totalImported +
        " 条记录，涉及 " +
        tableCount +
        " 张表";
      loadStorageInfo();
    } else {
      ElMessage.error("导入失败: " + (data?.error || "未知错误"));
    }
  } catch (err: any) {
    ElMessage.error("导入失败: " + (err?.response?.data?.msg || err.message));
  } finally {
    importingAmitia.value = false;
  }
}

onMounted(() => {
  void loadStorageInfo();
  void loadExportCharacters();
});
</script>

<style scoped>
.section-card { margin-bottom: 12px; }
.data-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 16px; }
.role-export { margin-top: 0; width: 100%; }
.character-select { width: min(100%, 240px); }
.storage-loading { padding: 16px; text-align: center; }
.page-summary { color: var(--text-secondary); font-size: var(--ac-font-size-sm); margin: 8px 0 24px; }
</style>
