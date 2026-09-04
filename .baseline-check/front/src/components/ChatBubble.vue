<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-bubble" :class="[message.role, { 'is-emote': isEmote }]">
    <div class="bubble-avatar">
      <el-avatar
        :size="32"
        :src="
          message.role === 'assistant'
            ? charAvatar
            : appStore.avatar || undefined
        "
      >
        <el-icon v-if="message.role === 'user'"><UserFilled /></el-icon>
        <template v-else>{{ charInitial }}</template>
      </el-avatar>
    </div>
    <div class="bubble-body">
      <div class="bubble-meta">
        <span class="bubble-name" v-if="message.role !== 'user'">{{
          charName
        }}</span>
        <span class="bubble-time" v-if="message.createdAt">{{
          fmtTime(message.createdAt)
        }}</span>
        <span
          class="bubble-latency"
          v-if="message.latencyMs && isDevMode"
          :title="`响应时间: ${message.latencyMs}ms`"
          >{{ message.latencyMs }}ms</span
        >
        <slot name="badges" :message="message" />
      </div>
      <EmoteMessage v-if="isEmote" :message="message" />
      <MediaAttachmentPreview
        v-else
        :image-url="(message as any).imageUrl"
        :video-url="(message as any).videoUrl"
      />
      <VoicePlayBar
        v-if="hasAudio && typingDone"
        :audio-url="(message as any).audioUrl"
        :audio-duration="(message as any).audioDuration"
        :message-content="message.content"
        :message-role="message.role"
        :character-id="characterId"
        :conversation-id="message.conversationId"
        :request-id="message.id"
        @click.stop
      />
      <div
        class="bubble-content"
        v-if="
          typingDone &&
          (renderedContent || hasReplyTo) &&
          (!hasAudio || textExpanded)
        "
        @touchstart="onTouchStart"
        @touchend="onTouchEnd"
        @touchmove="onTouchMove"
      >
        <div
          v-if="hasReplyTo"
          class="quote-block"
          @click="$emit('scroll-to-message', (message as any).replyToMessageId)"
        >
          <div class="quote-bar"></div>
          <div class="quote-body">
            <div class="quote-sender">
              {{ (message as any).replyToRole === "user" ? "你" : charName }}
            </div>
            <div class="quote-text">
              {{ ((message as any).replyToExcerpt || "").slice(0, 120) }}
            </div>
          </div>
        </div>
        <div
          class="bubble-text"
          v-if="renderedContent"
          v-html="renderedContent"
        ></div>
      </div>
      <div
        class="bubble-status"
        v-if="
          !isEmote &&
          (message.status === 'failed' || message.status === 'interrupted')
        "
      >
        <span class="status-tag" :class="message.status">
          {{ message.status === "failed" ? "发送失败" : "生成中断" }}
        </span>
        <el-button
          v-if="message.role === 'user' && message.status === 'failed'"
          text
          size="small"
          type="warning"
          @click="$emit('retry', message)"
          class="retry-btn"
        >
          <el-icon><Refresh /></el-icon> 重试
        </el-button>
      </div>
      <slot name="extension-content" :message="message" />
      <div
        class="text-toggle"
        v-if="hasAudio && message.content"
        @click="textExpanded = !textExpanded"
      >
        <span>{{ textExpanded ? "隐藏文本" : "显示文本" }}</span>
        <span class="text-toggle-arrow" :class="{ expanded: textExpanded }"
          >&#9660;</span
        >
      </div>
      <div
        class="bubble-actions"
        v-if="typingDone"
      >
        <el-button v-if="message.role === 'assistant' && !isEmote" text size="small" @click="copyContent">
          <el-icon><DocumentCopy /></el-icon>
        </el-button>
        <el-button
          v-if="message.role === 'assistant'"
          text
          size="small"
          @click="$emit('reply', message)"
          title="引用回复"
        >
          <el-icon><ChatLineSquare /></el-icon>
        </el-button>
        <slot name="actions" :message="message" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue";
