<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="chat-panel">
    <div class="chat-messages" ref="messagesRef">
      <ChatMessage
        v-for="msg in messages"
        :key="msg.id"
        :message="msg"
        :name="characterName"
        :avatar="characterAvatar"
      />
      <div v-if="loading" class="typing-indicator">
        <span></span><span></span><span></span>
      </div>
    </div>
    <ChatInput
      :disabled="loading"
      :sending="sending"
      :call-active="callActive"
      @send="$emit('send', $event)"
      @stop="$emit('stop')"
      @toggle-call="$emit('toggleCall')"
      @voiceAudio="$emit('voiceAudio', $event)"
      @voiceText="$emit('voiceText', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from "vue"
import type { Message } from "@/types"
import ChatMessage from "./ChatMessage.vue"
import ChatInput from "./ChatInput.vue"

const props = defineProps<{
  messages: Message[]
  loading?: boolean
  sending?: boolean
  callActive?: boolean
  characterName?: string
  characterAvatar?: string
}>()

defineEmits<{
  send: [text: string, imageBase64?: string, videoBase64?: string]
  stop: []
  toggleCall: []
  voiceAudio: [blob: Blob, transcript?: string, duration?: number]
  voiceText: [text: string]
}>()

const messagesRef = ref<HTMLElement>()

watch(() => props.messages.length, () => {
  nextTick(() => {
    if (messagesRef.value) {
      messagesRef.value.scrollTop = messagesRef.value.scrollHeight
    }
  })
})
</script>

<style scoped>
.chat-panel { display: flex; flex-direction: column; height: 100%; }
.chat-messages { flex: 1; overflow-y: auto; padding: 8px 0; }
.typing-indicator { display: flex; gap: 4px; padding: 12px 16px; }
.typing-indicator span { width: 8px; height: 8px; background: var(--el-color-primary); border-radius: 50%; animation: bounce 1.4s infinite ease-in-out both; }
.typing-indicator span:nth-child(1) { animation-delay: -0.32s; }
.typing-indicator span:nth-child(2) { animation-delay: -0.16s; }
@keyframes bounce { 0%, 80%, 100% { transform: scale(0); } 40% { transform: scale(1); } }
</style>
