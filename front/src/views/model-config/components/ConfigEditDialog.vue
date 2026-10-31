<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="visible"
    class="model-config-dialog"
    :title="editingId ? '编辑模型配置' : '新增模型配置'"
    width="min(520px, calc(100vw - 32px))"
    align-center
    destroy-on-close
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      @submit.prevent
    >
      <el-form-item label="名称" prop="name">
        <el-input
          v-model="form.name"
          placeholder="例如: 我的 GPT、本地 Ollama"
        />
      </el-form-item>

      <el-form-item label="类型" prop="apiType">
        <el-select
          :model-value="providerSelection"
          style="width: 100%"
          @change="selectProvider"
        >
          <el-option
            v-for="p in providerChoices"
            :key="p.value"
            :label="p.name"
            :value="p.value"
            :disabled="p.available === false"
          >
            <span>{{ p.available === false ? `${p.name} · ${p.unavailableReason || "不可用"}` : p.name }}</span>
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
        <div class="form-hint" v-if="currentProviderSchema">
          {{ currentProviderSchema.description || "" }}
        </div>
        <div
          v-if="currentProviderSchema"
          style="margin-top: 6px; display: flex; gap: 4px; flex-wrap: wrap"
        >
          <el-tag
            v-for="(val, key) in currentProviderSchema.capabilities"
            :key="key"
            size="small"
            :type="val ? 'success' : 'info'"
            :disable-transitions="true"
          >
            {{ capLabel(key) }}{{ val ? "" : " x" }}
          </el-tag>
          <el-tag
            size="small"
            :type="currentProviderSchema.available === false ? 'danger' : 'success'"
          >
            {{
              currentProviderSchema.available === false
                ? currentProviderSchema.unavailableReason || "不可用"
                : "运行环境可用"
            }}
          </el-tag>
        </div>
      </el-form-item>

      <el-form-item v-if="!isEmbeddedProvider" label="Base URL" prop="baseUrl">
        <el-input v-model="form.baseUrl" placeholder="http://127.0.0.1:11434" />
      </el-form-item>

      <el-form-item v-if="!isEmbeddedProvider" :label="isApiKeyOptional ? 'API Key (可选)' : 'API Key'" prop="apiKey">
        <el-input
          v-model="form.apiKey"
          type="password"
          show-password
          :placeholder="isApiKeyOptional ? '当前类型可留空' : (editingId ? '留空则保留现有 Key' : 'sk-...')"
        />
        <div class="form-hint">
          {{
            isApiKeyOptional
              ? '当前服务类型默认无需 API Key，如已配置鉴权可填写。'
              : '编辑时留空则保留现有 Key。'
          }}
        </div>
      </el-form-item>

      <template v-if="isEmbeddedProvider">
        <el-form-item
          v-for="field in providerFields"
          :key="field.key"
          :label="field.label"
          :required="field.required"
        >
          <el-select
            v-if="field.type === 'modelFile'"
            :model-value="providerConfig[field.key] || ''"
            style="width: 100%"
            :placeholder="field.placeholder || '选择本地模型文件'"
            @change="(value: string) => setProviderConfig(field.key, value)"
          >
            <el-option
              v-for="model in localModelsForField"
              :key="model.resourceUri"
              :label="`${model.name} · ${String(model.format).toUpperCase()} · ${formatBytes(model.sizeBytes)}`"
              :value="model.resourceUri"
            />
          </el-select>
          <div
            v-if="field.type === 'modelFile'"
            style="display: flex; gap: 8px; width: 100%; align-items: center"
          >
            <el-button
              size="small"
              :loading="uploadingField === field.key"
              @click="pickModelFile(field.key)"
            >
              导入{{ field.label }}
            </el-button>
            <span class="form-hint">{{ providerConfig[field.key] || "尚未选择" }}</span>
          </div>
          <el-switch
            v-else-if="field.type === 'boolean'"
            :model-value="providerConfig[field.key] === true"
            @update:model-value="(value: boolean) => setProviderConfig(field.key, value)"
          />
          <el-select
            v-else-if="field.type === 'select'"
            :model-value="providerConfig[field.key] || field.default || ''"
            style="width: 100%"
            @change="(value: string) => setProviderConfig(field.key, value)"
          >
            <el-option
              v-for="option in field.options || []"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
          <el-input-number
            v-else-if="field.type === 'number'"
            :model-value="providerConfig[field.key] ?? field.default"
            style="width: 100%"
            controls-position="right"
            @change="(value: number | undefined) => setProviderConfig(field.key, value ?? field.default ?? 0)"
          />
          <el-input
            v-else
            :model-value="providerConfig[field.key] || ''"
            :type="field.secret ? 'password' : 'text'"
            :show-password="field.secret"
            :placeholder="field.placeholder"
            @update:model-value="(value: string) => setProviderConfig(field.key, value)"
          />
          <div class="form-hint" v-if="field.help">{{ field.help }}</div>
        </el-form-item>
      </template>

      <el-form-item label="模型名称" prop="modelName">
        <div class="model-detect-wrap">
          <div class="model-detect-row">
            <el-input
              v-model="form.modelName"
              :placeholder="modelPlaceholder"
              class="model-input"
            />
            <el-button
              v-if="showDetect && !isEmbeddedProvider"
              type="success"
              size="small"
              :loading="detectingModels"
              @click="emit('detectModels')"
              :disabled="!form.baseUrl"
            >
              {{ detectingModels ? "检测中" : "检测可用模型" }}
            </el-button>
          </div>
          <div v-if="localDetectError || detectError" class="detect-error">
            {{ localDetectError || detectError }}
          </div>
          <div v-if="detectedModels.length > 0" class="detect-dropdown">
            <div class="detect-hint">
              已检测到 {{ detectedModels.length }} 个模型，点击选择：
            </div>
            <div
              v-for="m in detectedModels"
              :key="m.id"
              class="detect-option"
              :class="{ active: props.form.modelName === m.id }"
              @click="selectModel(m.id)"
            >
              {{ m.id }}
            </div>
          </div>
        </div>
      </el-form-item>

      <el-form-item v-if="showReasoningSettings" label="思考能力">
        <div class="reasoning-settings">
          <el-switch
            v-model="form.supportsReasoning"
            active-text="支持思考强度"
            inactive-text="不支持"
          />
          <el-select
            v-model="form.defaultReasoningEffort"
            :disabled="!form.supportsReasoning"
            style="width: 160px"
          >
            <el-option label="低" value="low" />
            <el-option label="中" value="medium" />
            <el-option label="高" value="high" />
            <el-option label="极高" value="xhigh" />
          </el-select>
        </div>
        <div class="form-hint">
          由用户声明模型能力，对话输入框会根据该设置展示思考强度。
        </div>
      </el-form-item>

      <el-form-item v-if="showReasoningSettings" label="视觉能力">
        <el-switch v-model="form.supportsVision" active-text="支持识图" />
        <div class="form-hint">开启并设为默认模型后，主模型承担图片识别，独立视觉模型暂停使用。</div>
      </el-form-item>

      <slot name="extraFields" />

      <el-row v-if="showAdvanced" :gutter="12">
        <el-col :span="12">
          <el-form-item label="温度">
            <el-input-number
              v-model="form.temperature"
              :min="0"
              :max="2"
              :step="0.1"
              :precision="1"
              controls-position="right"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="最大 Token">
            <el-input-number
              v-model="form.maxTokens"
              :min="256"
              :max="131072"
              :step="1024"
              controls-position="right"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <template v-if="showReasoning && 'budgetRateVersion' in form">
        <el-form-item label="预算计价版本" prop="budgetRateVersion">
          <el-input v-model="form.budgetRateVersion" placeholder="自定版本，如 provider-20261024" maxlength="128" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12"><el-form-item label="输入计价上界"><el-input-number v-model="form.budgetInputRate" :min="0" :max="100000000" :precision="0" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="输出计价上界"><el-input-number v-model="form.budgetOutputRate" :min="0" :max="100000000" :precision="0" /></el-form-item></el-col>
        </el-row>
        <p>单位：每百万 Token 的微美元（1 美元 = 1000000 微美元）。费用预算按此上界预留；启用时填写版本和两个正整数，未配置时受费用限制的委派不会调用模型。</p>
      </template>

      <el-row v-if="showAdvanced" :gutter="12">
        <el-col :span="12">
          <el-form-item label="超时(秒)">
            <el-input-number
              v-model="form.timeoutSeconds"
              :min="5"
              :max="300"
              controls-position="right"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="重试次数">
            <el-input-number
              v-model="form.retryCount"
              :min="0"
              :max="5"
              controls-position="right"
              style="width: 100%"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="saving"
        :disabled="currentProviderSchema?.available === false"
        @click="emit('save')"
        >保存</el-button
      >
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue";
import type { FormInstance, FormRules } from "element-plus";

