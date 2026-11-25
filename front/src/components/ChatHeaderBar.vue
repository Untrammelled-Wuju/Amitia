<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <header class="chat-header" data-amitia-ui="app.header">
    <div class="header-leading">
      <ExtensionSlot
        slot-id="chat.header.leading"
        :context="headerSlotContext"
        fallback="none"
        layout="inline"
        surface-role="header"
      />
      <div class="header-info">
        <ExtensionSlot
          slot-id="chat.header.lineage"
          :context="headerSlotContext"
          fallback="default"
          layout="inline"
          surface-role="header"
          render-fallback-while-scope-pending
        >
          <span class="header-conv-name">{{ displayTitle }}</span>
        </ExtensionSlot>
      </div>
      <div v-if="isGroup && groupMembers.length" class="header-group-avatars" role="group" aria-label="参与角色">
        <ChatHeaderAvatar
          v-for="member in groupMembers"
          :key="member.memberId"
          :character-id="member.characterId"
          :name="member.characterName"
          :avatar="member.characterAvatar"
        />
      </div>
      <ChatHeaderAvatar
        v-else-if="charAvatar || charName"
        :character-id="characterId"
        :name="charName || '当前角色'"
        :avatar="charAvatar"
      />
    </div>

    <div class="header-actions">
      <button
        v-if="convId"
        class="header-member-entry"
        type="button"
        :aria-label="memberActionLabel"
        :title="memberActionLabel"
        @click="$emit('manageGroup')"
      >
        <el-icon><Plus /></el-icon>
        <span class="member-entry-label">添加角色</span>
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
      <ExtensionSlot
        slot-id="chat.header.corner"
        :context="headerSlotContext"
        fallback="none"
        layout="inline"
        surface-role="header"
      />
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
  Plus,
} from "@element-plus/icons-vue";
import type { OwnedGroupMember } from "@/runtime/device-owned-chat";
import ChatHeaderAvatar from "./ChatHeaderAvatar.vue";
import ExtensionSlot from "./extension/ExtensionSlot.vue";

const props = withDefaults(defineProps<{
  characterId?: string;
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
const memberActionLabel = computed(() => props.isGroup ? "添加角色或管理群成员" : "添加角色到当前会话");
const displayTitle = computed(() => props.convId ? props.convTitle.trim() || "新对话" : "新对话");
const headerSlotContext = computed(() => ({
  characterId: "",
  conversationId: props.convId,
  sessionId: props.convId,
  lineageSessionId: props.convId,
  displayTitle: displayTitle.value,
  title: displayTitle.value,
  characterName: props.charName,
  characterAvatar: props.charAvatar,
  messagesCount: props.messagesCount,
  isGroup: props.isGroup === true,
  callActive: props.callActive,
}));

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
.header-info { display: flex; flex: 0 1 auto; flex-direction: column; justify-content: center; min-width: 0; max-width: none; }
.header-conv-name { overflow: hidden; color: var(--text-primary); font-size: 13px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.header-group-avatars { display: flex; align-items: center; flex: 0 0 auto; }
.header-group-avatars .header-character-avatar { border: 1px solid var(--chat-surface-bg, var(--surface-bg)); }
.header-group-avatars .header-character-avatar + .header-character-avatar { margin-left: -4px; }
.header-actions { display: flex; align-items: center; gap: 2px; flex: 0 0 auto; }
.header-icon-btn { display: grid; place-items: center; width: 30px; height: 30px; padding: 0; border: 0; border-radius: 7px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: 15px; }
.header-icon-btn:hover, .header-icon-btn:focus-visible { background: var(--control-hover-bg); color: var(--text-primary); outline: none; }
.header-icon-btn.active { background: var(--control-active-bg); color: var(--text-primary); }
.header-member-entry { display: inline-flex; align-items: center; justify-content: center; gap: 3px; height: 26px; padding: 0 6px; border: 1px solid color-mix(in srgb, var(--ac-color-primary) 28%, transparent); border-radius: 7px; background: color-mix(in srgb, var(--ac-color-primary) 10%, transparent); color: var(--ac-color-primary); cursor: pointer; font-size: 11px; font-weight: 650; white-space: nowrap; }
.header-member-entry:hover, .header-member-entry:focus-visible { border-color: color-mix(in srgb, var(--ac-color-primary) 58%, transparent); background: color-mix(in srgb, var(--ac-color-primary) 15%, transparent); outline: none; }
@media (max-width: 760px) {
  .chat-header { padding-inline: 8px; }
  .header-info { max-width: 120px; }
  .header-group-avatars .header-character-avatar + .header-character-avatar { margin-left: -6px; }
  .header-member-entry { width: 26px; padding: 0; }
  .member-entry-label { display: none; }
}
</style>
