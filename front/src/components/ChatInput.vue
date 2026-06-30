<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
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
        @keydown.enter.exact="handleEnterSend"
        @input="autoResize"
      />

      <button
        v-show="voiceMode"
        class="hold-btn"
        :class="{ holding: holding, recognizing: listening, 'slide-text': slideZone === 'text', 'slide-cancel': slideZone === 'cancel' }"
        @mousedown.prevent="startHold"
        @mouseup.prevent="endHold"
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
          @click="handleSendClick"
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
import { computed } from "vue"
import { Promotion, CloseBold, Microphone, Phone, Key, Picture, VideoCamera } from "@element-plus/icons-vue"
import { useTextInput } from "../composables/useTextInput"
import { useVoiceInput } from "../composables/useVoiceInput"
import { useMediaUpload } from "../composables/useMediaUpload"

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

const isDisabled = () => !!props.disabled
const isSending = () => !!props.sending

const textInput = useTextInput(
  emit as any,
  isDisabled,
  isSending,
)

const voiceInput = useVoiceInput(
  (text: string) => emit("voiceText", text),
  (blob: Blob, transcript?: string, duration?: number) => emit("voiceAudio", blob, transcript, duration),
  isDisabled,
  isSending,
)

const mediaUpload = useMediaUpload(
  (file: File, base64: string) => emit("image", file, base64),
  (file: File, videoUrl: string) => emit("video", file, videoUrl),
  () => emit("removeImage"),
  () => emit("removeVideo"),
)

const {
  text, inputRef, handleSend, sendWithImage, sendWithVideo,
  autoResize, focus, setText, clear: clearText, saveDraft,
} = textInput

const {
  voiceMode, holding, listening, slideZone,
  startHold, endHold, onTouchMove,
} = voiceInput

const {
  attachedImage, attachedImagePreview, fileInputRef, videoInputRef,
  attachedVideo, attachedVideoUrl, uploadingVideo,
  handleImageSelect, clearImage, handleVideoSelect, clearVideo,
} = mediaUpload

function handleEnterSend(e: KeyboardEvent) {
  if (attachedVideo.value) {
    if (uploadingVideo.value) return
    if (attachedVideoUrl.value) {
      sendWithVideo(text.value.trim() || "[视频]", attachedVideoUrl.value)
      clearVideo()
      return
    }
    return
  }
  if (attachedImage.value) {
    if (attachedImagePreview.value) {
      sendWithImage(text.value.trim(), attachedImagePreview.value)
      clearImage()
    } else {
      mediaUpload.fileToBase64(attachedImage.value).then((base64) => {
        sendWithImage(text.value.trim(), base64)
        clearImage()
      })
    }
    return
  }
  handleSend(e)
}

function handleSendClick() {
  if (attachedVideo.value) {
    if (uploadingVideo.value) return
    if (attachedVideoUrl.value) {
      sendWithVideo(text.value.trim() || "[视频]", attachedVideoUrl.value)
      clearVideo()
      return
    }
    return
  }
  if (attachedImage.value) {
    if (attachedImagePreview.value) {
      sendWithImage(text.value.trim(), attachedImagePreview.value)
      clearImage()
    } else {
      mediaUpload.fileToBase64(attachedImage.value).then((base64) => {
        sendWithImage(text.value.trim(), base64)
        clearImage()
      })
    }
    return
  }
  handleSend()
}

defineExpose({ focus, setText, clear: clearText })
</script>

<style scoped>
.chat-input-bar {
  display: flex;
  align-items: flex-end;
  gap: 6px;
  padding: 10px 12px;
  background: var(--ac-color-bg-primary);
  border-top: 1px solid var(--ac-color-border-light);
}

.input-wrapper {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 4px;
  padding: 6px 8px 6px 10px;
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border);
  border-radius: var(--ac-radius-md);
  min-height: 36px;
}

.input-left-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 2px;
  padding-right: 4px;
}

.image-btn.has-image {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
}

.video-btn.has-video {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  border-color: var(--ac-color-primary);
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
  align-self: center;
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
</style>


