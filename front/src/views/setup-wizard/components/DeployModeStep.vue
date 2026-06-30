<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-options">
    <label class="sw-option-card" :class="{ selected: modelValue === 'desktop-local' }">
      <input :checked="modelValue === 'desktop-local'" type="radio" value="desktop-local" @change="emit('update:modelValue', 'desktop-local')" />
      <div class="sw-option-body">
        <strong>Desktop Local</strong>
        <p>Run everything on this computer. Data stays local. Only accessible from this machine.</p>
      </div>
    </label>
    <label class="sw-option-card" :class="{ selected: modelValue === 'cloud-web' }">
      <input :checked="modelValue === 'cloud-web'" type="radio" value="cloud-web" @change="emit('update:modelValue', 'cloud-web')" />
      <div class="sw-option-body">
        <strong>Cloud Web</strong>
        <p>Deploy on a cloud server. Access from anywhere via web browser. Your computer does not need to stay on.</p>
      </div>
    </label>
  </div>
</template>

<script setup lang="ts">
defineProps<{ modelValue: string }>()
const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>()
</script>
