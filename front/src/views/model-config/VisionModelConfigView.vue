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
        使用云端视觉模型时，图片内容会发送到模型服务商。API Key 与视觉模型配置仅存储在本地服务器。
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
    />

    <TestResultDialog v-model="testResultVisible" :test-result="testResult" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { Plus } from "@element-plus/icons-vue";
import { useModelConfig } from "./composables/useModelConfig";
import ConfigCardList from "./components/ConfigCardList.vue";
import ConfigEditDialog from "./components/ConfigEditDialog.vue";
import TestResultDialog from "./components/TestResultDialog.vue";

const modelConfig = useModelConfig({
  apiBase: "/api/vision",
  activatePath: "/activate",
  withScenario: false,
  withDetect: false,
  defaultApiType: "volcengine",
  defaultModel: "doubao-seed-2-0-lite-260428",
  defaultBaseUrl: "https://ark.cn-beijing.volces.com/api/v3",
  modelLabel: "视觉模型",
  showAdvanced: false,
  showDetect: false,
  modelPlaceholder: "doubao-seed-2-0-lite-260428 / gpt-4o / gemini-2.0-flash",
  defaultIsActive: 1,
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
</style>
