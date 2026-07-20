<template>
  <div class="ob-stage-inner">
    <div class="ob-voice-experience">
      <div class="ob-voice-group">
      <div class="ob-voice-heading">
        <div class="ob-character-line">配置语音输出</div>
        <div class="ob-dialogue-note">选择语音服务商并配置语音合成参数。</div>
      </div>

      <div class="ob-voice-panel">

        <div class="ob-voice-model-card">
          <div class="ob-voice-model-head">
            <div>
              <div class="ob-voice-model-kicker">语音服务</div>
              <div class="ob-voice-model-title">语音服务配置</div>
            </div>
            <div class="ob-model-types ob-voice-model-modes">
              <button
                v-for="mode in voiceModelModes"
                :key="mode.value"
                class="ob-chip"
                :class="{ selected: voiceModelMode === mode.value }"
                @click="emit('update:voiceModelMode', mode.value)"
              >{{ mode.label }}</button>
            </div>
          </div>
          <div class="ob-form-stack ob-voice-model-fields" :class="{ 'ob-fields-disabled': voiceModelMode === 'disabled' }">
            <label class="ob-input-label">接口地址
              <input
                :value="voiceModelURL" :disabled="voiceModelMode === 'disabled'"
                @input="emit('update:voiceModelURL', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="ob-input-label">API Key
              <div class="ob-input-password-wrap">
                <input :value="voiceModelKey" :disabled="voiceModelMode === 'disabled'" @input="emit('update:voiceModelKey', ($event.target as HTMLInputElement).value)" :type="showVoiceApiKey ? 'text' : 'password'" placeholder="火山引擎 API Key" />
                <button type="button" class="ob-password-toggle" @click="showVoiceApiKey = !showVoiceApiKey" tabindex="-1">
                  <svg v-if="!showVoiceApiKey" viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  <svg v-else viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                </button>
              </div>
            </label>
            <label class="ob-input-label">语音合成模型
              <input
                :value="voiceModelResource" :disabled="voiceModelMode === 'disabled'"
                @input="emit('update:voiceModelResource', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="ob-input-label">默认音色
              <div class="ob-voice-model-test-row">
                <el-select
                  :model-value="voiceModelVoiceType" :disabled="voiceModelMode === 'disabled'"
                  @update:model-value="(val: string) => emit('update:voiceModelVoiceType', val)"
                  size="small"
                  style="flex:1"
                >
                  <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
                </el-select>
                <button class="ob-small-action" @click="emit('detectVoice')" :disabled="detectingVoice || voiceModelMode === 'disabled'">{{ detectingVoice ? '合成中' : voiceDetected ? '重新测试' : '测试声音' }}</button>
              </div>
            </label>
          </div>
          <div class="ob-model-status" :class="{ ok: voiceReady }">
            <span class="ob-model-dot"></span>
            <span>{{ voiceStatusText }}</span>
          </div>
          <div class="ob-model-setup-action-row">
            <a href="https://console.volcengine.com/speech/new/setting/apikeys?projectName=default" target="_blank" class="ob-model-settings-link">前往火山引擎获取 API Key</a>
            <button class="ob-model-setup-next" @click="emit('next')" :disabled="!canContinue">继续</button>
          </div>
        </div>
      </div>

      </div>
      <div class="ob-voice-spoken-line">试听并确认默认声音风格。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { useApi } from "../../../composables/useApi"

const showVoiceApiKey = ref(false)

const props = defineProps<{
  voiceModelMode: string
  voiceModelURL: string
  voiceModelKey: string
  voiceModelResource: string
  voiceModelVoiceType: string
  voiceReady: boolean
  voiceDetected: boolean
  detectingVoice: boolean
  voiceStatusText: string
}>()

const emit = defineEmits<{
  next: []
  detectVoice: []
  'update:voiceModelMode': [value: string]
  'update:voiceModelURL': [value: string]
  'update:voiceModelKey': [value: string]
  'update:voiceModelResource': [value: string]
  'update:voiceModelVoiceType': [value: string]
}>()


const voiceModelModes = [
  { label: "豆包语音", value: "volcengine" },
  { label: "暂不启用", value: "disabled" },
]

const { get } = useApi()
const voicePresets = ref<any[]>([{ name: "zh_female_vv_jupiter_bigtts", label: "vv - 活泼灵动女声" }])

onMounted(async () => {
  try { voicePresets.value = await get("/api/tts/voices") } catch { }
})


const canContinue = computed(() => {
  if (props.voiceModelMode === "disabled") return true
  return props.voiceReady
})
</script>
