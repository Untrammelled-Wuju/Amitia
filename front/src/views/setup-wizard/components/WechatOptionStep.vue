<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-options">
    <label class="sw-option-card" :class="{ selected: modelValue === true }">
      <input :checked="modelValue === true" type="radio" :value="true" @change="emit('update:modelValue', true)" />
      <div class="sw-option-body">
        <strong>Enable WeChat Access</strong>
        <p>Your companion can receive and respond to messages via WeChat. You will need to scan a QR code after setup.</p>
      </div>
    </label>
    <label class="sw-option-card" :class="{ selected: modelValue === false }">
      <input :checked="modelValue === false" type="radio" :value="false" @change="emit('update:modelValue', false)" />
      <div class="sw-option-body">
        <strong>Skip for Now</strong>
        <p>You can enable WeChat access later from the Settings page.</p>
      </div>
    </label>
  </div>
</template>

<script setup lang="ts">
defineProps<{ modelValue: boolean | null }>()
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void }>()
</script>
