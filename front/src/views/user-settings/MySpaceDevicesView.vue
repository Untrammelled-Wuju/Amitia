<template>
  <div class="my-space-devices">
    <div v-if="loading" class="device-page-loading"><el-skeleton :rows="5" animated /></div>
    <template v-else>
      <DevicePairingPanel :mode="mode" :serverURL="serverURL" @paired="handlePaired" />
      <BoundDevicesPanel :key="generation" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import BoundDevicesPanel from "@/components/BoundDevicesPanel.vue";
import DevicePairingPanel from "@/components/DevicePairingPanel.vue";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

const mode = ref<"local" | "cloud">("local");
const serverURL = ref("");
const loading = ref(true);
const generation = ref(0);

async function load() {
  loading.value = true;
  try {
    const config = await getDeploymentConfig();
    mode.value = config.mode;
    serverURL.value = config.mode === "cloud" ? config.serverURL || "" : "";
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : String(error));
  } finally {
    loading.value = false;
  }
}

function handlePaired(baseURL: string) {
  mode.value = "cloud";
  serverURL.value = baseURL;
  generation.value++;
}

onMounted(load);
</script>

<style scoped>
.my-space-devices { width: min(100%, 1040px); min-width: 0; display: grid; gap: 18px; }
.device-page-loading { padding: 24px; border-radius: 18px; background: var(--surface-bg-elevated); }
.my-space-devices :deep(.device-pairing-panel) { margin: 0; padding: 20px 22px; border-radius: 18px; background: var(--surface-bg-elevated); }
.my-space-devices :deep(.bound-device-management) { margin-top: 0; padding: 20px 22px; border-radius: 18px; background: var(--surface-bg-elevated); }
</style>
