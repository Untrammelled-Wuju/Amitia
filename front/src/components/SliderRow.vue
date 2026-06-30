<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="slider-row">
    <div class="sr-header">
      <span class="sr-label">{{ label }}</span>
    </div>
    <div class="sr-body">
      <span class="sr-left">{{ left }}</span>
      <div class="sr-slider-wrap">
        <el-slider
          :model-value="props.modelValue"
          @update:model-value="(v: number) => $emit('update:modelValue', Math.round(v))"
          :min="min"
          :max="max"
          :step="1"
          :show-tooltip="false"
          class="sr-slider"
        />
      </div>
      <span class="sr-right">{{ right }}</span>
      <span class="sr-value">{{ props.modelValue }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  modelValue?: number
  label: string
  left: string
  right: string
  min: number
  max: number
}>(), {
  modelValue: 50
})

defineEmits<{
  (e: "update:modelValue", v: number): void
}>()
</script>

<style scoped>
.slider-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sr-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.sr-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--ac-color-text);
}

.sr-body {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sr-left, .sr-right {
  font-size: 10px;
  color: var(--ac-color-text-placeholder);
  min-width: 24px;
}

.sr-left { text-align: right; }

.sr-slider-wrap {
  flex: 1;
}

.sr-slider {
  --el-slider-height: 4px;
}

.sr-value {
  font-size: 11px;
  font-weight: 700;
  color: var(--ac-color-primary);
  min-width: 22px;
  text-align: right;
}
</style>
