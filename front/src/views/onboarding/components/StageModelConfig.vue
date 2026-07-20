<template>
  <div class="ob-stage-inner">
    <div class="ob-model-page-experience">
      <div class="ob-model-page-heading">
        <div class="ob-character-line">连接语言模型</div>
        <div class="ob-dialogue-note">语言模型负责理解对话、组织回复和完成推理。请选择服务并完成连接检测。</div>
      </div>

      <div class="ob-model-page-panel">
        <div class="ob-model-page-card">
          <div class="ob-model-page-card-head">
            <div>
              <div class="ob-model-page-card-kicker">模型服务</div>
              <h3 class="ob-model-panel-title">语言模型配置</h3>
            </div>
            <div class="ob-model-types">
              <button
                v-for="t in modelTypes"
                :key="t.value"
                class="ob-chip"
                :class="{ selected: modelType === t.value }"
                @click="modelType = t.value"
              >{{ t.label }}</button>
            </div>
          </div>
          <p class="ob-model-panel-copy">这里只配置底层能力，不会改变角色的名字、身份或性格。</p>
          <div class="ob-form-stack ob-model-page-fields">
            <label class="ob-input-label">服务地址
              <input :value="baseUrl" @input="emit('update:baseUrl', ($event.target as HTMLInputElement).value)" placeholder="https://api.deepseek.com/v1" :class="{ 'ob-field-error': fieldErrors.baseUrl }" />
            </label>
            <label class="ob-input-label">API Key
              <input :value="apiKey" @input="emit('update:apiKey', ($event.target as HTMLInputElement).value)" type="password" placeholder="sk-..." :class="{ 'ob-field-error': fieldErrors.apiKey }" />
            </label>
            <label v-if="detectedModels.length > 0" class="ob-input-label ob-model-page-wide">模型名称
              <div class="ob-model-name-row">
                <el-select
                  :model-value="modelName"
                  @update:model-value="emit('update:modelName', $event)"
                  placeholder="选择一个模型"
                  class="ob-model-select"
                  :class="{ 'ob-field-error': fieldErrors.modelName }"
                >
                  <el-option
                    v-for="m in detectedModels"
                    :key="m.id"
                    :label="m.id"
                    :value="m.id"
                  />
                </el-select>
                <button class="ob-small-action ob-detect-inline" @click="emit('detect')" :disabled="detecting">
                  {{ detecting ? '检测中' : modelDetected ? '重新检测' : '检测模型' }}
                </button>
              </div>
            </label>
            <label v-else class="ob-input-label ob-model-page-wide">模型名称
              <div class="ob-model-name-row">
                <input :value="modelName" @input="emit('update:modelName', ($event.target as HTMLInputElement).value)" placeholder="例如 deepseek-chat" :class="{ 'ob-field-error': fieldErrors.modelName }" />
                <button class="ob-small-action ob-detect-inline" @click="emit('detect')" :disabled="detecting">
                  {{ detecting ? '检测中' : modelDetected ? '重新检测' : '检测模型' }}
                </button>
              </div>
            </label>
          </div>
          <div class="ob-model-status" :class="{ ok: modelReady }">
            <span class="ob-model-dot"></span>
            <span>{{ statusText }}</span>
          </div>
          <div class="ob-model-setup-action-row">
            <a href="https://platform.deepseek.com/api_keys" target="_blank" class="ob-model-settings-link">前往 DeepSeek 获取 API Key</a>
            <button
              class="ob-model-setup-next"
              @click="emit('next')"
              :disabled="!modelReady"
            >继续</button>
          </div>
        </div>
      </div>

      <div class="ob-model-page-spoken">连接成功后，Amitia 才能理解并回应你。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"

const props = defineProps<{
  detecting: boolean
  modelReady: boolean
  modelDetected: boolean
  detectedModels: Array<{id: string, ownedBy?: string}>
  fieldErrors: Record<string, boolean>
  statusText: string
  baseUrl: string
  apiKey: string
  modelName: string
}>()

const emit = defineEmits<{
  next: []
  detect: []
  'update:baseUrl': [value: string]
  'update:apiKey': [value: string]
  'update:modelName': [value: string]
}>()

const modelType = ref("online")

const modelTypes = [
  { label: "在线模型", value: "online" },
  { label: "本地模型", value: "local" },
  { label: "兼容服务", value: "compatible" },
]
</script>
