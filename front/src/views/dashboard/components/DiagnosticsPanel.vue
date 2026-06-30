<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card" v-if="diagResult">
    <template #header>
      <div class="section-header-row">
        <div class="header-left-group">
          <span class="panel-title">诊断报告</span>
          <span class="diag-time">{{ fmtDateShort(diagResult.timestamp) }}</span>
        </div>
        <el-button size="small" text :loading="diagLoading" @click="emit('runDiagnostics')">
          <el-icon v-if="!diagLoading"><Refresh /></el-icon>
          运行诊断
        </el-button>
      </div>
    </template>
    <div class="diag-summary">
      <div class="ds-overall" :class="diagResult.overallStatus">
        <el-tag :type="diagResult.overallStatus === 'healthy' ? 'success' : diagResult.overallStatus === 'degraded' ? 'warning' : 'danger'" size="large">
          {{ diagResult.overallStatus === 'healthy' ? '健康' : diagResult.overallStatus === 'degraded' ? '部分异常' : '存在错误' }}
        </el-tag>
      </div>
      <div class="ds-items">
        <div v-for="item in diagResult.items" :key="item.name" class="ds-item" :class="item.status">
          <span class="dsi-status" :class="item.status">
            <el-icon v-if="item.status === 'ok'"><CircleCheck /></el-icon>
            <el-icon v-else-if="item.status === 'warn'"><Warning /></el-icon>
            <el-icon v-else-if="item.status === 'error'"><CircleClose /></el-icon>
            <el-icon v-else><QuestionFilled /></el-icon>
          </span>
          <span class="dsi-name">{{ item.name }}</span>
          <span class="dsi-msg">{{ item.message }}</span>
        </div>
      </div>
      <div v-if="hasSuggestions" class="ds-suggestions">
        <div v-for="item in suggestionItems" :key="'sug-' + item.name" class="ds-suggestion-item">
          <el-icon :size="14"><Warning /></el-icon>
          <span class="dss-name">{{ item.name }}：</span>
          <span class="dss-text">{{ item.suggestion }}</span>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { Refresh, CircleCheck, Warning, CircleClose, QuestionFilled } from "@element-plus/icons-vue"

defineProps<{
  diagResult: any
  diagLoading: boolean
  hasSuggestions: boolean
  suggestionItems: any[]
  fmtDateShort: (d: string) => string
}>()

const emit = defineEmits<{
  (e: "runDiagnostics"): void
}>()
</script>

<style scoped>
.section-card { margin-bottom: 14px; }
.section-header-row { display: flex; justify-content: space-between; align-items: center; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }
.header-left-group { display: flex; align-items: center; gap: 10px; }
.diag-time { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }

.diag-summary { display: flex; flex-direction: column; gap: 10px; }
.ds-overall { display: flex; align-items: center; gap: 8px; }
.ds-items { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
@media (max-width: 640px) { .ds-items { grid-template-columns: 1fr; } }
.ds-item { display: flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); font-size: var(--ac-font-size-sm); }
.ds-item.warn { border-left: 2px solid var(--ac-color-warning); }
.ds-item.error { border-left: 2px solid var(--ac-color-error, #f56c6c); }
.dsi-status.ok { color: var(--ac-color-success); }
.dsi-status.warn { color: var(--ac-color-warning); }
.dsi-status.error { color: var(--ac-color-error, #f56c6c); }
.dsi-status.unknown { color: var(--ac-color-text-muted); }
.dsi-name { font-weight: 500; white-space: nowrap; flex-shrink: 0; }
.dsi-msg { color: var(--ac-color-text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }

.ds-suggestions { margin-top: 10px; display: flex; flex-direction: column; gap: 6px; }
.ds-suggestion-item { display: flex; align-items: flex-start; gap: 6px; padding: 6px 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-warning-bg, #fef0e6); font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); line-height: 1.5; }
.dss-name { font-weight: 600; white-space: nowrap; flex-shrink: 0; }
.dss-text { word-break: break-all; }
</style>
