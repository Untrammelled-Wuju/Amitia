<template>
  <div v-if="loading" v-loading="true" class="memory-loading" />
  <el-alert v-else-if="error" :title="error" type="error" :closable="false"><el-button @click="initialize">重试</el-button></el-alert>
  <section v-else-if="owned.enabled.value" class="owned-memory-page">
    <h2>记忆管理</h2>
    <el-select v-model="role" placeholder="请选择记忆所属角色" aria-label="记忆所属角色">
      <el-option v-for="item in owned.roles.value" :key="item.id" :label="item.name" :value="item.id" />
    </el-select>
    <DeviceOwnedMemoryPanel v-if="role" embedded :visible="true" :character-id="role" conversation-id="" />
    <el-empty v-else description="选择角色后查看其记忆；未配置角色时不能调用服务" />
  </section>
  <LegacyMemoryManagerView v-else />
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import LegacyMemoryManagerView from "./LegacyMemoryManagerView.vue";
import DeviceOwnedMemoryPanel from "@/components/DeviceOwnedMemoryPanel.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

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
    error.value = cause instanceof Error ? cause.message : "无法读取记忆的数据归属";
  } finally { loading.value = false; }
}

onMounted(initialize);
</script>

<style scoped>
.memory-loading { min-height: 240px; }
.owned-memory-page { padding: 20px; }
</style>
