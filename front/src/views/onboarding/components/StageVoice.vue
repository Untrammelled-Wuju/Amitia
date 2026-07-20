<template>
  <div class="ob-stage-inner">
    <div class="ob-voice-experience">
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
          <div v-if="voiceModelMode !== 'disabled'" class="ob-form-stack ob-voice-model-fields">
            <label class="ob-input-label">接口地址
              <input
                :value="voiceModelURL"
                @input="emit('update:voiceModelURL', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="ob-input-label">API Key
              <input
                :value="voiceModelKey"
                @input="emit('update:voiceModelKey', ($event.target as HTMLInputElement).value)"
                type="password"
                placeholder="火山引擎 API Key"
              />
            </label>
            <label class="ob-input-label">语音合成模型
              <input
                :value="voiceModelResource"
                @input="emit('update:voiceModelResource', ($event.target as HTMLInputElement).value)"
              />
            </label>
            <label class="ob-input-label">默认音色
              <div class="ob-voice-model-test-row">
                <el-select
                  :model-value="voiceModelVoiceType"
                  @update:model-value="(val: string) => emit('update:voiceModelVoiceType', val)"
                  size="small"
                  style="flex:1"
                >
                  <el-option v-for="v in voicePresets" :key="v.name" :label="v.label" :value="v.name" />
                </el-select>
                <button class="ob-small-action" @click="emit('detectVoice')" :disabled="detectingVoice">{{ detectingVoice ? '合成中' : voiceDetected ? '重新测试' : '测试声音' }}</button>
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

      <div class="ob-voice-spoken-line">试听并确认默认声音风格。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue"
import { useApi } from "../../../composables/useApi"


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
