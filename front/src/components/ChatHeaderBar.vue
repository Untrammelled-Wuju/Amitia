<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header">
    <el-button :icon="Menu" text circle size="small" class="menu-btn" @click="$emit('toggleDrawer')" />
    <div class="header-info">
      <span class="header-char-name">{{ charName || "选择角色" }}</span>
      <span class="header-char-desc" v-if="charName">{{ charIdentity || '暂无角色描述' }}</span>
      <span class="header-conv-title" v-if="convTitle">{{ convTitle }}</span>
    </div>
    <div class="header-style-select">
      <el-dropdown trigger="click" @command="(v: string) => $emit('update:replyStyle', v)">
        <span class="style-trigger">
          {{ styleLabel(replyStyle) }}
          <el-icon class="style-arrow"><ArrowDown /></el-icon>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="s in REPLY_STYLES"
              :key="s.value"
              :command="s.value"
              :class="{ 'is-active': replyStyle === s.value }"
            >
              <span>{{ s.label }}</span>
              <el-icon v-if="replyStyle === s.value" class="style-check"><Check /></el-icon>
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
    <div class="header-actions">
      <el-dropdown trigger="click">
        <el-button text circle size="small" :icon="MoreFilled" />
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="$emit('regenerate')" :disabled="!canRegenerate">
              <el-icon><Refresh /></el-icon> 重新生成回复
            </el-dropdown-item>
            <el-dropdown-item @click="$emit('clear')" :disabled="messagesCount === 0">
              <el-icon><Delete /></el-icon> 清空会话
            </el-dropdown-item>
            <el-dropdown-item divided @click="$emit('viewMemories')" v-if="convId">
              <el-icon><Collection /></el-icon> 查看相关记忆
            </el-dropdown-item>
            <el-dropdown-item @click="$emit('toggleCharPicker')">
              <el-icon><Switch /></el-icon> 切换角色
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import { Menu, MoreFilled, Refresh, Delete, Collection, Switch, ArrowDown, Check } from "@element-plus/icons-vue"

defineProps<{
  charName: string
  charIdentity: string
  convTitle: string
  replyStyle: string
  canRegenerate: boolean
  messagesCount: number
  convId: string
}>()

defineEmits<{
  toggleDrawer: []
  "update:replyStyle": [value: string]
  regenerate: []
  clear: []
  viewMemories: []
  toggleCharPicker: []
}>()

const REPLY_STYLES = [
  { value: "natural", label: "默认自然" },
  { value: "shorter", label: "更简短" },
  { value: "gentler", label: "更温柔" },
  { value: "humorous", label: "更幽默" },
  { value: "rational", label: "更理性" },
  { value: "quiet_listening", label: "安静倾听" },
  { value: "encouraging", label: "鼓励一点" },
]

const styleLabel = (v: string) => {
  return REPLY_STYLES.find(s => s.value === v)?.label || "Natural"
}
</script>

<style scoped>
.chat-header {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.menu-btn {
  flex-shrink: 0;
}
.header-info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  flex: 1;
  min-width: 0;
  position: relative;
}

.header-char-name {
  display: block;
  font-size: var(--ac-font-size-base);
  font-weight: 600;
  color: var(--ac-color-text);
}

.header-char-desc {
  display: block;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-conv-title {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  white-space: nowrap;
}

.header-actions {
  flex-shrink: 0;
}

.header-style-select {
  flex-shrink: 0;
  margin-right: 4px;
}

.style-trigger {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  cursor: pointer;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  border: 1px solid var(--ac-color-border-light);
  transition: all var(--ac-transition-fast);
}

.style-trigger:hover {
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
}

.style-arrow {
  font-size: 10px;
  transition: transform var(--ac-transition-fast);
}

.style-check {
  margin-left: 8px;
  color: var(--ac-color-primary);
}
</style>

