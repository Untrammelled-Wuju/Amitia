<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header">
    <el-button
      :icon="Menu"
      text
      circle
      size="small"
      class="menu-btn"
      @click="$emit('toggleDrawer')"
    />
    <el-avatar class="header-avatar" :size="34" :src="charAvatar || undefined">{{ (charName || "A").charAt(0) }}</el-avatar>
    <div class="header-info">
      <span class="header-char-name">{{ charName || "选择角色" }}</span>
      <span class="header-char-desc" v-if="charName">{{
        charIdentity || "暂无角色描述"
      }}</span>
      <span class="header-conv-title" v-if="convTitle">{{ convTitle }}</span>
    </div>
    <div class="header-actions">
      <slot name="extension-actions" />
      <button class="fa-btn" :class="{ active: callActive }" type="button" :aria-label="callActive ? '结束语音通话' : '开始语音通话'" :title="callActive ? '结束语音通话' : '开始语音通话'" @click="$emit('toggleCall')">
        <el-icon :size="18"><Phone /></el-icon>
      </button>
      <button
        class="fa-btn"
        :class="{ active: showProfiles }"
        @click="$emit('toggleProfiles')"
        title="显示画像"
      >
        <el-icon :size="18"><User /></el-icon>
      </button>
      <button
        class="fa-btn"
        :class="{ active: showMemInject }"
        @click="$emit('toggleMemInject')"
        title="记忆注入"
      >
        <el-icon :size="18"><Connection /></el-icon>
      </button>
      <el-dropdown trigger="click">
        <el-button text circle size="small" :icon="MoreFilled" />
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              @click="$emit('clear')"
              :disabled="messagesCount === 0"
            >
              <el-icon><Delete /></el-icon> 清空会话
            </el-dropdown-item>
            <el-dropdown-item
              divided
              @click="$emit('viewMemories')"
              v-if="convId"
            >
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
import {
  Menu,
  MoreFilled,
  Delete,
  Collection,
  Switch,
  User,
  Connection,
  Phone,
} from "@element-plus/icons-vue";

defineProps<{
  charName: string;
  charAvatar?: string;
  charIdentity: string;
  convTitle: string;
  messagesCount: number;
  convId: string;
  showProfiles: boolean;
  showMemInject: boolean;
  callActive: boolean;
}>();

defineEmits<{
  toggleDrawer: [];
  clear: [];
  viewMemories: [];
  toggleCharPicker: [];
  toggleProfiles: [];
  toggleMemInject: [];
  toggleCall: [];
}>();
</script>

<style scoped>
.chat-header {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 58px;
  padding: 8px 14px;
  background: var(--chat-header-bg);
  flex-shrink: 0;
  border-bottom: 1px solid var(--surface-border);
}

.menu-btn {
  flex-shrink: 0;
}
.header-avatar { flex: 0 0 auto; background: var(--control-active-bg); color: var(--text-primary); }
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
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.fa-btn:hover {
  color: var(--text-primary);
  border-color: var(--surface-border);
  background: var(--control-hover-bg);
}
.fa-btn:focus-visible { outline: 2px solid var(--surface-border-focus); outline-offset: 2px; }

.fa-btn.active {
  background: var(--ac-color-primary-bg);
  border-color: var(--ac-color-primary);
  color: var(--ac-color-primary);
}
</style>
