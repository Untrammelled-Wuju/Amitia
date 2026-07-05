<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="section-card">
    <div class="section-header-row">
      <span class="panel-title">最近错误</span>
      <el-button text circle @click="emit('refresh')">
        <el-icon><Refresh /></el-icon>
      </el-button>
    </div>
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
      <div class="empty-illustration">
        <el-icon class="doc-icon"><Document /></el-icon>
        <span class="check-badge"><el-icon><CircleCheck /></el-icon></span>
      </div>
      <span>暂无错误，一切正常</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { CircleCheck, Document, Refresh } from "@element-plus/icons-vue"

defineProps<{
  recentErrors: any[]
  fmtDateShort: (d: string) => string
}>()

const emit = defineEmits<{
  (e: "refresh"): void
}>()
</script>

<style scoped>
.section-card {
  min-height: 336px;
  padding: 22px;
  border: 1px solid var(--console-border);
  border-radius: 14px;
  background: var(--console-card);
  box-shadow: var(--console-shadow);
}
.section-header-row { display: flex; justify-content: space-between; align-items: center; }
.panel-title { font-size: 18px; font-weight: 800; color: var(--console-text); }

.error-list { display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto; }
.error-row { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; padding: 10px 12px; border-radius: 9px; background: var(--console-card-soft); }
.er-left { display: flex; align-items: flex-start; gap: 8px; flex: 1; min-width: 0; }
.er-msg { font-size: 14px; color: var(--console-text-muted); line-height: 1.4; word-break: break-all; }
.er-time { font-size: 12px; color: var(--console-text-muted); white-space: nowrap; flex-shrink: 0; }

.empty-hint { color: var(--console-text-muted); padding: 52px 0 0; }
.empty-hint.ok { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 16px; font-size: 15px; }
.empty-illustration { position: relative; width: 90px; height: 90px; display: flex; align-items: center; justify-content: center; color: #cbd5e1; }
.doc-icon { font-size: 78px; }
.check-badge {
  position: absolute;
  right: 7px;
  bottom: 18px;
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #ffffff;
  background: #22c55e;
  border: 3px solid #ffffff;
}
</style>
