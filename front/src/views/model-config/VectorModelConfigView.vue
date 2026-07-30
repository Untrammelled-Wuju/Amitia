<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="embedding-config">
    <el-alert
      title="隐私提示"
      type="info"
      description="API Key 与向量模型配置仅存储在本地服务器，不会上传至第三方。使用云端向量模型时，文本内容会发送到模型服务商进行向量化。"
      show-icon
      closable
      style="margin-bottom: 16px"
    />

    <el-card class="embedding-card" shadow="hover">
      <template #header>
        <div class="embedding-header">
          <span>向量模型</span>
          <el-tag size="small" type="success">{{ currentProviderName }}</el-tag>
        </div>
      </template>
      <div class="embedding-grid">
        <div class="embedding-item">
          <span class="embedding-label">服务提供方</span>
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
        <div class="embedding-item">
          <span class="embedding-label">API Key</span>
          <el-input
            v-model="apiKey"
            size="small"
            placeholder="API Key"
            type="password"
            show-password
            style="width: 260px"
          />
        </div>
        <div class="embedding-item">
          <span class="embedding-label">模型名称</span>
          <el-input
            v-model="modelName"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="embedding-item">
          <span class="embedding-label">接口地址</span>
          <el-input
            v-model="baseUrl"
            size="small"
            style="width: 260px"
          />
        </div>
        <div class="embedding-item">
          <span class="embedding-label">测试连接</span>
          <el-button size="small" @click="testEmbedding" :loading="testing"
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
      <div style="margin-top: 12px">
        <el-button
          type="primary"
          size="small"
          @click="saveEmbedding"
          :loading="saving"
          >保存向量模型配置</el-button
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
const modelName = ref("doubao-embedding-vision-251215");
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
    providers.value = (await get<any[]>("/api/embedding/providers")) || [];
  } catch {
    providers.value = [
      { id: "volcengine", name: "火山引擎", defaultBaseUrl: "https://ark.cn-beijing.volces.com/api/v3", defaultModel: "doubao-embedding-vision-251215" },
      { id: "openai", name: "OpenAI", defaultBaseUrl: "https://api.openai.com/v1", defaultModel: "text-embedding-3-large" },
      { id: "qwen", name: "通义千问", defaultBaseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1", defaultModel: "text-embedding-v3" },
      { id: "zhipu", name: "智谱", defaultBaseUrl: "https://open.bigmodel.cn/api/paas/v4", defaultModel: "embedding-3" },
      { id: "siliconflow", name: "硅基流动", defaultBaseUrl: "https://api.siliconflow.cn/v1", defaultModel: "BAAI/bge-large-zh-v1.5" },
      { id: "jina", name: "Jina AI", defaultBaseUrl: "https://api.jina.ai/v1", defaultModel: "jina-embeddings-v3" },
    ];
  }
}

async function fetchConfig() {
  try {
    const all = (await get<any[]>("/api/embedding/configs")) || [];
    if (all.length > 0) {
      const cfg = all[0];
      configId.value = cfg.id;
      if (cfg.apiKey) apiKey.value = cfg.apiKey;
      apiType.value = cfg.apiType || "volcengine";
      modelName.value = cfg.modelName || "doubao-embedding-vision-251215";
      baseUrl.value = cfg.baseUrl || "https://ark.cn-beijing.volces.com/api/v3";
    }
  } catch {}
}

async function saveEmbedding() {
  if (!apiKey.value.trim()) {
    ElMessage.warning("请输入API Key");
    return;
  }
  saving.value = true;
  try {
    const payload = {
      name: "向量模型",
      apiType: apiType.value,
      apiKey: apiKey.value,
      modelName: modelName.value,
      baseUrl: baseUrl.value,
    };
    if (configId.value) {
      await put(`/api/embedding/configs/${configId.value}`, payload);
    } else {
      const r = await post<{ id: number }>("/api/embedding/configs", {
        ...payload,
        isActive: 1,
      });
      configId.value = r.id;
    }
    testResult.value = "";
    ElMessage.success("向量模型配置保存成功");
  } catch (err: any) {
    ElMessage.error(err?.message || "保存失败");
  } finally {
    saving.value = false;
  }
}

async function testEmbedding() {
  testing.value = true;
  testResult.value = "";
  try {
    if (!configId.value && apiKey.value.trim()) {
      await saveEmbedding();
    }
    if (!configId.value) {
      testing.value = false;
      return;
    }
    const result = await post<any>(
      `/api/embedding/configs/${configId.value}/test`,
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
.embedding-config {
  padding: 0;
}
.embedding-card {
  margin-bottom: 16px;
}
.embedding-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 15px;
}
.embedding-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.embedding-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.embedding-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  min-width: 70px;
  flex-shrink: 0;
}
</style>
