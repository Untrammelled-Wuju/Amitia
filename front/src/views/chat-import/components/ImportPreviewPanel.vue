<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card" v-if="parseResult">
    <template #header>
      <span class="step-badge">2</span> 预览与编辑（{{ parseResult.items?.length || 0 }} 条消息）
      <span class="detected-tag">
        检测到：
        <el-tag size="small" type="info">{{ formatLabel(parseResult.detectedFormat) }}</el-tag>
      </span>
    </template>
    <div v-if="parseResult.warnings?.length" class="warnings-block">
      <el-alert
        v-for="(w, i) in parseResult.warnings"
        :key="i"
        :title="w.message || w"
        :type="warningType(w)"
        :closable="false"
        show-icon
        style="margin-bottom:4px"
      />
    </div>
    <el-alert
      v-if="parseResult.hasHighRisk"
      type="error"
      :closable="false"
      show-icon
      style="margin-bottom:8px"
    >
      <template #title>检测到高风险敏感数据，确认前请仔细检查。</template>
    </el-alert>
    <el-table :data="editableItems" stripe size="small" max-height="400">
      <el-table-column prop="lineNo" label="#" width="50" />
      <el-table-column label="发言者" width="100">
        <template #default="{row, $index}">
          <el-input v-model="editableItems[$index].speaker" size="small" />
        </template>
      </el-table-column>
      <el-table-column label="角色" width="90">
        <template #default="{row, $index}">
          <el-select v-model="editableItems[$index].role" size="small">
            <el-option label="用户" value="user" />
            <el-option label="AI" value="assistant" />
            <el-option label="系统" value="system" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column label="内容">
        <template #default="{row, $index}">
          <el-input
            v-model="editableItems[$index].content"
            size="small"
            :class="{ 'is-sensitive': row._sensitive }"
          />
        </template>
      </el-table-column>
      <el-table-column label="置信度" width="90" align="center">
        <template #default="{row}">
          <el-progress
            :percentage="Math.round((row.confidence || 0) * 100)"
            :stroke-width="6"
            :show-text="true"
            :color="confidenceColor(row.confidence)"
          />
        </template>
      </el-table-column>
      <el-table-column label="时间" width="100">
        <template #default="{row, $index}">
          <el-input
            v-model="editableItems[$index].timestamp"
            size="small"
            placeholder="HH:MM"
          />
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import { warningType, confidenceColor } from "../utils"

defineProps<{
  parseResult: any
  editableItems: any[]
}>()

function formatLabel(value: string) {
  if (value === "auto") return "自动"
  if (value === "standard") return "标准"
  if (value === "timestamp") return "时间戳"
  if (value === "multiline") return "多行"
  if (value === "wechat") return "微信"
  return value || "自动"
}
</script>

<style scoped>
.section-card { margin-bottom: 12px; }
.step-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px; border-radius: 50%;
  background: var(--ac-color-primary); color: #fff;
  font-size: 11px; font-weight: 700; margin-right: 6px; flex-shrink: 0;
}
.detected-tag { margin-left: 12px; font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }
.warnings-block { margin-bottom: 10px; }
:deep(.is-sensitive .el-input__inner) { border-color: var(--ac-color-danger) !important; background: rgba(200,90,90,0.04); }
</style>
