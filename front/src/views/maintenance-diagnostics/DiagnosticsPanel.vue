<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="section-header">
        <span class="section-title">
          <el-icon><Monitor /></el-icon> 一键诊断
        </span>
        <div class="section-actions">
          <el-button
            size="small"
            type="primary"
            :loading="diagLoading"
            @click="handleRunDiagnose"
          >
            {{ diagLoading ? '诊断中...' : '开始诊断' }}
          </el-button>
          <el-button
            size="small"
            :loading="exportLoading"
            :disabled="!diagResult"
            @click="handleExport"
          >
            {{ exportLoading ? '导出中...' : '导出诊断包' }}
          </el-button>
        </div>
      </div>
    </template>
    <div v-if="diagResult" class="diag-result">
      <div class="dr-overall" :class="diagResult.overallStatus">
        <el-tag :type="overallTagType" size="large">
          {{ overallLabel }}
        </el-tag>
        <span class="dr-time">{{ formatTime(diagResult.timestamp) }}</span>
      </div>
      <div class="dr-summary">
        <span class="drs-item ok"><el-icon><CircleCheck /></el-icon> 正常: {{ diagResult.summary.ok }}</span>
        <span class="drs-item warn" v-if="diagResult.summary.warn"><el-icon><WarningFilled /></el-icon> 警告: {{ diagResult.summary.warn }}</span>
        <span class="drs-item error" v-if="diagResult.summary.error"><el-icon><CircleCloseFilled /></el-icon> 错误: {{ diagResult.summary.error }}</span>
      </div>
      <div class="dr-list">
        <div
          v-for="item in diagResult.items"
          :key="item.name"
          class="dr-item"
          :class="item.status"
        >
          <span class="dri-icon">
            <el-icon v-if="item.status === 'ok'"><CircleCheck /></el-icon>
            <el-icon v-else-if="item.status === 'warn'"><WarningFilled /></el-icon>
            <el-icon v-else-if="item.status === 'error'"><CircleCloseFilled /></el-icon>
            <el-icon v-else><QuestionFilled /></el-icon>
          </span>
          <div class="dri-body">
            <div class="dri-name">{{ item.name }}</div>
            <div class="dri-msg">{{ item.message }}</div>
            <div v-if="item.details" class="dri-details">{{ item.details }}</div>
            <div v-if="item.status !== 'ok' && item.suggestion" class="dri-suggestion">
              <el-icon><InfoFilled /></el-icon> {{ item.suggestion }}
            </div>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="empty-hint">
      <el-icon><InfoFilled /></el-icon>
      点击"开始诊断"检查系统各组件运行状态
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { ElMessage } from "element-plus"
import {
  Monitor,
  CircleCheck, CircleCloseFilled, WarningFilled, QuestionFilled, InfoFilled,
} from "@element-plus/icons-vue"
import { runDiagnoseApi, exportDiagnosticApi } from "./api"
import { formatTime } from "./utils"
import type { DiagResult, ExportRecord } from "./types"

const emit = defineEmits<{
  exported: [record: ExportRecord]
}>()

const diagLoading = ref(false)
const exportLoading = ref(false)
const diagResult = ref<DiagResult | null>(null)

const overallTagType = computed(() => {
  if (!diagResult.value) return "info"
  if (diagResult.value.overallStatus === "healthy") return "success"
  if (diagResult.value.overallStatus === "degraded") return "warning"
  return "danger"
})

const overallLabel = computed(() => {
  if (!diagResult.value) return "未诊断"
  if (diagResult.value.overallStatus === "healthy") return "系统健康"
  if (diagResult.value.overallStatus === "degraded") return "部分异常"
  return "存在错误"
})

async function handleRunDiagnose() {
  diagLoading.value = true
  try {
    diagResult.value = await runDiagnoseApi()
    ElMessage.success("诊断完成")
  } catch (e: any) {
    ElMessage.error("诊断失败: " + (e.response?.data?.message || e.message))
  } finally {
    diagLoading.value = false
  }
}

async function handleExport() {
  exportLoading.value = true
  try {
    const record = await exportDiagnosticApi()
    emit("exported", record)
    ElMessage.success("诊断包已导出: " + record.file)
  } catch (e: any) {
    ElMessage.error("导出失败: " + (e.response?.data?.message || e.message))
  } finally {
    exportLoading.value = false
  }
}
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-actions {
  display: flex;
  gap: 8px;
}
.empty-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  padding: 16px 0;
}
.diag-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.dr-overall {
  display: flex;
  align-items: center;
  gap: 10px;
}
.dr-time {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}
.dr-summary {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  font-size: var(--ac-font-size-sm);
}
.drs-item {
  display: flex;
  align-items: center;
  gap: 4px;
}
.drs-item.ok { color: var(--ac-color-success); }
.drs-item.warn { color: var(--ac-color-warning); }
.drs-item.error { color: var(--ac-color-error, #f56c6c); }
.dr-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.dr-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  border: 1px solid var(--ac-color-border-light);
}
.dr-item.ok { border-left: 3px solid var(--ac-color-success); }
.dr-item.warn { border-left: 3px solid var(--ac-color-warning); background: #fef7e0; }
.dr-item.error { border-left: 3px solid var(--ac-color-error, #f56c6c); background: #fef0f0; }
.dri-icon { flex-shrink: 0; margin-top: 1px; }
.dri-icon .el-icon { font-size: 18px; }
.dr-item.ok .dri-icon { color: var(--ac-color-success); }
.dr-item.warn .dri-icon { color: var(--ac-color-warning); }
.dr-item.error .dri-icon { color: var(--ac-color-error, #f56c6c); }
.dri-body { flex: 1; min-width: 0; }
.dri-name { font-size: var(--ac-font-size-sm); font-weight: 600; }
.dri-msg { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); }
.dri-details { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 2px; word-break: break-all; }
.dri-suggestion {
  font-size: 12px;
  color: #e6a23c;
  margin-top: 4px;
  padding: 4px 8px;
  border-radius: 4px;
  background: #fef7e0;
  display: flex;
  align-items: flex-start;
  gap: 4px;
  line-height: 1.4;
}
</style>
