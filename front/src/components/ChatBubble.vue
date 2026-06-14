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
      
      <div class="bubble-image" v-if="(message as any).imageUrl" @click="showImagePreview = true" style="display:inline-block;max-width:150px;overflow:hidden">
        <img :src="(message as any).imageUrl" alt="用户上传图片" style="width:150px;height:120px;object-fit:cover;display:block;border-radius:6px;max-width:100%" />
        <div class="image-overlay">
          <el-icon><ZoomIn /></el-icon>
        </div>
      </div>
      <div class="bubble-video" v-if="(message as any).videoUrl" @click="handleVideoClick((message as any).videoUrl)">
        <video :src="(message as any).videoUrl" preload="metadata" />
        <div class="video-overlay">
          <el-icon size="28"><VideoPlay /></el-icon>
        </div>
      </div>
       </div>
      
      <div class="voice-bar" v-if="hasAudio && !isStreaming" @click="toggleVoice()" :class="{ playing: voicePlaying, loading: voiceLoading, 'voice-only': !message.content }">
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
  
    <el-dialog v-model="showImagePreview" title="图片预览" width="90%" :close-on-click-modal="true" class="image-preview-dialog">
      <img :src="(message as any).imageUrl" style="width:100%;max-height:70vh;object-fit:contain" />
    </el-dialog>
    <el-dialog v-model="showVideoPreview" title="视频预览" width="90%" :close-on-click-modal="true" class="video-preview-dialog" @closed="stopPreviewVideo">
      <video :src="previewVideoUrl" controls autoplay style="width:100%;max-height:70vh;border-radius:6px" />
    </el-dialog>
</div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from "vue"

import { DocumentCopy, Refresh, ZoomIn, VideoPlay } from "@element-plus/icons-vue"
import { ElMessage } from "element-plus"
import { useApi } from "../composables/useApi"


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
const showImagePreview = ref(false)
const showVideoPreview = ref(false)
const previewVideoUrl = ref('')
const textExpanded = ref(!((props.message as any).audioUrl))


const charInitial = computed(() => (props.charName || "AI").charAt(0))

const renderedContent = computed(() => {
  const text = props.message.content || ""
  const msg = props.message as any
  if (text === "[图片]" && msg.imageUrl) return ""
  if (text === "[视频]" && msg.videoUrl) return ""
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n/g, "<br>")
})

function fmtTime(dateStr: string): string {
  if (!dateStr) return ""
  try {
    const d = new Date(dateStr)
    return d.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
  } catch {
    return ""
  }
}

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

const { post } = useApi()
const voicePlaying = ref(false)
const voiceLoading = ref(false)
const voiceAudio = ref<HTMLAudioElement | null>(null)
const voiceDuration = ref('')

function formatDuration(seconds: number): string {
  if (!seconds || seconds <= 0) return ''
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  if (m > 0) return `${m}:${String(s).padStart(2, '0')}`
  return `${s}s`
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
    const res = await post<any>("/api/tts/synthesize", {
      characterId: props.characterId || undefined,
      text: props.message.content,
    })
    const url = res?.audioUrl
    if (!url) {
      ElMessage.warning("语音合成失败")
      return
    }
    stopVoice()
    const audio = new Audio(url)
    voiceAudio.value = audio
    voiceDuration.value = formatDuration(res?.duration || 0)
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
    ElMessage.error(err?.message || "语音合成失败")
  } finally {
    voiceLoading.value = false
  }
}

function stopVoice() {
  if (voiceAudio.value) {
    voiceAudio.value.pause()
    voiceAudio.value = null
  }
  voicePlaying.value = false
}

function handleVideoClick(url: string) {
  previewVideoUrl.value = url
  showVideoPreview.value = true
}

function stopPreviewVideo() {
  previewVideoUrl.value = ''
}

onUnmounted(() => {
  if (longPressTimer.value) clearTimeout(longPressTimer.value)
  stopVoice()
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
/* ======== Layout ======== */
.chat-bubble {
  display: flex;
  gap: 10px;
  padding: 8px 16px;
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

/* ======== Role colours ======== */
.chat-bubble.user .bubble-content {
  background: var(--ac-color-primary);
  color: #fff;
  border-top-right-radius: 2px;
}
.chat-bubble.assistant .bubble-content {
  background: var(--ac-color-bg-primary);
  border: 1px solid var(--ac-color-border-light);
  border-top-left-radius: 2px;
}
.chat-bubble.is-streaming .bubble-content {
  border-color: var(--ac-color-primary);
}

/* ======== Voice bar ======== */
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

/* Loading dots */
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

/* Playing wave dots */
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

/* ======== Text toggle ======== */
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

/* ======== Actions ======== */
.bubble-actions {
  display: flex; gap: 2px; padding: 2px 4px;
  opacity: 0; transition: opacity var(--ac-transition-fast);
}
.chat-bubble:hover .bubble-actions { opacity: 1; }

.voice-bar.voice-only { margin-top: 0; padding: 12px 18px; min-width: 160px; border-radius: 20px; }
.voice-bar.voice-only .voice-label { font-size: 14px; }
.voice-bar.voice-only .voice-dots { margin-left: 8px; }

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

/* ======== Image in bubble ======== */
.bubble-image {
  position: relative;
  display: inline-block;
  max-width: 150px;
  margin-top: 6px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  border: 1px solid var(--ac-color-border-light);
  transition: border-color 0.2s;
}

.bubble-image:hover {
  border-color: var(--ac-color-primary);
}

.bubble-image img {
  display: block;
  border-radius: 6px;
}

.image-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.2s;
  color: #fff;
  font-size: 24px;
}

.bubble-image:hover .image-overlay {
  opacity: 1;
}
.bubble-video {
  position: relative;
  border-radius: 6px;
  display: inline-block;
  max-width: 260px;
  min-width: 120px;
  cursor: pointer;
  overflow: hidden;
  flex-shrink: 0;
  margin-top: 6px;
}
.bubble-video video {
  display: block;
  width: 100%;
  max-width: 260px;
  height: 160px;
  object-fit: cover;
  border-radius: 6px;
}
.video-overlay {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: white;
  opacity: 0.7;
  transition: opacity 0.2s;
  pointer-events: none;
  text-shadow: 0 2px 8px rgba(0,0,0,0.5);
}
.bubble-video:hover .video-overlay {
  opacity: 1;
}

.image-preview-dialog .el-dialog__body {
  padding: 0;
}
.video-preview-dialog .el-dialog__body {
  padding: 0;
}
</style>
