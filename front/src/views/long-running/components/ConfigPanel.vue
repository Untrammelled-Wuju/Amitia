<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header><span class="section-title">配置</span></template>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-button v-if="!config" @click="loadConfig">重新加载配置</el-button>
    <div v-if="config" class="lr-config-form">
      <div class="lr-cfg-row">
        <span class="lr-cfg-label">最大并发任务数</span>
        <el-input-number
          v-model="config.maxTasks"
          :min="1"
          :max="20"
          size="small"
        />
      </div>
      <div class="lr-cfg-row">
        <span class="lr-cfg-label">统一超时（秒）</span>
        <el-input-number v-model="config.timeout.seconds" :min="30" :max="1800" :step="30" :disabled="config.timeout.disabled" size="small" />
        <el-switch v-model="config.timeout.disabled" active-text="禁用超时" />
      </div>
      <div>并发限制适用于当前服务实例；降低限制不终止已运行任务。统一超时同时用于其他操作，新建任务采用新策略。</div>
      <div class="lr-cfg-actions">
        <el-button
          type="primary"
          size="small"
          :loading="saving"
          @click="handleSave"
          >保存配置</el-button
        >
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { ElMessage } from "element-plus";
import { useCoreConfigurationAccess } from "@/composables/useCoreConfigurationAccess";
import { fetchConfigApi, saveConfigApi, type TaskMaintenanceConfig } from "../api";

const access = useCoreConfigurationAccess();
let loadedContext = "";
watch(access.contextKey, (value) => { if (loadedContext && value !== loadedContext) { config.value = null; error.value = "Core 或权限已变化，请重新加载配置"; } });
const config = ref<TaskMaintenanceConfig | null>(null);
const saving = ref(false);
const error = ref("");

async function loadConfig() {
  error.value = "";
  config.value = null;
  try {
    const context = await access.requireAccess();
    const data = await fetchConfigApi(context);
    await access.requireAccess(context);
    loadedContext = context;
    if (data) {
      config.value = data;
    }
  } catch (reason) { error.value = reason instanceof Error ? reason.message : "加载配置失败"; }
}

async function handleSave() {
  saving.value = true;
  try {
    if (!config.value) return;
    const context = await access.requireAccess(loadedContext);
    const saved = await saveConfigApi(config.value, context);
    await access.requireAccess(context);
    config.value = saved;
    ElMessage.success("执行器已应用配置");
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : "保存失败，请重新加载配置");
    await loadConfig();
  } finally {
    saving.value = false;
  }
}

onMounted(loadConfig);
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
