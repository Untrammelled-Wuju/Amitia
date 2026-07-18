<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="health-modules" v-if="runtimeHealth">
    <div class="health-header">
      <span class="panel-title">服务状态</span>
      <el-button text circle :loading="runtimeHealthLoading" @click="emit('runHealthCheck')">
        <el-icon :size="16"><Refresh /></el-icon>
      </el-button>
    </div>
    <div class="health-module-grid">
      <div
        v-for="m in runtimeHealth.modules"
        :key="m.module"
        class="health-module-item"
        :class="'hm-' + m.status"
      >
        <div class="module-icon"><el-icon><Cpu /></el-icon></div>
        <div class="hmi-body">
          <div class="hmi-label">{{ healthModuleLabel(m.module) }}</div>
          <div class="hmi-status" :class="m.status">{{ healthStatusLabel(m.status) }}</div>
        </div>
        <div class="hmi-detail" v-if="m.detail">{{ m.detail }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Cpu, Refresh } from "@element-plus/icons-vue"

defineProps<{
  runtimeHealth: any
  runtimeHealthLoading: boolean
  healthModuleLabel: (m: string) => string
  healthStatusLabel: (s: string) => string
}>()

const emit = defineEmits<{
  (e: "runHealthCheck"): void
}>()
</script>

<style scoped>
.health-modules {
  min-height: 336px;
  padding: 22px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: none;
}
.health-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.panel-title { font-size: 18px; font-weight: 800; color: var(--console-text); }
.health-module-grid { display: grid; grid-template-columns: 1fr; gap: 10px; }
.health-module-item {
  min-height: 54px;
  background: var(--ac-color-surface);
  border-radius: 9px;
  padding: 12px 14px;
  border: 1px solid var(--console-border-soft);
  display: grid;
  grid-template-columns: 22px 1fr auto;
  align-items: center;
  gap: 12px;
}
.health-module-item:hover { background: var(--tp-control-hover); }
.module-icon { display: flex; color: var(--console-text-muted); font-size: 18px; }
.hmi-body { display: flex; align-items: center; gap: 20px; min-width: 0; }
.hmi-label { min-width: 76px; font-size: 14px; font-weight: 600; color: var(--console-text-secondary); }
.hmi-status { display: inline-flex; align-items: center; gap: 6px; font-size: 14px; font-weight: 400; color: var(--ac-color-success); }
.hmi-status::before { content: ""; width: 20px; height: 20px; border-radius: 50%; background: var(--status-ok-bg); }
.hmi-status.warning { color: var(--ac-color-warning); }
.hmi-status.warning::before { background: var(--status-off-bg); }
.hmi-status.error { color: var(--ac-color-danger); }
.hmi-status.error::before { background: var(--status-off-bg); }
.hmi-detail { font-size: 13px; color: var(--console-text-muted); word-break: break-all; }

@media (max-width: 768px) {
  .health-module-grid { grid-template-columns: 1fr; }
}
</style>
