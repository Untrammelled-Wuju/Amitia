<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-banners">
    <div v-if="modelMissing" class="config-banner">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>
          模型未配置 &mdash;
          <router-link to="/settings/model" class="banner-link"
            >去配置模型</router-link
          >
        </template>
      </el-alert>
    </div>

    <div v-if="isOffline" class="offline-banner">
      <el-alert type="warning" :closable="false" show-icon>
        <template #title>当前网络不可用，消息暂未发送</template>
      </el-alert>
    </div>

    <div v-if="modelError" class="error-banner">
      <el-alert type="error" closable show-icon @close="$emit('closeError')">
        <template #title>{{ modelError }}</template>
      </el-alert>
    </div>

    <div v-if="importContext" class="import-banner">
      <el-alert
        type="success"
        :closable="true"
        show-icon
        @close="$emit('closeImport')"
      >
        <template #title>
          基于导入记录继续对话
          <span class="import-badge">import</span>
          <button type="button" class="detail-btn" @click="$emit('toggleImportDetail')">{{ showImportDetail ? "收起" : "详情" }}</button>
        </template>
        <template #default v-if="showImportDetail">
          <div class="import-detail">
            <p v-if="importContext.summary" class="import-summary">
              {{ importContext.summary }}
            </p>
            <p v-if="importContext.memoryCount" class="import-memories">
              {{ importContext.memoryCount }} 条导入记录可用于上下文
            </p>
          </div>
        </template>
      </el-alert>
    </div>

    <div v-if="convSummary" class="summary-banner">
      <el-alert type="info" :closable="false">
        <template #title>
          会话摘要
          <el-button
            text
            size="small"
            style="margin-left: 8px"
            @click="$emit('toggleSummary')"
          >
            {{ showSummary ? "收起" : "展开" }}
          </el-button>
        </template>
        <template #default v-if="showSummary">
          <div class="summary-text">{{ convSummary }}</div>
        </template>
      </el-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  modelMissing: boolean;
  isOffline: boolean;
  modelError: string;
  importContext: any;
  showImportDetail: boolean;
  convSummary: string;
  showSummary: boolean;
}>();

defineEmits<{
  closeError: [];
  closeImport: [];
  toggleSummary: [];
  toggleImportDetail: [];
}>();
</script>

<style scoped>
.chat-banners { flex: 0 0 auto; padding: 8px 14px 0; }
.config-banner, .offline-banner, .error-banner, .import-banner, .summary-banner { flex-shrink: 0; margin: 0 0 6px; }
.chat-banners :deep(.el-alert) { min-height: 34px; padding: 6px 10px; border: 1px solid color-mix(in srgb, currentColor 12%, transparent); border-radius: 7px; }
.chat-banners :deep(.el-alert__title) { font-size: 11px; line-height: 1.4; }
.banner-link { color: var(--ac-color-warning); font-weight: 600; text-decoration: none; }
.banner-link:hover { text-decoration: underline; }
.import-badge { margin-left: 6px; padding: 1px 5px; border-radius: 999px; background: var(--ac-color-success-bg); color: var(--ac-color-success); font-size: 9px; }
.detail-btn { margin-left: 8px; padding: 0; border: 0; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 10px; }
.detail-btn:hover { color: var(--text-primary); }
.import-detail { margin-top: 4px; }
.import-summary, .import-memories { margin: 0; color: var(--text-muted); font-size: 10px; }
.import-memories { margin-top: 2px; }
.summary-text { margin-top: 4px; white-space: pre-wrap; color: var(--text-secondary); font-size: 11px; line-height: 1.55; }
@media (max-width: 768px) { .chat-banners { padding: 6px 8px 0; } }
</style>
