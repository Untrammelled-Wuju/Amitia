<template>
  <div v-if="loading" v-loading="true" class="data-loading" :data-amitia-ui="semanticTarget" />
  <el-alert v-else-if="error" :title="error" type="error" :closable="false" :data-amitia-ui="semanticTarget"><el-button @click="initialize">重试</el-button></el-alert>
  <section v-else-if="owned.enabled.value" class="owned-data-page" :data-amitia-ui="semanticTarget">
    <header class="data-page-header">
      <h2>{{ title }}</h2>
      <div v-if="initialKind !== 'profile'" class="role-field">
        <label :for="roleSelectId">查看范围</label>
        <el-select :id="roleSelectId" v-model="viewSelection" filterable aria-label="记忆查看范围" class="role-select">
          <el-option label="全部记忆" value="all" />
          <el-option label="全局记忆" value="global" />
          <el-option v-for="item in owned.roles.value" :key="item.id" :label="item.name" :value="`role:${item.id}`" />
        </el-select>
      </div>
    </header>
    <DeviceOwnedMemoryPanel v-if="role" :key="`${owned.coreId.value}/${role}`" embedded fixed-kind :requested-memory-view="initialKind === 'profile' ? 'global' : viewSelection" :visible="true" :character-id="role" conversation-id="" :initial-kind="initialKind" />
    <el-empty v-else description="选择角色后查看其数据；未配置角色时不能调用服务" />
  </section>
  <slot v-else />
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, useId } from "vue";
import DeviceOwnedMemoryPanel from "./DeviceOwnedMemoryPanel.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";

const props = defineProps<{ title: string; initialKind: string; semanticTarget?: string }>();
const owned = useDeviceOwnedConversation();
const loading = ref(true);
const error = ref("");
const role = ref("");
const viewSelection = ref("all");
const roleSelectId = useId();
let generation = 0;
let providerTimer: ReturnType<typeof setInterval> | undefined;

async function initialize() {
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  try {
    const enabled = await owned.refresh();
    const deployment = await getDeploymentConfig();
    if (ticket !== generation) return;
    if (!enabled && deployment.mode === "cloud") throw new Error("当前 Core 的设备数据服务尚未就绪，请恢复服务后重试");
    if (enabled) selectAuthorityRole();
  } catch (cause) {
    if (ticket === generation) error.value = cause instanceof Error ? cause.message : "无法读取数据归属";
  } finally { if (ticket === generation) loading.value = false; }
}

function selectAuthorityRole() {
  const selected = viewSelection.value.startsWith("role:") ? viewSelection.value.slice(5) : "";
  role.value = owned.roles.value.some(item => item.id === selected) ? selected : owned.selectInitialRole(role.value) || owned.roles.value[0]?.id || "";
}
watch([owned.roles, viewSelection], () => { if (owned.enabled.value) selectAuthorityRole(); });
onMounted(() => {
  void initialize();
  window.addEventListener("amitia:runtime-connection-changed", initialize);
  providerTimer = setInterval(() => { void owned.refresh().catch(() => { loading.value = false; error.value = "当前 Core 的数据服务暂不可用，请恢复服务后重试"; }); }, 3000);
});
onUnmounted(() => {
  generation++;
  if (providerTimer) clearInterval(providerTimer);
  window.removeEventListener("amitia:runtime-connection-changed", initialize);
});
</script>

<style scoped>
.data-loading { min-height: 240px; }
.owned-data-page { padding: 20px; min-width: 0; }
.data-page-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; margin-bottom: 20px; }
.data-page-header h2 { margin: 0; color: var(--ac-color-text-primary); font-size: var(--ac-font-size-xl); font-weight: 600; line-height: 1.5; }
.role-field { display: flex; align-items: center; gap: 12px; min-width: 0; max-width: 100%; }
.role-field label { flex-shrink: 0; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.role-select { width: 240px; max-width: 100%; min-width: 0; }
@media (max-width: 640px) {
  .owned-data-page { padding: 16px; }
  .data-page-header { align-items: stretch; flex-direction: column; }
  .role-field { align-items: stretch; flex-direction: column; gap: 8px; }
  .role-select { width: 100%; }
  .role-select :deep(.el-select__wrapper) { min-height: 44px; }
}
</style>
