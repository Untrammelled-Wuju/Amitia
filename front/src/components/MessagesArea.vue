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
    <div
      class="pull-indicator"
      :class="{ pulling: isPulling, ready: pullReady }"
    >
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
      <ChatEmptyStateExtensionHost :context="extensionContext" />
    </div>

    <div v-for="msg in messages" :key="msg.id" :data-message-id="msg.id">
      <UIProviderHost
        capability="conversation.message_renderer"
        :provider-id="messageRendererId(msg)"
        :fallback="ChatBubble"
        :context="{ ...(extensionContext || {}), message: messageContext(msg) }"
        :actions="messageActions(msg)"
        :message="msg"
        :char-name="charName"
        :char-avatar="charAvatar"
        :character-id="characterId"
        :status="msg.status"
        @retry="$emit('retry', $event)"
        @reply="$emit('reply', $event)"
        @scroll-to-message="(id) => scrollToMessage(id)"
            ><template #badges><MessageBadgeExtensionHost :message-id="msg.id" :message-type="msg.type || 'text'" :direction="msg.role === 'user' ? 'outgoing' : msg.role === 'assistant' ? 'incoming' : 'system'" :sender-type="msg.role === 'user' ? 'user' : msg.role === 'assistant' ? 'character' : 'system'" :character-id="characterId" :conversation-id="msg.conversationId || ''" /></template><template #extension-content><MessageExtensionHost :message="{
          messageId: msg.id,
          type: msg.type || 'text',
          direction: msg.role === 'user' ? 'outgoing' : msg.role === 'assistant' ? 'incoming' : 'system',
          senderType: msg.role === 'user' ? 'user' : msg.role === 'assistant' ? 'character' : 'system',
          createdAt: msg.createdAt || '',
          status: msg.status || 'sent',
          hasText: !!(msg.content || msg.text),
          attachmentTypes: msg.attachments?.map((a: any) => a.type) || [],
          extensionType: msg.extensionType || msg.extension_type || '',
          content: msg.content || msg.text || '',
          metadata: msg.metadata || {},
        }"
        :character-id="characterId"
        :conversation-id="msg.conversationId || ''"
      /></template><template #actions><MessageActionExtensionHost :message-id="msg.id" :message-type="msg.type || 'text'" :direction="msg.role === 'user' ? 'outgoing' : msg.role === 'assistant' ? 'incoming' : 'system'" :sender-type="msg.role === 'user' ? 'user' : msg.role === 'assistant' ? 'character' : 'system'" :character-id="characterId" :conversation-id="msg.conversationId || ''" /></template></UIProviderHost>
    </div>

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
import { onMounted, ref } from "vue";
import { ChatDotRound, ArrowDown, Loading } from "@element-plus/icons-vue";
import ChatBubble from "./ChatBubble.vue";
import MessageExtensionHost from "./extension/chat/MessageExtensionHost.vue";
import ChatEmptyStateExtensionHost from "./extension/chat/ChatEmptyStateExtensionHost.vue";
import MessageActionExtensionHost from "./extension/chat/MessageActionExtensionHost.vue";
import MessageBadgeExtensionHost from "./extension/chat/MessageBadgeExtensionHost.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";

const props = defineProps<{
  messages: any[];
  charName: string;
  charAvatar: string;
  characterId: string;
  sending: boolean;
  showScrollBtn: boolean;
  isPulling: boolean;
  pullReady: boolean;
  pullLoading: boolean;
  pullText: string;
  extensionContext?: Record<string, unknown>;
  providerActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
}>();

const emit = defineEmits<{
  scroll: [];
  wheel: [e: WheelEvent];
  touchStart: [e: TouchEvent];
  touchMove: [e: TouchEvent];
  touchEnd: [];
  retry: [msg: any];
  reply: [msg: any];
  scrollToBottom: [];
}>();

const store = useExtensionUIStore();
function messageContext(msg: any) {
  return {
    messageId: msg.id,
    type: msg.type || "text",
    role: msg.role,
    status: msg.status,
    content: msg.content || msg.text || "",
    attachments: msg.attachments || [],
    metadata: msg.metadata || {},
  };
}
function messageRendererId(msg: any): string | undefined {
  return resolveMessageRenderer(
    store.getProviders("conversation.message_renderer"),
    store.getResolvedProvider("conversation.message_renderer"),
    msg,
    store.snapshot?.providerContext,
  )?.providerId;
}
function messageActions(msg: any) {
  return {
    ...(props.providerActions ?? {}),
    "conversation.retry": async () => emit("retry", msg),
    "conversation.reply": async () => emit("reply", msg),
    "conversation.scrollToMessage": async (input?: unknown) => scrollToMessage(String((input as any)?.messageId ?? input ?? msg.id)),
  };
}
onMounted(() => { if (!store.snapshot) void store.refreshSnapshot(); });

function scrollToMessage(messageId: string) {
  if (!rootEl.value) return;
  const el = rootEl.value.querySelector(`[data-message-id="${messageId}"]`);
  if (el) {
    el.scrollIntoView({ behavior: "smooth", block: "center" });
    el.classList.add("highlight-flash");
    setTimeout(() => el.classList.remove("highlight-flash"), 2000);
  }
}

const rootEl = ref<HTMLElement>();
defineExpose({ rootEl });
</script>

<style scoped>
.messages-area {
  width: 100%;
  margin: 0 auto;
  flex: 1 1 0;
  min-height: 0;
  overflow-y: scroll;
  overscroll-behavior-y: contain;
  padding: 0 32px 28px;
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
  margin-bottom: 14px;
  opacity: 0.58;
}

.empty-text {
  margin-bottom: 7px;
  color: var(--text-primary);
  font-size: 18px;
  font-weight: 520;
  letter-spacing: -0.2px;
}

.empty-hint {
  max-width: 360px;
  color: var(--text-muted);
  font-size: 12px;
}

.empty-chat :deep(.extension-slot) { width: min(100%, 680px); margin-top: 20px; }

.messages-area > [data-message-id] { width: min(100%, 820px); margin: 0 auto; }
@media (max-width: 768px) { .messages-area { padding: 12px 8px; } }

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
  padding: 0;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  transition: opacity var(--ac-transition-fast);
  opacity: 0;
  height: 0;
  overflow: hidden;
}

.pull-indicator.pulling {
  opacity: 0.6;
  height: 36px;
  padding: 8px 0;
}
.pull-indicator.ready {
  opacity: 1;
  color: var(--ac-color-primary);
  height: 36px;
  padding: 8px 0;
}

.pull-icon.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 768px) {
  .messages-area {
    padding: 12px 12px;
  }
}

.highlight-flash {
  animation: highlight-fade 2s ease-out;
}

@keyframes highlight-fade {
  0% {
    background-color: var(--tp-primary-light-9);
  }
  100% {
    background-color: transparent;
  }
}
</style>
