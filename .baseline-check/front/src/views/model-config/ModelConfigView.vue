<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">模型配置</h2>

    <el-tabs :model-value="activeTab" @tab-change="onTabChange">
      <el-tab-pane label="普通模型" name="llm" />
      <el-tab-pane label="语音模型" name="voice" />
      <el-tab-pane label="语音识别" name="asr" />
      <el-tab-pane label="视觉模型" name="vision" />
      <el-tab-pane label="向量模型" name="embedding" />
      <el-tab-pane label="生图模型" name="imagegen" />
    </el-tabs>

    <router-view />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from "vue";
import { useRouter, useRoute } from "vue-router";

const router = useRouter();
const route = useRoute();

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
  if (route.path === "/settings/model") {
    router.replace("/settings/model/llm");
  }
});
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
</style>
