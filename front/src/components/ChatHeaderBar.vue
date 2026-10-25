<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header" data-amitia-ui="app.header">
    <div class="header-leading">
      <div class="header-info">
        <span class="header-conv-name">{{ convTitle || "新对话" }}</span>
      </div>
      <span
        v-if="charAvatar || charName"
        class="header-character-avatar"
        :title="charName ? `当前角色：${charName}` : '当前角色'"
        :aria-label="charName ? `当前角色：${charName}` : '当前角色'"
      >
        <img v-if="charAvatar" :src="charAvatar" :alt="charName || '当前角色'" />
        <span v-else>{{ characterInitial }}</span>
      </span>
    </div>

    <div class="header-actions">
      <button
        v-if="convId"
        class="header-member-entry"
        type="button"
        :aria-label="isGroup ? `群成员，共 ${groupMembers.length} 人，添加角色` : '添加角色到当前会话'"
        :title="isGroup ? `群成员：${groupMembers.map((member) => member.characterName).join('、')}` : '添加角色到当前会话'"
        @click="$emit('manageGroup')"
      >
        <span v-if="isGroup && groupMembers.length" class="member-entry-avatars">
          <el-avatar v-for="member in groupMembers.slice(0, 3)" :key="member.memberId" :size="22" :src="member.characterAvatar || undefined">{{ member.characterName.slice(0, 1) }}</el-avatar>
        </span>
        <span v-else class="member-entry-icon"><el-icon><UserFilled /></el-icon></span>
        <span class="member-entry-label">{{ isGroup ? `${groupMembers.length} 位成员` : "添加角色" }}</span>
        <span class="member-entry-plus"><el-icon><Plus /></el-icon></span>
      </button>
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
        <el-dropdown-item command="voice" data-amitia-ui="component.list_item">
              <el-icon><Microphone /></el-icon> 语音通话
            </el-dropdown-item>
            <el-dropdown-item command="video" data-amitia-ui="component.list_item">
              <el-icon><VideoCamera /></el-icon> 视频通话
            </el-dropdown-item>
            <el-dropdown-item command="screen" data-amitia-ui="component.list_item">
              <el-icon><Monitor /></el-icon> 屏幕通话
            </el-dropdown-item>
            <el-dropdown-item v-if="callActive" command="end" data-amitia-ui="component.list_item" divided>
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
            <el-dropdown-item data-amitia-ui="component.list_item" @click="$emit('clear')" :disabled="messagesCount === 0">
              <el-icon><Delete /></el-icon> 清空会话
            </el-dropdown-item>
            <el-dropdown-item v-if="convId" data-amitia-ui="component.list_item" divided @click="$emit('viewMemories')">
              <el-icon><Collection /></el-icon> 查看相关记忆
            </el-dropdown-item>
            <el-dropdown-item command="summary" data-amitia-ui="component.list_item" :disabled="!hasSummary">
              <el-icon><Document /></el-icon> 会话摘要
            </el-dropdown-item>
            <el-dropdown-item data-amitia-ui="component.list_item" @click="$emit('toggleCharPicker')">
              <el-icon><Switch /></el-icon> 选择角色
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from "vue";
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
  UserFilled,
  Plus,
} from "@element-plus/icons-vue";
import type { OwnedGroupMember } from "@/runtime/device-owned-chat";

const props = withDefaults(defineProps<{
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
  isGroup?: boolean;
  groupMembers?: OwnedGroupMember[];
}>(), {
  groupMembers: () => [],
});
const characterInitial = computed(() => String(props.charName || "角").trim().slice(0, 1).toUpperCase());

const emit = defineEmits<{
  clear: [];
  viewMemories: [];
  toggleCharPicker: [];
  toggleProfiles: [];
  toggleMemInject: [];
  toggleCall: [];
  startCall: [mode: "voice" | "video" | "screen"];
  viewSummary: [];
  manageGroup: [];
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
.header-leading { display: flex; flex: 1; align-items: center; min-width: 0; gap: 8px; overflow: hidden; }
.header-avatar { flex: 0 0 auto; background: var(--control-active-bg); color: var(--text-primary); font-size: 11px; }
.header-info { display: flex; flex: 0 1 auto; flex-direction: column; justify-content: center; min-width: 0; max-width: none; }
.header-conv-name { overflow: hidden; color: var(--text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.header-character-avatar { display: grid; place-items: center; width: 20px; height: 20px; flex: 0 0 20px; overflow: hidden; border-radius: 50%; background: var(--control-active-bg); color: var(--text-primary); font-size: 9px; font-weight: 650; line-height: 1; }
.header-character-avatar img { width: 100%; height: 100%; object-fit: cover; }
.header-actions { display: flex; align-items: center; gap: 2px; flex: 0 0 auto; }
.header-icon-btn { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 15px; }
.header-icon-btn:hover, .header-icon-btn:focus-visible { background: var(--control-hover-bg); color: var(--text-primary); outline: none; }
.header-icon-btn.active { background: var(--control-active-bg); color: var(--text-primary); }
.header-member-entry { position: relative; display: inline-flex; align-items: center; gap: 6px; height: 32px; padding: 0 9px 0 5px; border: 1px solid color-mix(in srgb, var(--ac-color-primary) 28%, transparent); border-radius: 999px; background: color-mix(in srgb, var(--ac-color-primary) 10%, transparent); color: var(--ac-color-primary); cursor: pointer; font-size: 11px; font-weight: 650; }
.header-member-entry:hover, .header-member-entry:focus-visible { border-color: color-mix(in srgb, var(--ac-color-primary) 58%, transparent); background: color-mix(in srgb, var(--ac-color-primary) 15%, transparent); outline: none; }
.member-entry-avatars { display: flex; width: 44px; flex: 0 0 44px; }
.member-entry-avatars :deep(.el-avatar) { flex: 0 0 22px; width: 22px; height: 22px; margin-left: -7px; border: 2px solid var(--chat-surface-bg, #fff); font-size: 9px; }
.member-entry-avatars :deep(.el-avatar:first-child) { margin-left: 0; }
.member-entry-icon { display: grid; place-items: center; width: 24px; height: 24px; flex: 0 0 24px; border-radius: 50%; background: color-mix(in srgb, var(--ac-color-primary) 14%, transparent); }
.member-entry-label { max-width: 70px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.member-entry-plus { display: grid; place-items: center; width: 15px; height: 15px; flex: 0 0 15px; border-radius: 50%; background: var(--ac-color-primary); color: var(--tp-text-on-primary, #fff); font-size: 10px; }
@media (max-width: 760px) {
  .chat-header { padding-inline: 8px; }
  .header-info { max-width: 120px; }
  .header-member-entry { width: 36px; padding: 0 6px; justify-content: center; }
  .member-entry-label { display: none; }
  .member-entry-plus { position: absolute; margin: -20px 0 0 24px; }
}
</style>
