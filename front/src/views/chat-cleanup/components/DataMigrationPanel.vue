<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card class="section-card migration-card">
    <template #header>
      <div class="migration-header">
        <span class="card-title">数据迁移</span>
        <div v-if="migrationStatus" class="migration-stats">
          <div class="migration-stat-item">
            <span class="ms-label">数据版本</span>
            <span class="ms-value">v{{ migrationStatus.currentVersion }}</span>
          </div>
          <div v-if="migrationStatus.pendingCount > 0" class="migration-stat-item">
            <span class="ms-label">待处理</span>
            <span class="ms-value pending">{{ migrationStatus.pendingCount }}</span>
          </div>
        </div>
      </div>
    </template>
    <div v-if="!migrationStatus" class="migration-loading">加载中...</div>
    <template v-else>
      <div v-if="migrationStatus.lastMigration" style="margin-bottom: 12px">
        <div class="migration-history-title">最近迁移记录</div>
        <div class="cleanup-report">
          <div class="report-item">
            <span class="report-label">版本：</span>
            <span class="report-value">{{ migrationStatus.lastMigration.version }}</span>
          </div>
          <div class="report-item">
            <span class="report-label">时间：</span>
            <span class="report-value">{{ migrationStatus.lastMigration.appliedAt?.slice(0, 19) || "—" }}</span>
          </div>
          <div class="report-item">
            <span class="report-label">状态：</span>
            <span class="report-value">{{ migrationStatus.lastMigration.status }}</span>
          </div>
        </div>
      </div>
      <div v-if="migrationStatus.pendingCount > 0">
        <el-alert
          type="info"
          :title="`${migrationStatus.pendingCount} 个待执行的迁移`"
          :closable="false"
          show-icon
          style="margin-bottom: 12px"
        />
        <div class="confirm-row" style="margin-bottom: 8px">
          <span class="confirm-label">输入「确认检查」以检查：</span>
          <el-input
            v-model="migCheckConfirm"
            placeholder="确认检查"
            style="width: 140px"
            size="small"
          />
        </div>
        <div class="migration-actions">
          <el-button
            type="primary"
            :disabled="migCheckConfirm !== '确认检查'"
            :loading="migChecking"
            @click="checkMigrations"
          >
            执行迁移检查
          </el-button>
        </div>
      </div>
    </template>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { apiClient } from "../../../composables/useApi"

const migrationStatus = ref<any>(null)
const migChecking = ref(false)
const migCheckConfirm = ref("")

onMounted(async () => {
  await loadMigrations()
})

async function loadMigrations() {
  try {
    const res = await apiClient.get("/api/storage/migrations")
    const d = res.data?.data || res.data
    migrationStatus.value = d
  } catch {
    migrationStatus.value = null
  }
}

async function checkMigrations() {
  if (migCheckConfirm.value !== "确认检查") return
  migChecking.value = true
  try {
    const res = await apiClient.post("/api/storage/migrations/check", {
      confirmText: "确认检查",
    })
    const d = res.data?.data || res.data
    migrationStatus.value = d
    migCheckConfirm.value = ""
    ElMessage.success(d.message || "检查完成")
  } catch (err: any) {
    ElMessage.error("检查失败: " + (err.response?.data?.message || err.message))
  } finally {
    migChecking.value = false
  }
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
.migration-card {
  border-color: var(--el-border-color-light);
}
.migration-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.migration-stats {
  display: flex;
  gap: 24px;
}
.migration-stat-item {
  text-align: center;
}
.ms-label {
  display: block;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.ms-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.ms-value.pending {
  color: var(--el-color-warning);
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
.migration-actions {
  margin-top: 16px;
}
.cleanup-report {
  font-size: 14px;
}
.report-item {
  padding: 6px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.report-label {
  color: var(--el-text-color-secondary);
}
.report-value {
  color: var(--el-text-color-primary);
  font-weight: 500;
}
.confirm-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.confirm-label {
  font-size: 14px;
  color: var(--el-text-color-regular);
}
@media (max-width: 600px) {
  .migration-stats {
    flex-wrap: wrap;
    gap: 12px;
  }
}
</style>
