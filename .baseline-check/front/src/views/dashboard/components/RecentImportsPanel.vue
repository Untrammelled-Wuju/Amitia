<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="section-card">
    <div class="section-header-row">
      <span class="panel-title">最近导入批次</span>
    </div>
    <div v-if="recentImports.length > 0" class="list-compact">
      <div v-for="b in recentImports" :key="b.id" class="list-item">
        <span class="li-title">{{ b.title }}</span>
        <span class="li-meta"
          >{{ b.itemCount || 0 }}条 · {{ fmtDateShort(b.createdAt) }}</span
        >
      </div>
    </div>
    <div v-else class="empty-hint">
      <div class="empty-illustration">
        <el-icon><Download /></el-icon>
      </div>
      <span>暂无导入记录</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { Download } from "@element-plus/icons-vue";

defineProps<{
  recentImports: any[];
  fmtDateShort: (d: string) => string;
}>();
</script>

<style scoped>
.section-card {
  min-height: 286px;
  padding: 22px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: none;
}
.section-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.panel-title {
  font-size: 18px;
  font-weight: 800;
  color: var(--console-text);
}

.list-compact {
  margin-top: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.list-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 44px;
  padding: 8px 12px;
  border-radius: 9px;
  background: var(--ac-color-surface);
  font-size: 14px;
}
.li-title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}
.li-meta {
  font-size: 12px;
  color: var(--console-text-muted);
  white-space: nowrap;
  margin-left: 8px;
}

.empty-hint {
  height: 210px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 14px;
  color: var(--console-text-muted);
  font-size: 15px;
}
.empty-illustration {
  width: 88px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--ac-color-text-muted);
  border: 4px solid var(--ac-color-text-muted);
  border-radius: 10px;
  font-size: 46px;
}
</style>
