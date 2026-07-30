<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <el-alert
      type="info"
      :closable="false"
      show-icon
      style="margin-bottom: 14px"
    >
      <template #title>
        API Key 与语音配置仅存储在本地服务器，不会上传至第三方。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showDialog(null)"
        >新增配置</el-button
      >
    </div>

    <ConfigCardList
      :configs="configs"
      :testing-id="testingId"
      :providers="providers"
      @test="testConfig"
      @edit="showDialog"
      @set-active="setActive"
      @delete="delConfig"
      @add="showDialog(null)"
    />

    <ConfigEditDialog
      ref="editDialogRef"
      v-model="dialogVisible"
      :editing-id="editingId"
      :form="form"
      :rules="rules"
      :providers="providers"
      :current-provider-schema="currentProviderSchema"
      :detecting-models="detectingModels"
      :detected-models="detectedModels"
      :detect-error="detectError"
      :saving="saving"
      :show-advanced="showAdvanced"
      :show-detect="showDetect"
      :model-placeholder="modelPlaceholder"
      @save="saveConfig"
      @on-provider-change="onProviderChange"
      @detect-models="detectModels"
    >
      <template #extraFields>
        <el-form-item label="语音类型">
          <el-select
            v-if="isVolcengine"
            v-model="form.voiceType"
            filterable
            placeholder="选择语音"
            style="width: 100%"
          >
            <el-option
              v-for="v in voicePresets"
              :key="v.name"
              :label="v.label"
              :value="v.name"
            />
          </el-select>
          <el-input
            v-else
            v-model="form.voiceType"
            placeholder="语音名称"
          />
        </el-form-item>

        <el-row :gutter="12">
          <el-col :span="8">
            <el-form-item label="语速">
              <el-input-number
                v-model="form.speed"
                :min="0.5"
                :max="2.0"
                :step="0.1"
                :precision="1"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="音调">
              <el-input-number
                v-model="form.pitch"
                :min="-12"
                :max="12"
                :step="1"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="音量">
              <el-input-number
                v-model="form.volume"
                :min="0.5"
                :max="2.0"
                :step="0.1"
                :precision="1"
                controls-position="right"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <template v-if="isVolcengine">
          <el-form-item label="复刻资源ID">
            <el-input
              v-model="form.cloneResourceId"
              placeholder="volc.megatts.timbre"
            />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="8">
              <el-form-item label="APP ID">
                <el-input
                  v-model="form.realtimeAppId"
                  placeholder="火山引擎APP ID"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="Access Token">
                <el-input
                  v-model="form.realtimeAccessToken"
                  type="password"
                  show-password
                  placeholder="Access Token"
                />
              </el-form-item>
            </el-col>
            <el-col :span="8">
              <el-form-item label="Secret Key">
                <el-input
                  v-model="form.realtimeSecretKey"
                  type="password"
                  show-password
                  placeholder="Secret Key"
                />
              </el-form-item>
            </el-col>
          </el-row>
          <div class="quick-links">
            <span>快捷入口：</span>
            <el-link
              href="https://console.volcengine.com/speech/new/setting/apikeys?projectName=default"
              target="_blank"
              class="quick-link"
              >API Key管理</el-link
            >
            <el-link
              href="https://console.volcengine.com/speech/service/10035?AppID=3815252154"
              target="_blank"
              class="quick-link"
              >豆包语音合成模型</el-link
            >
            <el-link
              href="https://console.volcengine.com/speech/service/10036?AppID=3815252154"
              target="_blank"
              class="quick-link"
              >声音复刻模型</el-link
            >
            <el-link
              href="https://console.volcengine.com/speech/new/experience/clone"
              target="_blank"
              class="quick-link"
              >声音复刻</el-link
            >
            <el-link
              href="https://console.volcengine.com/speech/new/voices"
              target="_blank"
              class="quick-link"
              >音色管理</el-link
            >
          </div>
        </template>
      </template>
    </ConfigEditDialog>

    <TestResultDialog v-model="testResultVisible" :test-result="testResult" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from "vue";
import { Plus } from "@element-plus/icons-vue";
import { useApi } from "../../composables/useApi";
import { useModelConfig } from "./composables/useModelConfig";
import ConfigCardList from "./components/ConfigCardList.vue";
import ConfigEditDialog from "./components/ConfigEditDialog.vue";
import TestResultDialog from "./components/TestResultDialog.vue";

const { get } = useApi();

const modelConfig = useModelConfig({
  apiBase: "/api/tts",
  activatePath: "/activate",
  withScenario: false,
  withDetect: false,
  defaultApiType: "volcengine",
  defaultModel: "seed-tts-2.0",
  defaultBaseUrl: "https://openspeech.bytedance.com",
  modelLabel: "语音模型",
  showAdvanced: false,
  showDetect: false,
  modelPlaceholder: "seed-tts-2.0 / tts-1 / eleven_multilingual_v2",
  defaultIsActive: 1,
  extraFormFields: {
    voiceType: "zh_female_vv_uranus_bigtts",
    speed: 1.0,
    pitch: 0,
    volume: 1.0,
    cloneResourceId: "volc.megatts.timbre",
    realtimeAppId: "",
    realtimeAccessToken: "",
    realtimeSecretKey: "",
  },
  transformPayload: (payload: any) => {
    return {
      name: payload.name,
      apiType: payload.apiType,
      apiKey: payload.apiKey,
      baseUrl: payload.baseUrl,
      resourceId: payload.modelName,
      voiceType: payload.voiceType,
      speed: payload.speed,
      pitch: payload.pitch,
      volume: payload.volume,
      cloneResourceId: payload.cloneResourceId,
      realtimeAppId: payload.realtimeAppId,
      realtimeAccessToken: payload.realtimeAccessToken,
      realtimeSecretKey: payload.realtimeSecretKey,
    };
  },
  transformConfig: (config: any) => {
    if (!config) return config;
    return {
      ...config,
      modelName: config.resourceId || config.modelName || "",
    };
  },
  testResultMapper: (result: any) => ({
    success: true,
    latencyMs: 0,
    message: result?.msg || "连接测试成功",
    reply: "",
  }),
});

const {
  configs,
  providers,
  currentProviderSchema,
  dialogVisible,
  detectingModels,
  detectedModels,
  detectError,
  editingId,
  saving,
  testingId,
  testResultVisible,
  testResult,
  form,
  rules,
  showAdvanced,
  showDetect,
  modelPlaceholder,
  showDialog,
  saveConfig,
  testConfig,
  setActive,
  delConfig,
  onProviderChange,
  detectModels,
} = modelConfig;

const isVolcengine = computed(() => form.apiType === "volcengine");
const voicePresets = ref<any[]>([]);

onMounted(async () => {
  try {
    voicePresets.value = (await get<any[]>("/api/tts/voices")) || [];
  } catch {
    voicePresets.value = [];
  }
});

const editDialogRef = ref<InstanceType<typeof ConfigEditDialog>>();

watch(dialogVisible, async (v) => {
  if (v) {
    await nextTick();
    modelConfig.dialogFormRef.value = editDialogRef.value?.formRef ?? null;
  }
});
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}
.quick-links {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}
.quick-link {
  color: var(--el-color-primary) !important;
  text-decoration: underline !important;
}
.quick-link:hover {
  color: var(--el-color-primary) !important;
  text-decoration: underline !important;
}
</style>
