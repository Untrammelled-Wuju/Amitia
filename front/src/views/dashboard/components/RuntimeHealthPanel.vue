<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="health-modules" v-if="runtimeHealth">
    <div class="health-header">
      <span class="panel-title">服务状态</span>
      <el-button size="small" :loading="runtimeHealthLoading" @click="emit('runHealthCheck')">
        <el-icon :size="14" style="margin-right:4px"><Refresh /></el-icon>立即检查
      </el-button>
    </div>
    <div class="health-module-grid">
      <div
        v-for="m in runtimeHealth.modules"
        :key="m.module"
        class="health-module-item"
        :class="'hm-' + m.status"
      >
        <div class="hmi-indicator" :class="m.status"></div>
        <div class="hmi-body">
          <div class="hmi-label">{{ healthModuleLabel(m.module) }}</div>
          <div class="hmi-status">{{ healthStatusLabel(m.status) }}</div>
        </div>
        <div class="hmi-detail" v-if="m.detail">{{ m.detail }}</div>
        <div class="hmi-suggestion" v-if="m.suggestion">{{ m.suggestion }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Refresh } from "@element-plus/icons-vue"

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
.health-modules { margin-top: 12px; margin-bottom: 14px; }
.health-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }
.health-module-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; }
.health-module-item {
  background: var(--ac-color-bg-secondary); border-radius: 6px; padding: 12px;
  border: 1px solid var(--ac-color-border-light); display: flex; flex-direction: column; gap: 4px;
}
.health-module-item.hm-error { border-left: 3px solid #d4644c; background: #fdf5f4; }
.health-module-item.hm-warning { border-left: 3px solid #b8952e; background: #fdf8ef; }
.hmi-indicator { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.hmi-indicator.ok { background: #5a9e6f; }
.hmi-indicator.warning { background: #b8952e; }
.hmi-indicator.error { background: #d4644c; }
.hmi-indicator.unknown { background: #aaa; }
.hmi-body { display: flex; align-items: center; gap: 10px; }
.hmi-label { font-size: 13px; font-weight: 600; color: var(--ac-color-text); }
.hmi-status { font-size: 11px; color: var(--ac-color-text-muted); }
.hmi-detail { font-size: 11px; color: var(--ac-color-text-secondary); word-break: break-all; }
.hmi-suggestion { font-size: 11px; color: #8b6914; margin-top: 2px; }

@media (max-width: 768px) {
  .health-module-grid { grid-template-columns: 1fr; }
}
</style>
