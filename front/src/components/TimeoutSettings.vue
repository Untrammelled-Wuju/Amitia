<template>
  <el-card shadow="never" class="section-card" data-amitia-ui="component.card" v-loading="loading">
    <template #header>超时控制</template>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <details v-if="errorDetail"><summary>查看原始错误</summary><pre>{{ errorDetail }}</pre></details>
    <el-form label-position="top">
      <el-form-item label="停用超时">
        <span class="timeout-control" :tabindex="controlReason ? 0 : undefined" :role="controlReason ? 'button' : undefined" :aria-label="controlReason ? `停用超时：${controlReason}` : undefined" @click.capture="explainUnavailable($event, controlReason)" @keydown.enter="explainUnavailable($event, controlReason)" @keydown.space="explainUnavailable($event, controlReason)"><el-switch v-model="settings.disabled" data-amitia-ui="component.switch" :disabled="!!controlReason" /></span>
        <span class="timeout-hint">开启后，功能调用不再因执行时间过长而中止</span>
      </el-form-item>
      <el-form-item :label="`超时时间：${formatTimeout(settings.seconds)}`">
        <div class="timeout-slider" :tabindex="sliderReason ? 0 : undefined" :role="sliderReason ? 'button' : undefined" :aria-label="sliderReason ? `超时时间：${sliderReason}` : undefined" @click.capture="explainUnavailable($event, sliderReason)" @keydown.enter="explainUnavailable($event, sliderReason)" @keydown.space="explainUnavailable($event, sliderReason)"><el-slider v-model="settings.seconds" data-amitia-ui="component.slider" :min="30" :max="1800" :step="30"
          :format-tooltip="formatTimeout" :disabled="!!sliderReason" /></div>
      </el-form-item>
      <p class="timeout-hint">统一控制模型、工具与媒体处理等功能的执行时限。保存后对新调用生效；仍可主动取消。</p>
      <el-button v-if="error" data-amitia-ui="component.button" @click="load">重新加载</el-button>
      <span v-else class="timeout-control" :tabindex="controlReason ? 0 : undefined" :role="controlReason ? 'button' : undefined" :aria-label="controlReason ? `保存：${controlReason}` : undefined" @click.capture="explainUnavailable($event, controlReason)" @keydown.enter="explainUnavailable($event, controlReason)" @keydown.space="explainUnavailable($event, controlReason)"><el-button data-amitia-ui="component.button" type="primary" :loading="saving" :disabled="!!controlReason" @click="save">保存</el-button></span>
    </el-form>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { setOperationTimeout, formatTimeout } from "@/runtime/operation-timeout";
import { originalErrorText } from "@/runtime/original-error";

const settings = reactive({ disabled: false, seconds: 180 });
const loading = ref(true);
const saving = ref(false);
const error = ref("");
const errorDetail = ref("");
const controlReason = computed(() => loading.value ? "正在加载超时设置，请稍后重试。" : saving.value ? "正在保存，请稍后重试。" : error.value ? "无法读取超时设置，请先重新加载并查看原始错误。" : "");
const sliderReason = computed(() => controlReason.value || (settings.disabled ? "请先关闭“停用超时”。" : ""));
function explainUnavailable(event: Event, reason: string) {
  if (!reason) return;
  event.preventDefault();
  event.stopPropagation();
  ElMessage.warning(reason);
}
async function load() {
  loading.value = true;
  error.value = "";
  errorDetail.value = "";
  try {
    const response = await apiClient.get("/api/runtime/timeout/config");
    Object.assign(settings, response.data);
    setOperationTimeout(response.config.baseURL || "", settings);
  } catch (caughtError) {
    error.value = "无法加载超时设置，请重试";
    errorDetail.value = originalErrorText(caughtError);
  }
  finally { loading.value = false; }
}
async function save() {
  if (controlReason.value) { ElMessage.warning(controlReason.value); return; }
  saving.value = true;
  errorDetail.value = "";
  try {
    const response = await apiClient.put("/api/runtime/timeout/config", { ...settings });
    Object.assign(settings, response.data);
    setOperationTimeout(response.config.baseURL || "", settings);
    ElMessage.success("超时设置已保存，对新调用生效");
  } catch (caughtError) {
    errorDetail.value = originalErrorText(caughtError);
    ElMessage.error("保存超时设置失败，请查看原始错误。");
  } finally { saving.value = false; }
}
onMounted(load);
</script>

<style scoped>
.section-card { margin-bottom: 12px; border: 1px solid var(--ac-color-border-light); }
.timeout-hint { color: var(--ac-color-text-secondary); font-size: 13px; line-height: 1.6; }
.timeout-control { display: inline-flex; }
.timeout-control + .timeout-hint { margin-left: 12px; }
.timeout-slider { width: 100%; }
.timeout-control[role="button"] :deep(.el-switch),
.timeout-control[role="button"] :deep(.el-button),
.timeout-slider[role="button"] :deep(.el-slider) { pointer-events: none; }
.el-slider { margin: 0 12px; }
pre { white-space: pre-wrap; overflow-wrap: anywhere; user-select: text; }
</style>
