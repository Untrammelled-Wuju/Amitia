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
    </div>
    <div class="header-actions">
      <button class="fa-btn" :class="{ active: showProfiles }" @click="$emit('toggleProfiles')" title="显示画像"><el-icon :size="18"><User /></el-icon></button>
      <button class="fa-btn" :class="{ active: showMemInject }" @click="$emit('toggleMemInject')" title="记忆注入"><el-icon :size="18"><Connection /></el-icon></button>
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
import { Menu, MoreFilled, Refresh, Delete, Collection, Switch, User, Connection } from "@element-plus/icons-vue"

defineProps<{
  charName: string
  charIdentity: string
  convTitle: string
  canRegenerate: boolean
  messagesCount: number
  convId: string
  showProfiles: boolean
  showMemInject: boolean
}>()

defineEmits<{
  toggleDrawer: []
  regenerate: []
  clear: []
  viewMemories: []
  toggleCharPicker: []
  toggleProfiles: []
  toggleMemInject: []
}>()
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
  display: flex;
  align-items: center;
  gap: 6px;
}

.fa-btn {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  border: 1px solid var(--ac-color-border);
  background: var(--ac-color-surface);
  color: var(--ac-color-text-muted);
  cursor: pointer;
  font-size: 14px;
  box-shadow: var(--ac-shadow-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.fa-btn:hover {
  color: var(--ac-color-text);
  border-color: var(--ac-color-text-muted);
}

.fa-btn.active {
  background: var(--ac-color-primary-bg);
  border-color: var(--ac-color-primary);
  color: var(--ac-color-primary);
}
</style>
