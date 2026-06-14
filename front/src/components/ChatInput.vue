<template>
  <div class="chat-input-bar">

    <input ref="fileInputRef" type="file" accept="image/*" style="display:none" @change="handleImageSelect" />
    <input ref="videoInputRef" type="file" accept="video/*" style="display:none" @change="handleVideoSelect" />
    <div v-if="attachedImagePreview" class="image-preview-bar">
      <div class="preview-thumb" :style="{ backgroundImage: 'url(' + attachedImagePreview + ')' }"></div>
      <span class="preview-name">{{ attachedImage?.name || '图片' }}</span>
      <el-button :icon="CloseBold" circle size="small" class="preview-remove" @click="clearImage" />
    </div>
    <div v-if="attachedVideo" class="video-preview-bar">
      <el-icon size="20" color="#409eff"><VideoCamera /></el-icon>
      <span class="preview-name">{{ attachedVideo.name }}</span>
      <span v-if="uploadingVideo" class="upload-status">上传中...</span>
      <span v-else-if="attachedVideoUrl" class="upload-status ready">就绪</span>
      <el-button :icon="CloseBold" circle size="small" class="preview-remove" @click="clearVideo" :disabled="uploadingVideo" />
    </div>
    <div class="input-wrapper">
      <div class="input-left-actions">

        <el-button
          :icon="Picture"
          circle
          size="small"
          class="image-btn"
          :class="{ 'has-image': !!attachedImagePreview }"
          @click="fileInputRef?.click()"
          title="上传图片"
        />
        <el-button
          :icon="VideoCamera"
          circle
          size="small"
          class="video-btn"
          :class="{ 'has-video': !!attachedVideo }"
          @click="videoInputRef?.click()"
          title="上传视频"
        />
        <el-button
          :icon="voiceMode ? Key : Microphone"
          circle
          size="small"
          class="mode-toggle-btn"
          @click="voiceMode = !voiceMode"
          :title="voiceMode ? '切换到文字输入' : '切换到语音输入'"
        />
      </div>

      <div class="input-body">
        <textarea
        v-show="!voiceMode"
        ref="inputRef"
        v-model="text"
        class="input-field"
        :placeholder="sending ? 'AI 回复中...' : '输入消息...'"
        :disabled="disabled || sending"
        rows="1"
        @keydown.enter.exact="handleSend"
        @input="autoResize"
      />

      <button
        v-show="voiceMode"
        class="hold-btn"
        :class="{ holding: holding, recognizing: listening, 'slide-text': slideZone === 'text', 'slide-cancel': slideZone === 'cancel' }"
        @mousedown.prevent="startHold"
        @mouseup.prevent="endHold"
        @mouseleave.prevent="endHold"
        @touchstart.prevent="startHold"
        @touchmove.prevent="onTouchMove"
        @touchend.prevent="endHold"
        @touchcancel.prevent="endHold"
        :disabled="disabled || sending"
      >
        <template v-if="slideZone === 'cancel'">
          <span class="hold-text slide-hint cancel-hint">
            <span class="cancel-icon" />
            松开 取消
          </span>
        </template>
        <template v-else-if="slideZone === 'text'">
          <span class="hold-text slide-hint text-hint">
            <span class="text-icon" />
            松开 转文字
          </span>
        </template>
        <template v-else-if="holding">
          <span class="hold-text">
            <span class="hold-dot" />
            松开 发送语音
          </span>
        </template>
        <template v-else-if="listening">
          <span class="hold-text">
            <span class="hold-pulse" />
            识别中...
          </span>
        </template>
        <template v-else>
          <span class="hold-text">按住 说话</span>
        </template>
      </button>
      </div>

      <div class="input-actions">
        <el-button
          v-if="sending"
          type="danger"
          :icon="CloseBold"
          circle
          size="small"
          @click="$emit('stop')"
          title="停止生成"
        />
        <el-button
          v-if="!voiceMode && !sending"
          type="primary"
          :icon="Promotion"
          circle
          size="small"
          :disabled="disabled || uploadingVideo || (!text.trim() && !attachedImagePreview && !attachedVideo)"
          @click="handleSend"
          title="发送 (Enter)"
        />
      </div>
    </div>
    <el-button
      :type="callActive ? 'danger' : 'default'"
      :icon="Phone"
      circle
      :class="{ 'call-btn-outer': true, 'is-calling': callActive }"
      @click="$emit('toggleCall')"
      title="语音通话"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch, onUnmounted } from "vue"
