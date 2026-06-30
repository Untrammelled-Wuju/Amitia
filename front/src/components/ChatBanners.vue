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
          <router-link to="/model" class="banner-link">去配置模型</router-link>
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
      <el-alert type="success" :closable="true" show-icon @close="$emit('closeImport')">
        <template #title>
          Chatting based on imported records
          <span class="import-badge">import</span>
        </template>
        <template #default v-if="showImportDetail">
          <div class="import-detail">
            <p v-if="importContext.summary" class="import-summary">{{ importContext.summary }}</p>
            <p v-if="importContext.memoryCount" class="import-memories">
              {{ importContext.memoryCount }} confirmed memories available
            </p>
          </div>
        </template>
      </el-alert>
    </div>

    <div v-if="convSummary" class="summary-banner">
      <el-alert type="info" :closable="false">
        <template #title>
          会话摘要
          <el-button text size="small" style="margin-left:8px" @click="$emit('toggleSummary')">
            {{ showSummary ? '收起' : '展开' }}
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
  modelMissing: boolean
  isOffline: boolean
  modelError: string
  importContext: any
  showImportDetail: boolean
  convSummary: string
  showSummary: boolean
}>()

defineEmits<{
  closeError: []
  closeImport: []
  toggleSummary: []
}>()
</script>

<style scoped>
.config-banner {
  flex-shrink: 0;
  margin: 0 0 8px;
}

.offline-banner,
.error-banner {
  flex-shrink: 0;
  margin: 0 0 8px;
}

.banner-link {
  font-weight: 600;
  text-decoration: underline;
  color: var(--el-color-warning-dark-2);
}

.import-banner {
  flex-shrink: 0;
  margin: 0 0 8px;
}

.import-badge {
  font-size: 10px;
  background: var(--ac-color-success);
  color: #fff;
  border-radius: 3px;
  padding: 0 4px;
  margin-left: 6px;
}

.import-detail {
  margin-top: 4px;
}

.import-summary {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  margin: 0;
}

.import-memories {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin: 2px 0 0;
}

.summary-banner {
  margin-bottom: 6px;
  flex-shrink: 0;
}

.summary-text {
  white-space: pre-wrap;
  font-size: var(--ac-font-size-xs);
  line-height: 1.6;
  color: var(--ac-color-text-secondary);
  margin-top: 4px;
}
</style>
