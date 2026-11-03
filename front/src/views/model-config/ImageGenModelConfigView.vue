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
        使用云端生图模型时，图片提示词会发送到模型服务商。API Key 与生图模型配置仅存储在本地服务器。
      </template>
    </el-alert>

    <div class="toolbar">
      <el-button type="primary" :icon="Plus" @click="showDialog(null)"
        >新增配置</el-button
      >
      <el-button
        :icon="MagicStick"
        :disabled="generating"
        @click="generationVisible = true"
        >生成测试</el-button
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

    <el-dialog v-model="generationVisible" title="生成测试" width="720px">
      <el-form label-position="top">
        <el-form-item label="提示词">
          <el-input
            v-model="generationPrompt"
            type="textarea"
            :rows="4"
            maxlength="4096"
            show-word-limit
            placeholder="描述希望生成的图片"
          />
        </el-form-item>
        <el-form-item label="尺寸">
          <el-select v-model="generationSize" style="width: 220px">
            <el-option label="1024 × 1024" value="1024x1024" />
            <el-option label="1280 × 720" value="1280x720" />
            <el-option label="720 × 1280" value="720x1280" />
          </el-select>
        </el-form-item>
        <el-form-item label="参考图资源 URI（可选，逗号分隔）">
          <el-input
            v-model="generationReferences"
            placeholder="amitia://artifacts/..."
          />
        </el-form-item>
      </el-form>
      <div v-if="generatedImages.length" class="generated-grid">
        <img
          v-for="image in generatedImages"
          :key="image"
          :src="image"
          alt="生成结果"
        />
      </div>
      <template #footer>
        <el-button @click="generationVisible = false">关闭</el-button>
        <el-button
          type="primary"
          :loading="generating"
          :disabled="!generationPrompt.trim()"
          @click="generateImage"
          >开始生成</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import { ElMessage } from "element-plus";
import { MagicStick, Plus } from "@element-plus/icons-vue";
import { useApi } from "@/composables/useApi";
import { resolveConversationMediaUrl } from "@/conversation/rendering/media";
import { useModelConfig } from "./composables/useModelConfig";
import ConfigCardList from "./components/ConfigCardList.vue";
import ConfigEditDialog from "./components/ConfigEditDialog.vue";
import TestResultDialog from "./components/TestResultDialog.vue";

const modelConfig = useModelConfig({
  apiBase: "/api/imagegen",
  activatePath: "/activate",
  withScenario: false,
  withDetect: false,
  defaultApiType: "seedream",
  defaultModel: "doubao-seedream-5-0",
  defaultBaseUrl: "https://ark.cn-beijing.volces.com/api/v3",
  modelLabel: "生图模型",
  showAdvanced: false,
  showDetect: false,
  modelPlaceholder: "doubao-seedream-5-0 / dall-e-3 / stable-image-core",
  defaultIsActive: 1,
  payloadFields: ["name", "apiType", "apiKey", "baseUrl", "modelName", "isActive", "enabled"],
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

const { post } = useApi();
const editDialogRef = ref<InstanceType<typeof ConfigEditDialog>>();
const generationVisible = ref(false);
const generationPrompt = ref("");
const generationSize = ref("1024x1024");
const generationReferences = ref("");
const generating = ref(false);
const generatedImages = ref<string[]>([]);

async function generateImage() {
  if (!generationPrompt.value.trim()) return;
  generating.value = true;
  generatedImages.value = [];
  try {
    const [width, height] = generationSize.value.split("x").map(Number);
    const result = await post<{
      images?: Array<{ resourceUri: string }>;
      provider?: string;
    }>("/api/image-intelligence/generate", {
      prompt: generationPrompt.value.trim(),
      width,
      height,
      count: 1,
      referenceResourceUris: generationReferences.value.split(",").map((item) => item.trim()).filter(Boolean),
    });
    const images = result?.images || [];
    generatedImages.value = await Promise.all(images.map((item) => resolveConversationMediaUrl(item.resourceUri)));
    if (!generatedImages.value.length) throw new Error("生图服务未返回图片");
    ElMessage.success(`生成完成${result?.provider ? ` · ${result.provider}` : ""}`);
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    generating.value = false;
  }
}

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

.generated-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.generated-grid img {
  width: 100%;
  border-radius: 8px;
  border: 1px solid var(--ac-color-border-light);
}
</style>
