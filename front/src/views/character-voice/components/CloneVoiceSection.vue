<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="mode-section">
    <div class="clone-list" v-if="clonedVoices.length > 0">
      <div class="clone-label-row">
        <span class="form-item-label">已保存的复刻音色</span>
      </div>
      <div
        v-for="v in clonedVoices"
        :key="v.speakerId"
        class="clone-option"
        :class="{ active: customVoiceId === v.speakerId }"
        @click="emit('select', v.speakerId)"
      >
        <div class="clone-info">
          <span class="clone-name">{{ v.name }}</span>
          <span class="clone-id">{{ v.speakerId }}</span>
        </div>
        <div class="clone-actions">
          <el-button size="small" @click.stop="emit('preview', v.speakerId)" :loading="previewCloneId === v.speakerId">试听</el-button>
          <el-button size="small" type="danger" @click.stop="emit('delete', v.speakerId, v.name)">删除</el-button>
        </div>
      </div>
    </div>

    <div class="clone-new-section">
      <div class="form-item-label">训练新音色</div>
      <p class="sub-desc">填入已购买的复刻槽位ID并上传语音样本</p>
      <div class="clone-form-row">
        <el-input v-model="trainSpeakerIdModel" placeholder="槽位ID，如 S_xxxxxxxx" size="default" style="width:220px" />
        <el-input v-model="trainVoiceNameModel" placeholder="备注名称" size="default" style="width:140px" />
      </div>
      <div class="clone-upload-row">
        <el-upload
          :auto-upload="false"
          :limit="1"
          accept=".mp3,.wav,.m4a,.webm,.ogg"
          :on-change="(f: any) => emit('fileChange', f)"
          :file-list="cloneFileList"
        >
          <el-button type="primary" plain size="small">选择音频文件</el-button>
          <template #tip>
            <span class="upload-tip">真实人声，10-30秒</span>
          </template>
        </el-upload>
      </div>
      <el-button type="success" @click="emit('train')" :loading="trainLoading" :disabled="!trainSpeakerId.trim() || !cloneFile" size="small">
        开始训练
      </el-button>
      <span v-if="trainResult" class="clone-result">{{ trainResult }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  clonedVoices: any[]
  customVoiceId: string
  previewCloneId: string
  trainSpeakerId: string
  trainVoiceName: string
  cloneFile: File | null
  cloneFileList: any[]
  trainLoading: boolean
  trainResult: string
}>()

const emit = defineEmits<{
  (e: "select", speakerId: string): void
  (e: "preview", speakerId: string): void
  (e: "delete", speakerId: string, name: string): void
  (e: "fileChange", file: any): void
  (e: "train"): void
  (e: "update:trainSpeakerId", v: string): void
  (e: "update:trainVoiceName", v: string): void
}>()

const trainSpeakerIdModel = computed({
  get: () => props.trainSpeakerId,
  set: (v) => emit("update:trainSpeakerId", v),
})

const trainVoiceNameModel = computed({
  get: () => props.trainVoiceName,
  set: (v) => emit("update:trainVoiceName", v),
})
</script>

<style scoped>
.mode-section {
  margin-bottom: 16px;
}
.clone-list {
  margin-bottom: 16px;
}
.clone-label-row {
  margin-bottom: 6px;
}
.form-item-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
  margin-bottom: 8px;
}
.clone-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 6px;
  margin-bottom: 6px;
  cursor: pointer;
  transition: border-color 0.2s, background 0.2s;
}
.clone-option:hover {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}
.clone-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-light-9);
}
.clone-info {
  display: flex;
  gap: 12px;
  align-items: center;
}
.clone-name {
  font-size: 13px;
  font-weight: 500;
}
.clone-id {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  font-family: monospace;
}
.clone-actions {
  display: flex;
  gap: 6px;
}
.clone-new-section {
  padding-top: 12px;
  border-top: 1px solid var(--ac-color-border-light);
}
.sub-desc {
  font-size: 12px;
  color: var(--ac-color-text-muted);
  margin: 0 0 12px;
}
.clone-form-row {
  display: flex;
  gap: 10px;
  margin-bottom: 8px;
}
.clone-upload-row {
  margin-bottom: 10px;
}
.upload-tip {
  font-size: 11px;
  color: var(--ac-color-text-placeholder);
}
.clone-result {
  margin-left: 10px;
  font-size: 13px;
  color: var(--el-color-success);
}
@media (max-width: 700px) {
  .clone-form-row {
    flex-direction: column;
  }
  .clone-option {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>
