<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="section-header-row">
        <span class="panel-title">最近错误</span>
        <el-button size="small" text @click="emit('refresh')">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </template>
    <div v-if="recentErrors.length > 0" class="error-list">
      <div v-for="(err, idx) in recentErrors" :key="idx" class="error-row">
        <div class="er-left">
          <el-tag :type="err.severity === 'error' ? 'danger' : 'warning'" size="small" effect="dark">
            {{ err.action || err.targetType || "错误" }}
          </el-tag>
          <span class="er-msg">{{ err.details || err.message || "未知错误" }}</span>
        </div>
        <span class="er-time">{{ fmtDateShort(err.createdAt) }}</span>
      </div>
    </div>
    <div v-else class="empty-hint ok">
      <el-icon><CircleCheck /></el-icon>
      暂无错误，一切正常
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { Refresh, CircleCheck } from "@element-plus/icons-vue"

defineProps<{
  recentErrors: any[]
  fmtDateShort: (d: string) => string
}>()

const emit = defineEmits<{
  (e: "refresh"): void
}>()
</script>

<style scoped>
.section-card { margin-bottom: 14px; }
.section-header-row { display: flex; justify-content: space-between; align-items: center; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }

.error-list { display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto; }
.error-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; padding: 8px 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); }
.er-left { display: flex; align-items: flex-start; gap: 8px; flex: 1; min-width: 0; }
.er-msg { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); line-height: 1.4; word-break: break-all; }
.er-time { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); white-space: nowrap; flex-shrink: 0; }

.empty-hint { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-muted); padding: 8px 0; }
.empty-hint.ok { color: var(--ac-color-success); display: flex; align-items: center; gap: 6px; }
</style>
