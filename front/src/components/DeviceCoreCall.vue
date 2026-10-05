<template>
  <el-dialog :model-value="visible" :title="`通过 Core 调用 · ${label || deviceId}`" width="min(680px, 95vw)" @update:model-value="close">
    <p>服务提供者：{{ coreId || '加载中' }} · 角色与新数据所有者：{{ ownerId || '加载中' }}</p>
    <el-alert title="AI 计算由 Core 负责，数据按目标设备的统筹状态保存；调用还需目标设备的能力授权。" type="info" :closable="false" />
    <el-alert v-if="error" :title="error" type="warning" :closable="false" />
    <el-select v-model="roleId" :disabled="running || loading" placeholder="选择本次调用的角色" aria-label="调用角色"><el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" /></el-select>
    <el-input v-model="message" type="textarea" :rows="4" :disabled="running" placeholder="输入任务或问题" :maxlength="32000" />
    <p v-if="reply" class="reply">{{ reply }}</p>
    <p v-if="saved">已由 {{ savedOwner }} 确认保存</p>
    <el-button :loading="loading" :disabled="running" @click="initialize">刷新角色与服务</el-button>
    <el-button v-if="running" @click="interrupt">中断调用</el-button>
    <el-button v-else type="primary" :disabled="loading || !roleId || !message.trim()" @click="send">发起调用</el-button>
  </el-dialog>
</template>

<script setup lang="ts">
import { onUnmounted, ref, watch } from "vue";
import { useApi } from "@/composables/useApi";
import { streamOwnedChat, type OwnedExecutionScope } from "@/runtime/device-owned-chat";

const props = defineProps<{ visible: boolean; deviceId: string; label?: string }>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
const api = useApi();
const coreId = ref("");
const ownerId = ref("");
const roles = ref<Array<{ id: string; name: string; revision: number }>>([]);
const expectedScope = ref<OwnedExecutionScope | null>(null);
const roleId = ref("");
const message = ref("");
const reply = ref("");
const error = ref("");
const saved = ref(false);
const savedOwner = ref("");
const loading = ref(false);
const running = ref(false);
let generation = 0;
let controller: AbortController | null = null;
let requestId = "";

async function initialize() {
  const ticket = ++generation;
  loading.value = true;
  error.value = "";
  roles.value = [];
  roleId.value = "";
  expectedScope.value = null;
  try {
    const state = await api.get<{ coreId: string; coordinationAvailable: boolean }>("/api/device-mesh/v1/coordination/me");
    if (!state.coordinationAvailable) throw new Error("Core 的设备业务服务尚未就绪");
    const result = await api.get<{ roles: Array<{ id: string; name: string; revision: number }>; roleOwnerId: string; executionScope: OwnedExecutionScope }>("/api/device-mesh/v1/business/roles", { targetDeviceId: props.deviceId });
    const confirmed = await api.get<{ coreId: string; coordinationAvailable: boolean }>("/api/device-mesh/v1/coordination/me");
    if (confirmed.coreId !== state.coreId || !confirmed.coordinationAvailable) throw new Error("服务提供者已变化，请刷新后重新选择角色");
    if (!result.executionScope || result.executionScope.coreId !== state.coreId || result.executionScope.targetDeviceId !== props.deviceId || result.executionScope.resourceOwnerId !== result.roleOwnerId) throw new Error("调用缺少有效的目标权限快照，请刷新后重试");
    if (ticket !== generation || !props.visible) return;
    coreId.value = state.coreId;
    ownerId.value = result.roleOwnerId;
    roles.value = result.roles;
    expectedScope.value = result.executionScope;
    if (result.roles.length === 1) roleId.value = result.roles[0].id;
    if (!result.roles.length) error.value = "目标数据来源没有可用角色，拒绝调用";
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "调用准备失败"; }
  finally { if (ticket === generation) loading.value = false; }
}

async function send() {
  if (running.value || !roleId.value || !message.value.trim() || !expectedScope.value) return;
  const selectedRole = roles.value.find(role => role.id === roleId.value);
  if (!selectedRole) return;
  const ticket = generation;
  const target = props.deviceId;
  const expectedCore = coreId.value;
  const expectedOwner = ownerId.value;
  const current = new AbortController();
  controller = current;
  requestId = crypto.randomUUID();
  running.value = true;
  reply.value = "";
  saved.value = false;
  error.value = "";
  let checking = false;
  const checkProvider = async () => {
    if (checking || current.signal.aborted || ticket !== generation) return;
    checking = true;
    try {
      const state = await api.get<{ coreId: string; coordinationAvailable: boolean }>("/api/device-mesh/v1/coordination/me");
      if (ticket !== generation || controller !== current) return;
      if (state.coreId !== expectedCore || !state.coordinationAvailable) {
        error.value = state.coreId !== expectedCore ? `云端服务提供者已从「${expectedCore}」切换为「${state.coreId}」，当前调用已中断。` : "Core 服务已暂停，当前调用已中断。";
        current.abort();
      }
    } catch (cause) {
      if (ticket === generation && controller === current) {
        error.value = "无法确认当前 Core 服务，调用已中断；恢复服务后再发起调用。";
        current.abort();
      }
    } finally { checking = false; }
  };
  const providerTimer = setInterval(() => { void checkProvider(); }, 3000);
  try {
    const result = await streamOwnedChat({ requestId, characterId: roleId.value, targetDeviceId: target, message: message.value.trim(), expectedExecutionScope: { ...expectedScope.value, roleId: roleId.value, roleRevision: selectedRole.revision } }, current.signal, event => {
      if (ticket !== generation) return;
      const scope = event.executionScope || event.data?.executionScope;
      if (scope && (scope.coreId !== expectedCore || scope.targetDeviceId !== target || scope.resourceOwnerId !== expectedOwner)) {
        error.value = scope.coreId !== expectedCore ? `云端服务提供者已从「${expectedCore}」切换为「${scope.coreId}」，当前调用已中断。` : "目标设备的数据归属已变化，当前调用已中断；刷新后再发起调用。";
        current.abort();
        throw new Error(error.value);
      }
      if (event.type === "delta" && !event.reasoning) reply.value += event.text || "";
    });
    if (ticket === generation) {
      reply.value = result.reply;
      saved.value = result.saved;
      savedOwner.value = result.executionScope.resourceOwnerId;
    }
  } catch (cause) { if (ticket === generation) error.value = current.signal.aborted ? error.value || "当前调用已中断，未确认的结果不会显示为已保存。" : cause instanceof Error ? cause.message : "调用失败"; }
  finally { clearInterval(providerTimer); if (controller === current) { running.value = false; controller = null; requestId = ""; } }
}

async function interrupt() {
  const active = requestId;
  controller?.abort();
  if (active) {
    try { await api.post(`/api/device-mesh/v1/business/messages/${encodeURIComponent(active)}/interrupt`); }
    catch (cause) { error.value = cause instanceof Error ? cause.message : "中断确认暂未返回"; }
  }
}

function close(value: boolean) {
  if (!value) { generation++; void interrupt(); }
  emit("update:visible", value);
}

watch(() => [props.visible, props.deviceId], () => {
  generation++;
  controller?.abort();
  reply.value = "";
  saved.value = false;
  if (props.visible && props.deviceId) void initialize();
}, { immediate: true });
onUnmounted(() => { generation++; void interrupt(); });
</script>

<style scoped>
.reply { white-space: pre-wrap; overflow-wrap: anywhere; padding: 12px 0; }
.el-select, .el-textarea { width: 100%; margin: 12px 0; }
</style>
