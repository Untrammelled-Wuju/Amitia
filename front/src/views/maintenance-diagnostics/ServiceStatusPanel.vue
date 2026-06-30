<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="section-header">
        <span class="section-title">
          <el-icon><Odometer /></el-icon> 服务状态
        </span>
        <el-button size="small" @click="handleFetchStatus" :loading="statusLoading">刷新</el-button>
      </div>
    </template>
    <div v-if="statusData" class="status-simple">
      <div class="ss-overall">
        <span class="ss-label">系统状态</span>
        <el-tag :type="statusData.status === 'healthy' ? 'success' : 'warning'" size="large">
          {{ statusData.status === 'healthy' ? '健康' : '部分异常' }}
        </el-tag>
        <span class="ss-time" v-if="statusData.lastCheck">上次检查: {{ statusData.lastCheck }}</span>
      </div>
      <div v-if="statusData.issues && statusData.issues.length > 0" class="ss-issues">
        <div class="ss-issues-title">
          <el-icon><WarningFilled /></el-icon> 发现问题 ({{ statusData.issues.length }})
        </div>
        <div v-for="(issue, idx) in statusData.issues" :key="idx" class="ss-issue-item">
          <span class="ssi-type">{{ issue.type }}</span>
          <span class="ssi-msg">{{ issue.msg }}</span>
        </div>
      </div>
      <div v-else class="ss-no-issues">
        <el-icon><CircleCheck /></el-icon> 未发现问题
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import {
  Odometer,
  CircleCheck, WarningFilled,
} from "@element-plus/icons-vue"
import { fetchStatusApi } from "./api"
import type { StatusData } from "./types"

const statusLoading = ref(false)
const statusData = ref<StatusData | null>(null)

async function handleFetchStatus() {
  statusLoading.value = true
  try {
    statusData.value = await fetchStatusApi()
  } catch (e: any) {
    // silent
  } finally {
    statusLoading.value = false
  }
}

onMounted(() => {
  handleFetchStatus()
})

defineExpose({ fetchStatus: handleFetchStatus })
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  display: flex;
  align-items: center;
  gap: 6px;
}
.status-simple {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.ss-overall {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
}
.ss-label {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text);
}
.ss-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-left: auto;
}
.ss-issues {
  padding: 10px 12px;
  border-radius: var(--ac-radius-md);
  background: #fef7e0;
  border: 1px solid #f5dab1;
}
.ss-issues-title {
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: #e6a23c;
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 6px;
}
.ss-issue-item {
  display: flex;
  gap: 8px;
  padding: 4px 0;
  font-size: 13px;
}
.ssi-type {
  font-weight: 600;
  color: var(--ac-color-text);
  text-transform: uppercase;
  font-size: 11px;
  padding: 0 4px;
  border-radius: 2px;
  background: #f5dab1;
  white-space: nowrap;
}
.ssi-msg {
  color: var(--ac-color-text-secondary);
}
.ss-no-issues {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-success);
}
@media (max-width: 600px) {
  .status-simple {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
