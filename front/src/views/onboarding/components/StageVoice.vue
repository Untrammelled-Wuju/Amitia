<template>
  <div class="ob-stage-inner">
    <div class="ob-voice-experience">
      <div class="ob-voice-heading">
        <div class="ob-character-line">配置语音输出</div>
        <div class="ob-dialogue-note">当前选择：{{ currentVoiceDesc }}</div>
      </div>

      <div class="ob-voice-panel">
        <div class="ob-voice-section-head">
          <div class="ob-voice-title">声音气质</div>
          <div class="ob-voice-copy">选择默认声音风格并连接语音服务。完成角色设定后，Amitia 将使用该声音与你交流。</div>
        </div>
        <div class="ob-voice-list">
          <div
            v-for="voice in voices"
            :key="voice.name"
            class="ob-voice"
            :class="{ selected: voiceStyle === voice.name }"
            @click="emit('update:voiceStyle', voice.name)"
          >
            <button class="ob-voice-play" type="button" @click.stop="emit('playVoice', voice.name)">▶</button>
            <div>
              <div class="ob-voice-name">{{ voice.name }}</div>
              <div class="ob-voice-desc">{{ voice.desc }}</div>
            </div>
            <span class="ob-voice-check">✓</span>
          </div>
        </div>

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
            <label class="ob-input-label ob-voice-model-key">API Key
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
                <input
                  :value="voiceModelVoiceType"
                  @input="emit('update:voiceModelVoiceType', ($event.target as HTMLInputElement).value)"
                />
                <button class="ob-small-action" @click="emit('detectVoice')" :disabled="detectingVoice">测试声音</button>
              </div>
            </label>
          </div>
          <div class="ob-model-status" :class="{ ok: voiceReady }">
            <span class="ob-model-dot"></span>
            <span>{{ voiceStatusText }}</span>
          </div>
          <div class="ob-model-setup-action-row">
            <button class="ob-model-setup-next" @click="emit('next')" :disabled="!canContinue">继续</button>
          </div>
        </div>
      </div>

      <div class="ob-voice-spoken-line">试听并确认默认声音风格。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  voiceStyle: string
  voiceModelMode: string
  voiceModelKey: string
  voiceModelResource: string
  voiceModelVoiceType: string
  voiceReady: boolean
  detectingVoice: boolean
  voiceStatusText: string
}>()

const emit = defineEmits<{
  next: []
  playVoice: [name: string]
  detectVoice: []
  'update:voiceStyle': [value: string]
  'update:voiceModelMode': [value: string]
  'update:voiceModelKey': [value: string]
  'update:voiceModelResource': [value: string]
  'update:voiceModelVoiceType': [value: string]
}>()

const voices = [
  { name: "温和", desc: "平静自然，适合长时间交流" },
  { name: "清晰", desc: "表达直接，信息更利落" },
  { name: "柔和", desc: "语气更轻，陪伴感更明显" },
]

const voiceModelModes = [
  { label: "豆包语音", value: "volcengine" },
  { label: "暂不启用", value: "disabled" },
]

const currentVoiceDesc = computed(() => {
  const v = voices.find((v) => v.name === props.voiceStyle)
  return v ? `${v.name} · ${v.desc}` : "请选择声音风格"
})

const canContinue = computed(() => {
  if (props.voiceModelMode === "disabled") return true
  return props.voiceReady
})
</script>
