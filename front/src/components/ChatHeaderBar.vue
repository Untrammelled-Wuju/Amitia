<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header">
    <div class="header-leading">
      <button class="header-icon-btn" type="button" aria-label="打开对话与角色" title="对话与角色" @click="$emit('toggleDrawer')">
        <el-icon><Menu /></el-icon>
      </button>
      <el-avatar class="header-avatar" :size="28" :src="charAvatar || undefined">{{ (charName || "A").charAt(0) }}</el-avatar>
      <div class="header-info">
        <span class="header-char-name">{{ charName || "选择角色" }}</span>
        <span v-if="charName" class="header-char-desc">{{ charIdentity || "AI 陪伴角色" }}</span>
      </div>
    </div>

    <div v-if="convTitle" class="header-conv-title" :title="convTitle">{{ convTitle }}</div>

    <div class="header-actions">
      <slot name="extension-actions" />
      <button
        class="header-icon-btn"
        :class="{ active: callActive }"
        type="button"
        :aria-label="callActive ? '结束语音通话' : '开始语音通话'"
        :title="callActive ? '结束语音通话' : '开始语音通话'"
        @click="$emit('toggleCall')"
      >
        <el-icon><Phone /></el-icon>
      </button>
      <button class="header-icon-btn" :class="{ active: showProfiles }" type="button" title="用户画像" aria-label="用户画像" @click="$emit('toggleProfiles')">
        <el-icon><User /></el-icon>
      </button>
      <button class="header-icon-btn" :class="{ active: showMemInject }" type="button" title="记忆注入" aria-label="记忆注入" @click="$emit('toggleMemInject')">
        <el-icon><Connection /></el-icon>
      </button>
      <el-dropdown trigger="click">
        <button class="header-icon-btn" type="button" aria-label="更多" title="更多">
          <el-icon><MoreFilled /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item v-if="canRegenerate" @click="$emit('regenerate')">
              <el-icon><Refresh /></el-icon> 重新生成
            </el-dropdown-item>
            <el-dropdown-item @click="$emit('clear')" :disabled="messagesCount === 0">
              <el-icon><Delete /></el-icon> 清空会话
            </el-dropdown-item>
            <el-dropdown-item v-if="convId" divided @click="$emit('viewMemories')">
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
  Refresh,
} from "@element-plus/icons-vue";

defineProps<{
  charName: string;
  charAvatar?: string;
  charIdentity: string;
  convTitle: string;
  canRegenerate?: boolean;
  messagesCount: number;
  convId: string;
  showProfiles: boolean;
  showMemInject: boolean;
  callActive: boolean;
}>();

defineEmits<{
  toggleDrawer: [];
  regenerate: [];
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
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 5px 14px;
  background: transparent;
  border-bottom: 1px solid transparent;
  flex-shrink: 0;
}
.header-leading { display: flex; align-items: center; min-width: 0; gap: 8px; }
.header-avatar { flex: 0 0 auto; background: var(--control-active-bg); color: var(--text-primary); font-size: 11px; }
.header-info { display: flex; flex-direction: column; justify-content: center; min-width: 0; max-width: 280px; }
.header-char-name { overflow: hidden; color: var(--text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.header-char-desc { margin-top: 1px; overflow: hidden; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.header-conv-title { position: absolute; left: 50%; top: 50%; max-width: min(38vw, 460px); transform: translate(-50%, -50%); overflow: hidden; color: var(--text-muted); font-size: 12px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; pointer-events: none; }
.header-actions { display: flex; align-items: center; gap: 2px; flex: 0 0 auto; }
.header-icon-btn { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 15px; }
.header-icon-btn:hover, .header-icon-btn:focus-visible { background: var(--control-hover-bg); color: var(--text-primary); outline: none; }
.header-icon-btn.active { background: var(--control-active-bg); color: var(--ac-color-primary); }
@media (max-width: 760px) {
  .chat-header { padding-inline: 8px; }
  .header-char-desc, .header-conv-title { display: none; }
  .header-info { max-width: 120px; }
}
</style>
