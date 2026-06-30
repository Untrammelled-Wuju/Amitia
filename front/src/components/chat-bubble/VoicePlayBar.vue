<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="voice-bar" :class="{ playing: voicePlaying, loading: voiceLoading, 'voice-only': !hasContent }" @click="toggleVoice">
    <div class="voice-icon-wrap">
      <svg viewBox="0 0 28 28" class="voice-wx-icon" :class="{ active: voicePlaying }">
        <rect class="voice-body" x="2" y="8" width="5" height="12" rx="1.5" />
        <path class="voice-wave w1" d="M9 11a3 3 0 010 6" />
        <path class="voice-wave w2" d="M11.5 8.5a6 6 0 010 11" />
        <path class="voice-wave w3" d="M14 6a9 9 0 010 16" />
      </svg>
      <div class="voice-anim-dots" v-if="voiceLoading">
        <span class="dot" />
        <span class="dot" />
        <span class="dot" />
      </div>
    </div>
    <span class="voice-label">{{ voiceLoading ? '加载中' : voicePlaying ? '播放中' : '播放语音' }}</span>
    <span class="voice-dots" v-if="voicePlaying">
      <span v-for="i in 5" :key="i" class="vdot" :style="{ animationDelay: (i * 0.12) + 's' }" />
    </span>
    <span class="voice-sec" v-if="voiceDuration && !voicePlaying">{{ voiceDuration }}</span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onUnmounted } from "vue"
import { ElMessage } from "element-plus"
import { useApi } from "@/composables/useApi"
import { formatDuration } from "./utils"

const props = defineProps<{
  audioUrl?: string
  audioDuration?: number
  messageContent?: string
  messageRole?: string
  characterId?: string
}>()

const hasContent = computed(() => !!props.messageContent)

const { post } = useApi()
const voicePlaying = ref(false)
const voiceLoading = ref(false)
const voiceAudio = ref<HTMLAudioElement | null>(null)
const voiceDuration = ref('')

function stopVoice() {
  if (voiceAudio.value) {
    voiceAudio.value.pause()
    voiceAudio.value = null
  }
  voicePlaying.value = false
}

async function toggleVoice() {
  if (voiceLoading.value) return
  if (voicePlaying.value) {
    stopVoice()
    return
  }
  if (voiceAudio.value) {
    const audio = voiceAudio.value
    await audio.play()
    voicePlaying.value = true
    audio.onended = () => {
      voicePlaying.value = false
      voiceAudio.value = null
    }
    audio.onerror = () => {
      voicePlaying.value = false
      voiceAudio.value = null
      ElMessage.warning("播放失败")
    }
    return
  }
  voiceLoading.value = true
  try {
    let url = props.audioUrl || ""
    if (!url && props.messageRole === "assistant") {
      const res = await post<any>("/api/tts/synthesize", {
        characterId: props.characterId || undefined,
        text: props.messageContent || "",
      })
      url = res?.audioUrl
    }
    if (!url) {
      ElMessage.warning("语音加载失败")
      return
    }
    stopVoice()
    const audio = new Audio(url)
    voiceAudio.value = audio
    voiceDuration.value = formatDuration(props.audioDuration || 0)
    await audio.play()
    voicePlaying.value = true
    voiceLoading.value = false
    audio.onended = () => {
      voicePlaying.value = false
      voiceAudio.value = null
    }
    audio.onerror = () => {
      voicePlaying.value = false
      voiceAudio.value = null
      ElMessage.warning("播放失败")
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "语音播放失败")
  } finally {
    voiceLoading.value = false
  }
}

onUnmounted(() => { stopVoice() })

defineExpose({ stopVoice })
</script>

<style scoped>
.voice-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  margin-bottom: 4px;
  cursor: pointer;
  border-radius: 12px;
  background: var(--ac-color-bg-secondary);
  transition: background 0.2s, box-shadow 0.2s;
  user-select: none;
  font-size: 13px;
  min-width: 140px;
  width: fit-content;
  max-width: 100%;
}
.voice-bar:hover {
  background: var(--ac-color-primary-bg);
  box-shadow: 0 0 0 1px var(--ac-color-primary);
}
.voice-bar.playing {
  background: var(--ac-color-primary-bg);
}
.voice-bar.loading {
  opacity: 0.7;
  cursor: wait;
}

.voice-icon-wrap {
  position: relative;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.voice-wx-icon {
  width: 28px;
  height: 28px;
  fill: none;
  stroke: var(--ac-color-text-secondary);
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
  transition: stroke 0.2s;
}
.voice-bar.playing .voice-wx-icon {
  stroke: var(--ac-color-primary);
}

.voice-wx-icon .voice-body { fill: var(--ac-color-text-secondary); transition: fill 0.2s; }
.voice-bar.playing .voice-wx-icon .voice-body { fill: var(--ac-color-primary); }

.voice-wx-icon .voice-wave { opacity: 0.3; transition: opacity 0.15s; }
.voice-bar.playing .voice-wx-icon .voice-wave { animation: wavePulse 0.8s ease-in-out infinite; }
.voice-bar.playing .voice-wx-icon .voice-wave.w2 { animation-delay: 0.15s; }
.voice-bar.playing .voice-wx-icon .voice-wave.w3 { animation-delay: 0.3s; }

@keyframes wavePulse {
  0%, 100% { opacity: 0.3; }
  50% { opacity: 1; }
}

.voice-anim-dots {
  position: absolute; bottom: -2px; display: flex; gap: 3px;
}
.voice-anim-dots .dot {
  width: 4px; height: 4px; border-radius: 50%;
  background: var(--ac-color-text-muted);
  animation: dotHop 0.6s ease-in-out infinite;
}
.voice-anim-dots .dot:nth-child(2) { animation-delay: 0.15s; }
.voice-anim-dots .dot:nth-child(3) { animation-delay: 0.3s; }

@keyframes dotHop {
  0%, 100% { transform: translateY(0); opacity: 0.3; }
  50% { transform: translateY(-4px); opacity: 1; }
}

.voice-label {
  font-size: 13px; color: var(--ac-color-text-secondary); white-space: nowrap;
  transition: color 0.3s;
}
.voice-bar.playing .voice-label { color: var(--ac-color-primary); font-weight: 500; }

.voice-dots { display: flex; align-items: flex-end; gap: 2px; height: 14px; }
.voice-dots .vdot {
  width: 3px; border-radius: 2px;
  background: var(--ac-color-primary);
  animation: barBounce 0.5s ease-in-out infinite alternate;
}
.voice-dots .vdot:nth-child(1) { height: 8px; }
.voice-dots .vdot:nth-child(2) { height: 12px; }
.voice-dots .vdot:nth-child(3) { height: 6px; }
.voice-dots .vdot:nth-child(4) { height: 14px; }
.voice-dots .vdot:nth-child(5) { height: 10px; }

@keyframes barBounce {
  0% { transform: scaleY(0.4); }
  100% { transform: scaleY(1); }
}

.voice-sec {
  font-size: 12px; color: var(--ac-color-text-muted);
  margin-left: auto;
}

.voice-bar.voice-only { margin-top: 0; padding: 12px 18px; min-width: 160px; border-radius: 20px; }
.voice-bar.voice-only .voice-label { font-size: 14px; }
.voice-bar.voice-only .voice-dots { margin-left: 8px; }
</style>
