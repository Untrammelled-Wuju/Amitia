<!-- SPDX-FileCopyrightText: 2026 Peng Xu -->
<!-- SPDX-License-Identifier: AGPL-3.0-only -->
<template>
    <div v-if="showCandidates && candidates.length > 0" class="candidate-list">
      <div v-for="c in candidates" :key="c.id" class="candidate-card">
        <div class="cc-header">
          <el-tag size="small" :type="c.importance > 7 ? 'danger' : 'info'">{{ typeLabel(c.memoryType) }}</el-tag>
          <span class="cc-importance">重要: {{ c.importance }}/10</span>
        </div>
        <div class="cc-key">{{ c.key }}</div>
        <div class="cc-value">{{ c.value }}</div>
        <div class="cc-source">来源: {{ c.sourceText || "提取" }}</div>
        <div class="cc-actions">
          <el-button size="small" type="primary" @click="confirmCandidate(c)">确认保存</el-button>
          <el-button size="small" @click="editCandidate(c)">编辑</el-button>
          <el-button size="small" type="danger" @click="deleteCandidateItem(c)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- Memory list -->
    <el-table ref="tableRef" :data="memories" stripe size="small" style="margin-top:10px" @selection-change="handleSelectionChange">
</template>

<script setup lang="ts">
defineProps<{ candidates: any[]; showCandidates: boolean }>()
const emit = defineEmits<{ confirm: [c: any]; edit: [c: any]; deleteItem: [c: any]; "toggle-show": [] }>()
</script>

<style scoped>
.candidate-list { display: flex; flex-direction: column; gap: 8px; margin: 10px 0; }
.candidate-card { padding: 12px; border-radius: var(--ac-radius-md); background: var(--ac-color-warning-bg, rgba(200,146,74,0.08)); border: 1px solid var(--ac-color-warning-border, rgba(200,146,74,0.2)); }
.cc-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; }
.cc-importance { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }
.cc-key { font-weight: 600; font-size: var(--ac-font-size-sm); margin-bottom: 2px; }
.cc-value { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-secondary); margin-bottom: 4px; }
.cc-source { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-bottom: 6px; }
.cc-actions { display: flex; gap: 6px; }
.source-badge { font-size: var(--ac-font-size-xs); padding: 1px 6px; border-radius: 4px; background: var(--ac-color-bg-secondary); }
</style>