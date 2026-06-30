<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-label">总会话数</div>
        <div class="stat-value">{{ stats.totalConversations }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">总消息数</div>
        <div class="stat-value">{{ stats.totalMessages }}</div>
      </div>
      <div class="stat-card">
        <div class="stat-label">数据库大小</div>
        <div class="stat-value">{{ stats.dbSize }}</div>
      </div>
    </div>

    <el-card class="section-card vacuum-card">
      <template #header>
        <span class="card-title">数据库优化</span>
      </template>
      <div style="display: flex; align-items: center; gap: 12px">
        <el-button
          type="success"
          :loading="vacuumLoading"
          @click="runVacuum"
        >
          执行 VACUUM 优化
        </el-button>
        <span v-if="vacuumResult" style="font-size: 13px; color: var(--el-color-success)">
          已完成：释放 {{ vacuumResult.freedFormatted }}
          <template v-if="vacuumResult.sizeBeforeFormatted">
            ({{ vacuumResult.sizeBeforeFormatted }} → {{ vacuumResult.sizeAfterFormatted }})
          </template>
        </span>
        <span v-else style="font-size: 12px; color: var(--el-text-color-placeholder)">
          优化可压缩数据库文件，回收已删除数据占用的空间
        </span>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { useDatabaseHealth } from "../composables/useDatabaseHealth"

const { stats, vacuumLoading, vacuumResult, loadStats, runVacuum } = useDatabaseHealth()

onMounted(async () => {
  await loadStats()
})
</script>

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
  margin-bottom: 20px;
}
.stat-card {
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 16px;
}
.stat-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}
.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.section-card {
  margin-bottom: 16px;
  border: 1px solid var(--el-border-color-light);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
}
.vacuum-card {
  border-color: var(--el-color-info-light-5);
}

@media (max-width: 600px) {
  .stat-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
