<template>
  <div class="ob-stage-inner">
    <div class="ob-model-page-experience">
      <div class="ob-model-page-group">
        <div class="ob-model-page-heading">
          <div class="ob-character-line">连接语言模型</div>
          <div class="ob-dialogue-note">
            语言模型负责理解对话、组织回复和完成推理。请选择服务并完成连接检测。
          </div>
        </div>

        <div class="ob-model-page-panel">
          <div class="ob-model-page-card">
            <div class="ob-model-page-card-head">
              <div>
                <div class="ob-model-page-card-kicker">
                  模型服务（Open AI格式）
                </div>
                <h3 class="ob-model-panel-title">语言模型配置</h3>
              </div>
              <div class="ob-model-types">
                <button
                  v-for="t in modelTypes"
                  :key="t.value"
                  class="ob-chip"
                  :class="{ selected: modelType === t.value }"
                  @click="onTypeChange(t.value)"
                >
                  {{ t.label }}
                </button>
              </div>
            </div>
            <p class="ob-model-panel-copy">
              这里只配置底层能力，不会改变角色的名字、身份或性格。
            </p>
            <div class="ob-form-stack ob-model-page-fields">
              <div
                v-if="modelType === 'compatible'"
                style="display: flex; gap: 11px; align-items: flex-start; grid-column: 1 / -1"
              >
                <label class="ob-input-label" style="flex: 0 0 200px"
                  >服务地址
                  <input
                    :value="baseUrl"
                    @input="
                      emit(
                        'update:baseUrl',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                    placeholder="https://api.deepseek.com/v1"
                    :class="{ 'ob-field-error': fieldErrors.baseUrl }"
                  />
                </label>
                <label class="ob-input-label" style="flex: 1; min-width: 0"
                  >API Key
                  <div class="ob-input-password-wrap">
                    <input
                      :value="apiKey"
                      @input="
                        emit(
                          'update:apiKey',
                          ($event.target as HTMLInputElement).value,
                        )
                      "
                      :type="showApiKey ? 'text' : 'password'"
                      placeholder="sk-..."
                      :class="{ 'ob-field-error': fieldErrors.apiKey }"
                    />
                    <button
                      type="button"
                      class="ob-password-toggle"
                      @click="showApiKey = !showApiKey"
                      tabindex="-1"
                    >
                      <svg
                        v-if="!showApiKey"
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
                          d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"
                        />
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
                  >服务地址
                  <input
                    :value="baseUrl"
                    @input="
                      emit(
                        'update:baseUrl',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                    :placeholder="
                      modelType === 'local'
                        ? 'http://localhost:11434/v1'
                        : 'https://api.deepseek.com/v1'
                    "
                    :class="{ 'ob-field-error': fieldErrors.baseUrl }"
                  />
                </label>
                <label v-if="modelType !== 'local'" class="ob-input-label"
                  >API Key
                  <div class="ob-input-password-wrap">
                    <input
                      :value="apiKey"
                      @input="
                        emit(
                          'update:apiKey',
                          ($event.target as HTMLInputElement).value,
                        )
                      "
                      :type="showApiKey ? 'text' : 'password'"
                      placeholder="sk-..."
                      :class="{ 'ob-field-error': fieldErrors.apiKey }"
                    />
                    <button
                      type="button"
                      class="ob-password-toggle"
                      @click="showApiKey = !showApiKey"
                      tabindex="-1"
                    >
                      <svg
                        v-if="!showApiKey"
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
              <label
                v-if="detectedModels.length > 0"
                class="ob-input-label ob-model-page-wide"
                >模型名称
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
                  <button
                    class="ob-small-action ob-detect-inline"
                    @click="emit('detect')"
                    :disabled="detecting"
                  >
                    {{
                      detecting
                        ? "检测中"
                        : modelDetected
                          ? "重新检测"
                          : "检测模型"
                    }}
                  </button>
                </div>
              </label>
              <label v-else class="ob-input-label ob-model-page-wide"
                >模型名称
                <div class="ob-model-name-row">
                  <input
                    :value="modelName"
                    @input="
                      emit(
                        'update:modelName',
                        ($event.target as HTMLInputElement).value,
                      )
                    "
                    :placeholder="
                      modelType === 'local'
                        ? '例如 llama3'
                        : '例如 deepseek-chat'
                    "
                    :class="{ 'ob-field-error': fieldErrors.modelName }"
                  />
                  <button
                    class="ob-small-action ob-detect-inline"
                    @click="emit('detect')"
                    :disabled="detecting"
                  >
                    {{
                      detecting
                        ? "检测中"
                        : modelDetected
                          ? "重新检测"
                          : "检测模型"
                    }}
                  </button>
                </div>
              </label>
            </div>
            <div class="ob-model-status" :class="{ ok: modelReady }">
              <span class="ob-model-dot"></span>
              <span>{{ statusText }}</span>
            </div>
            <div class="ob-model-setup-action-row">
              <a
                v-if="modelType !== 'local'"
                href="https://platform.deepseek.com/api_keys"
                target="_blank"
                class="ob-model-settings-link"
                >前往 DeepSeek 获取 API Key</a
              >
              <span v-else class="ob-model-settings-link"
                >使用本地 Ollama 服务</span
              >
              <button
                class="ob-model-setup-next"
                @click="emit('next')"
                :disabled="!modelReady"
              >
                继续
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="ob-model-page-spoken">
        连接成功后，Amitia 才能理解并回应你。
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import { useApi } from "../../../composables/useApi";

const showApiKey = ref(false);
const { get } = useApi();
const providers = ref<any[]>([]);
const selectedProvider = ref("");

onMounted(async () => {
  try {
    providers.value = (await get<any[]>("/api/model/providers")) || [];
  } catch {
    providers.value = [];
  }
});

function onProviderSelect(providerId: string) {
  selectedProvider.value = providerId;
  const provider = providers.value.find((p: any) => p.id === providerId);
  if (provider) {
    if (provider.defaultBaseUrl) {
      emit("update:baseUrl", provider.defaultBaseUrl);
    }
    if (provider.defaultModel) {
      emit("update:modelName", provider.defaultModel);
    }
  }
}

const props = defineProps<{
  detecting: boolean;
  modelReady: boolean;
  modelDetected: boolean;
  detectedModels: Array<{ id: string; ownedBy?: string }>;
  fieldErrors: Record<string, boolean>;
  statusText: string;
  baseUrl: string;
  apiKey: string;
  modelName: string;
  modelType: string;
}>();

const emit = defineEmits<{
  next: [];
  detect: [];
  "update:baseUrl": [value: string];
  "update:apiKey": [value: string];
  "update:modelName": [value: string];
  "update:modelType": [value: string];
}>();

const modelTypes = [
  { label: "在线模型", value: "online" },
  { label: "本地模型", value: "local" },
  { label: "兼容服务", value: "compatible" },
];

function onTypeChange(value: string) {
  emit("update:modelType", value);
}

watch(
  () => props.modelType,
  (val) => {
    if (val === "local") {
      emit("update:baseUrl", "http://localhost:11434/v1");
      emit("update:apiKey", "ollama");
    } else {
      emit("update:baseUrl", "https://api.deepseek.com/v1");
      emit("update:apiKey", "");
    }
    selectedProvider.value = "";
  },
);
</script>

<style scoped>
.ob-provider-select :deep(.el-select__wrapper) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  font-size: 9px;
  border: 1px solid var(--ob-line);
  border-radius: 999px;
  background: transparent;
  box-shadow: none;
  color: var(--ob-muted);
  transition:
    border-color 0.25s ease,
    background 0.25s ease;
}

.ob-provider-select :deep(.el-select__wrapper:hover) {
  border-color: rgba(200, 121, 91, 0.45);
  background: var(--ob-accent-soft);
  color: var(--ob-text);
}

.ob-provider-select :deep(.el-select__wrapper.is-focused) {
  border-color: rgba(200, 121, 91, 0.45);
  background: var(--ob-accent-soft);
  color: var(--ob-text);
  box-shadow: none;
}

.ob-provider-select :deep(.el-select__placeholder) {
  font-size: 9px;
  color: var(--ob-muted);
}

.ob-provider-select :deep(.el-select__selected-item) {
  font-size: 9px;
  color: var(--ob-text);
}

.ob-provider-select :deep(.el-select__caret) {
  font-size: 9px;
  color: var(--ob-muted);
}
</style>