import { Promotion, CloseBold, Microphone, Phone, Key, Picture, PictureFilled, VideoCamera } from "@element-plus/icons-vue"

const props = defineProps<{
  disabled?: boolean
  sending?: boolean
  callActive?: boolean
}>()

const emit = defineEmits<{
  send: [text: string, imageBase64?: string, videoBase64?: string]
  stop: []
  toggleCall: []
  voiceText: [text: string]
  voiceAudio: [blob: Blob, transcript?: string, duration?: number]
  image: [file: File, base64: string]
  removeImage: []
  video: [file: File, videoUrl: string]
  removeVideo: []
}>()

const DRAFT_KEY = "webchat_draft"
const attachedImage = ref<File | null>(null)
const attachedImagePreview = ref<string | null>(null)
const fileInputRef = ref<HTMLInputElement>()
const videoInputRef = ref<HTMLInputElement>()
const attachedVideo = ref<File | null>(null)
const attachedVideoUrl = ref<string | null>(null)
const uploadingVideo = ref(false)
const text = ref(localStorage.getItem(DRAFT_KEY) || "")
const inputRef = ref<HTMLTextAreaElement>()
const voiceMode = ref(false)
const holding = ref(false)
const listening = ref(false)
const slideZone = ref<'none' | 'text' | 'cancel'>('none')
const touchStartY = ref(0)
const lastTranscript = ref('')
let recognition: any = null
let mediaRecorder: MediaRecorder | null = null
let audioChunks: Blob[] = []
let onRecordingComplete: (() => void) | null = null
let recordingStartTime = 0

function saveDraft() {
  if (text.value.trim()) {
    localStorage.setItem(DRAFT_KEY, text.value)
  } else {
    localStorage.removeItem(DRAFT_KEY)
  }
}

function handleSend(e?: KeyboardEvent) {
  if (attachedVideo.value) {
    if (uploadingVideo.value) return
    if (attachedVideoUrl.value) {
      const trimmed = text.value.trim() || "[视频]"
      emit("send", trimmed, undefined, attachedVideoUrl.value)
      text.value = ""
      localStorage.removeItem(DRAFT_KEY)
      clearVideo()
      nextTick(() => autoResize())
    }
    return
  }
  if (attachedImage.value) {
    if (attachedImagePreview.value) {
      const trimmed = text.value.trim()
      emit("send", trimmed, attachedImagePreview.value)
      text.value = ""
      localStorage.removeItem(DRAFT_KEY)
      clearImage()
      nextTick(() => autoResize())
    } else {
      fileToBase64(attachedImage.value).then(base64 => {
        const trimmed = text.value.trim()
        emit("send", trimmed, base64)
        text.value = ""
        localStorage.removeItem(DRAFT_KEY)
        clearImage()
        nextTick(() => autoResize())
      })
    }
    return
  }
  if (e) e.preventDefault()
  const trimmed = text.value.trim()
  if (!trimmed || props.disabled || props.sending) return
  emit("send", trimmed)
  text.value = ""; localStorage.removeItem(DRAFT_KEY)
  nextTick(() => autoResize())
}

function autoResize() {
  const el = inputRef.value
  if (!el) return
  el.style.height = "auto"
  el.style.height = Math.min(el.scrollHeight, 120) + "px"
}

watch(text, () => { saveDraft() }, { flush: 'post' })

function focus() {
  inputRef.value?.focus()
}

const SLIDE_TEXT_THRESHOLD = 60
const SLIDE_CANCEL_THRESHOLD = 120
const VOICE_END_DELAY = 400

function startHold(e: TouchEvent | MouseEvent) {
  if (props.disabled || props.sending) return
  holding.value = true
  slideZone.value = 'none'
  lastTranscript.value = ''
  if ('touches' in e) {
    touchStartY.value = e.touches[0].clientY
  } else {
    touchStartY.value = e.clientY
  }
  startRecording()
  startListening()
}

function onTouchMove(e: TouchEvent) {
  if (!holding.value) return
  const currentY = e.touches[0].clientY
  const deltaY = touchStartY.value - currentY
  if (deltaY > SLIDE_CANCEL_THRESHOLD) {
    slideZone.value = 'cancel'
  } else if (deltaY > SLIDE_TEXT_THRESHOLD) {
    slideZone.value = 'text'
  } else {
    slideZone.value = 'none'
  }
}

