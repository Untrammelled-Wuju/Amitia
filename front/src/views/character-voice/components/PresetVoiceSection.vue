<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="mode-section">
    <div class="form-item">
      <label>选择音色</label>
      <el-select v-model="voiceTypeModel" placeholder="选择音色" size="default" style="width:100%" @change="emit('voiceTypeChange')">
        <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
      </el-select>
      <span class="form-hint">从火山引擎预设音色中选择</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  voiceType: string
  voicePresets: { name: string; label: string; gender: string }[]
}>()

const emit = defineEmits<{
  (e: "update:voiceType", v: string): void
  (e: "voiceTypeChange"): void
}>()

const voiceTypeModel = computed({
  get: () => props.voiceType,
  set: (v) => emit("update:voiceType", v),
})
</script>

<style scoped>
.mode-section {
  margin-bottom: 16px;
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
</style>
