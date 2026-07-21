<template>
  <div class="ob-stage-inner">
    <div class="ob-model-page-experience">
      <div class="ob-model-page-group">
      <div class="ob-model-page-heading">
        <div class="ob-character-line">设置图片理解能力</div>
        <div class="ob-dialogue-note">启用后，Amitia 可以理解你发送的图片，并将图片内容加入对话上下文。</div>
      </div>

      <div class="ob-model-page-panel">
        <div class="ob-model-page-card">
          <div class="ob-model-page-card-head">
            <div>
              <div class="ob-model-page-card-kicker">视觉服务</div>
              <h3 class="ob-model-panel-title">视觉模式</h3>
            </div>
            <div class="ob-model-types">
              <button
                v-for="mode in visionModes"
                :key="mode.value"
                class="ob-chip"
                :class="{ selected: visionMode === mode.value }"
                @click="emit('update:visionMode', mode.value)"
              >{{ mode.label }}</button>
            </div>
          </div>
          <p class="ob-model-panel-copy">可使用独立视觉模型、沿用支持多模态的语言模型，或暂不启用。</p>
          <div class="ob-form-stack ob-model-page-fields" :class="{ 'ob-fields-disabled': fieldsDisabled }">
            <label class="ob-input-label">接口地址
              <input
                :value="visionModelURL" :disabled="fieldsDisabled"
                @input="emit('update:visionModelURL', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="ob-input-label">API Key
              <div class="ob-input-password-wrap">
                <input :value="visionModelKey" :disabled="fieldsDisabled" @input="emit('update:visionModelKey', ($event.target as HTMLInputElement).value)" :type="showVisionApiKey ? 'text' : 'password'" placeholder="火山引擎 API Key" />
                <button type="button" class="ob-password-toggle" @click="showVisionApiKey = !showVisionApiKey" tabindex="-1">
                  <svg v-if="!showVisionApiKey" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                </button>
              </div>
            </label>
            <label class="ob-input-label ob-model-page-wide">视觉模型
              <div class="ob-model-name-row">
              <input
                :value="visionModelName" :disabled="fieldsDisabled"
                @input="emit('update:visionModelName', ($event.target as HTMLInputElement).value)"
              />
                <button class="ob-small-action ob-detect-inline" @click="emit('detect')" :disabled="detecting || fieldsDisabled">
                  {{ detecting ? '检测中' : visionDetected ? '重新检测' : '检测' }}
                </button>
              </div>
            </label>
          </div>
          <div class="ob-model-status" :class="{ ok: visionReady }">
            <span class="ob-model-dot"></span>
            <span>{{ statusText }}</span>
          </div>
          <div class="ob-model-setup-action-row">
            <a href="https://console.volcengine.com/ark/region:ark+cn-beijing/model/detail?Id=doubao-seed-2-0-lite-260428" target="_blank" class="ob-model-settings-link">前往火山引擎获取 API Key</a>
            <button class="ob-model-setup-next" @click="emit('next')" :disabled="!canContinue">继续</button>
          </div>
        </div>
      </div>

      </div>
      <div class="ob-model-page-spoken">启用后，图片也能成为对话的一部分。</div>
    </div>
  </div>
</template>

<script setup lang="ts">

import { computed, ref } from "vue"
const showVisionApiKey = ref(false)
const props = defineProps<{
  visionMode: string
  modelReady: boolean
  visionModelKey: string
  visionModelName: string
  visionModelURL: string
  visionReady: boolean
  visionDetected: boolean
  detecting: boolean
  statusText: string
}>()

const emit = defineEmits<{
  next: []
  detect: []
  'update:visionMode': [value: string]
  'update:visionModelKey': [value: string]
  'update:visionModelName': [value: string]
  'update:visionModelURL': [value: string]
}>()

const visionModes = [
  { label: "独立视觉模型", desc: "单独配置，能力和费用更易管理", value: "dedicated" },
  { label: "沿用语言模型", desc: "语言模型支持图片理解时使用", value: "inherit" },
  { label: "暂不启用", desc: "只处理文字和语音", value: "disabled" },
]

const canContinue = computed(() => {
  if (props.visionMode === "disabled") return true
  if (props.visionMode === "inherit") return props.modelReady
  return props.visionReady
})

const fieldsDisabled = computed(() => props.visionMode === "disabled" || props.visionMode === "inherit")
</script>