function endHold() {
  if (!holding.value) return
  const zone = slideZone.value
  holding.value = false
  slideZone.value = 'none'

  setTimeout(() => {
    stopListening()

    if (zone === 'cancel') {
      stopRecording()
      audioChunks = []
      return
    }

    if (zone === 'text') {
      stopRecording()
      audioChunks = []
      if (lastTranscript.value) {
        text.value = text.value ? text.value + lastTranscript.value : lastTranscript.value
        saveDraft()
      }
      voiceMode.value = false
      return
    }

    onRecordingComplete = () => {
      if (audioChunks.length > 0) {
        const blob = new Blob(audioChunks, { type: 'audio/webm' })
        const transcript = lastTranscript.value || undefined
        audioChunks = []
        const duration = Math.round((Date.now() - recordingStartTime) / 1000)
        emit("voiceAudio", blob, transcript, duration)
      }
      voiceMode.value = false
    }
    stopRecording()
  }, VOICE_END_DELAY)
}

function startRecording() {
  navigator.mediaDevices.getUserMedia({ audio: true }).then((stream) => {
    recordingStartTime = Date.now()
    audioChunks = []
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') ? 'audio/webm;codecs=opus' : 'audio/webm'
    mediaRecorder = new MediaRecorder(stream, { mimeType })
    mediaRecorder.ondataavailable = (e: BlobEvent) => {
      if (e.data.size > 0) audioChunks.push(e.data)
    }
    mediaRecorder.onstop = () => {
      stream.getTracks().forEach(t => t.stop())
      if (onRecordingComplete) {
        onRecordingComplete()
        onRecordingComplete = null
      }
    }
    mediaRecorder.start()
  }).catch(() => {
    holding.value = false
  })
}

function stopRecording() {
  if (mediaRecorder && mediaRecorder.state !== 'inactive') {
    mediaRecorder.stop()
  }
  mediaRecorder = null
}

function startListening() {
  const SpeechRecognition = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition
  if (!SpeechRecognition) {
    return
  }
  if (recognition) {
    try { recognition.stop() } catch {}
    recognition = null
  }
  recognition = new SpeechRecognition()
  recognition.lang = "zh-CN"
  recognition.interimResults = true
  recognition.maxAlternatives = 1
  recognition.continuous = false

  recognition.onstart = () => { listening.value = true }
  recognition.onresult = (event: any) => {
    let finalTranscript = ''
    for (let i = event.resultIndex; i < event.results.length; i++) {
      if (event.results[i].isFinal) {
        finalTranscript += event.results[i][0].transcript
      }
    }
    if (finalTranscript) {
      lastTranscript.value = finalTranscript
    } else if (event.results.length > 0) {
      lastTranscript.value = event.results[event.results.length - 1][0].transcript
    }
  }
  recognition.onerror = () => {
    listening.value = false
  }
  recognition.onend = () => { listening.value = false }

  try {
    recognition.start()
  } catch {
    listening.value = false
  }
}

function stopListening() {
  if (recognition) {
    try { recognition.stop() } catch {}
    recognition = null
  }
  listening.value = false
}

onUnmounted(() => {
  stopListening()
  stopRecording()
})

function handleImageSelect(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!file.type.startsWith('image/')) {
    return
  }
  attachedImage.value = file
  fileToBase64(file).then(dataUrl => {
    attachedImagePreview.value = dataUrl
    emit('image', file, dataUrl)
  })
  input.value = ''
}

function clearImage() {
  attachedImage.value = null
  attachedImagePreview.value = null
  emit('removeImage')
}

function handleVideoSelect(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!file.type.startsWith('video/')) return
  attachedVideo.value = file
  uploadingVideo.value = true
  const formData = new FormData()
  formData.append('video', file)
  const token = localStorage.getItem('ai-companion-token') || ''
  fetch('/api/video/upload', { method: 'POST', headers: { Authorization: 'Bearer ' + token }, body: formData })
    .then(res => res.json())
    .then(data => {
      const videoUrl = data?.data?.videoUrl || data?.videoUrl || ''
      if (videoUrl) {
        attachedVideoUrl.value = videoUrl
        emit('video', file, videoUrl)
      }
    })
    .catch(() => {})
    .finally(() => { uploadingVideo.value = false })
  input.value = ''
}

function clearVideo() {
  attachedVideo.value = null
  attachedVideoUrl.value = null
  uploadingVideo.value = false
  emit('removeVideo')
}

function fileToBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(reader.result as string)
    reader.onerror = () => reject(new Error('文件读取失败'))
    reader.readAsDataURL(file)
  })
}

defineExpose({ focus, clear: () => { text.value = ""; localStorage.removeItem(DRAFT_KEY) } })
</script>

<style scoped>
.chat-input-bar {
  padding: 10px 14px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.input-wrapper {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-lg);
  padding: 6px 6px 6px 6px;
  border: 1px solid var(--ac-color-border);
  transition: border-color var(--ac-transition-fast);
  box-shadow: var(--ac-shadow-sm);
  min-height: 44px;
}

.input-wrapper:focus-within {
  border-color: var(--ac-color-primary);
}

.input-left-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding-left: 4px;
}

.mode-toggle-btn {
  transition: all 0.2s ease;
}

.input-body {
  flex: 1;
  min-height: 36px;
  display: flex;
  align-items: center;
}

.input-field {
  width: 100%;
  border: none;
  box-sizing: border-box;
  display: block;
  background: transparent;
  outline: none;
  font-family: var(--ac-font-family);
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text);
  resize: none;
  min-height: 24px;
  max-height: 120px;
  line-height: 1.5;
  padding: 3px 0;
}

.input-field::placeholder {
  color: var(--ac-color-text-placeholder);
}

.input-field:disabled {
  opacity: 0.7;
}

.hold-btn {
  width: 100%;
  min-height: 36px;
  border: 1.5px solid var(--ac-color-border);
  background: var(--ac-color-bg-primary);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
  transition: all 0.15s ease;
  outline: none;
  font-family: var(--ac-font-family);
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  padding: 0 16px;
  box-sizing: border-box;
  margin: 0;
}

.hold-btn:active {
  transform: scale(0.98);
}

.hold-btn.holding {
  background: var(--ac-color-primary-bg);
  border-color: var(--ac-color-primary);
  color: var(--ac-color-primary);
}

.hold-btn.recognizing {
  background: var(--el-color-danger-light-9);
  border-color: var(--el-color-danger);
  color: var(--el-color-danger);
}

.hold-btn.slide-text {
  background: #e8f5e9;
  border-color: #66bb6a;
  color: #388e3c;
}

.hold-btn.slide-cancel {
  background: #fbe9e7;
  border-color: #ef5350;
  color: #d32f2f;
}

.hold-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.hold-text {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.slide-hint {
  font-size: 15px;
  font-weight: 600;
}

.cancel-hint {
  color: #d32f2f;
}

.text-hint {
  color: #388e3c;
}

.hold-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ac-color-primary);
  animation: holdPulse 1s ease-in-out infinite;
}

.hold-pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-danger);
  animation: holdPulse 1s ease-in-out infinite;
}

.cancel-icon {
  width: 6px;
  height: 22px;
  background: #d32f2f;
  border-radius: 3px;
  display: inline-block;
}

.text-icon {
  display: inline-block;
  font-size: 20px;
  line-height: 1;
}

.text-icon::before {
  content: 'T';
  color: #388e3c;
  font-weight: 800;
  font-family: Georgia, serif;
}

@keyframes holdPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(1.3); }
}

.input-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}

.call-btn-outer {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  font-size: 18px;
}

.is-calling {
  background: var(--el-color-success-light-9) !important;
  border-color: var(--el-color-success) !important;
  color: var(--el-color-success) !important;
}

@media (max-width: 768px) {
  .chat-input-bar {
    padding: 8px 10px;
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
  }

  .input-field {
    font-size: 16px;
  }

  .input-wrapper {
    padding: 8px 8px 8px 8px;
    border-radius: 20px;
  }

  .hold-btn {
    height: 40px;
    font-size: 16px;
    border-radius: 10px;
  }
}

.upload-status {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  flex-shrink: 0;
}
.upload-status.ready {
  color: var(--ac-color-success);
}
</style>


/* ======== Image preview ======== */
.image-preview-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  margin-bottom: 4px;
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-sm);
}

.preview-thumb {
  width: 36px;
  height: 36px;
  border-radius: 4px;
  background-size: cover;
  background-position: center;
  border: 1px solid var(--ac-color-border);
  flex-shrink: 0;
}

.preview-name {
  flex: 1;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.preview-remove {
  flex-shrink: 0;
}

.video-preview-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  margin-bottom: 4px;
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-sm);
}
.video-btn.has-video {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
}
.image-btn.has-image {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
}
