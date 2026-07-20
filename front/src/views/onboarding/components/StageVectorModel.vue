<template>
  <div class="ob-stage-inner">
    <div class="ob-model-page-experience">
      <div class="ob-model-page-group">
      <div class="ob-model-page-heading">
        <div class="ob-character-line">配置长期记忆检索</div>
        <div class="ob-dialogue-note">向量模型会为记忆生成语义索引，帮助 Amitia 在之后的对话中找到相关内容。</div>
      </div>

      <div class="ob-model-page-panel">
        <div class="ob-model-page-card">
          <div class="ob-model-page-card-head">
            <div>
              <div class="ob-model-page-card-kicker">记忆检索</div>
              <h3 class="ob-model-panel-title">向量模型配置</h3>
            </div>
          </div>
          <p class="ob-model-panel-copy">它只负责查找相关记忆，不会自行创建、修改或删除记忆。</p>
          <div class="ob-form-stack ob-model-page-fields">
            <label class="ob-input-label ob-model-page-wide">接口地址
              <div class="ob-model-page-detect-row">
                <input
                  :value="vectorModelURL"
                  @input="emit('update:vectorModelURL', ($event.target as HTMLInputElement).value)"
                />
              </div>
            </label>
            <label class="ob-input-label">API Key
              <div class="ob-input-password-wrap">
                <input :value="vectorModelKey" @input="emit('update:vectorModelKey', ($event.target as HTMLInputElement).value)" :type="showVectorApiKey ? 'text' : 'password'" placeholder="火山引擎 API Key" />
                <button type="button" class="ob-password-toggle" @click="showVectorApiKey = !showVectorApiKey" tabindex="-1">
                  <svg v-if="!showVectorApiKey" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                </button>
              </div>
            </label>
            <label class="ob-input-label">向量模型
              <div class="ob-model-name-row">
                <input
                  :value="vectorModelName"
                  @input="emit('update:vectorModelName', ($event.target as HTMLInputElement).value)"
                />
                <button class="ob-small-action ob-detect-inline" @click="emit('detect')" :disabled="detecting">{{ detecting ? '检测中' : vectorDetected ? '重新检测' : '检测' }}</button>
              </div>
            </label>
          </div>
          <div class="ob-model-status" :class="{ ok: vectorReady }">
            <span class="ob-model-dot"></span>
            <span>{{ statusText }}</span>
          </div>
          <div class="ob-model-setup-action-row">
            <a href="https://console.volcengine.com/ark/region:ark+cn-beijing/model/detail?Id=doubao-embedding-vision" target="_blank" class="ob-model-settings-link">前往火山引擎获取 API Key</a>
            <button class="ob-model-setup-next" @click="emit('next')" :disabled="!vectorReady">继续</button>
          </div>
        </div>
      </div>

      </div>
      <div class="ob-model-page-spoken">配置完成后，长期记忆才能被更准确地检索。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  vectorModelKey: string
  vectorModelName: string
  vectorModelURL: string
  vectorReady: boolean
  vectorDetected: boolean
  detecting: boolean
  statusText: string
}>()

import { ref } from "vue"
const showVectorApiKey = ref(false)
const emit = defineEmits<{
  next: []
  detect: []
  'update:vectorModelKey': [value: string]
  'update:vectorModelName': [value: string]
  'update:vectorModelURL': [value: string]
}>()
</script>
