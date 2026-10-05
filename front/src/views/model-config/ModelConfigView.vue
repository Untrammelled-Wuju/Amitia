<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">模型配置</h2>

    <el-alert v-if="!checking && !canConfigure" title="AI 服务由云端 Core 提供。当前设备没有云端管理员权限，请在 Core 控制页面修改模型配置。" type="info" :closable="false" show-icon />
    <el-tabs v-if="canConfigure" :model-value="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="普通模型" name="llm" />
      <el-tab-pane label="语音模型" name="voice" />
      <el-tab-pane label="语音识别" name="asr" />
      <el-tab-pane label="视觉模型" name="vision" />
      <el-tab-pane label="向量模型" name="embedding" />
      <el-tab-pane label="生图模型" name="imagegen" />
    </el-tabs>

    <div v-if="checking" v-loading="true" class="access-loading" />
    <router-view v-else-if="canConfigure" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { getDeploymentConfig } from '@/runtime/runtime-adapter';
import { useApi } from '@/composables/useApi';

const router = useRouter();
const route = useRoute();
const checking = ref(true);
const canConfigure = ref(false);
const api = useApi();

async function checkAccess() {
  checking.value = true;
  canConfigure.value = false;
  try {
    const deployment = await getDeploymentConfig();
    if (deployment.mode === 'cloud') {
      const policy = await api.get<{ canAdminister: boolean }>('/api/device-mesh/v1/coordination/me');
      canConfigure.value = policy.canAdminister;
    } else { canConfigure.value = true; }
  } catch { canConfigure.value = false; }
  finally { checking.value = false; }
}

const activeTab = computed(() => {
  const path = route.path;
  if (path.includes("/model/llm")) return "llm";
  if (path.includes("/model/voice")) return "voice";
  if (path.includes("/model/asr")) return "asr";
  if (path.includes("/model/vision")) return "vision";
  if (path.includes("/model/embedding")) return "embedding";
  if (path.includes("/model/imagegen")) return "imagegen";
  return "llm";
});

function onTabChange(name: string) {
  router.push(`/settings/model/${name}`);
}

onMounted(() => {
  void checkAccess();
  window.addEventListener('amitia:execution-scope-changed', checkAccess);
  if (route.path === "/settings/model") {
    router.replace("/settings/model/llm");
  }
});
onUnmounted(() => window.removeEventListener('amitia:execution-scope-changed', checkAccess));
</script>

<style scoped>
.page {
  margin: 0 auto;
  padding: 20px 16px;
}
.page-title {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 12px;
}
.access-loading { min-height: 120px; }
</style>