const props = defineProps<{
  modelValue: boolean;
  editingId: number | null;
  form: any;
  rules: FormRules;
  providers: any[];
  currentProviderSchema: any;
  detectingModels: boolean;
  detectedModels: { id: string; owned_by?: string }[];
  detectError: string;
  saving: boolean;
  showAdvanced?: boolean;
  showDetect?: boolean;
  modelPlaceholder?: string;
  showReasoning?: boolean;
  localModels?: any[];
  uploadLocalModel?: (file: File, name?: string) => Promise<any>;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "save"): void;
  (e: "onProviderChange", apiType: string): void;
  (e: "detectModels"): void;
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
});

const isApiKeyOptional = computed(() => {
  const t = props.form?.apiType;
  return t === "ollama" || t === "edge" || t === "cosyvoice";
});
const showAdvanced = computed(() => props.showAdvanced ?? true);
const showDetect = computed(() => props.showDetect ?? true);
const modelPlaceholder = computed(() => props.modelPlaceholder ?? "gpt-4o-mini / qwen2.5:7b / deepseek-chat");
const showReasoningSettings = computed(() => props.showReasoning === true);
const isEmbeddedProvider = computed(() => props.currentProviderSchema?.runtimeKind === "embedded");
const providerConfig = computed<Record<string, any>>(() => {
  props.form.providerConfig ||= {};
  return props.form.providerConfig;
});
const providerFields = computed(() =>
  (props.currentProviderSchema?.configFields || []).filter(
    (field: any) => !["baseUrl", "apiKey", "modelName"].includes(field.key),
  ),
);
const localModelsForField = computed(() => {
  const formats = new Set(props.currentProviderSchema?.modelFormats || []);
  return (props.localModels || []).filter((model) => formats.size === 0 || formats.has(model.format));
});
const uploadingField = ref("");
const providerChoices = computed(() => props.providers.flatMap((provider) => {
  if (props.showReasoning && provider.id === "openai") {
    return [
      { ...provider, value: "openai:openai_chat", name: "OpenAI · Chat Completions" },
      { ...provider, value: "openai:openai_responses", name: "OpenAI · Responses" },
    ];
  }
  return [{ ...provider, value: provider.id }];
}));
const providerSelection = computed(() => {
  if (props.showReasoning && props.form.apiType === "openai") {
    return `openai:${props.form.protocol || "openai_responses"}`;
  }
  return props.form.apiType;
});
function selectProvider(value: string) {
  const [apiType, protocol] = value.startsWith("openai:")
    ? ["openai", value.slice("openai:".length)]
    : [value, ""];
  props.form.apiType = apiType;
  emit("onProviderChange", apiType);
  if (props.showReasoning) {
    props.form.protocol = protocol || props.providers.find((provider) => provider.id === apiType)?.defaultProtocol || "";
  }
}

