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
                >
                  {{ mode.label }}
                </button>
              </div>
            </div>
            <p class="ob-voice-model-desc">
              可选择豆包语音服务，或暂不启用语音输出能力。
            </p>
            <div
              class="ob-form-stack ob-voice-model-fields"
              :class="{ 'ob-fields-disabled': voiceModelMode === 'disabled' }"
            >
              <div
                v-if="voiceModelMode === 'compatible'"
                style="display: flex; gap: 11px; align-items: flex-start; grid-column: 1 / -1"
              >
                <label class="ob-input-label" style="flex: 0 0 200px"
                  >服务地址
                  <input
                    :value="voiceModelURL"
                    :disabled="voiceModelMode === 'disabled'"
                    @input="
                      emit(
                        'update:voiceModelURL',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                    placeholder="https://openspeech.bytedance.com/api/v1"
                  />
                </label>
                <label class="ob-input-label" style="flex: 1; min-width: 0"
                  >API Key
                  <div class="ob-input-password-wrap">
                    <input
                      :value="voiceModelKey"
                      :disabled="voiceModelMode === 'disabled'"
                      @input="
                        emit(
                          'update:voiceModelKey',
                          ($event.target as HTMLInputElement).value,
                        )
                      "
                      :type="showVoiceApiKey ? 'text' : 'password'"
                      placeholder="API Key"
                    />
                    <button
                      type="button"
                      class="ob-password-toggle"
                      @click="showVoiceApiKey = !showVoiceApiKey"
                      tabindex="-1"
                    >
                      <svg
                        v-if="!showVoiceApiKey"
                        viewBox="0 0 24 24"
                        width="15"
                        height="15"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                      <svg
                        v-else
                        viewBox="0 0 24 24"
                        width="15"
                        height="15"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <path
                          d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                        />
                        <line x1="1" y1="1" x2="23" y2="23" />
                      </svg>
                    </button>
                  </div>
                </label>
                <div
                  class="ob-input-label"
                  style="flex: 0 0 80px"
                >
                  服务厂商
                  <el-select
                    :model-value="selectedProvider"
                    @update:model-value="onProviderSelect"
                    placeholder="选择厂商"
                    class="ob-provider-select"
                    clearable
                    style="width: 100%"
                  >
                    <el-option
                      v-for="p in providers"
                      :key="p.id"
                      :label="p.name"
                      :value="p.id"
                    >
                      <span>{{ p.name }}</span>
                      <span
                        style="
                          float: right;
                          font-size: 11px;
                          color: var(--el-text-color-secondary);
                        "
                        >{{ p.id }}</span
                      >
                    </el-option>
                  </el-select>
                </div>
              </div>
              <template v-else>
                <label class="ob-input-label"
                  >接口地址
                  <input
                    :value="voiceModelURL"
                    :disabled="voiceModelMode === 'disabled'"
                    @input="
                      emit(
                        'update:voiceModelURL',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                  />
                </label>
                <label class="ob-input-label"
                  >API Key
                  <div class="ob-input-password-wrap">
                    <input
                      :value="voiceModelKey"
                      :disabled="voiceModelMode === 'disabled'"
                      @input="
                        emit(
                          'update:voiceModelKey',
                          ($event.target as HTMLInputElement).value,
                        )
                      "
                      :type="showVoiceApiKey ? 'text' : 'password'"
                      placeholder="火山引擎 API Key"
                    />
                    <button
                      type="button"
                      class="ob-password-toggle"
                      @click="showVoiceApiKey = !showVoiceApiKey"
                      tabindex="-1"
                    >
                      <svg
                        v-if="!showVoiceApiKey"
                        viewBox="0 0 24 24"
                        width="15"
                        height="15"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                        <circle cx="12" cy="12" r="3" />
                      </svg>
                      <svg
                        v-else
                        viewBox="0 0 24 24"
                        width="15"
                        height="15"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      >
                        <path
                          d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"
                        />
                        <line x1="1" y1="1" x2="23" y2="23" />
                      </svg>
                    </button>
                  </div>
                </label>
              </template>
              <label class="ob-input-label"
                >语音合成模型
                <input
                  :value="voiceModelResource"
                  :disabled="voiceModelMode === 'disabled'"
                  @input="
                    emit(
                      'update:voiceModelResource',
                      ($event.target as HTMLInputElement).value,
                    )
                  "
                />
              </label>
              <label class="ob-input-label"
                >默认音色
                <div class="ob-voice-model-test-row">
                  <input
                    v-if="voiceModelMode === 'compatible'"
                    :value="voiceModelVoiceType"
                    :disabled="voiceModelMode === 'disabled'"
                    @input="
                      emit(
                        'update:voiceModelVoiceType',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                    placeholder="输入音色 ID"
                    style="flex: 1"
                  />
                  <el-select
                    v-else
                    :model-value="voiceModelVoiceType"
                    :disabled="voiceModelMode === 'disabled'"
                    @update:model-value="
                      (val: string) => emit('update:voiceModelVoiceType', val)
                    "
                    class="ob-provider-select"
                    style="flex: 1"
                  >
                    <el-option
                      v-for="v in voicePresets"
                      :key="v.name"
                      :label="v.label"
                      :value="v.name"
                    />
                  </el-select>
                  <button
                    class="ob-small-action"
                    @click="emit('detectVoice')"
                    :disabled="detectingVoice || voiceModelMode === 'disabled'"
                  >
                    {{
                      detectingVoice
                        ? "合成中"
                        : voiceDetected
                          ? "重新测试"
                          : "测试声音"
                    }}
                  </button>
                </div>
              </label>
            </div>
            <div class="ob-model-status" :class="{ ok: voiceReady }">
              <span class="ob-model-dot"></span>
              <span>{{ voiceStatusText }}</span>
            </div>
            <div class="ob-model-setup-action-row">
              <a
                href="https://console.volcengine.com/speech/new/setting/apikeys?projectName=default"
                target="_blank"
                class="ob-model-settings-link"
                >前往火山引擎获取 API Key</a
              >
              <button
                class="ob-model-setup-next"
                @click="emit('next')"
                :disabled="!canContinue"
              >
                继续
              </button>
            </div>
          </div>
        </div>
      </div>
      <div class="ob-voice-spoken-line">试听并确认默认声音风格。</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { useApi } from "../../../composables/useApi";

const showVoiceApiKey = ref(false);

const props = defineProps<{
  voiceModelMode: string;
  voiceModelURL: string;
  voiceModelKey: string;
  voiceModelResource: string;
  voiceModelVoiceType: string;
  voiceReady: boolean;
  voiceDetected: boolean;
  detectingVoice: boolean;
  voiceStatusText: string;
}>();

const emit = defineEmits<{
  next: [];
  detectVoice: [];
  "update:voiceModelMode": [value: string];
  "update:voiceModelURL": [value: string];
  "update:voiceModelKey": [value: string];
  "update:voiceModelResource": [value: string];
  "update:voiceModelVoiceType": [value: string];
}>();

const voiceModelModes = [
  { label: "豆包语音", value: "volcengine" },
  { label: "兼容服务", value: "compatible" },
  { label: "暂不启用", value: "disabled" },
];

const { get } = useApi();
const voicePresets = ref<any[]>([]);
const providers = ref<any[]>([]);
const selectedProvider = ref("");

onMounted(async () => {
  try {
    voicePresets.value = await get("/api/tts/voices");
  } catch {}
  try {
    providers.value = (await get<any[]>("/api/tts/providers")) || [];
  } catch {
    providers.value = [];
  }
});

function onProviderSelect(providerId: string) {
  selectedProvider.value = providerId;
  const provider = providers.value.find((p: any) => p.id === providerId);
  if (provider) {
    if (provider.defaultBaseUrl) {
      emit("update:voiceModelURL", provider.defaultBaseUrl);
    }
    if (provider.defaultModel) {
      emit("update:voiceModelResource", provider.defaultModel);
    }
  }
}

const canContinue = computed(() => {
  if (props.voiceModelMode === "disabled") return true;
  return props.voiceReady;
});
</script>

<style scoped>
.ob-provider-select :deep(.el-select__wrapper) {
  height: 36px;
  min-height: 36px;
  padding: 0 12px;
  font-size: 10px;
  border: 1px solid var(--ob-line);
  border-radius: 11px;
  background: rgba(255, 255, 255, 0.035);
  box-shadow: none;
  color: var(--ob-text);
  transition: border-color 0.25s ease;
}

.ob-provider-select :deep(.el-select__wrapper:hover) {
  border-color: rgba(200, 121, 91, 0.56);
}

.ob-provider-select :deep(.el-select__wrapper.is-focused) {
  border-color: rgba(200, 121, 91, 0.56);
  box-shadow: none;
}

.ob-provider-select :deep(.el-select__placeholder) {
  font-size: 10px;
  color: var(--ob-muted);
}

.ob-provider-select :deep(.el-select__selected-item) {
  font-size: 10px;
  color: var(--ob-text);
}

.ob-provider-select :deep(.el-select__caret) {
  font-size: 10px;
  color: var(--ob-muted);
}
</style>
