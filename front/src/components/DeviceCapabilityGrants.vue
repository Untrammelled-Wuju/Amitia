<template>
  <el-dialog :model-value="visible" :title="`${label || deviceId} · 能力授权`" width="min(640px, 95vw)" @update:model-value="$emit('update:visible', $event)">
    <el-alert title="仅授予明确的调用设备和能力。关闭授权后，正在进行的相关调用会中断。" type="info" :closable="false" />
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <el-form label-position="top">
      <el-form-item label="调用设备"><el-select v-model="caller" placeholder="选择调用设备"><el-option v-for="device in devices.filter(item => item.deviceId !== deviceId && item.trustState === 'trusted')" :key="device.deviceId" :label="device.label || device.deviceId" :value="device.deviceId" /></el-select></el-form-item>
      <el-form-item label="能力标识"><el-select v-model="capability" filterable allow-create default-first-option placeholder="选择能力或输入其他能力标识"><el-option label="AI 对话" value="ai.chat" /><el-option label="设备任务" value="task.execute" /></el-select></el-form-item>
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
import { ref, watch } from "vue";
import { useApi } from "@/composables/useApi";

type Grant = { callerId: string; targetId: string; capability: string; allowed: boolean; revision: number };
const props = defineProps<{ visible: boolean; deviceId: string; label?: string; devices: Array<{ deviceId: string; label: string; trustState: string }> }>();
const emit = defineEmits<{ "update:visible": [value: boolean]; changed: [] }>();
const api = useApi();
const caller = ref("");
const capability = ref("ai.chat");
const grants = ref<Grant[]>([]);
const loading = ref(false);
const busy = ref(false);
const error = ref("");
const coreId = ref("");
let generation = 0;

async function load() {
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  try {
    const before = await api.get<{ coreId: string }>("/api/device-mesh/v1/coordination/me");
    const result = await api.get<{ grants: Grant[] }>(`/api/device-mesh/v1/business/devices/${encodeURIComponent(props.deviceId)}/grants`);
    const after = await api.get<{ coreId: string }>("/api/device-mesh/v1/coordination/me");
    if (!before.coreId || before.coreId !== after.coreId) throw new Error("服务提供者已变化，请刷新后重新授权");
    if (ticket === generation) { grants.value = result.grants || []; coreId.value = before.coreId; }
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "授权加载失败"; }
  finally { if (ticket === generation) loading.value = false; }
}

async function save(allowed: boolean) {
  if (busy.value) return;
  const ticket = generation;
  const target = props.deviceId;
  const name = capability.value.trim();
  const previous = grants.value.find(item => item.callerId === caller.value && item.capability === name);
  busy.value = true;
  error.value = "";
  try {
    await api.put(`/api/device-mesh/v1/business/devices/${encodeURIComponent(target)}/grants`, { callerId: caller.value, capability: name, allowed, expectedRevision: previous?.revision || 0, expectedCoreId: coreId.value });
    if (ticket === generation) { emit("changed"); await load(); }
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "授权更新失败，请刷新后重试"; }
  finally { busy.value = false; }
}

async function toggle(grant: Grant) {
  caller.value = grant.callerId;
  capability.value = grant.capability;
  await save(!grant.allowed);
}

watch(() => [props.visible, props.deviceId], () => {
  generation++;
  grants.value = [];
  caller.value = "";
  capability.value = "ai.chat";
  if (props.visible && props.deviceId) void load();
}, { immediate: true });
</script>

<style scoped>
.grant-row { display: flex; justify-content: space-between; gap: 12px; align-items: center; padding: 12px 0; overflow-wrap: anywhere; }
</style>
