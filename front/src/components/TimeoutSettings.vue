<template>
  <el-card shadow="never" class="section-card" v-loading="loading">
    <template #header>超时控制</template>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form label-position="top">
      <el-form-item label="停用超时">
        <el-switch v-model="settings.disabled" :disabled="loading || saving || !!error" />
        <span class="timeout-hint">开启后，功能调用不再因执行时间过长而中止</span>
      </el-form-item>
      <el-form-item :label="`超时时间：${formatTimeout(settings.seconds)}`">
        <el-slider v-model="settings.seconds" :min="30" :max="1800" :step="30"
          :format-tooltip="formatTimeout" :disabled="settings.disabled || loading || saving || !!error" />
      </el-form-item>
      <p class="timeout-hint">统一控制模型、工具与媒体处理等功能的执行时限。保存后对新调用生效；仍可主动取消。</p>
      <el-button v-if="error" @click="load">重新加载</el-button>
      <el-button v-else type="primary" :loading="saving" :disabled="loading" @click="save">保存</el-button>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { setOperationTimeout, formatTimeout } from "@/runtime/operation-timeout";

const settings = reactive({ disabled: false, seconds: 180 });
const loading = ref(true);
const saving = ref(false);
const error = ref("");
async function load() {
  loading.value = true;
  error.value = "";
  try {
    const response = await apiClient.get("/api/runtime/timeout/config");
    Object.assign(settings, response.data);
    setOperationTimeout(response.config.baseURL || "", settings);
  } catch { error.value = "无法加载超时设置，请重试"; }
  finally { loading.value = false; }
}
async function save() {
  saving.value = true;
  try {
    const response = await apiClient.put("/api/runtime/timeout/config", { ...settings });
    Object.assign(settings, response.data);
    setOperationTimeout(response.config.baseURL || "", settings);
    ElMessage.success("超时设置已保存，对新调用生效");
  } finally { saving.value = false; }
}
onMounted(load);
</script>

<style scoped>
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.timeout-hint { color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; }
.el-switch + .timeout-hint { margin-left: 12px; }
.el-slider { margin: 0 12px; }
</style>
