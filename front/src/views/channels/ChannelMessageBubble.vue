<template>
  <div class="channel-message" :class="message.role">
    <div class="message-meta">{{ roleLabel }} · {{ formatTime(message.createdAt) }}</div>
    <img v-if="message.imageUrl" class="message-image" :src="message.imageUrl" :alt="message.altText || '渠道图片'" />
    <video v-if="message.videoUrl" class="message-video" :src="message.videoUrl" controls preload="metadata"></video>
    <audio v-if="message.audioUrl" class="message-audio" :src="message.audioUrl" controls preload="metadata"></audio>
    <div v-if="message.content" class="message-content">{{ message.content }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  message: Record<string, any>;
}>();

const roleLabel = computed(() => props.message.role === "user" ? "对方" : "Amitia");

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
</script>

<style scoped>
.channel-message { max-width: min(680px, 82%); padding: 9px 12px; border-radius: 10px; background: var(--ac-color-surface); }
.channel-message.user { align-self: flex-end; background: var(--ac-color-primary-bg); }
.message-meta { color: var(--text-muted); font-size: 10px; }
.message-content { margin-top: 5px; white-space: pre-wrap; word-break: break-word; color: var(--text-primary); font-size: 13px; line-height: 1.55; }
.message-image, .message-video { display: block; max-width: 100%; max-height: 420px; margin-top: 7px; border-radius: 8px; }
.message-video, .message-audio { width: 100%; }
.message-audio { margin-top: 7px; }
</style>
