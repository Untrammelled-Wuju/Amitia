<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div class="param-grid">
      <div class="param-item">
        <label>语速 <span class="param-value">{{ voiceSpeed }}</span></label>
        <el-slider v-model="voiceSpeedModel" :min="0.5" :max="2.0" :step="0.1" />
      </div>
      <div class="param-item">
        <label>音高 <span class="param-value">{{ voicePitch }}</span></label>
        <el-slider v-model="voicePitchModel" :min="-12" :max="12" :step="1" />
      </div>
      <div class="param-item">
        <label>音量 <span class="param-value">{{ voiceVolume }}</span></label>
        <el-slider v-model="voiceVolumeModel" :min="0" :max="2" :step="0.1" />
      </div>
    </div>

    <div class="extra-grid">
      <div class="form-item">
        <label>情感</label>
        <el-select v-model="emotionModel" placeholder="默认" size="default" style="width:100%" clearable :disabled="!currentVoiceSupportsEmotion">
          <el-option v-for="e in emotions" :key="e.value" :label="e.label" :value="e.value" />
        </el-select>
        <span class="form-hint">{{ currentVoiceSupportsEmotion ? '设置语音情感色彩' : '当前音色不支持情感参数' }}</span>
      </div>
      <div class="form-item">
        <label>情感强度 {{ emotionScale || 4 }}</label>
        <el-slider v-model="emotionScaleModel" :min="1" :max="5" :step="1" :disabled="!emotion || !currentVoiceSupportsEmotion" />
        <span class="form-hint">仅设置情感后生效，1~5，默认为4</span>
      </div>
      <div class="form-item">
        <label>句尾静音 {{ silenceDuration }}ms</label>
        <el-slider v-model="silenceDurationModel" :min="0" :max="5000" :step="100" />
        <span class="form-hint">句尾追加静音时长，0~5000ms</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  voiceSpeed: number
  voicePitch: number
  voiceVolume: number
  emotion: string
  emotionScale: number
  silenceDuration: number
  currentVoiceSupportsEmotion: boolean
  emotions: { value: string; label: string }[]
}>()

const emit = defineEmits<{
  (e: "update:voiceSpeed", v: number): void
  (e: "update:voicePitch", v: number): void
  (e: "update:voiceVolume", v: number): void
  (e: "update:emotion", v: string): void
  (e: "update:emotionScale", v: number): void
  (e: "update:silenceDuration", v: number): void
}>()

const voiceSpeedModel = computed({ get: () => props.voiceSpeed, set: (v) => emit("update:voiceSpeed", v) })
const voicePitchModel = computed({ get: () => props.voicePitch, set: (v) => emit("update:voicePitch", v) })
const voiceVolumeModel = computed({ get: () => props.voiceVolume, set: (v) => emit("update:voiceVolume", v) })
const emotionModel = computed({ get: () => props.emotion, set: (v) => emit("update:emotion", v) })
const emotionScaleModel = computed({ get: () => props.emotionScale, set: (v) => emit("update:emotionScale", v) })
const silenceDurationModel = computed({ get: () => props.silenceDuration, set: (v) => emit("update:silenceDuration", v) })
</script>

<style scoped>
.param-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px 20px;
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}

.extra-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 14px 20px;
  margin-top: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--ac-color-border-light);
}

.param-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.param-item label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  display: flex;
  align-items: center;
  gap: 8px;
}

.param-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
}

.form-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-item label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
}

.form-hint {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
  line-height: 1.3;
}

@media (max-width: 700px) {
  .param-grid, .extra-grid {
    grid-template-columns: 1fr;
  }
}
</style>
