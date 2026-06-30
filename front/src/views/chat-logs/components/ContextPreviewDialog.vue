<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog v-model="visible" title="上下文预览" width="700px" top="5vh" :close-on-click-modal="false">
    <div v-if="loading" style="text-align:center;padding:40px">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p style="margin-top:12px">加载中...</p>
    </div>
    <div v-else-if="data" class="ctx-preview">
      <div class="ctxp-section">
        <div class="ctxp-label">角色</div>
        <div class="ctxp-value">{{ data.character?.name }} ({{ data.character?.identity }})</div>
      </div>
      <div class="ctxp-section">
        <div class="ctxp-label">最近消息</div>
        <div class="ctxp-value">{{ data.recentMessageCount }} 条 | 估算总字符: {{ data.estimatedChars }}</div>
      </div>
      <div class="ctxp-section" v-if="data.usedMemories?.length">
        <div class="ctxp-label">使用记忆 ({{ data.usedMemories.length }}条)</div>
        <div class="ctxp-memories">
          <div v-for="(m, i) in data.usedMemories" :key="i" class="ctxp-memory-item">
            <el-tag size="small" type="info">{{ m.memoryType }}</el-tag>
            <span>{{ m.key }}: {{ m.value }}</span>
          </div>
        </div>
      </div>
      <div class="ctxp-section" v-if="data.usedSummary">
        <div class="ctxp-label">会话摘要</div>
        <div class="ctxp-value ctxp-pre">{{ data.usedSummary }}</div>
      </div>
      <div class="ctxp-section" v-if="data.usedImportContext">
        <div class="ctxp-label">导入背景</div>
        <div class="ctxp-value ctxp-pre">{{ data.usedImportContext }}</div>
      </div>
      <div class="ctxp-section">
        <div class="ctxp-label">System Prompt 预览</div>
        <div class="ctxp-value ctxp-pre ctxp-prompt">{{ data.promptPreview }}</div>
      </div>
      <div class="ctxp-section">
        <div class="ctxp-label">最近消息内容</div>
        <div class="ctxp-messages">
          <div v-for="(m, i) in data.recentMessages" :key="i" class="ctxp-msg-item" :class="m.role">
            <span class="ctxp-msg-role">{{ m.role === 'user' ? '用户' : 'AI' }}</span>
            <span class="ctxp-msg-content">{{ m.content }}</span>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { Loading } from "@element-plus/icons-vue"

const visible = defineModel<boolean>({ required: true })

defineProps<{
  loading: boolean
  data: any
}>()
</script>

<style scoped>
.ctx-preview { display: flex; flex-direction: column; gap: 12px; max-height: 60vh; overflow-y: auto; }
.ctxp-label { font-weight: 600; font-size: 13px; color: var(--ac-color-text-secondary); margin-bottom: 4px; }
.ctxp-value { font-size: 13px; color: var(--ac-color-text); }
.ctxp-pre { white-space: pre-wrap; word-break: break-word; font-size: 12px; background: var(--ac-color-bg-secondary); padding: 8px 10px; border-radius: var(--ac-radius-sm); max-height: 150px; overflow-y: auto; }
.ctxp-prompt { max-height: 200px; font-family: monospace; font-size: 11px; }
.ctxp-memories { display: flex; flex-direction: column; gap: 4px; }
.ctxp-memory-item { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.ctxp-messages { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.ctxp-msg-item { display: flex; gap: 8px; padding: 4px 8px; border-radius: var(--ac-radius-sm); font-size: 12px; }
.ctxp-msg-item.user { background: #e8f4fd; }
.ctxp-msg-item.assistant { background: #f5f5f5; }
.ctxp-msg-role { font-weight: 600; flex-shrink: 0; min-width: 30px; }
.ctxp-msg-content { word-break: break-word; }
</style>
