<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <span class="panel-title">最近导入批次</span>
    </template>
    <div v-if="recentImports.length > 0" class="list-compact">
      <div v-for="b in recentImports" :key="b.id" class="list-item">
        <span class="li-title">{{ b.title }}</span>
        <span class="li-meta">{{ b.itemCount || 0 }}条 · {{ fmtDateShort(b.createdAt) }}</span>
      </div>
    </div>
    <div v-else class="empty-hint">暂无导入记录</div>
  </el-card>
</template>

<script setup lang="ts">
defineProps<{
  recentImports: any[]
  fmtDateShort: (d: string) => string
}>()
</script>

<style scoped>
.section-card { margin-bottom: 14px; }
.panel-title { font-size: var(--ac-font-size-sm); font-weight: 600; color: var(--ac-color-text); }

.list-compact { display: flex; flex-direction: column; gap: 6px; }
.list-item { display: flex; justify-content: space-between; align-items: center; padding: 4px 0; font-size: var(--ac-font-size-sm); }
.li-title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.li-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); white-space: nowrap; margin-left: 8px; }

.empty-hint { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-muted); padding: 8px 0; }
</style>
