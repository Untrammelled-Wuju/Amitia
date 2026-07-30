<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="vision-config">
    <el-alert
      title="隐私提示"
      type="info"
      description="API Key 与视觉模型配置仅存储在本地服务器，不会上传至第三方。使用云端视觉模型时，图片内容会发送到模型服务商。"
      show-icon
      closable
      style="margin-bottom: 16px"
    />

    <el-card class="vision-card" shadow="hover">
      <template #header>
        <div class="vision-header">
          <span>视觉模型</span>
          <el-tag size="small" type="warning">{{ currentProviderName }}</el-tag>
        </div>
      </template>
      <div class="vision-grid">
        <div class="vision-item">
          <span class="vision-label">服务提供方</span>
          <el-select
            v-model="apiType"
            size="small"
            placeholder="选择服务提供方"
            style="width: 260px"
            @change="onProviderChange"
          >
            <el-option
              v-for="p in providers"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </div>
        <div class="vision-item">
          <span class="vision-label">API Key</span>
          <el-input
            v-model="apiKey"
            size="small"
            placeholder="API Key"
            type="password"
            show-password
            style="width: 260px"
          />
        </div>
        <div class="vision-item">
          <span class="vision-label">模型名称</span>
          <el-input
            v-model="modelName"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="vision-item">
          <span class="vision-label">接口地址</span>
          <el-input
            v-model="baseUrl"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="vision-item">
          <span class="vision-label">测试连接</span>
          <el-button size="small" @click="testVision" :loading="testing"
            >测试</el-button
          >
          <span
            v-if="testResult"
            :style="{
              color:
                testResult === 'ok'
                  ? 'var(--ac-color-success)'
                  : 'var(--ac-color-danger)',
              marginLeft: '8px',
            }"
            >{{ testResult === "ok" ? "连接正常" : "连接失败" }}</span
          >
        </div>
      </div>
      <div
        v-if="currentProvider?.docsUrl"
        style="
          margin-top: 6px;
          font-size: 12px;
          color: var(--el-text-color-secondary);
        "
      >
        <el-link
          class="quick-link"
          :href="currentProvider.docsUrl"
          target="_blank"
        >
          前往 {{ currentProviderName }} 获取 API Key
        </el-link>
      </div>
      <div style="margin-top: 12px">
        <el-button
          type="primary"
          size="small"
          @click="saveVision"
          :loading="saving"
          >保存视觉模型配置</el-button
        >
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../composables/useApi";

const { get, post, put } = useApi();

const apiKey = ref("");
const apiType = ref("volcengine");
const modelName = ref("doubao-seed-2-0-lite-260428");
const baseUrl = ref("https://ark.cn-beijing.volces.com/api/v3");
const saving = ref(false);
const testing = ref(false);
const testResult = ref("");
const visionConfigId = ref<number | null>(null);
const providers = ref<any[]>([]);

const currentProvider = computed(() =>
  providers.value.find((p) => p.id === apiType.value)
);
const currentProviderName = computed(
  () => currentProvider.value?.name || "未选择"
);

function onProviderChange() {
  const p = currentProvider.value;
  if (p) {
    if (p.defaultBaseUrl) baseUrl.value = p.defaultBaseUrl;
    if (p.defaultModel) modelName.value = p.defaultModel;
  }
}

async function fetchProviders() {
  try {
    providers.value = (await get<any[]>("/api/vision/providers")) || [];
  } catch {
    providers.value = [
      { id: "volcengine", name: "火山引擎", defaultBaseUrl: "https://ark.cn-beijing.volces.com/api/v3", defaultModel: "doubao-seed-2-0-lite-260428" },
      { id: "openai", name: "OpenAI", defaultBaseUrl: "https://api.openai.com/v1", defaultModel: "gpt-4o" },
      { id: "gemini", name: "Google Gemini", defaultBaseUrl: "https://generativelanguage.googleapis.com", defaultModel: "gemini-2.0-flash" },
      { id: "qwen", name: "通义千问VL", defaultBaseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1", defaultModel: "qwen-vl-plus" },
      { id: "zhipu", name: "智谱GLM-4V", defaultBaseUrl: "https://open.bigmodel.cn/api/paas/v4", defaultModel: "glm-4v-plus" },
      { id: "siliconflow", name: "硅基流动", defaultBaseUrl: "https://api.siliconflow.cn/v1", defaultModel: "Qwen/Qwen2-VL-72B-Instruct" },
    ];
  }
}

async function fetchVisionConfig() {
  try {
    const all = (await get<any[]>("/api/vision/configs")) || [];
    if (all.length > 0) {
      const vision = all[0];
      visionConfigId.value = vision.id;
      if (vision.apiKey) apiKey.value = vision.apiKey;
      apiType.value = vision.apiType || "volcengine";
      modelName.value = vision.modelName || "doubao-seed-2-0-lite-260428";
      baseUrl.value =
        vision.baseUrl || "https://ark.cn-beijing.volces.com/api/v3";
    }
  } catch {}
}

async function saveVision() {
  if (!apiKey.value.trim()) {
    ElMessage.warning("请输入API Key");
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: "视觉模型",
      apiType: apiType.value,
      apiKey: apiKey.value,
      modelName: modelName.value,
      baseUrl: baseUrl.value,
    };
    if (visionConfigId.value) {
      await put(`/api/vision/configs/${visionConfigId.value}`, payload);
    } else {
      const r = await post<{ id: number }>("/api/vision/configs", {
        ...payload,
        isActive: 1,
      });
      visionConfigId.value = r.id;
    }
    testResult.value = "";
    ElMessage.success("视觉模型配置保存成功");
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function testVision() {
  testing.value = true;
  testResult.value = "";
  try {
    if (!visionConfigId.value && apiKey.value.trim()) {
      await saveVision();
    }
    if (!visionConfigId.value) {
      testing.value = false;
      return;
    }
    const result = await post<any>(
      `/api/vision/configs/${visionConfigId.value}/test`,
      { configId: visionConfigId.value },
    );
    testResult.value = result?.success !== false ? "ok" : "fail";
  } catch {
    testResult.value = "fail";
  } finally {
    testing.value = false;
  }
}

onMounted(async () => {
  await fetchProviders();
  fetchVisionConfig();
});
</script>

<style scoped>
.vision-config {
  padding: 0;
}
.vision-card {
  margin-bottom: 16px;
}
.vision-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 15px;
}
.vision-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.vision-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.vision-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  min-width: 70px;
  flex-shrink: 0;
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