const formRef = ref<FormInstance>();
const localDetectError = ref("");

defineExpose({ formRef });

watch(
  () => props.detectError,
  (val) => {
    localDetectError.value = val;
  },
  { immediate: true },
);

function selectModel(modelId: string) {
  props.form.modelName = modelId;
  localDetectError.value = "";
}

function setProviderConfig(key: string, value: any) {
  providerConfig.value[key] = value;
}

function pickModelFile(key: string) {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".gguf,.mnn";
  input.onchange = async () => {
    const file = input.files?.[0];
    if (!file) return;
    uploadingField.value = key;
    localDetectError.value = "";
    try {
      if (!props.uploadLocalModel) throw new Error("本地模型导入接口不可用");
      const model = await props.uploadLocalModel(file, file.name.replace(/\.[^.]+$/, ""));
      setProviderConfig(key, model.resourceUri);
      if (!props.form.modelName) props.form.modelName = model.name;
      setProviderConfig("localModelId", model.name);
    } catch (error: any) {
      localDetectError.value = error?.message || "模型导入失败";
    } finally {
      uploadingField.value = "";
    }
  };
  input.click();
}

function formatBytes(value: number) {
  const size = Number(value || 0);
  if (size >= 1024 ** 3) return `${(size / 1024 ** 3).toFixed(2)} GiB`;
  if (size >= 1024 ** 2) return `${(size / 1024 ** 2).toFixed(1)} MiB`;
  return `${Math.max(0, Math.round(size / 1024))} KiB`;
}

function capLabel(key: string | number): string {
  const labels: Record<string, string> = {
    chat: "聊天",
    stream: "流式",
    vision: "视觉",
    tools: "工具",
    embeddings: "嵌入",
    local: "本地",
    remote: "远程",
  };
  const name = String(key);
  return labels[name] || name.replace(/_/g, " ");
}
</script>

<style scoped>
.model-config-dialog {
  display: flex;
  flex-direction: column;
}
.model-config-dialog :deep(.el-dialog__header),
.model-config-dialog :deep(.el-dialog__footer) {
  flex-shrink: 0;
}
.model-config-dialog :deep(.el-dialog__body) {
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}
.form-hint {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-top: 4px;
  line-height: 1.4;
}

.reasoning-settings {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.model-detect-wrap {
  position: relative;
}
.model-detect-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.model-detect-row .model-input {
  flex: 1;
  min-width: 180px;
}
.detect-error {
  color: var(--ac-color-danger);
  font-size: 12px;
  margin-top: 4px;
}
.detect-dropdown {
  background: var(--ac-color-surface);
  border: 1px solid var(--ac-color-primary-border);
  border-radius: var(--ac-radius-sm);
  box-shadow: var(--ac-shadow-md);
  max-height: 180px;
  overflow-y: auto;
  margin-top: 6px;
}
.detect-hint {
  padding: 6px 10px;
  font-size: 11px;
  color: var(--ac-color-text-muted);
  border-bottom: 1px solid var(--ac-color-border-light);
}
.detect-option {
  padding: 8px 10px;
  cursor: pointer;
  font-size: 13px;
  color: var(--ac-color-text);
  transition: background 0.15s;
}
.detect-option:hover {
  background: var(--ac-color-primary-bg);
}
.detect-option.active {
  background: var(--ac-color-primary-bg);
  color: var(--ac-color-primary);
  font-weight: 600;
}
</style>
