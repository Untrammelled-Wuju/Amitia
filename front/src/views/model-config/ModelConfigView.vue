<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="page">
    <h2 class="page-title">模型配置</h2>

    <div class="sub-nav">
      <router-link to="/model/llm" class="sub-nav-item" active-class="sub-nav-active">普通模型</router-link>
      <router-link to="/model/voice" class="sub-nav-item" active-class="sub-nav-active">语音模型</router-link>
      <router-link to="/model/vision" class="sub-nav-item" active-class="sub-nav-active">视觉模型</router-link>
      <router-link to="/model/embedding" class="sub-nav-item" active-class="sub-nav-active">向量模型</router-link>
    </div>

    <router-view />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from "vue"
import { useRouter } from "vue-router"

const router = useRouter()

onMounted(() => {
  if (router.currentRoute.value.path === "/model") {
    router.replace("/model/llm")
  }
})
</script>

<style scoped>
.page { max-width: 960px; margin: 0 auto; padding: 20px 16px; }
.page-title { font-size: 20px; font-weight: 600; margin: 0 0 12px; }

.sub-nav {
  display: flex;
  gap: 0;
  margin-bottom: 16px;
  border-bottom: 2px solid var(--el-border-color-light);
}
.sub-nav-item {
  padding: 8px 20px;
  font-size: 14px;
  color: var(--el-text-color-secondary);
  text-decoration: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: all .2s;
}
.sub-nav-item:hover { color: var(--el-color-primary); }
.sub-nav-active {
  color: var(--el-color-primary);
  border-bottom-color: var(--el-color-primary);
  font-weight: 500;
}
</style>
