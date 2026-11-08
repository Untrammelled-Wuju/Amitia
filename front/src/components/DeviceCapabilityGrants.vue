<template>
  <el-dialog :model-value="visible" :title="`${label || deviceId} · 能力授权`" width="min(640px, 95vw)" @update:model-value="$emit('update:visible', $event)">
    <el-alert title="仅授予明确的调用设备和能力。关闭授权后，正在进行的相关调用会中断。" type="info" :closable="false" />
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form label-position="top">
      <el-form-item label="调用设备"><el-select v-model="caller" placeholder="选择调用设备"><el-option v-for="device in devices.filter(item => item.deviceId !== deviceId && item.trustState === 'trusted')" :key="device.deviceId" :label="device.label || device.deviceId" :value="device.deviceId" /></el-select></el-form-item>
      <el-form-item label="能力标识"><el-select v-model="capability" filterable placeholder="目标设备当前开放的能力"><el-option v-for="id in availableCapabilities" :key="id" :label="capabilityLabel(id)" :value="id" /></el-select></el-form-item>
      <el-button type="primary" :disabled="!caller || !capability.trim()" :loading="busy" @click="save(true)">允许调用</el-button>
    </el-form>
    <div v-loading="loading">
      <article v-for="grant in grants" :key="`${grant.callerId}/${grant.capability}`" class="grant-row">
        <span>{{ devices.find(item => item.deviceId === grant.callerId)?.label || grant.callerId }} · {{ grant.capability }}</span>
        <el-button :loading="busy" @click="toggle(grant)">{{ grant.allowed ? '撤销' : '重新授予' }}</el-button>
      </article>
      <el-empty v-if="!loading && !grants.length" description="暂无跨设备能力授权" />
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { useApi } from "@/composables/useApi";
import { useDeviceManagementIntent, deviceManagementRequestConfig, type DeviceManagementIntent } from "@/composables/useDeviceManagementIntent";

type Grant = { callerId: string; targetId: string; capability: string; allowed: boolean; revision: number };
type CapabilityCatalog = { capabilities: string[]; revision: number };
const props = defineProps<{ visible: boolean; deviceId: string; label?: string; devices: Array<{ deviceId: string; label: string; trustState: string }> }>();
const emit = defineEmits<{ "update:visible": [value: boolean]; changed: [] }>();
const api = useApi();
const caller = ref("");
const capability = ref("ai.chat");
const grants = ref<Grant[]>([]);
const availableCapabilities = ref<string[]>([]);
const loading = ref(false);
const busy = ref(false);
const error = ref("");
const coreId = ref("");
let generation = 0;
let intent: DeviceManagementIntent | null = null;
let catalogReloadTimer: ReturnType<typeof setTimeout> | undefined;
const labels: Record<string, string> = {
  "ai.chat": "文本与对话",
  "ai.vision": "识图",
  "ai.speech": "语音",
  "ai.image": "图像生成",
  "task.execute": "主动任务",
  "mcp.invoke": "MCP",
  "skill.invoke": "Skill",
  "workflow.execute": "工作流",
  "extension.invoke": "扩展能力",
  "browser.invoke": "浏览器",
  "desktop.pet": "桌宠",
  "game.execute": "游戏模式",
};
const capabilityLabel = (id: string) => labels[id] ? `${labels[id]} · ${id}` : id;
const management = useDeviceManagementIntent(() => { generation++; intent = null; grants.value = []; error.value = "原Core或设备权限已变化，请重新打开授权页面"; });

async function load() {
  management.release(intent);
  intent = null;
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  try {
    const original = await management.capture();
    if (!original.state.canAdminister) throw new Error("只有统筹中心管理员可以管理设备调用授权");
    const [result, catalog] = await Promise.all([
      api.get<{ grants: Grant[] }>(`/api/device-mesh/v1/business/devices/${encodeURIComponent(props.deviceId)}/grants`, undefined, deviceManagementRequestConfig(original)),
      api.get<CapabilityCatalog>(`/api/device-mesh/v1/business/devices/${encodeURIComponent(props.deviceId)}/capabilities`, undefined, deviceManagementRequestConfig(original)),
    ]);
    await management.validate(original);
    if (ticket === generation) {
      grants.value = result.grants || [];
      availableCapabilities.value = Array.isArray(catalog.capabilities) ? catalog.capabilities : [];
      if (!availableCapabilities.value.includes(capability.value)) capability.value = availableCapabilities.value[0] || "";
      coreId.value = original.state.coreId;
      intent = original;
    }
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "授权加载失败"; }
  finally { if (ticket === generation) loading.value = false; }
}

async function save(allowed: boolean) {
  if (busy.value) return;
  const ticket = generation;
  const original = intent;
  if (!original) { error.value = "请先重新加载原设备授权"; return; }
  const target = props.deviceId;
  const name = capability.value.trim();
  const previous = grants.value.find(item => item.callerId === caller.value && item.capability === name);
  busy.value = true;
  error.value = "";
  try {
    await management.validate(original);
    await api.put(`/api/device-mesh/v1/business/devices/${encodeURIComponent(target)}/grants`, { callerId: caller.value, capability: name, allowed, expectedRevision: previous?.revision || 0, expectedCoreId: original.state.coreId }, deviceManagementRequestConfig(original));
    await management.validate(original, target === original.state.policy.deviceId ? 1 : 0);
    if (ticket === generation) { await load(); emit("changed"); }
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "授权更新失败，请刷新后重试"; }
  finally { busy.value = false; }
}

async function toggle(grant: Grant) {
  caller.value = grant.callerId;
  capability.value = grant.capability;
  await save(!grant.allowed);
}

watch(() => [props.visible, props.deviceId], () => {
  management.release(intent);
  intent = null;
  generation++;
  grants.value = [];
  availableCapabilities.value = [];
  caller.value = "";
  capability.value = "ai.chat";
  if (props.visible && props.deviceId) void load();
}, { immediate: true });

function onCatalogChanged(event: Event) {
  const detail = (event as CustomEvent<{ deviceId?: string }>).detail;
  if (!props.visible || (detail?.deviceId && detail.deviceId !== props.deviceId)) return;
  if (catalogReloadTimer) clearTimeout(catalogReloadTimer);
  catalogReloadTimer = setTimeout(() => {
    catalogReloadTimer = undefined;
    if (props.visible) void load();
  }, 250);
}
onMounted(() => window.addEventListener("amitia:device-capabilities-changed", onCatalogChanged));
onUnmounted(() => {
  window.removeEventListener("amitia:device-capabilities-changed", onCatalogChanged);
  if (catalogReloadTimer) clearTimeout(catalogReloadTimer);
});
</script>

<style scoped>
.grant-row { display: flex; justify-content: space-between; gap: 12px; align-items: center; padding: 12px 0; overflow-wrap: anywhere; }
</style>
