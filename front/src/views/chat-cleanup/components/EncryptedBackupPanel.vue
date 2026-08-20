<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card class="section-card backup-card">
    <template #header>
      <span class="card-title">数据库备份与恢复</span>
    </template>
    <div class="backup-create-row" style="margin-bottom: 16px">
      <div class="backup-hint">创建当前数据目录的安全快照，可用于恢复数据库。</div>
      <el-button type="primary" :loading="backupCreating" @click="createBackup">
        创建备份
      </el-button>
    </div>

    <div v-if="backupListLoaded">
      <div v-if="backupList.length === 0" class="empty-text">暂无备份</div>
      <div v-else>
        <div class="migration-history-title">备份列表</div>
        <div class="cleanup-report">
          <div
            v-for="(b, idx) in backupList"
            :key="b.name || idx"
            class="report-item"
          >
            <div>
              <div style="font-weight: 500">{{ b.name }}</div>
              <div class="backup-meta">
                {{ formatCreatedAt(b) }} · {{ formatSize(b) }}
              </div>
            </div>
            <el-button size="small" @click="openRestore(b)">恢复</el-button>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="migration-loading">加载中...</div>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "../../../composables/useApi";

const emit = defineEmits<{
  (e: "restore", backup: any): void;
}>();

const backupCreating = ref(false);
const backupList = ref<any[]>([]);
const backupListLoaded = ref(false);

onMounted(loadBackups);

function normalizeBackups(payload: any): any[] {
  const body = payload?.data ?? payload;
  if (Array.isArray(body)) return body;
  if (Array.isArray(body?.backups)) return body.backups;
  return [];
}

async function loadBackups() {
  try {
    const res = await apiClient.get("/api/storage/backups");
    backupList.value = normalizeBackups(res.data);
  } catch {
    backupList.value = [];
  } finally {
    backupListLoaded.value = true;
  }
}

async function createBackup() {
  backupCreating.value = true;
  try {
    await apiClient.post("/api/storage/backups");
    ElMessage.success("备份创建成功");
    await loadBackups();
  } catch (err: any) {
    ElMessage.error("创建失败: " + (err.response?.data?.message || err.response?.data?.error || err.message));
  } finally {
    backupCreating.value = false;
  }
}

function formatCreatedAt(backup: any) {
  const value = backup?.createdAt || backup?.modTime || backup?.created_at;
  return value ? String(value).slice(0, 19) : "—";
}

function formatSize(backup: any) {
  if (backup?.sizeFormatted) return backup.sizeFormatted;
  const value = Number(backup?.size || backup?.sizeBytes || 0);
  if (!Number.isFinite(value) || value <= 0) return "—";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`;
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

function openRestore(backup: any) {
  emit("restore", backup);
}
</script>

<style scoped>
.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
}
.backup-card {
  border-color: var(--el-border-color-light);
}
.backup-create-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.backup-hint,
.empty-text,
.migration-loading {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.migration-history-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}
.cleanup-report {
  font-size: 14px;
}
.report-item {
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.backup-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
@media (max-width: 600px) {
  .backup-create-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