import {
  DocumentCopy,
  Refresh,
  ChatLineSquare,
  UserFilled,
} from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import VoicePlayBar from "./chat-bubble/VoicePlayBar.vue";
import MediaAttachmentPreview from "./chat-bubble/MediaAttachmentPreview.vue";
import EmoteMessage from "./chat-bubble/EmoteMessage.vue";
import { fmtTime } from "./chat-bubble/utils";
import { useAppStore } from "@/stores/app";

const isDevMode = import.meta.env.DEV;

const props = defineProps<{
  message: {
    id?: string;
    conversationId?: string;
    role: string;
    content: string;
    createdAt?: string;
    latencyMs?: number;
    tokens?: number;
    status?: string;
    audioUrl?: string;
    audioDuration?: number;
    typingStart?: number;
    typingDone?: boolean;
  };
  charName?: string;
  charAvatar?: string;
  isStreaming?: boolean;
  status?: string;
  characterId?: string;
}>();

const emit = defineEmits<{
  retry: [message: any];
  reply: [message: any];
  "scroll-to-message": [id: string];
}>();

const appStore = useAppStore();

const hasAudio = computed(() => !!(props.message as any).audioUrl);
const isEmote = computed(() => {
  const message = props.message as any;
  return (
    message.msgType === "emote" ||
    message.msg_type === "emote" ||
    message.contentType === "emote" ||
    message.content_type === "emote" ||
    !!message.emoteId ||
    !!message.emote_id
  );
});
const textExpanded = ref(!(props.message as any).audioUrl);

const hasReplyTo = computed(() => !!(props.message as any).replyToMessageId);

watch(
  () => (props.message as any).audioUrl,
  (val) => {
    if (val && props.message.role === "assistant") {
      textExpanded.value = false;
    }
  },
);

const charInitial = computed(() => (props.charName || "AI").charAt(0));

const typingDone = computed(() => {
  const msg = props.message as any;
  if (msg.role !== "assistant") return true;
  if (!msg.typingStart) return true;
  return msg.typingDone === true;
});

const renderedContent = computed(() => {
  if (isEmote.value) return "";
  const raw = (props.message as any).content;
  const text = typeof raw === "string" ? raw : "";
  const msg = props.message as any;
  if (text === "[图片]" && msg.imageUrl) return "";
  if (text === "[视频]" && msg.videoUrl) return "";
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n/g, "<br>");
});

const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null);
const longPressTriggered = ref(false);
const touchStartY = ref(0);

function onTouchStart(e: TouchEvent) {
  longPressTriggered.value = false;
  touchStartY.value = e.touches[0].clientY;
  longPressTimer.value = setTimeout(() => {
    longPressTriggered.value = true;
    copyContent();
    if (navigator.vibrate) {
      navigator.vibrate(10);
    }
  }, 500);
}

function onTouchMove(e: TouchEvent) {
  if (Math.abs(e.touches[0].clientY - touchStartY.value) > 10) {
    if (longPressTimer.value) {
      clearTimeout(longPressTimer.value);
      longPressTimer.value = null;
    }
  }
}

function onTouchEnd() {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value);
    longPressTimer.value = null;
  }
  if (longPressTriggered.value) {
    longPressTriggered.value = false;
    return;
  }
}

onUnmounted(() => {
  if (longPressTimer.value) clearTimeout(longPressTimer.value);
});

async function copyContent() {
  try {
    await navigator.clipboard.writeText(props.message.content);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败");
  }
}
</script>

