<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>配置 AI 模型</h2>
    <p class="step-desc">阿米提亚需要连接大语言模型才能对话。</p>
    <el-form label-position="top" size="default">
      <el-form-item label="API 类型">
        <el-select v-model="apiTypeModel" style="width:100%">
          <el-option label="OpenAI 兼容 (DeepSeek等)" value="openai-compatible" />
          <el-option label="Ollama (本地)" value="ollama" />
          <el-option label="自定义 HTTP" value="custom-http" />
        </el-select>
      </el-form-item>
      <el-form-item label="API Base URL">
        <el-input v-model="baseUrlModel" placeholder="https://api.deepseek.com/v1" />
      </el-form-item>
      <el-form-item label="API Key">
        <el-input v-model="apiKeyModel" type="password" placeholder="sk-..." show-password />
      </el-form-item>
      <el-form-item label="模型名称">
        <div class="model-detect-wrap">
          <div class="model-detect-row">
            <el-input v-model="modelNameModel" placeholder="请输入模型名称" class="model-input" />
            <el-button type="success" :loading="detectingModels" @click="emit('detect')" :disabled="!baseUrl || !apiKey">
              {{ detectingModels ? '检测中...' : '检测模型' }}
            </el-button>
          </div>
          <div v-if="detectError" class="detect-error">{{ detectError }}</div>
          <Transition name="dropdown">
            <div v-if="detectedModels.length > 0" class="detect-dropdown">
              <div class="detect-hint">检测到 {{ detectedModels.length }} 个模型，点击：</div>
              <div v-for="m in detectedModels" :key="m.id" class="detect-option" :class="{ active: modelNameModel === m.id }" @click="emit('pickModel', m.id)">
                {{ m.id }}
              </div>
            </div>
          </Transition>
        </div>
      </el-form-item>
    </el-form>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
const props = defineProps<{
  apiType: string; baseUrl: string; apiKey: string; modelName: string
  detectingModels: boolean; detectedModels: { id: string; owned_by?: string }[]; detectError: string
}>()
const emit = defineEmits<{
  (e: "update:apiType", v: string): void
  (e: "update:baseUrl", v: string): void
  (e: "update:apiKey", v: string): void
  (e: "update:modelName", v: string): void
  (e: "detect"): void
  (e: "pickModel", id: string): void
}>()
const apiTypeModel = computed({ get: () => props.apiType, set: (v) => emit("update:apiType", v) })
const baseUrlModel = computed({ get: () => props.baseUrl, set: (v) => emit("update:baseUrl", v) })
const apiKeyModel = computed({ get: () => props.apiKey, set: (v) => emit("update:apiKey", v) })
const modelNameModel = computed({ get: () => props.modelName, set: (v) => emit("update:modelName", v) })
</script>
