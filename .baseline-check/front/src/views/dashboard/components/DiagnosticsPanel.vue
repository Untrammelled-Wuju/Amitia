<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="section-card">
    <div class="section-header-row">
      <span class="panel-title">诊断报告</span>
      <div class="header-actions">
        <span v-if="diagResult" class="diag-time">
          <el-icon><Clock /></el-icon>
          {{ fmtDateShort(diagResult.timestamp) }}
        </span>
        <el-button text :loading="diagLoading" @click="emit('runDiagnostics')">
          查看完整报告
          <el-icon v-if="!diagLoading"><Right /></el-icon>
        </el-button>
      </div>
    </div>
    <div v-if="diagResult" class="diag-summary">
      <div class="ds-overall" :class="diagResult.overallStatus">
        <span class="status-dot"></span>
        {{
          diagResult.overallStatus === "healthy"
            ? "全部正常"
            : diagResult.overallStatus === "degraded"
              ? "部分异常"
              : "存在错误"
        }}
      </div>
      <div class="ds-items">
        <div
          v-for="item in diagResult.items"
          :key="item.name"
          class="ds-item"
          :class="item.status"
        >
          <span class="dsi-status" :class="item.status">
            <el-icon v-if="item.status === 'ok'"><CircleCheck /></el-icon>
            <el-icon v-else-if="item.status === 'warn'"><Warning /></el-icon>
            <el-icon v-else-if="item.status === 'error'"
              ><CircleClose
            /></el-icon>
            <el-icon v-else><QuestionFilled /></el-icon>
          </span>
          <span class="dsi-name">{{ item.name }}</span>
          <span class="dsi-msg">{{ item.message }}</span>
        </div>
      </div>
      <div v-if="hasSuggestions" class="ds-suggestions">
        <div
          v-for="item in suggestionItems"
          :key="'sug-' + item.name"
          class="ds-suggestion-item"
        >
          <el-icon :size="14"><Warning /></el-icon>
          <span class="dss-name">{{ item.name }}：</span>
          <span class="dss-text">{{ item.suggestion }}</span>
        </div>
      </div>
    </div>
    <div v-else class="empty-hint">
      <span class="status-dot"></span>
      正在等待诊断结果
    </div>
  </section>
</template>

<script setup lang="ts">
import {
  CircleCheck,
  Warning,
  CircleClose,
  QuestionFilled,
  Clock,
  Right,
} from "@element-plus/icons-vue";

defineProps<{
  diagResult: any;
  diagLoading: boolean;
  hasSuggestions: boolean;
  suggestionItems: any[];
  fmtDateShort: (d: string) => string;
}>();

const emit = defineEmits<{
  (e: "runDiagnostics"): void;
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
.header-actions {
  display: flex;
  align-items: center;
  gap: 18px;
}
.diag-time {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--console-text-muted);
}

.diag-summary {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 0;
  border: 1px solid var(--console-border-soft);
  border-radius: 9px;
  overflow: hidden;
}
.ds-overall {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  color: var(--ac-color-warning);
  background: var(--console-diagnostic-warn-bg);
  font-size: 15px;
  font-weight: 750;
}
.ds-overall.healthy {
  color: var(--ac-color-success);
  background: var(--console-diagnostic-ok-bg);
}
.status-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: currentColor;
}
.ds-items {
  display: grid;
  grid-template-columns: 1fr;
}
@media (max-width: 640px) {
  .ds-items {
    grid-template-columns: 1fr;
  }
}
.ds-item {
  display: grid;
  grid-template-columns: 22px minmax(130px, 1fr) minmax(140px, 1.2fr);
  align-items: center;
  gap: 8px;
  min-height: 40px;
  padding: 8px 16px;
  border-top: 1px solid var(--console-border-soft);
  background: var(--ac-color-surface);
  font-size: 14px;
}
.dsi-status.ok {
  color: var(--ac-color-success);
}
.dsi-status.warn {
  color: var(--ac-color-warning);
}
.dsi-status.error {
  color: var(--ac-color-danger);
}
.dsi-status.unknown {
  color: var(--ac-color-text-muted);
}
.dsi-name {
  font-weight: 500;
  white-space: nowrap;
  flex-shrink: 0;
}
.dsi-msg {
  color: var(--console-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  text-align: right;
}

.ds-suggestions {
  margin-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.ds-suggestion-item {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 8px;
  background: var(--console-diagnostic-warn-bg);
  font-size: 12px;
  color: var(--console-text-muted);
  line-height: 1.5;
}
.dss-name {
  font-weight: 600;
  white-space: nowrap;
  flex-shrink: 0;
}
.dss-text {
  word-break: break-all;
}
.empty-hint {
  margin-top: 18px;
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--console-text-muted);
  font-size: 14px;
}
</style>
