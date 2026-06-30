<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="maintenance-page">
    <div class="mp-header">
      <h2 class="page-title">维护与诊断</h2>
      <span class="mp-subtitle">单用户运维工具 - 仅服务部署者本人</span>
    </div>
    <el-alert
      v-if="showRestartWarning"
      title="高风险操作警告"
      type="warning"
      :closable="true"
      show-icon
      class="mp-alert"
      @close="showRestartWarning = false"
    >
      <template #default>
        <p>重启 Bridge 和重载配置是高风险操作，可能影响正在进行的对话。</p>
        <p>所有操作都会记录到审计日志中。</p>
      </template>
    </el-alert>
    <DiagnosticsPanel @exported="onExported" />
    <ServiceStatusPanel />
    <OperationsPanel />
    <ExportHistoryPanel :history="exportHistory" />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import DiagnosticsPanel from "./DiagnosticsPanel.vue"
import ServiceStatusPanel from "./ServiceStatusPanel.vue"
import OperationsPanel from "./OperationsPanel.vue"
import ExportHistoryPanel from "./ExportHistoryPanel.vue"
import type { ExportRecord } from "./types"

const showRestartWarning = ref(true)
const exportHistory = ref<ExportRecord[]>([])

function onExported(record: ExportRecord) {
  exportHistory.value.unshift(record)
  if (exportHistory.value.length > 10) exportHistory.value.pop()
}
</script>

<style scoped>
.maintenance-page {
  padding: 0 0 24px 0;
}
.mp-header {
  margin-bottom: 16px;
}
.page-title {
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
  margin: 0 0 4px 0;
}
.mp-subtitle {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
.mp-alert {
  margin-bottom: 12px;
}
</style>
