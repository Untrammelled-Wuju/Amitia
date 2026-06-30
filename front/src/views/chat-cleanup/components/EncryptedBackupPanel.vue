<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card class="section-card backup-card">
    <template #header>
      <span class="card-title">加密备份与恢复</span>
    </template>
    <div class="backup-create-row" style="margin-bottom: 16px">
      <el-input
        v-model="backupPassword"
        type="password"
        show-password
        placeholder="设置备份密码（至少4位）"
        style="flex: 1; max-width: 320px"
      />
      <el-button
        type="primary"
        :disabled="!backupPassword || backupPassword.length < 4"
        :loading="backupCreating"
        @click="createEncryptedBackup"
      >
        创建加密备份
      </el-button>
    </div>

    <div v-if="backupListLoaded">
      <div v-if="backupList.length === 0" style="font-size: 13px; color: var(--el-text-color-placeholder)">
        暂无加密备份
      </div>
      <div v-else>
        <div class="migration-history-title">备份列表</div>
        <div class="cleanup-report">
          <div
            v-for="(b, idx) in backupList"
            :key="idx"
            class="report-item"
            style="display: flex; justify-content: space-between; align-items: center"
          >
            <div>
              <div style="font-weight: 500">{{ b.name }}</div>
              <div style="font-size: 12px; color: var(--el-text-color-secondary)">
                {{ b.createdAt?.slice(0, 19) || "—" }} · {{ b.sizeFormatted }}
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
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../../composables/useApi"

const emit = defineEmits<{
  (e: "restore", backup: any): void
}>()

const backupPassword = ref("")
const backupCreating = ref(false)
const backupList = ref<any[]>([])
const backupListLoaded = ref(false)

onMounted(async () => {
  await loadBackups()
})

async function loadBackups() {
  try {
    const res = await apiClient.get("/api/storage/backups")
    backupList.value = res.data?.data || res.data || []
    backupListLoaded.value = true
  } catch {
    backupList.value = []
    backupListLoaded.value = true
  }
}

async function createEncryptedBackup() {
  if (!backupPassword.value || backupPassword.value.length < 4) return
  backupCreating.value = true
  try {
    const res = await apiClient.post("/api/storage/backup/encrypted", {
      password: backupPassword.value,
    })
    const d = res.data?.data || res.data
    backupPassword.value = ""
    ElMessage.success("加密备份创建成功: " + d.backupName)
    await loadBackups()
  } catch (err: any) {
    ElMessage.error("创建失败: " + (err.response?.data?.message || err.message))
  } finally {
    backupCreating.value = false
  }
}

function openRestore(backup: any) {
  emit("restore", backup)
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
  gap: 12px;
}
.migration-loading {
  font-size: 13px;
  color: var(--el-text-color-placeholder);
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
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
@media (max-width: 600px) {
  .backup-create-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
