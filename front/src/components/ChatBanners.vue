<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-banners">
    <ConversationDividerLine v-if="modelMissing" class="chat-banner-line">
      模型未配置
      <router-link to="/settings/model" class="banner-link"
        >去配置模型</router-link
      >
    </ConversationDividerLine>

    <ConversationDividerLine v-if="isOffline" class="chat-banner-line">
      当前网络不可用，消息暂未发送
    </ConversationDividerLine>

    <ConversationDividerLine
      v-if="importContext"
      class="chat-banner-line"
    >
      基于导入记录继续对话
      <span class="import-badge">import</span>
      <button type="button" class="detail-btn" @click="$emit('toggleImportDetail')">{{ showImportDetail ? "收起" : "详情" }}</button>
      <button type="button" class="detail-btn" @click="$emit('closeImport')">关闭</button>
    </ConversationDividerLine>

    <div v-if="importContext && showImportDetail" class="import-detail">
      <p v-if="importContext.summary" class="import-summary">
        {{ importContext.summary }}
      </p>
      <p v-if="importContext.memoryCount" class="import-memories">
        {{ importContext.memoryCount }} 条导入记录可用于上下文
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import ConversationDividerLine from "./ConversationDividerLine.vue";

defineProps<{
  modelMissing: boolean;
  isOffline: boolean;
  importContext: any;
  showImportDetail: boolean;
}>();

defineEmits<{
  closeImport: [];
  toggleImportDetail: [];
}>();
</script>

<style scoped>
.chat-banners { display: contents; }
.chat-banner-line { margin-bottom: 12px; }
.banner-link { color: var(--ac-color-warning); font-weight: 600; text-decoration: none; }
.banner-link:hover { text-decoration: underline; }
.import-badge { margin-left: 6px; padding: 1px 5px; border-radius: 999px; background: var(--ac-color-success-bg); color: var(--ac-color-success); font-size: 9px; }
.detail-btn { margin-left: 8px; padding: 0; border: 0; background: transparent; color: var(--text-secondary); cursor: pointer; font: inherit; font-size: 10px; }
.detail-btn:hover { color: var(--text-primary); }
.import-detail { width: min(100%, 820px); margin: 0 auto 12px; text-align: center; }
.import-summary, .import-memories { margin: 0; color: var(--text-muted); font-size: 10px; }
.import-memories { margin-top: 2px; }
</style>
