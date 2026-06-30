<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="logs-page">
    <h2 class="page-title">聊天记录</h2>

    <el-alert type="info" :closable="true" show-icon style="margin-bottom:14px">
      <template #title>聊天记录保存在你自己的设备或服务器上，你可随时删除。导出文件可能包含隐私信息，请妥善保管。</template>
    </el-alert>

    <div class="logs-layout">
      <ConversationListPanel
        :convs="convs"
        :conv-keyword="convKeyword"
        :channel-filter="channelFilter"
        :conv-page="convPage"
        :conv-total="convTotal"
        :selected-conv-id="selectedConvId"
        @update:conv-keyword="convKeyword = $event"
        @update:channel-filter="channelFilter = $event"
        @update:conv-page="convPage = $event"
        @search="fetchConvs"
        @filter-change="fetchConvs"
        @page-change="convPage = $event; fetchConvs()"
        @select="selectConv"
      />

      <main class="msg-detail" v-if="selectedConv">
        <div class="detail-header">
          <div class="dh-info">
            <span class="dh-title">{{ selectedConv.title || (selectedConv.channel === 'qq' ? 'QQ聊天' : selectedConv.channel === 'wechat' ? '微信聊天' : '新对话') }}</span>
            <span class="dh-meta">{{ channelLabel(selectedConv.channel) }} · {{ selectedConv.messageCount || 0 }}条</span>
          </div>
          <div class="dh-actions">
            <el-dropdown trigger="click">
              <el-button size="small">导出<el-icon style="margin-left:4px"><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="exportConv('markdown')">Markdown</el-dropdown-item>
                  <el-dropdown-item @click="exportConv('json')">JSON</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" @click="clearConv" :disabled="!messages.length">清空</el-button>
            <el-button size="small" @click="fetchContextPreview">上下文预览</el-button>
            <el-dropdown trigger="click" style="margin-left:4px">
              <el-button size="small">切换角色<el-icon style="margin-left:4px"><ArrowDown /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-for="c in characters"
                    :key="c.id"
                    @click="switchCharacter(c.id)"
                    :class="{ 'is-active': selectedConv?.characterId === c.id }"
                  >
                    {{ c.name }}
                    <el-tag size="small" type="success" v-if="c.isActive" style="margin-left:6px">当前</el-tag>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button size="small" @click="genSummary">Summary</el-button>
            <el-select v-model="continueCharId" placeholder="角色" size="small" style="width:120px" v-if="selectedConv?.source?.startsWith('import')">
              <el-option v-for="c in characters" :key="c.id" :label="c.name" :value="c.id" />
            </el-select>
            <el-button size="small" type="primary" @click="continueChat" v-if="selectedConv?.source?.startsWith('import')">转为对话</el-button>
            <el-button size="small" type="danger" @click="delConv">删除</el-button>
          </div>
        </div>

        <div class="detail-summary" v-if="currentSummary">
          <el-alert :closable="false" show-icon>
            <template #title>
              <span>会话摘要 ({{ fmtTime(currentSummary.updatedAt || currentSummary.createdAt) }})</span>
              <el-button text size="small" style="margin-left:8px" @click="viewSummary">详情</el-button>
              <el-button text size="small" type="danger" @click="delSummary">删除</el-button>
            </template>
          </el-alert>
          <el-dialog v-model="summaryVisible" title="会话摘要" width="500px">
            <div class="summary-content">{{ currentSummary?.summaryText }}</div>
            <div class="summary-meta" v-if="currentSummary?.updatedAt || currentSummary?.createdAt">
              生成时间: {{ fmtTime(currentSummary.updatedAt || currentSummary.createdAt) }}
            </div>
          </el-dialog>
        </div>

        <div class="detail-filters">
          <el-input v-model="roleFilter" placeholder="搜索消息" size="small" clearable style="width:160px" />
          <el-select v-model="roleFilter" placeholder="角色" size="small" clearable style="width:90px;margin-left:8px">
            <el-option label="用户" value="user" /><el-option label="AI" value="assistant" />
          </el-select>
        </div>

        <div class="msg-list" ref="msgListRef">
          <div v-for="m in filteredMessages" :key="m.id" class="msg-item" :class="m.role">
            <div class="mi-header">
              <span class="mi-role">{{ m.role === 'user' ? '用户' : 'AI' }}</span>
              <span class="mi-time">{{ fmtTime(m.createdAt) }}</span>
              <span class="mi-source" v-if="m.source">{{ m.source }}</span>
              <span class="mi-model" v-if="m.modelName">{{ m.modelName }}</span>
              <el-tag v-if="moodMap[m.id]" size="small" type="warning" class="mi-mood">{{ moodEmoji(moodMap[m.id]) }} {{ moodMap[m.id] }}</el-tag>
              <el-tag v-if="feedbackMap[m.id]?.length" size="small" type="success" class="mi-feedback">{{ feedbackMap[m.id][0].feedbackType }} ({{ feedbackMap[m.id].length }})</el-tag>
              <el-button text size="small" type="danger" class="mi-delete" @click="delMsg(m.id)">删除</el-button>
            </div>
            <div class="mi-content">{{ m.content }}</div>
            <div class="mi-metadata" v-if="devMode && m.metadata">
              <pre>{{ JSON.stringify(m.metadata, null, 2) }}</pre>
            </div>
          </div>
        </div>

        <el-pagination
          v-if="msgTotal > 50"
          :model-value="msgPage"
          :page-size="50"
          :total="msgTotal"
          layout="prev,next"
          size="small"
          @current-change="msgPage = $event; fetchMessages()"
          style="margin-top:8px;justify-content:center"
        />
      </main>

      <main class="msg-detail empty" v-else>
        <el-empty description="选择左侧会话查看详情" :image-size="60" />
      </main>
    </div>

    <ContextPreviewDialog
      v-model="ctxPreviewVisible"
      :loading="ctxPreviewLoading"
      :data="ctxPreview"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { ArrowDown } from "@element-plus/icons-vue"
