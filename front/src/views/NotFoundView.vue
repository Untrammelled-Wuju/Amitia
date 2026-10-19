<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="not-found">
    <div class="nf-content">
      <div class="nf-code">404</div>
      <h2 class="nf-title">{{ error ? "页面加载失败" : "页面未找到" }}</h2>
      <p class="nf-desc">{{ error ? "渲染过程中发生了错误" : "您访问的页面不存在或暂时无法加载" }}</p>
      <div v-if="error" class="nf-error">
        <el-alert :title="error" type="error" :closable="false" show-icon />
      </div>
      <div class="nf-actions">
        <el-button type="primary" size="large" @click="goHome">返回首页</el-button>
        <el-button size="large" @click="retry" :loading="retrying">重试</el-button>
        <el-button size="large" @click="bugReportVisible = true">提交报错信息</el-button>
      </div>
    </div>
    <BugReportDialog v-model="bugReportVisible" :page-path="route.fullPath" />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import BugReportDialog from "@/components/feedback/BugReportDialog.vue"

defineProps<{
  error?: string | null
}>()

const router = useRouter()
const route = useRoute()
const retrying = ref(false)
const bugReportVisible = ref(false)

function goHome() {
  router.replace("/chat")
}

function retry() {
  retrying.value = true
  window.location.reload()
}
</script>

<style scoped>
.not-found {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 400px;
  padding: 40px 24px;
}

.nf-content {
  text-align: center;
  max-width: 480px;
}

.nf-code {
  font-size: 96px;
  font-weight: 700;
  line-height: 1;
  color: var(--el-color-primary);
  opacity: 0.35;
  margin-bottom: 8px;
  user-select: none;
}

.nf-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 0 0 8px;
}

.nf-desc {
font-size: 13px;
color: var(--el-text-color-secondary);
margin: 0 0 24px;
line-height: 1.6;
}

.nf-error {
  margin-bottom: 24px;
}

.nf-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}
</style>
