<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="char-voice-page">
    <div class="page-header">
      <div>
        <h2>拟态语音</h2>
        <p class="page-desc">为当前角色配置独立的音色语音系统</p>
      </div>
    </div>

    <div class="cards">
      <div class="card">
        <div class="card-body">
          <div class="mode-row">
            <span class="mode-label">音色模式</span>
            <el-radio-group v-model="voiceMode" @change="onModeChange">
              <el-radio value="preset">预设音色</el-radio>
              <el-radio value="clone">复刻音色</el-radio>
            </el-radio-group>
          </div>

          <PresetVoiceSection
            v-if="voiceMode === 'preset'"
            v-model:voice-type="form.voiceType"
            :voice-presets="voicePresets"
            @voice-type-change="onVoiceTypeChange"
          />

          <CloneVoiceSection
            v-if="voiceMode === 'clone'"
            :cloned-voices="clonedVoices"
            :custom-voice-id="form.customVoiceId"
            :preview-clone-id="previewCloneId"
            v-model:train-speaker-id="trainSpeakerId"
            v-model:train-voice-name="trainVoiceName"
            :clone-file="cloneFile"
            :clone-file-list="cloneFileList"
            :train-loading="trainLoading"
            :train-result="trainResult"
            @select="selectCloneVoice"
            @preview="previewClone"
            @delete="deleteClone"
            @file-change="onCloneFileChange"
            @train="submitTrain"
          />

          <VoiceSettingsPanel
            v-model:voice-speed="form.voiceSpeed"
            v-model:voice-pitch="form.voicePitch"
            v-model:voice-volume="form.voiceVolume"
            v-model:emotion="form.emotion"
            v-model:emotion-scale="form.emotionScale"
            v-model:silence-duration="form.silenceDuration"
            :current-voice-supports-emotion="currentVoiceSupportsEmotion"
            :emotions="emotions"
          />

          <VoicePreviewBar
            v-model:preview-text="previewText"
            :preview-loading="previewLoading"
            :preview-audio="previewAudio"
            :saving="saving"
            @preview="doPreview"
            @save="saveVoice"
            @reset="resetForm"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useCharacterVoice } from "./composables/useCharacterVoice"
import PresetVoiceSection from "./components/PresetVoiceSection.vue"
import CloneVoiceSection from "./components/CloneVoiceSection.vue"
import VoiceSettingsPanel from "./components/VoiceSettingsPanel.vue"
import VoicePreviewBar from "./components/VoicePreviewBar.vue"

const {
  voicePresets, emotions, saving, previewLoading, previewText, previewAudio,
  voiceMode, form,
  trainSpeakerId, trainVoiceName, cloneFile, cloneFileList, trainLoading, trainResult,
  clonedVoices, previewCloneId,
  currentVoiceSupportsEmotion,
  onModeChange, selectCloneVoice, onVoiceTypeChange, onCloneFileChange,
  submitTrain, previewClone, deleteClone,
  doPreview, saveVoice, resetForm,
} = useCharacterVoice()
</script>

<style scoped>
.char-voice-page {
  padding: 20px 24px;
  max-width: 780px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 16px;
}

.page-header h2 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}

.page-desc {
  font-size: 13px;
  color: var(--ac-color-text-muted);
  margin: 4px 0 0;
}

.cards {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.card {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  overflow: hidden;
}

.card-body {
  padding: 16px;
}

.mode-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.mode-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--ac-color-text);
}
</style>
