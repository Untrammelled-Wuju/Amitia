<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-form">
    <p class="sw-hint">Choose a default personality for your AI-Amitia companion. You can change this later.</p>
    <div v-if="characters.length === 0" class="sw-notice">
      No characters found. A default character will be created automatically.
    </div>
    <div v-else class="sw-char-list">
      <label
        v-for="c in characters"
        :key="c.id"
        class="sw-option-card sw-char-card"
        :class="{ selected: modelValue === c.id }"
      >
        <input :checked="modelValue === c.id" type="radio" :value="c.id" @change="emit('update:modelValue', c.id)" />
        <div class="sw-option-body">
          <strong>{{ c.name }}</strong>
          <p>{{ c.identity || 'No description' }}</p>
        </div>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: string
  characters: { id: string; name: string; identity: string }[]
}>()

const emit = defineEmits<{ (e: "update:modelValue", v: string): void }>()
</script>
