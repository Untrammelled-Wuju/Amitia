<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="import-page">
    <h2 class="page-title">导入聊天记录</h2>

    <el-alert type="warning" :closable="true" show-icon style="margin-bottom:12px">
      <template #title>
        导入内容可能包含隐私信息，需自行移除验证码、密码、银行卡号和身份证号再导入。
      </template>
    </el-alert>

    <ImportInputPanel
      :raw-text="rawText"
      :batch-title="batchTitle"
      :parse-format="parseFormat"
      :parsing="parsing"
      :show-speaker-options="showSpeakerOptions"
      :user-speaker-input="userSpeakerInput"
      :assistant-speaker-input="assistantSpeakerInput"
      @update:raw-text="rawText = $event"
      @update:batch-title="batchTitle = $event"
      @update:parse-format="parseFormat = $event"
      @update:show-speaker-options="showSpeakerOptions = $event"
      @update:user-speaker-input="userSpeakerInput = $event"
      @update:assistant-speaker-input="assistantSpeakerInput = $event"
      @parse="handleParse"
      @file-change="onFileChange"
    />

    <ImportPreviewPanel
      :parse-result="parseResult"
      :editable-items="editableItems"
    />

    <el-card shadow="never" class="section-card" v-if="parseResult">
      <template #header>
        <span class="step-badge">3</span> 确认导入
      </template>
      <div class="confirm-options">
        <el-checkbox v-model="genSummary">生成会话摘要</el-checkbox>
        <el-checkbox v-model="extractMemories">提取记忆候选项</el-checkbox>
        <span class="confirm-hint">
          将从导入的消息创建一个新的会话。
        </span>
      </div>
      <div style="margin-top:12px">
        <el-button
          type="primary"
          size="large"
          :loading="confirming"
          :disabled="editableItems.length === 0"
          @click="handleConfirm"
        >
          确认导入（{{ editableItems.length }} 条消息）
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card" v-if="importedBatchId">
      <template #header>
        <span class="step-badge">4</span> 导入后处理
      </template>
      <div class="post-actions">
        <el-button :loading="genSummaryLoading" @click="handleGenSummary" v-if="genSummary">
          生成摘要
        </el-button>
        <el-button :loading="extractLoading" @click="handleExtractMemories" v-if="extractMemories">
          提取记忆
        </el-button>
        <router-link v-if="importedConvId" :to="'/logs'" class="inline-link">
          查看已导入会话
        </router-link>
      </div>
      <div v-if="memCandidates.length > 0" style="margin-top:10px">
        <div v-for="c in memCandidates.slice(0, 5)" :key="c.key" class="mem-candidate">
          <el-tag size="small">{{ c.key }}</el-tag>
          <span class="mc-val">{{ c.value }}</span>
          <span class="mc-imp">重要性： {{ c.importance }}/10</span>
        </div>
        <el-button text size="small" @click="router.push('/memory')" v-if="memCandidates.length > 0">
          在记忆中管理
        </el-button>
      </div>
    </el-card>

    <ImportHistoryPanel
      :batches="batches"
      @view="viewBatch"
      @delete="delBatch"
    />

    <el-dialog v-model="detailVisible" title="批次详情" width="600px">
      <el-table :data="detailItems" size="small" max-height="350">
        <el-table-column prop="senderName" label="发言者" width="80" />
        <el-table-column prop="role" label="角色" width="70" />
        <el-table-column prop="content" label="内容" show-overflow-tooltip />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import ImportInputPanel from "./components/ImportInputPanel.vue"
import ImportPreviewPanel from "./components/ImportPreviewPanel.vue"
import ImportHistoryPanel from "./components/ImportHistoryPanel.vue"
import { useImportWizard } from "./useImportWizard"

const {
  rawText,
  batchTitle,
  parseFormat,
  parsing,
  showSpeakerOptions,
  userSpeakerInput,
  assistantSpeakerInput,
  parseResult,
  editableItems,
  confirming,
  genSummary,
  extractMemories,
  importedBatchId,
  importedConvId,
  genSummaryLoading,
  extractLoading,
  memCandidates,
  handleParse,
  onFileChange,
  handleConfirm,
  handleGenSummary,
  handleExtractMemories,
  batches,
  detailVisible,
  detailItems,
  fetchBatches,
  viewBatch,
  delBatch,
  router,
} = useImportWizard()

onMounted(fetchBatches)
</script>

<style scoped>
.import-page { }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin-bottom: 14px; color: var(--ac-color-text); }
.section-card { margin-bottom: 12px; }

.step-badge {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px; border-radius: 50%;
  background: var(--ac-color-primary); color: #fff;
  font-size: 11px; font-weight: 700; margin-right: 6px; flex-shrink: 0;
}

.confirm-options { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.confirm-hint { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); flex-basis: 100%; margin-top: 4px; }

.post-actions { margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--ac-color-border-light); display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
.inline-link { display: inline-block; margin-left: 8px; font-size: var(--ac-font-size-sm); color: var(--ac-color-primary); text-decoration: underline; }

.mem-candidate { display: flex; align-items: center; gap: 10px; padding: 6px 0; }
.mc-val { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mc-imp { font-size: 10px; color: var(--ac-color-text-muted); white-space: nowrap; }
</style>
