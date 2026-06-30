<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header><span class="section-title">配置</span></template>
    <div class="lr-config-form">
      <div class="lr-cfg-row">
        <span class="lr-cfg-label">最大任务数</span>
        <el-input-number v-model="config.maxTasks" :min="1" :max="20" size="small" />
      </div>
      <div class="lr-cfg-row">
        <span class="lr-cfg-label">超时时间 (分钟)</span>
        <el-input-number v-model="config.timeoutMinutes" :min="5" :max="120" size="small" />
      </div>
      <div class="lr-cfg-actions">
        <el-button type="primary" size="small" :loading="saving" @click="handleSave">保存配置</el-button>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { ElMessage } from "element-plus"
import { fetchConfigApi, saveConfigApi, type LongRunningConfig } from "../api"

const config = ref<LongRunningConfig>({
  maxTasks: 5,
  timeoutMinutes: 30,
})
const saving = ref(false)

async function loadConfig() {
  try {
    const data = await fetchConfigApi()
    if (data) { config.value = data }
  } catch {}
}

async function handleSave() {
  saving.value = true
  try {
    await saveConfigApi({
      maxTasks: config.value.maxTasks,
      timeoutMinutes: config.value.timeoutMinutes,
    })
    ElMessage.success("配置已保存")
  } catch {
  } finally {
    saving.value = false
  }
}

onMounted(loadConfig)
</script>

<style scoped>
.section-card {
  margin-bottom: 14px;
  border: 1px solid var(--ac-color-border-light);
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.lr-config-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.lr-cfg-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.lr-cfg-label {
  font-size: 13px;
  color: var(--ac-color-text);
  min-width: 130px;
}
.lr-cfg-actions {
  margin-top: 4px;
}
</style>
