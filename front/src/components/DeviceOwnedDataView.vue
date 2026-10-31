<template>
  <section class="owned-data-page" :data-amitia-ui="semanticTarget">
    <ExtensionPageHeader
      v-if="parentTitle && parentPath"
      :title="title"
      :description="description"
      :parent-title="parentTitle"
      :parent-path="parentPath"
      breadcrumb-label="数据页面层级"
    />
    <header v-else class="data-page-header">
      <h2>{{ title }}</h2>
    </header>
    <div v-if="loading" v-loading="true" class="data-loading" />
    <el-alert v-else-if="error" :title="error" type="error" :closable="false">
      <el-button @click="initialize">重试</el-button>
    </el-alert>
    <template v-else-if="owned.enabled.value">
      <div class="data-toolbar">
        <div class="toolbar-field">
            <label :for="`${roleSelectId}-source`">数据来源</label>
            <el-select :id="`${roleSelectId}-source`" v-model="dataSource" filterable aria-label="记忆数据来源" class="role-select">
              <el-option label="当前 Core 的数据" value="current" />
              <el-option v-for="item in historicalRoles" :key="item.id" :label="`原设备历史 · ${item.name}`" :value="`history:${item.id}`" />
            </el-select>
        </div>
        <div class="toolbar-field">
            <label :for="roleSelectId">查看范围</label>
            <el-select :id="roleSelectId" v-model="viewSelection" filterable aria-label="记忆查看范围" class="role-select">
              <el-option v-for="option in viewOptions" :key="option.value" :label="option.label" :value="option.value" />
            </el-select>
        </div>
        <div class="toolbar-field search-field">
          <label :for="`${roleSelectId}-search`">搜索</label>
          <el-input :id="`${roleSelectId}-search`" v-model="searchKeyword" clearable aria-label="搜索记忆" placeholder="搜索标题或内容" />
        </div>
      </div>
      <DeviceOwnedMemoryPanel v-if="role" :key="`${owned.coreId.value}/${role}/${mode || initialKind}`" embedded fixed-kind :source="dataSource" :requested-memory-view="viewSelection" :search="searchKeyword" :visible="true" :character-id="role" conversation-id="" :initial-kind="initialKind" :mode="mode" :kinds="kinds" />
      <el-empty v-else description="选择角色后查看其数据；未配置角色时不能调用服务" />
    </template>
    <el-alert v-else title="当前 Core 的设备数据服务尚未就绪，请恢复服务后重试" type="warning" :closable="false" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, useId } from "vue";
import DeviceOwnedMemoryPanel from "./DeviceOwnedMemoryPanel.vue";
import ExtensionPageHeader from "@/views/extensions/components/ExtensionPageHeader.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { getDeploymentConfig } from "@/runtime/runtime-adapter";
import { ownedMemoryViewOptions } from "@/runtime/owned-memory-view-options";

const props = defineProps<{
  title: string;
  initialKind: string;
  description?: string;
  semanticTarget?: string;
  mode?: string;
  kinds?: string[];
  parentTitle?: string;
  parentPath?: string;
}>();
const owned = useDeviceOwnedConversation();
const loading = ref(true);
const error = ref("");
const role = ref("");
const viewSelection = ref("all");
const searchKeyword = ref("");
const dataSource = ref("current");
const historicalRoles = ref<Array<{ id: string; name: string }>>([]);
const viewOptions = computed(() => ownedMemoryViewOptions(dataSource.value, owned.roles.value, historicalRoles.value, props.initialKind));
watch(viewOptions, options => {
  if (!options.some(option => option.value === viewSelection.value)) viewSelection.value = options[0].value;
}, { flush: "sync" });
const roleSelectId = useId();
let generation = 0;
let historicalGeneration = 0;
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
  const selected = dataSource.value === "current" && viewSelection.value.startsWith("role:") ? viewSelection.value.slice(5) : "";
  role.value = owned.roles.value.some(item => item.id === selected) ? selected : owned.selectInitialRole(role.value) || owned.roles.value[0]?.id || "";
}
async function loadHistoricalRoles() {
  const ticket = ++historicalGeneration;
  const selectedRole = role.value;
  const selectedCore = owned.coreId.value;
  if (!role.value) {
    historicalRoles.value = [];
    return;
  }
  try {
    const roles = await owned.historicalRoles(selectedRole);
    if (ticket === historicalGeneration && selectedRole === role.value && selectedCore === owned.coreId.value) historicalRoles.value = roles;
  } catch (error) {
  console.error("[original-error]", error);
if (ticket === historicalGeneration) historicalRoles.value = [];
  }
}
watch([owned.roles, viewSelection], () => { if (owned.enabled.value) selectAuthorityRole(); });
watch(role, () => { if (owned.enabled.value) void loadHistoricalRoles(); });
watch(owned.coreId, () => {
  historicalGeneration++;
  historicalRoles.value = [];
  dataSource.value = "current";
  if (owned.enabled.value) void loadHistoricalRoles();
}, { flush: "sync" });
onMounted(() => {
  void initialize();
  window.addEventListener("amitia:runtime-connection-changed", initialize);
  providerTimer = setInterval(() => { void owned.refresh().catch((error) => {
  console.error("[original-error]", error);
loading.value = false; error.value = "当前 Core 的数据服务暂不可用，请恢复服务后重试"; }); }, 3000);
});
onUnmounted(() => {
  generation++;
  historicalGeneration++;
  if (providerTimer) clearInterval(providerTimer);
  window.removeEventListener("amitia:runtime-connection-changed", initialize);
});
</script>

<style scoped>
.data-loading { min-height: 240px; }
.owned-data-page { padding: 0; min-width: 0; }
.data-page-header { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; margin-bottom: 20px; }
.data-page-header h2 { margin: 0; color: var(--ac-color-text-primary); font-size: var(--ac-font-size-xl); font-weight: 600; line-height: 1.5; }
.data-toolbar { display: flex; align-items: flex-end; flex-wrap: wrap; gap: 16px; padding: 16px; margin: 20px 0; border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); background: var(--ac-color-surface); }
.toolbar-field { display: flex; flex: 0 1 240px; flex-direction: column; gap: 8px; min-width: 0; }
.toolbar-field label { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.5; }
.search-field { flex: 1 1 280px; }
.role-select { width: 100%; min-width: 0; }
@media (max-width: 640px) {
  .owned-data-page { padding: 0; }
  .data-page-header { align-items: stretch; flex-direction: column; }
  .data-toolbar { align-items: stretch; flex-direction: column; gap: 12px; }
  .toolbar-field { flex: auto; }
  .role-select { width: 100%; }
  .role-select :deep(.el-select__wrapper) { min-height: 44px; }
}
</style>
