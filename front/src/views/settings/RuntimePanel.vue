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
        <el-descriptions-item label="调度器状态"
          ><el-tag
            :type="timingOverview.schedulerRunning ? 'success' : 'danger'"
            size="small"
            >{{ timingOverview.schedulerRunning ? "运行中" : "已停止" }}</el-tag
          ></el-descriptions-item
        >
        <el-descriptions-item label="已启用规则">{{
          timingOverview.enabledRuleCount ?? 0
        }}</el-descriptions-item>
        <el-descriptions-item label="规则总数">{{
          timingOverview.totalRuleCount ?? 0
        }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { apiClient } from "@/composables/useApi";
import StatusPanel from "../long-running/components/StatusPanel.vue";
import ManualActionsPanel from "../long-running/components/ManualActionsPanel.vue";
import ConfigPanel from "../long-running/components/ConfigPanel.vue";

const statusPanelRef = ref<InstanceType<typeof StatusPanel> | null>(null);
const timingOverview = ref<any>({
  schedulerRunning: false,
  enabledRuleCount: 0,
  totalRuleCount: 0,
});

function refreshStatus() {
  statusPanelRef.value?.refresh();
}

async function loadTimingOverview() {
  try {
    const { data } = await apiClient.get("/api/proactive/status");
    if (data) timingOverview.value = data;
  } catch {}
}

onMounted(() => {
  void loadTimingOverview();
});
</script>

<style scoped>
.runtime-panel {
}
.section-card {
  margin-bottom: 12px;
  border: 1px solid var(--ac-color-border-light);
}
.form-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>
