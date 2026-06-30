<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div class="preview-row">
      <el-input v-model="previewTextModel" placeholder="输入试听文本" style="flex:1" />
      <el-button type="primary" :loading="previewLoading" @click="emit('preview')">试听</el-button>
    </div>
    <div v-if="previewAudio" class="audio-preview">
      <audio :src="previewAudio" controls style="width:100%" />
    </div>
    <div class="save-bar">
      <el-button type="primary" :loading="saving" @click="emit('save')">保存配置</el-button>
      <el-button @click="emit('reset')">重置</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  previewText: string
  previewLoading: boolean
  previewAudio: string
  saving: boolean
}>()

const emit = defineEmits<{
  (e: "update:previewText", v: string): void
  (e: "preview"): void
  (e: "save"): void
  (e: "reset"): void
}>()

const previewTextModel = computed({
  get: () => props.previewText,
  set: (v) => emit("update:previewText", v),
})
</script>

<style scoped>
.preview-row {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
}
.audio-preview {
  margin-top: 12px;
}
.save-bar {
  display: flex;
  gap: 10px;
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--ac-color-border-light);
}
@media (max-width: 700px) {
  .preview-row {
    flex-direction: column;
  }
}
</style>
