<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card class="section-card backup-card">
    <template #header>
      <span class="card-title">灾难恢复备份</span>
    </template>
    <div class="backup-create-grid">
      <el-input v-model="passphrase" type="password" show-password autocomplete="new-password" placeholder="可选口令，设置后归档加密" />
      <el-input v-model="passphraseConfirm" type="password" show-password autocomplete="new-password" placeholder="再次输入口令" />
      <el-button type="primary" :loading="creating" :disabled="!canCreate" @click="createBackup">创建完整备份</el-button>
    </div>
    <div class="backup-hint">完整备份用于当前设备恢复；跨设备迁移请使用“数据管理”中的可迁移导出。</div>
    <div v-if="loaded">
      <div v-if="backups.length === 0" class="empty-text">暂无备份</div>
      <div v-for="backup in backups" :key="backup.name" class="backup-row">
        <div class="backup-info">
          <div class="backup-name">
            {{ backup.name }}
            <el-tag v-if="backup.encrypted" type="success" size="small">加密</el-tag>
          </div>
          <div class="backup-meta">{{ backup.createdAt }} · {{ formatSize(backup.size) }}</div>
        </div>
        <div class="backup-actions">
          <el-button size="small" @click="emit('restore', backup)">恢复</el-button>
          <el-button size="small" type="danger" plain @click="removeBackup(backup)">删除</el-button>
        </div>
      </div>
    </div>
    <div v-else class="migration-loading">加载中...</div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  createDataPortabilityBackup,
  deleteDataPortabilityBackup,
  listDataPortabilityBackups,
  type DataPortabilityBackup,
} from "@/runtime/data-portability";

const emit = defineEmits<{
  (event: "restore", backup: DataPortabilityBackup): void;
}>();

const creating = ref(false);
const loaded = ref(false);
const backups = ref<DataPortabilityBackup[]>([]);
const passphrase = ref("");
const passphraseConfirm = ref("");

const canCreate = computed(() => !passphrase.value || (passphrase.value.length >= 8 && passphrase.value === passphraseConfirm.value));

async function loadBackups() {
  loaded.value = false;
  try {
    backups.value = await listDataPortabilityBackups();
  } catch (error: any) {
    ElMessage.error(error?.message || "加载备份失败");
  } finally {
    loaded.value = true;
  }
}

async function createBackup() {
  creating.value = true;
  try {
    await createDataPortabilityBackup(passphrase.value);
    passphrase.value = "";
    passphraseConfirm.value = "";
    ElMessage.success("完整备份已创建");
    await loadBackups();
  } catch (error: any) {
    ElMessage.error(error?.message || "创建备份失败");
  } finally {
    creating.value = false;
  }
}

async function removeBackup(backup: DataPortabilityBackup) {
  try {
    await ElMessageBox.confirm(`确定删除备份「${backup.name}」？删除后无法恢复。`, "删除备份", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
  } catch {
    return;
  }
  try {
    await deleteDataPortabilityBackup(backup.name);
    ElMessage.success("备份已删除");
    await loadBackups();
  } catch (error: any) {
    ElMessage.error(error?.message || "删除备份失败");
  }
}

function formatSize(value: number) {
  if (value >= 1024 * 1024 * 1024) return `${(value / 1024 / 1024 / 1024).toFixed(2)} GB`;
  if (value >= 1024 * 1024) return `${(value / 1024 / 1024).toFixed(2)} MB`;
  if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${value} B`;
}

onMounted(loadBackups);
</script>

<style scoped>
.backup-card { margin-bottom: 12px; }
.backup-create-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 10px; }
.backup-hint, .empty-text, .migration-loading { color: var(--el-text-color-secondary); font-size: 12px; }
.backup-hint { margin: 10px 0 16px; }
.backup-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 0; border-top: 1px solid var(--surface-border); }
.backup-info { min-width: 0; }
.backup-name { display: flex; align-items: center; gap: 8px; overflow-wrap: anywhere; }
.backup-meta { margin-top: 4px; color: var(--el-text-color-secondary); font-size: 12px; }
.backup-actions { display: flex; gap: 8px; flex-shrink: 0; }
@media (max-width: 760px) {
  .backup-create-grid { grid-template-columns: 1fr; }
  .backup-row { align-items: flex-start; flex-direction: column; }
}
</style>
