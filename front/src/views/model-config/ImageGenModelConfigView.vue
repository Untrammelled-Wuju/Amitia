<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="imagegen-config">
    <el-alert
      title="隐私提示"
      type="info"
      description="API Key 与生图模型配置仅存储在本地服务器，不会上传至第三方。使用云端生图模型时，图片提示词会发送到模型服务商。"
      show-icon
      closable
      style="margin-bottom: 16px"
    />

    <el-card class="imagegen-card" shadow="hover">
      <template #header>
        <div class="imagegen-header">
          <span>生图模型</span>
          <el-tag size="small" type="warning">{{ currentProviderName }}</el-tag>
        </div>
      </template>
      <div class="imagegen-grid">
        <div class="imagegen-item">
          <span class="imagegen-label">服务提供方</span>
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
        <div class="imagegen-item">
          <span class="imagegen-label">API Key</span>
          <el-input
            v-model="apiKey"
            size="small"
            placeholder="API Key"
            type="password"
            show-password
            style="width: 260px"
          />
        </div>
        <div class="imagegen-item">
          <span class="imagegen-label">模型名称</span>
          <el-input
            v-model="modelName"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="imagegen-item">
          <span class="imagegen-label">接口地址</span>
          <el-input
            v-model="baseUrl"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="imagegen-item">
          <span class="imagegen-label">测试连接</span>
          <el-button size="small" @click="testImageGen" :loading="testing"
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
        v-if="currentProvider"
        style="
          margin-top: 6px;
          font-size: 12px;
          color: var(--el-text-color-secondary);
        "
      >
        <span>当前提供方：{{ currentProviderName }}，默认模型：{{ currentProvider.defaultModel }}</span>
      </div>
      <div style="margin-top: 12px">
        <el-button
          type="primary"
          size="small"
          @click="saveImageGen"
          :loading="saving"
          >保存生图模型配置</el-button
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
const apiType = ref("seedream");
const modelName = ref("doubao-seedream-5-0");
const baseUrl = ref("https://ark.cn-beijing.volces.com/api/v3");
const saving = ref(false);
const testing = ref(false);
const testResult = ref("");
const configId = ref<number | null>(null);
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
    providers.value = (await get<any[]>("/api/imagegen/providers")) || [];
  } catch {
    providers.value = [
      { id: "seedream", name: "火山引擎 Seedream", defaultBaseUrl: "https://ark.cn-beijing.volces.com/api/v3", defaultModel: "doubao-seedream-5-0" },
      { id: "openai", name: "OpenAI DALL-E", defaultBaseUrl: "https://api.openai.com/v1", defaultModel: "dall-e-3" },
      { id: "stability", name: "Stability AI", defaultBaseUrl: "https://api.stability.ai/v2beta", defaultModel: "stable-image-core" },
      { id: "tongyi", name: "阿里通义万相", defaultBaseUrl: "https://dashscope.aliyuncs.com/api/v1", defaultModel: "wanx2.1-t2i-turbo" },
      { id: "cogview", name: "智谱 CogView", defaultBaseUrl: "https://open.bigmodel.cn/api/paas/v4", defaultModel: "cogview-3-plus" },
      { id: "siliconflow", name: "硅基流动", defaultBaseUrl: "https://api.siliconflow.cn/v1", defaultModel: "Kwai-Kolors/Kolors" },
    ];
  }
}

async function fetchConfig() {
  try {
    const all = (await get<any[]>("/api/imagegen/configs")) || [];
    if (all.length > 0) {
      const cfg = all[0];
      configId.value = cfg.id;
      if (cfg.apiKey) apiKey.value = cfg.apiKey;
      apiType.value = cfg.apiType || "seedream";
      modelName.value = cfg.modelName || "doubao-seedream-5-0";
      baseUrl.value =
        cfg.baseUrl || "https://ark.cn-beijing.volces.com/api/v3";
    }
  } catch {}
}

async function saveImageGen() {
  if (!apiKey.value.trim()) {
    ElMessage.warning("请输入API Key");
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: "生图模型",
      apiType: apiType.value,
      apiKey: apiKey.value,
      modelName: modelName.value,
      baseUrl: baseUrl.value,
    };
    if (configId.value) {
      await put(`/api/imagegen/configs/${configId.value}`, payload);
    } else {
      const r = await post<{ id: number }>("/api/imagegen/configs", {
        ...payload,
        isActive: 1,
      });
      configId.value = r.id;
    }
    testResult.value = "";
    ElMessage.success("生图模型配置保存成功");
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function testImageGen() {
  testing.value = true;
  testResult.value = "";
  try {
    if (!configId.value && apiKey.value.trim()) {
      await saveImageGen();
    }
    if (!configId.value) {
      testing.value = false;
      return;
    }
    const result = await post<any>(
      `/api/imagegen/configs/${configId.value}/test`,
      { configId: configId.value },
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
  fetchConfig();
});
</script>

<style scoped>
.imagegen-config {
  padding: 0;
}
.imagegen-card {
  margin-bottom: 16px;
}
.imagegen-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 15px;
}
.imagegen-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.imagegen-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.imagegen-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  min-width: 70px;
  flex-shrink: 0;
}
</style>
