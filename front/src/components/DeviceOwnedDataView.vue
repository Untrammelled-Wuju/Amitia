<template>
  <div v-if="loading" v-loading="true" class="data-loading" />
  <el-alert v-else-if="error" :title="error" type="error" :closable="false"><el-button @click="initialize">重试</el-button></el-alert>
  <section v-else-if="owned.enabled.value" class="owned-data-page">
    <h2>{{ title }}</h2>
    <el-select v-model="role" placeholder="请选择数据所属角色" aria-label="数据所属角色">
      <el-option v-for="item in owned.roles.value" :key="item.id" :label="item.name" :value="item.id" />
    </el-select>
    <DeviceOwnedMemoryPanel v-if="role" :key="`${owned.coreId.value}/${role}`" embedded :visible="true" :character-id="role" conversation-id="" :initial-kind="initialKind" />
    <el-empty v-else description="选择角色后查看其数据；未配置角色时不能调用服务" />
  </section>
  <slot v-else />
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import DeviceOwnedMemoryPanel from "./DeviceOwnedMemoryPanel.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

defineProps<{ title: string; initialKind: string }>();
const owned = useDeviceOwnedConversation();
const loading = ref(true);
const error = ref("");
const role = ref("");

async function initialize() {
  loading.value = true;
  error.value = "";
  try {
    const enabled = await owned.refresh();
    const deployment = await getDeploymentConfig();
    if (!enabled && deployment.mode === "cloud") throw new Error("当前 Core 的设备数据服务尚未就绪，请恢复服务后重试");
    if (enabled) role.value = owned.selectInitialRole(role.value);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法读取数据归属";
  } finally { loading.value = false; }
}

onMounted(initialize);
</script>

<style scoped>
.data-loading { min-height: 240px; }
.owned-data-page { padding: 20px; }
</style>
