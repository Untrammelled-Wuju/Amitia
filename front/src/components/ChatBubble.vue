<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-bubble" :class="[message.role, { 'is-streaming': isStreaming }]">
    <div class="bubble-avatar">
      <el-avatar :size="32" :src="message.role === 'assistant' ? charAvatar : undefined">
        {{ message.role === "user" ? "U" : charInitial }}
      </el-avatar>
    </div>
    <div class="bubble-body">
      <div class="bubble-meta">
        <span class="bubble-name" v-if="message.role !== 'user'">{{ charName }}</span>
        <span class="bubble-time" v-if="message.createdAt">{{ fmtTime(message.createdAt) }}</span>
        <span class="bubble-latency" v-if="message.latencyMs">{{ message.latencyMs }}ms</span>
      </div>
      <MediaAttachmentPreview
        :image-url="(message as any).imageUrl"
        :video-url="(message as any).videoUrl"
      />
      <VoicePlayBar
        v-if="hasAudio && !isStreaming"
        :audio-url="(message as any).audioUrl"
        :audio-duration="(message as any).audioDuration"
        :message-content="message.content"
        :message-role="message.role"
        :character-id="characterId"
        @click.stop
      />
      <div class="bubble-content" v-if="renderedContent && (!hasAudio || textExpanded)" v-html="renderedContent" @touchstart="onTouchStart" @touchend="onTouchEnd" @touchmove="onTouchMove" style="word-break:break-word;overflow-wrap:break-word"></div>
      <div class="bubble-status" v-if="message.status === 'failed' || message.status === 'interrupted'">
        <span class="status-tag" :class="message.status">
          {{ message.status === 'failed' ? '发送失败' : '生成中断' }}
        </span>
        <el-button v-if="message.role === 'user' && message.status === 'failed'" text size="small" type="warning" @click="$emit('retry', message)" class="retry-btn">
          <el-icon><Refresh /></el-icon> 重试
        </el-button>
      </div>
      <div class="text-toggle" v-if="hasAudio && message.content" @click="textExpanded = !textExpanded">
        <span>{{ textExpanded ? '隐藏文本' : '显示文本' }}</span>
        <span class="text-toggle-arrow" :class="{ expanded: textExpanded }">&#9660;</span>
      </div>
      <div class="bubble-actions" v-if="message.role === 'assistant' && !isStreaming">
        <el-button text size="small" @click="copyContent">
          <el-icon><DocumentCopy /></el-icon>
        </el-button>
        <slot name="actions" :message="message" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue"
import { DocumentCopy, Refresh } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import VoicePlayBar from "./chat-bubble/VoicePlayBar.vue"
import MediaAttachmentPreview from "./chat-bubble/MediaAttachmentPreview.vue"
import { fmtTime } from "./chat-bubble/utils"

const props = defineProps<{
  message: {
    id?: string
    role: string
    content: string
    createdAt?: string
    latencyMs?: number
    tokens?: number
    status?: string
    audioUrl?: string
    audioDuration?: number
  }
  charName?: string
  charAvatar?: string
  isStreaming?: boolean
  status?: string
  characterId?: string
}>()

const emit = defineEmits<{
  retry: [message: any]
}>()

const hasAudio = computed(() => !!((props.message as any).audioUrl))
const textExpanded = ref(!((props.message as any).audioUrl))

watch(
  () => (props.message as any).audioUrl,
  (val) => {
    if (val && props.message.role === "assistant") {
      textExpanded.value = false
    }
  }
)

const charInitial = computed(() => (props.charName || "AI").charAt(0))

const renderedContent = computed(() => {
  const raw = (props.message as any).content
  const text = typeof raw === "string" ? raw : ""
  const msg = props.message as any
  if (text === "[图片]" && msg.imageUrl) return ""
  if (text === "[视频]" && msg.videoUrl) return ""
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n/g, "<br>")
})
const longPressTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const longPressTriggered = ref(false)
const touchStartY = ref(0)

function onTouchStart(e: TouchEvent) {
  longPressTriggered.value = false
  touchStartY.value = e.touches[0].clientY
  longPressTimer.value = setTimeout(() => {
    longPressTriggered.value = true
    copyContent()
    if (navigator.vibrate) { navigator.vibrate(10) }
  }, 500)
}

function onTouchMove(e: TouchEvent) {
  if (Math.abs(e.touches[0].clientY - touchStartY.value) > 10) {
    if (longPressTimer.value) {
      clearTimeout(longPressTimer.value)
      longPressTimer.value = null
    }
  }
}

function onTouchEnd() {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value)
    longPressTimer.value = null
  }
  if (longPressTriggered.value) {
    longPressTriggered.value = false
    return
  }
}

onUnmounted(() => {
  if (longPressTimer.value) clearTimeout(longPressTimer.value)
})

async function copyContent() {
  try {
    await navigator.clipboard.writeText(props.message.content)
    ElMessage.success("已复制")
  } catch {
    ElMessage.warning("复制失败")
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
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.chat-bubble.user { flex-direction: row-reverse; }
.chat-bubble.user .bubble-avatar { flex-shrink: 0; }
.bubble-body { max-width: 80%; min-width: 60px; }

.bubble-meta {
  display: flex; align-items: center; gap: 6px;
  margin-bottom: 3px; padding: 0 4px;
}
.chat-bubble.user .bubble-meta { justify-content: flex-end; }
.bubble-name { font-size: var(--ac-font-size-xs); font-weight: 500; color: var(--ac-color-text-secondary); }
.bubble-time { font-size: 10px; color: var(--ac-color-text-muted); }
.bubble-latency { font-size: 10px; color: var(--ac-color-text-placeholder); }

.bubble-content {
  padding: 10px 14px; border-radius: var(--ac-radius-md);
  font-size: var(--ac-font-size-sm); line-height: 1.65;
  word-break: break-word; white-space: pre-wrap;
}

.chat-bubble.user .bubble-content {
  background: var(--ac-color-bg-primary);
  border: 1px solid var(--ac-color-border-light);
  border-top-right-radius: 2px;
}
.chat-bubble.assistant .bubble-content {
  background: var(--ac-color-primary);
  color: #fff;
  border-top-left-radius: 2px;
}
.chat-bubble.is-streaming .bubble-content {
  border-color: var(--ac-color-primary);
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
  display: flex; gap: 2px; padding: 2px 4px;
  opacity: 0; transition: opacity var(--ac-transition-fast);
}
.chat-bubble:hover .bubble-actions { opacity: 1; }

@media (max-width: 768px) {
  .bubble-body { max-width: 88%; }
  .bubble-actions { opacity: 1; }
}

.bubble-status {
  display: flex; align-items: center; gap: 6px;
  padding: 2px 4px; margin-bottom: 2px;
}
.status-tag { font-size: 11px; padding: 1px 8px; border-radius: 3px; line-height: 1.6; }
.status-tag.failed { color: #d35; background: #fef0f0; }
.status-tag.interrupted { color: #b88230; background: #fef8e7; }
.retry-btn { font-size: 11px; }
.bubble-source-tag {
  font-size: 10px; padding: 0 5px; border-radius: 3px; line-height: 1.6;
  color: #8b5e3c; background: #fef6e8; border: 1px solid #f0dba8;
}
.bubble-source-tag.tool {
  color: #4a6fa5; background: #eef3fa; border: 1px solid #c8d6e5;
}
</style>
