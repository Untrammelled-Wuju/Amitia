<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <aside class="conv-sidebar">
    <div class="sidebar-toolbar">
      <el-input :model-value="convKeyword" @update:model-value="$emit('update:convKeyword', $event)" placeholder="搜索..." size="small" clearable @clear="$emit('search')" @keyup.enter="$emit('search')">
        <template #prefix><el-icon><Search /></el-icon></template>
      </el-input>
    </div>
    <div class="sidebar-filters">
      <el-select :model-value="channelFilter" @update:model-value="$emit('update:channelFilter', $event)" placeholder="频道" size="small" clearable @change="$emit('filterChange')">
        <el-option v-for="ch in CHANNELS" :key="ch.value" :label="ch.label" :value="ch.value" />
      </el-select>
    </div>
    <div class="conv-list" v-if="convs.length > 0">
      <div
        v-for="c in convs"
        :key="c.id"
        class="conv-item"
        :class="{ active: selectedConvId === c.id }"
        @click="$emit('select', c)"
      >
        <div class="ci-title">{{ c.title || (c.channel === 'qq' ? 'QQ聊天' : c.channel === 'wechat' ? '微信聊天' : '新对话') }}</div>
        <div class="ci-meta">
          <el-tag size="small" type="info">{{ channelLabel(c.channel) }}</el-tag>
          <span>{{ c.messageCount || 0 }}条</span>
          <span class="ci-time">{{ fmtShort(c.updatedAt || c.createdAt) }}</span>
        </div>
        <div class="ci-preview" v-if="c.lastMessage">{{ c.lastMessage }}</div>
      </div>
    </div>
    <el-empty v-else description="暂无会话" :image-size="50" />
    <el-pagination
      v-if="convTotal > 20"
      :model-value="convPage"
      :page-size="20"
      :total="convTotal"
      layout="prev,next"
      size="small"
      @current-change="$emit('pageChange', $event)"
      style="margin-top:8px;justify-content:center"
    />
  </aside>
</template>

<script setup lang="ts">
import { Search } from "@element-plus/icons-vue"
import { CHANNELS, channelLabel, fmtShort } from "../utils"

defineProps<{
  convs: any[]
  convKeyword: string
  channelFilter: string
  convPage: number
  convTotal: number
  selectedConvId: string
}>()

defineEmits<{
  'update:convKeyword': [value: string]
  'update:channelFilter': [value: string]
  'update:convPage': [value: number]
  search: []
  filterChange: []
  pageChange: [page: number]
  select: [conv: any]
}>()
</script>

<style scoped>
.conv-sidebar {
  width: 280px; flex-shrink: 0; overflow-y: auto;
  border-right: 1px solid var(--ac-color-border-light);
  padding: 8px;
  display: flex; flex-direction: column;
}
.sidebar-toolbar { margin-bottom: 6px; }
.sidebar-filters { margin-bottom: 8px; }
.conv-list { flex: 1; overflow-y: auto; }
.conv-item {
  padding: 8px; cursor: pointer;
  border-radius: var(--ac-radius-sm); margin-bottom: 4px;
  transition: background var(--ac-transition-fast);
}
.conv-item:hover { background: var(--ac-color-surface-hover); }
.conv-item.active { background: var(--ac-color-primary-bg); border-left: 3px solid var(--ac-color-primary); }
.ci-title { font-size: var(--ac-font-size-sm); font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ci-meta { display: flex; gap: 6px; align-items: center; margin-top: 3px; font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }
.ci-preview { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); margin-top: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ci-time { margin-left: auto; }
</style>
