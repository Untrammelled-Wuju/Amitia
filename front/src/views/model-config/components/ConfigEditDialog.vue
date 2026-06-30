<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="visible"
    :title="editingId ? '编辑模型配置' : '新增模型配置'"
    width="520px"
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
        <el-input v-model="form.name" placeholder="例如: 我的 GPT、本地 Ollama" />
      </el-form-item>

      <el-form-item label="类型" prop="apiType">
        <el-select v-model="form.apiType" style="width:100%" @change="(v: string) => emit('onProviderChange', v)">
          <el-option
            v-for="p in providers"
            :key="p.id"
            :label="p.name"
            :value="p.id"
          >
            <span>{{ p.name }}</span>
            <span style="float:right;font-size:11px;color:var(--el-text-color-secondary)">{{ p.id }}</span>
          </el-option>
        </el-select>
        <div class="form-hint" v-if="currentProviderSchema">
          {{ currentProviderSchema.description || '' }}
        </div>
        <div v-if="currentProviderSchema" style="margin-top:6px;display:flex;gap:4px;flex-wrap:wrap">
          <el-tag
            v-for="(val, key) in currentProviderSchema.capabilities"
            :key="key"
            size="small"
            :type="val ? 'success' : 'info'"
            :disable-transitions="true"
          >
            {{ capLabel(key) }}{{ val ? '' : ' x' }}
          </el-tag>
        </div>
      </el-form-item>

      <el-form-item label="Base URL" prop="baseUrl">
        <el-input v-model="form.baseUrl" placeholder="http://127.0.0.1:11434" />
      </el-form-item>

      <el-form-item label="API Key" prop="apiKey">
        <el-input
          v-model="form.apiKey"
          type="password"
          show-password
          :placeholder="editingId ? '留空则保留现有 Key' : 'sk-...'"
        />
        <div class="form-hint">
          Ollama 本地模式可以不填。编辑时留空则保留现有 Key。
        </div>
      </el-form-item>

      <el-form-item label="模型名称" prop="modelName">
        <div class="model-detect-wrap">
          <div class="model-detect-row">
            <el-input v-model="form.modelName" placeholder="gpt-4o-mini / qwen2.5:7b / deepseek-chat" class="model-input" />
            <el-button type="success" size="small" :loading="detectingModels" @click="emit('detectModels')" :disabled="!form.baseUrl">
              {{ detectingModels ? '检测中' : '检测可用模型' }}
            </el-button>
          </div>
          <div v-if="localDetectError || detectError" class="detect-error">{{ localDetectError || detectError }}</div>
          <div v-if="detectedModels.length > 0" class="detect-dropdown">
            <div class="detect-hint">已检测到 {{ detectedModels.length }} 个模型，点击选择：</div>
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

      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="温度">
            <el-input-number
              v-model="form.temperature"
              :min="0"
              :max="2"
              :step="0.1"
              :precision="1"
              controls-position="right"
              style="width:100%"
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
              style="width:100%"
            />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="超时(秒)">
            <el-input-number
              v-model="form.timeoutSeconds"
              :min="5"
              :max="300"
              controls-position="right"
              style="width:100%"
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
              style="width:100%"
            />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="emit('save')">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from "vue"
import type { FormInstance, FormRules } from "element-plus"

const props = defineProps<{
  modelValue: boolean
  editingId: number | null
  form: any
  rules: FormRules
  providers: any[]
  currentProviderSchema: any
  detectingModels: boolean
  detectedModels: { id: string; owned_by?: string }[]
  detectError: string
  saving: boolean
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
  (e: "save"): void
  (e: "onProviderChange", apiType: string): void
  (e: "detectModels"): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
})

const formRef = ref<FormInstance>()
const localDetectError = ref("")

defineExpose({ formRef })

watch(() => props.detectError, (val) => {
  localDetectError.value = val
}, { immediate: true })

function selectModel(modelId: string) {
  props.form.modelName = modelId
  localDetectError.value = ""
}

function capLabel(key: string | number): string {
  const labels: Record<string, string> = {
    chat: "聊天", stream: "流式", vision: "视觉", tools: "工具",
    embeddings: "嵌入", local: "本地", remote: "远程",
  }
  const name = String(key)
  return labels[name] || name.replace(/_/g, " ")
}
</script>

<style scoped>
.form-hint {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  margin-top: 4px;
  line-height: 1.4;
}

.model-detect-wrap { position: relative; }
.model-detect-row { display: flex; gap: 8px; }
.model-detect-row .model-input { flex: 1; }
.detect-error { color: var(--ac-color-danger, #f56c6c); font-size: 12px; margin-top: 4px; }
.detect-dropdown { background: var(--ac-color-surface); border: 1px solid var(--ac-color-primary-border); border-radius: var(--ac-radius-sm); box-shadow: var(--ac-shadow-md); max-height: 180px; overflow-y: auto; margin-top: 6px; }
.detect-hint { padding: 6px 10px; font-size: 11px; color: var(--ac-color-text-muted); border-bottom: 1px solid var(--ac-color-border-light); }
.detect-option { padding: 8px 10px; cursor: pointer; font-size: 13px; color: var(--ac-color-text); transition: background .15s; }
.detect-option:hover { background: var(--ac-color-primary-bg); }
.detect-option.active { background: var(--ac-color-primary-bg); color: var(--ac-color-primary); font-weight: 600; }
</style>
