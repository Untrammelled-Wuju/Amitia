<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card" v-if="history.length > 0">
    <template #header>
      <span class="section-title">
        <el-icon><FolderOpened /></el-icon> 导出记录
      </span>
    </template>
    <div class="export-list">
      <div v-for="(item, idx) in history" :key="idx" class="export-row">
        <span class="er-file">{{ item.file }}</span>
        <span class="er-time">{{ formatTime(item.timestamp) }}</span>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { FolderOpened } from "@element-plus/icons-vue"
import { formatTime } from "./utils"
import type { ExportRecord } from "./types"

defineProps<{
  history: ExportRecord[]
}>()
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  display: flex;
  align-items: center;
  gap: 6px;
}
.export-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.export-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 10px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  font-size: 13px;
}
.er-file {
  flex: 1;
  font-weight: 500;
  color: var(--ac-color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.er-time {
  color: var(--ac-color-text-muted);
  white-space: nowrap;
}
</style>
