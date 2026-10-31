<template>
  <div class="space-identity-page" v-loading="loading">
    <section class="identity-card">
      <div class="identity-card-heading">
        <h2>身份信息</h2>
      </div>
      <div v-for="field in identityFields" :key="field.key" class="identity-field">
        <h3>{{ field.label }}</h3>
        <div class="identity-value-row">
          <code>{{ identity[field.key] || "未读取" }}</code>
          <el-button size="small" :disabled="!identity[field.key]" @click="copyIdentity(identity[field.key])">复制</el-button>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "@/composables/useApi";
import { getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";
import { originalErrorMessage } from "@/runtime/original-error";

const loading = ref(false);
const identity = reactive({ spaceId: "", instanceId: "", deviceId: "", runtimeId: "" });
const identityFields = [
  { key: "spaceId", label: "Space ID" },
  { key: "instanceId", label: "Instance ID" },
  { key: "deviceId", label: "Device ID" },
  { key: "runtimeId", label: "Runtime ID" },
] as const;

async function loadIdentity() {
  loading.value = true;
  try {
    const results = await Promise.allSettled([
      apiClient.get("/api/public/core/info").then((res) => {
        const info = res.data?.data || res.data || {};
        identity.spaceId = String(info.spaceId || "");
        identity.instanceId = String(info.instanceId || info.cloudId || "");
        if (!identity.spaceId) throw new Error("Space ID 暂不可用");
      }),
      (async () => {
        const device = window.amitiaDesktop
          ? await window.amitiaDesktop.getMeshIdentity()
          : getWebDeviceMeshIdentity();
        if (!device?.deviceId || !device.runtimeId) throw new Error("当前设备身份暂不可用");
        identity.deviceId = device.deviceId;
        identity.runtimeId = device.runtimeId;
      })(),
    ]);
    const failed = results.filter((result) => result.status === "rejected");
    if (failed.length) ElMessage.error(failed.map((result) => originalErrorMessage((result as PromiseRejectedResult).reason)).join("\n"));
  } finally {
    loading.value = false;
  }
}

async function copyIdentity(value: string) {
  if (!value) return;
  try {
    if (window.amitiaDesktop?.writeClipboardText) {
      await window.amitiaDesktop.writeClipboardText(value);
    } else {
      await navigator.clipboard.writeText(value);
    }
    ElMessage.success("已复制");
  } catch (error) {
  console.error("[original-error]", error);
ElMessage.error(error instanceof Error ? error.message : String(error));
  }
}

onMounted(loadIdentity);
</script>

<style scoped>
.space-identity-page { width: min(100%, 1040px); min-width: 0; }
.identity-card { min-width: 0; padding: 24px; border-radius: 20px; background: var(--surface-bg-elevated); }
.identity-card-heading { margin-bottom: 24px; }
.identity-card-heading h2 { margin: 0; font-size: 18px; line-height: 1.4; }
.identity-field { min-width: 0; }
.identity-field + .identity-field { margin-top: 22px; }
.identity-field h3 { margin: 0 0 10px; color: var(--text-secondary); font-size: 13px; font-weight: 500; }
.identity-value-row { display: flex; align-items: center; gap: 10px; }
.identity-value-row code { min-width: 0; flex: 1; overflow-wrap: anywhere; padding: 13px 14px; border-radius: 11px; background: var(--workbench-bg); color: var(--text-primary); font-size: 13px; }
.identity-value-row .el-button { flex-shrink: 0; }
@media (max-width: 760px) { .identity-card { padding: 20px; } .identity-value-row { align-items: flex-start; } }
</style>
