<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="section-header-row">
        <span class="section-title">运行状态</span>
        <el-button size="small" :loading="loading" @click="refresh">刷新</el-button>
      </div>
    </template>
    <div v-if="status" class="lr-status-grid">
      <div class="lr-stat-card">
        <div class="lr-stat-label">运行状态</div>
        <div class="lr-stat-value">
          <el-tag :type="status.running ? 'success' : 'info'" size="large">
            {{ status.running ? '运行中' : '空闲' }}
          </el-tag>
        </div>
      </div>
      <div class="lr-stat-card">
        <div class="lr-stat-label">活跃任务数</div>
        <div class="lr-stat-value">{{ status.tasks?.length ?? 0 }}</div>
      </div>
      <div class="lr-stat-card">
        <div class="lr-stat-label">最后活动</div>
        <div class="lr-stat-value lr-sm">
          {{ status.tasks?.length > 0 ? fmtTime(status.tasks[0].updated_at) : '-' }}
        </div>
      </div>
    </div>
    <div v-if="status && status.tasks && status.tasks.length > 0" class="lr-task-list">
      <div class="lr-task-list-title">任务列表</div>
      <div v-for="task in status.tasks" :key="task.id" class="lr-task-row">
        <span class="lr-task-title">{{ task.title || '未命名任务' }}</span>
        <span class="lr-task-time">{{ fmtTime(task.updated_at) }}</span>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { fetchStatusApi, type LongRunningStatus } from "../api"
import { fmtTime } from "../utils"

const status = ref<LongRunningStatus | null>(null)
const loading = ref(false)

async function refresh() {
  loading.value = true
  try {
    status.value = await fetchStatusApi()
  } catch {
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
})

defineExpose({ refresh })
</script>

<style scoped>
.section-card {
  margin-bottom: 14px;
  border: 1px solid var(--ac-color-border-light);
}
.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.lr-status-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}
.lr-stat-card {
  padding: 12px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  text-align: center;
}
.lr-stat-label {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-bottom: 4px;
}
.lr-stat-value {
  font-size: 18px;
  font-weight: 700;
  color: var(--ac-color-text);
}
.lr-stat-value.lr-sm {
  font-size: 13px;
  font-weight: 500;
}
.lr-task-list {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.lr-task-list-title {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text-secondary);
  margin-bottom: 4px;
}
.lr-task-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  font-size: 13px;
}
.lr-task-title {
  font-weight: 500;
  color: var(--ac-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  margin-right: 12px;
}
.lr-task-time {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  white-space: nowrap;
}
@media (max-width: 600px) {
  .lr-status-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
