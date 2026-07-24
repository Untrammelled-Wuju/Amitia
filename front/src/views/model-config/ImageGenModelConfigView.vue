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
          <el-tag size="small" type="warning">豆包 Seedream 5.0</el-tag>
        </div>
      </template>
      <div class="imagegen-grid">
        <div class="imagegen-item">
          <span class="imagegen-label">API Key</span>
          <el-input
            v-model="apiKey"
            size="small"
            placeholder="火山引擎API Key"
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
        style="
          margin-top: 6px;
          font-size: 12px;
          color: var(--el-text-color-secondary);
        "
      >
        <el-link
          class="quick-link"
          href="https://console.volcengine.com/ark/region:cn-beijing/model/detail?name=doubao-seedream-5-0&agentMode=close"
          target="_blank"
        >
          前往火山引擎控制台开通生图模型并获取 API Key
        </el-link>
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
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useApi } from "../../composables/useApi";

const { get, post, put } = useApi();

const apiKey = ref("");
const modelName = ref("doubao-seedream-5-0");
const baseUrl = ref("https://ark.cn-beijing.volces.com/api/v3");
const saving = ref(false);
const testing = ref(false);
const testResult = ref("");
const configId = ref<number | null>(null);

async function fetchConfig() {
  try {
    const all = (await get<any[]>("/api/imagegen/configs")) || [];
    if (all.length > 0) {
      const cfg = all[0];
      configId.value = cfg.id;
      if (cfg.apiKey) apiKey.value = cfg.apiKey;
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

onMounted(() => {
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
  min-width: 60px;
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