<style scoped>
.chat-bubble {
  display: flex;
  gap: 10px;
  padding: 8px 0;
  align-items: flex-start;
  animation: bubbleIn 0.25s ease;
}
@keyframes bubbleIn {
  from {
    opacity: 0;
    transform: translateY(6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.chat-bubble.user {
  flex-direction: row-reverse;
}
.chat-bubble.user .bubble-avatar {
  flex-shrink: 0;
}
.bubble-body {
  max-width: min(80%, 760px);
  min-width: 60px;
}
.chat-bubble.is-emote .bubble-body {
  min-width: 0;
}

.bubble-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 3px;
  padding: 0 4px;
}
.chat-bubble.user .bubble-meta {
  justify-content: flex-end;
}
.bubble-name {
  font-size: var(--ac-font-size-xs);
  font-weight: 500;
  color: var(--ac-color-text-secondary);
}
.bubble-time {
  font-size: 10px;
  color: var(--ac-color-text-muted);
}
.bubble-latency {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
}

.bubble-content {
  padding: 0;
  border-radius: var(--ac-radius-md);
  font-size: var(--ac-font-size-sm);
  line-height: 1.65;
  word-break: break-word;
  overflow: hidden;
}
.chat-bubble.user .bubble-content {
  background: var(--ac-color-bg-primary);
  border: 1px solid var(--ac-color-border-light);
  border-top-right-radius: 2px;
}
.chat-bubble.assistant .bubble-content {
  background: transparent;
  border: 0;
  border-top-left-radius: var(--radius-xs);
}

.quote-block {
  display: flex;
  gap: 0;
  cursor: pointer;
  transition: background 0.15s;
}
.quote-block:hover {
  background: rgba(0, 0, 0, 0.03);
}

.quote-bar {
  width: 3px;
  min-width: 3px;
  background: var(--ac-color-text-muted);
  border-radius: 2px;
  margin: 8px 8px 8px 10px;
}

.quote-body {
  flex: 1;
  padding: 7px 10px 7px 0;
  min-width: 0;
}

.quote-sender {
  font-size: 11px;
  font-weight: 500;
  color: var(--ac-color-text-secondary);
  margin-bottom: 2px;
}

.quote-text {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  white-space: normal;
}

.bubble-text {
  padding: 8px 14px 10px 14px;
  white-space: pre-wrap;
}

.bubble-text:first-child {
  padding-top: 10px;
}

.quote-block + .bubble-text {
  padding-top: 4px;
}

.text-toggle {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  margin-top: 2px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
  cursor: pointer;
  user-select: none;
  border-radius: 4px;
  transition: color 0.2s;
}
.text-toggle:hover {
  color: var(--ac-color-primary);
}
.text-toggle-arrow {
  font-size: 10px;
  transition: transform 0.2s;
}
.text-toggle-arrow.expanded {
  transform: rotate(180deg);
}

.bubble-actions {
  display: flex;
  gap: 2px;
  padding: 2px 4px;
  opacity: 0;
  transition: opacity var(--ac-transition-fast);
}
.chat-bubble:hover .bubble-actions {
  opacity: 1;
}
.bubble-actions :deep(.el-button:focus-visible) { outline: 2px solid var(--surface-border-focus); outline-offset: 2px; }

@media (max-width: 768px) {
  .bubble-body {
    max-width: 88%;
  }
  .bubble-actions {
    opacity: 1;
  }
}

.bubble-status {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 2px 4px;
  margin-bottom: 2px;
}
.status-tag {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 3px;
  line-height: 1.6;
}
.status-tag.failed {
  color: var(--ac-color-danger);
  background: var(--ac-color-danger-bg);
}
.status-tag.interrupted {
  color: var(--ac-color-warning);
  background: var(--ac-color-warning-bg);
}
.retry-btn {
  font-size: 11px;
}
.bubble-source-tag {
  font-size: 10px;
  padding: 0 5px;
  border-radius: 3px;
  line-height: 1.6;
  color: var(--ac-color-warning);
  background: var(--ac-color-warning-bg);
  border: 1px solid var(--ac-color-warning);
}
.bubble-source-tag.tool {
  color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
  border: 1px solid var(--ac-color-primary);
}

/* Keep companion semantics but use Codex-like restrained message surfaces. */
.chat-bubble { padding: 10px 0; gap: 10px; }
.bubble-body { max-width: min(82%, 720px); }
.chat-bubble.user .bubble-content {
  background: var(--surface-bg-elevated);
  border-color: var(--surface-border);
  border-radius: 12px;
}
.chat-bubble.assistant .bubble-content { background: transparent; }
.chat-bubble.assistant .bubble-text { padding: 5px 1px 7px; }
.chat-bubble.user .bubble-text { padding: 8px 12px 9px; }
.bubble-meta { margin-bottom: 2px; }
.bubble-actions { padding-inline: 1px; }

</style>