import ConversationListPanel from "./components/ConversationListPanel.vue"
import ContextPreviewDialog from "./components/ContextPreviewDialog.vue"
import { useConversationLogs } from "./useConversationLogs"
import { channelLabel, fmtTime, moodEmoji } from "./utils"

const {
  characters,
  convs,
  convKeyword,
  continueCharId,
  channelFilter,
  convPage,
  convTotal,
  selectedConv,
  selectedConvId,
  messages,
  msgPage,
  msgTotal,
  roleFilter,
  msgListRef,
  filteredMessages,
  fetchConvs,
  selectConv,
  fetchMessages,
  delMsg,
  moodMap,
  feedbackMap,
  clearConv,
  delConv,
  exportConv,
  currentSummary,
  summaryVisible,
  genSummaryLoading,
  genSummary,
  viewSummary,
  delSummary,
  devMode,
  ctxPreviewVisible,
  ctxPreviewLoading,
  ctxPreview,
  fetchContextPreview,
  switchCharacter,
  continueChat,
  loadCharacters,
} = useConversationLogs()

onMounted(() => { fetchConvs(); loadCharacters() })
</script>

<style scoped>
.logs-page { padding: 0 24px 24px; }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin: 0 0 14px 0; color: var(--ac-color-text); }
.logs-layout { display: flex; gap: 0; height: calc(100vh - 200px); min-height: 400px; border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-md); overflow: hidden; }
.msg-detail { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-width: 0; padding: 12px; }
.msg-detail.empty { align-items: center; justify-content: center; }
.detail-header { display: flex; align-items: center; justify-content: space-between; padding-bottom: 8px; border-bottom: 1px solid var(--ac-color-border-light); flex-shrink: 0; }
.dh-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.dh-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-left: 10px; }
.dh-actions { display: flex; gap: 6px; }
.detail-filters { padding: 6px 0; flex-shrink: 0; }
.msg-list { flex: 1; overflow-y: auto; }
.msg-item { padding: 12px; border-bottom: 1px solid var(--ac-color-border-light); }
.msg-item.assistant { background: var(--ac-color-bg-secondary); }
.mi-header { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.mi-role { font-weight: 600; font-size: var(--ac-font-size-xs); }
.mi-time { font-size: 10px; color: var(--ac-color-text-muted); }
.mi-source { font-size: 10px; color: var(--ac-color-text-placeholder); background: var(--ac-color-surface); padding: 0 4px; border-radius: 3px; }
.mi-model { font-size: 10px; color: var(--ac-color-text-placeholder); }
.mi-delete { margin-left: auto; opacity: 0; transition: opacity var(--ac-transition-fast); }
.msg-item:hover .mi-delete { opacity: 1; }
.mi-content { font-size: var(--ac-font-size-sm); line-height: 1.6; white-space: pre-wrap; word-break: break-word; }
.mi-metadata { margin-top: 8px; padding: 8px; background: #1e1e1e; color: #d4d4d4; border-radius: 4px; }
.mi-metadata pre { margin: 0; font-size: 11px; font-family: Consolas, monospace; white-space: pre-wrap; word-break: break-all; }
.detail-summary { margin-bottom: 8px; }
.summary-content { white-space: pre-wrap; line-height: 1.7; font-size: var(--ac-font-size-sm); }
.summary-meta { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 10px; }

@media (max-width: 768px) {
  .logs-page { max-width: 100%; height: 100%; }
  .logs-layout { flex-direction: column; }
  .msg-detail { flex: 1; overflow: hidden; }
  .detail-header { flex-wrap: wrap; gap: 6px; }
  .dh-actions { width: 100%; overflow-x: auto; flex-wrap: nowrap; gap: 4px; }
  .dh-actions .el-button { white-space: nowrap; font-size: var(--ac-font-size-xs); }
  .msg-item { padding: 10px; }
  .mi-header { flex-wrap: wrap; gap: 4px; }
}
</style>
