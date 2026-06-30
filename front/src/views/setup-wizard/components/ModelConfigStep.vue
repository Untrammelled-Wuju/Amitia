<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="sw-form">
    <div class="sw-field">
      <label>API Type</label>
      <select v-model="apiTypeModel">
        <option value="openai-compatible">OpenAI Compatible (DeepSeek, etc.)</option>
        <option value="ollama">Ollama (Local)</option>
        <option value="custom-http">Custom HTTP</option>
      </select>
    </div>
    <div class="sw-field">
      <label>API Base URL</label>
      <input v-model="baseUrlModel" type="text" placeholder="https://api.deepseek.com/v1" />
    </div>
    <div class="sw-field">
      <label>API Key</label>
      <input v-model="apiKeyModel" type="password" placeholder="sk-..." />
    </div>
    <div class="sw-field">
      <label>Model Name</label>
      <input v-model="modelNameModel" type="text" placeholder="deepseek-chat" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  apiType: string
  baseUrl: string
  apiKey: string
  modelName: string
}>()

const emit = defineEmits<{
  (e: "update:apiType", v: string): void
  (e: "update:baseUrl", v: string): void
  (e: "update:apiKey", v: string): void
  (e: "update:modelName", v: string): void
}>()

const apiTypeModel = computed({ get: () => props.apiType, set: (v) => emit("update:apiType", v) })
const baseUrlModel = computed({ get: () => props.baseUrl, set: (v) => emit("update:baseUrl", v) })
const apiKeyModel = computed({ get: () => props.apiKey, set: (v) => emit("update:apiKey", v) })
const modelNameModel = computed({ get: () => props.modelName, set: (v) => emit("update:modelName", v) })
</script>
