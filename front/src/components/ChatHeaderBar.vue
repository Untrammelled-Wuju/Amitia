<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header">
    <div class="header-leading">
      <div class="header-info">
        <span class="header-conv-name">{{ convTitle || "新对话" }}</span>
        <span class="header-brand-desc">Amitia · AI 陪伴角色</span>
      </div>
    </div>

    <div class="header-actions">
      <slot name="extension-actions" />
      <el-dropdown trigger="click" @command="handleCallCommand">
        <button
          class="header-icon-btn"
          :class="{ active: callActive }"
          type="button"
          :aria-label="callActive ? '通话中' : '发起通话'"
          :title="callActive ? '通话中' : '发起通话'"
        >
          <el-icon><Phone /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="voice">
              <el-icon><Microphone /></el-icon> 语音通话
            </el-dropdown-item>
            <el-dropdown-item command="video">
              <el-icon><VideoCamera /></el-icon> 视频通话
            </el-dropdown-item>
            <el-dropdown-item command="screen">
              <el-icon><Monitor /></el-icon> 屏幕通话
            </el-dropdown-item>
            <el-dropdown-item v-if="callActive" command="end" divided>
              <el-icon><CircleClose /></el-icon> 结束通话
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <button class="header-icon-btn" :class="{ active: showProfiles }" type="button" title="用户画像" aria-label="用户画像" @click="$emit('toggleProfiles')">
        <el-icon><User /></el-icon>
      </button>
      <button class="header-icon-btn" :class="{ active: showMemInject }" type="button" title="记忆注入" aria-label="记忆注入" @click="$emit('toggleMemInject')">
        <el-icon><Connection /></el-icon>
      </button>
      <el-dropdown trigger="click" @command="handleMoreCommand">
        <button class="header-icon-btn" type="button" aria-label="更多" title="更多">
          <el-icon><MoreFilled /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item @click="$emit('clear')" :disabled="messagesCount === 0">
              <el-icon><Delete /></el-icon> 清空会话
            </el-dropdown-item>
            <el-dropdown-item v-if="convId" divided @click="$emit('viewMemories')">
              <el-icon><Collection /></el-icon> 查看相关记忆
            </el-dropdown-item>
            <el-dropdown-item command="summary" :disabled="!hasSummary">
              <el-icon><Document /></el-icon> 会话摘要
            </el-dropdown-item>
            <el-dropdown-item @click="$emit('toggleCharPicker')">
              <el-icon><Switch /></el-icon> 选择角色
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import {
  MoreFilled,
  Delete,
  Collection,
  Switch,
  User,
  Connection,
  Phone,
  Microphone,
  VideoCamera,
  Monitor,
  CircleClose,
  Document,
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
  hasSummary?: boolean;
}>();

const emit = defineEmits<{
  clear: [];
  viewMemories: [];
  toggleCharPicker: [];
  toggleProfiles: [];
  toggleMemInject: [];
  toggleCall: [];
  startCall: [mode: "voice" | "video" | "screen"];
  viewSummary: [];
}>();

function handleCallCommand(command: string | number | object) {
  if (command === "end") {
    emit("toggleCall");
    return;
  }
  if (command === "voice" || command === "video" || command === "screen") {
    emit("startCall", command);
  }
}

function handleMoreCommand(command: string | number | object) {
  if (command === "summary") {
    emit("viewSummary");
  }
}
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
.header-conv-name { overflow: hidden; color: var(--text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.header-brand-desc { margin-top: 1px; overflow: hidden; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.header-actions { display: flex; align-items: center; gap: 2px; flex: 0 0 auto; }
.header-icon-btn { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 15px; }
.header-icon-btn:hover, .header-icon-btn:focus-visible { background: var(--control-hover-bg); color: var(--text-primary); outline: none; }
.header-icon-btn.active { background: var(--control-active-bg); color: var(--ac-color-primary); }
@media (max-width: 760px) {
  .chat-header { padding-inline: 8px; }
  .header-brand-desc { display: none; }
  .header-info { max-width: 120px; }
}
</style>
