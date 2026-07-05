<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="runtime-panel">
    <StatusPanel ref="statusPanelRef" />
    <ManualActionsPanel @action-completed="refreshStatus" />
    <ConfigPanel />

    <el-card shadow="never" class="section-card">
      <template #header><span>回复时机判断</span></template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="功能状态"><el-tag :type="timingOverview.enabled ? 'success' : 'info'" size="small">{{ timingOverview.enabled ? '已启用' : '已禁用' }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="模型判断"><el-tag :type="timingOverview.useModelCheck ? 'success' : 'warning'" size="small">{{ timingOverview.useModelCheck ? '已启用' : '仅规则' }}</el-tag></el-descriptions-item>
        <el-descriptions-item label="Web 等待">{{ timingOverview.webWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="微信等待">{{ timingOverview.wechatWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="最大等待">{{ timingOverview.maxWaitMs }}ms</el-descriptions-item>
        <el-descriptions-item label="缓冲区总数">{{ timingOverview.bufferCounts?.total || 0 }}</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top: 8px; display: flex; gap: 8px; flex-wrap: wrap">
        <el-tag size="small" type="info">等待中: {{ timingOverview.bufferCounts?.waiting || 0 }}</el-tag>
        <el-tag size="small" type="warning">检查中: {{ timingOverview.bufferCounts?.checking || 0 }}</el-tag>
        <el-tag size="small" type="primary">回复中: {{ timingOverview.bufferCounts?.replying || 0 }}</el-tag>
        <el-tag size="small" type="danger">已暂停: {{ timingOverview.bufferCounts?.paused || 0 }}</el-tag>
        <el-tag size="small" type="danger">失败: {{ timingOverview.bufferCounts?.failed || 0 }}</el-tag>
      </div>
      <div v-if="timingOverview.recentFailures?.length" style="margin-top: 12px">
        <div class="form-tip" style="font-weight: 600; margin-bottom: 4px">最近失败记录：</div>
        <div v-for="(f, i) in timingOverview.recentFailures.slice(0, 5)" :key="i" class="form-tip">{{ f.created_at?.slice(0, 19) }} {{ f.details?.slice(0, 80) }}</div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import axios from "axios"
import { getApiBaseURL } from "../../runtime/runtime-adapter"
import StatusPanel from "../long-running/components/StatusPanel.vue"
import ManualActionsPanel from "../long-running/components/ManualActionsPanel.vue"
import ConfigPanel from "../long-running/components/ConfigPanel.vue"

const apiBaseUrl = ref("")
const statusPanelRef = ref<InstanceType<typeof StatusPanel> | null>(null)
const timingOverview = ref<any>({ enabled: false, bufferCounts: {} })

function refreshStatus() {
  statusPanelRef.value?.refresh()
}

async function loadTimingOverview() {
  try {
    const { data } = await axios.get(apiBaseUrl.value + "/api/reply-timing/overview")
    if (data?.data) timingOverview.value = data.data
  } catch {}
}

onMounted(async () => {
  apiBaseUrl.value = await getApiBaseURL()
  loadTimingOverview()
})
</script>

<style scoped>
.runtime-panel { }
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.form-tip { font-size: 12px; color: var(--el-text-color-secondary); margin-top: 4px; }
</style>
