<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div
    ref="rootEl"
    class="messages-area"
    @scroll="$emit('scroll')"
    @wheel="$emit('wheel', $event)"
    @touchstart="$emit('touchStart', $event)"
    @touchmove="$emit('touchMove', $event)"
    @touchend="$emit('touchEnd')"
  >
    <div class="pull-indicator" :class="{ pulling: isPulling, ready: pullReady }">
      <el-icon :size="18" class="pull-icon" :class="{ spin: pullLoading }">
        <Loading v-if="pullLoading" />
        <ArrowDown v-else />
      </el-icon>
      <span>{{ pullText }}</span>
    </div>

    <div v-if="messages.length === 0 && !sending" class="empty-chat">
      <div class="empty-icon">
        <el-icon :size="48"><ChatDotRound /></el-icon>
      </div>
      <p class="empty-text">你好，我是 {{ charName || "AI 陪伴角色" }}</p>
      <p class="empty-hint">随时可以和我聊聊天，我在这里陪你。</p>
    </div>

    <ChatBubble
      v-for="msg in messages"
      :key="msg.id"
      :message="msg"
      :char-name="charName"
      :char-avatar="charAvatar"
      :is-streaming="msg.id === 'streaming'"
      :status="msg.status"
      @retry="$emit('retry', $event)"
    />

    <transition name="fade">
      <el-button
        v-if="showScrollBtn"
        :icon="ArrowDown"
        circle
        size="small"
        class="scroll-btn"
        @click="$emit('scrollToBottom')"
      />
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ChatDotRound, ArrowDown, Loading } from "@element-plus/icons-vue"
import ChatBubble from "./ChatBubble.vue"

defineProps<{
  messages: any[]
  charName: string
  charAvatar: string
  sending: boolean
  showScrollBtn: boolean
  isPulling: boolean
  pullReady: boolean
  pullLoading: boolean
  pullText: string
}>()

defineEmits<{
  scroll: []
  wheel: [e: WheelEvent]
  touchStart: [e: TouchEvent]
  touchMove: [e: TouchEvent]
  touchEnd: []
  retry: [msg: any]
  scrollToBottom: []
}>()

const rootEl = ref<HTMLElement>()
defineExpose({ rootEl })
</script>

<style scoped>
.messages-area {
  align-self: center;
  width: 100%;
  margin: 0 auto;
  flex: 1 1 0;
  min-height: 0;
  overflow-y: scroll;
  overscroll-behavior-y: contain;
  padding: 12px 16px;
  position: relative;
}

.empty-chat {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  padding: 40px 20px;
}

.empty-icon {
  color: var(--ac-color-text-muted);
  margin-bottom: 16px;
  opacity: 0.5;
}

.empty-text {
  font-size: var(--ac-font-size-lg);
  color: var(--ac-color-text-secondary);
  margin-bottom: 8px;
}

.empty-hint {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
  max-width: 280px;
}

.scroll-btn {
  position: sticky;
  bottom: 8px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  box-shadow: var(--ac-shadow-md);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--ac-transition-fast);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.pull-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 0;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  transition: opacity var(--ac-transition-fast);
  opacity: 0;
  height: 36px;
}

.pull-indicator.pulling { opacity: 0.6; }
.pull-indicator.ready { opacity: 1; color: var(--ac-color-primary); }

.pull-icon.spin { animation: spin 0.8s linear infinite; }

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .messages-area {
    padding: 12px 12px;
  }
}
</style>
