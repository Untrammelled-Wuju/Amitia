<template>
  <component :is="embedded ? 'section' : ElDrawer" :model-value="visible" title="记忆与数据归属" size="min(620px, 100vw)" @update:model-value="$emit('update:visible', $event)">
    <el-alert :title="owned.coordinated.value ? '新记忆由当前 Core 保存和管理' : '新记忆由当前设备保存和管理，Core 负责计算'" type="info" :closable="false" />
    <p class="owner-line">服务：{{ owned.coreId.value }} · 数据所有者：{{ owner || '加载中' }}</p>
	<el-select v-if="owned.coordinated.value" v-model="source" aria-label="记忆数据来源" class="layer-select">
	  <el-option label="当前 Core 的数据" value="current" />
	  <el-option v-for="role in historicalRoles" :key="role.id" :label="`原设备历史 · ${role.name}`" :value="`history:${role.id}`" />
	</el-select>
	<el-alert v-if="source !== 'current'" title="原设备历史数据可供 Core 调用；请在原设备管理这些历史记录。" type="info" :closable="false" />
    <el-select v-model="kind" aria-label="记忆层" class="layer-select">
      <el-option v-for="layer in layers" :key="layer.id" :label="layer.label" :value="layer.id" />
    </el-select>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <DeviceOwnedMemoryManagement v-if="source === 'current' && kind === 'memory' && loadedScope && !loading" :execution-scope="loadedScope" @saved="load()" />
    <div v-if="source === 'current' && ['vector', 'graph'].includes(kind)" class="projection-status">
      <p v-for="layer in projection?.status?.layers || []" :key="layer.kind">{{ layer.kind === 'vector' ? '向量' : '图谱' }}：已处理 {{ layer.current }}/{{ layer.total }} · 待更新 {{ layer.pending }} · 重试 {{ layer.retrying }}</p>
      <el-alert v-if="projectionNotice" :title="projectionNotice" type="info" :closable="false" />
      <el-button :disabled="!projection || loading" :loading="rebuilding" @click="rebuild">重建当前角色索引</el-button>
    </div>
    <div v-loading="loading">
      <article v-for="row in rows" :key="`${row.ownerId}/${row.kind}/${row.id}`" class="owned-memory-card">
        <div class="card-title">{{ row.body.key || row.body.title || row.id }}</div>
        <div class="record-text">{{ content(row) }}</div>
        <div class="record-state">
          <el-tag size="small">版本 {{ row.revision }}</el-tag>
          <el-tag v-if="row.body.allowContextUse === false" size="small" type="warning">已停用</el-tag>
          <el-tag v-if="row.body.expiresAt" size="small" type="info">到期 {{ row.body.expiresAt }}</el-tag>
        </div>
        <div v-if="['memory', 'summary'].includes(row.kind) && source === 'current' && row.revision > 0" class="record-actions">
          <el-button size="small" :disabled="loading" @click="editContent(row)">编辑</el-button>
          <template v-if="row.kind === 'memory'">
            <el-button size="small" :disabled="loading" @click="setUse(row)">{{ row.body.allowContextUse === false ? '允许用于对话' : '停用' }}</el-button>
            <el-button size="small" :disabled="loading" @click="setExpiry(row)">到期时间</el-button>
            <el-button size="small" :disabled="loading" @click="setArchive(row)">{{ row.body.archivedAt ? '取消归档' : '归档' }}</el-button>
          </template>
          <el-button size="small" type="danger" :disabled="loading" @click="remove(row)">删除</el-button>
        </div>
      </article>
      <el-empty v-if="!loading && !error && rows.length === 0" description="当前角色在此数据来源中暂无记录" />
    </div>
    <el-button v-if="next || nextLegacy" :loading="loading" @click="load(true)">加载更多</el-button>
    <el-button :disabled="loading" @click="load(false)">刷新</el-button>
  </component>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";
import { ElDrawer, ElMessageBox } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import DeviceOwnedMemoryManagement from "./DeviceOwnedMemoryManagement.vue";

const props = defineProps<{ visible: boolean; characterId: string; conversationId: string; embedded?: boolean; initialKind?: string }>();
defineEmits<{ "update:visible": [value: boolean] }>();
const owned = useDeviceOwnedConversation();
const layers = [
  { id: "memory", label: "原始记忆" }, { id: "working", label: "工作记忆" },
  { id: "profile", label: "用户画像" }, { id: "episodic", label: "情节记忆" },
  { id: "fact", label: "事实记忆" }, { id: "vector", label: "向量索引" },
  { id: "graph", label: "关系图谱" }, { id: "summary", label: "对话摘要" },
];
const kind = ref(props.initialKind || "memory");
const rows = ref<any[]>([]);
const next = ref("");
const nextLegacy = ref("");
const source = ref("current");
const historicalRoles = ref<Array<{id:string;name:string}>>([]);
const owner = ref("");
const error = ref("");
const loading = ref(false);
const projection = ref<any>(null);
const projectionNotice = ref("");
const rebuilding = ref(false);
const loadedScope = ref<OwnedExecutionScope>();
let generation = 0;
let scope = "";
let providerTimer: ReturnType<typeof setInterval> | undefined;

function content(row: any): string {
  const value = row.body.content;
  if (row.kind === "vector") return `${value?.values?.length || 0} 维 · 模型 ${String(value?.modelFingerprint || "待重建").slice(0, 16)}`;
  if (typeof value === "string") return value;
  return String(value?.value || value?.text || value?.summary || JSON.stringify(value || row.body));
}

async function load(older = false) {
  const ticket = ++generation;
  const currentKind = kind.value;
  if (!older) { projection.value = null; projectionNotice.value = ""; }
  if (!older) loadedScope.value = undefined;
  if (!older) { rows.value = []; next.value = ""; nextLegacy.value = ""; scope = ""; }
  error.value = "";
  loading.value = true;
  try {
    if (owned.coordinated.value && historicalRoles.value.length === 0) {
      const catalog = await owned.historicalRoles(props.characterId);
      if (ticket !== generation || !props.visible) return;
      historicalRoles.value = catalog;
    }
    const historical = source.value.startsWith("history:");
	    const result = await owned.data(currentKind, props.characterId, historical ? "" : props.conversationId, !historical && older ? next.value : "", historical ? { historicalRoleId: source.value.slice(8), historicalCursor: older ? next.value : "", historicalLegacyCursor: older ? nextLegacy.value : "" } : { legacyCursor: older ? nextLegacy.value : "" });
    if (ticket !== generation || !props.visible) return;
    const currentScope = JSON.stringify(result.executionScope, (key, value) => ["requestId", "turnId", "executionId"].includes(key) ? undefined : value);
    if (older && scope !== currentScope) throw new Error("服务或数据归属已变化，请刷新后继续查看");
    scope = currentScope;
    const snapshot = historical ? result.historicalSnapshot : result.snapshot;
    if (!snapshot) throw new Error("原设备历史数据暂不可用");
    if (snapshot.ownerId !== (historical ? result.executionScope.targetDeviceId : result.executionScope.resourceOwnerId)) throw new Error("记忆数据所有者已变化，请重新加载");
    owner.value = snapshot.ownerId;
    loadedScope.value = result.executionScope;
    const merged = new Map((older ? rows.value : []).map((row: any) => [`${row.ownerId}/${row.kind}/${row.id}`, row]));
    for (const row of snapshot.resources.filter((row: any) => row.kind === currentKind)) {
      if (row.ownerId !== snapshot.ownerId || row.roleId !== (historical ? source.value.slice(8) : props.characterId)) throw new Error("记忆所属设备或角色无效，请重新加载");
      merged.set(`${row.ownerId}/${row.kind}/${row.id}`, { ...row, executionScope: result.executionScope });
    }
    const legacy = currentKind === "memory" ? snapshot.legacyMemories : currentKind === "profile" ? snapshot.legacyProfiles : currentKind === "episodic" ? snapshot.legacyEpisodes : [];
    for (const row of legacy || []) {
      const id = `legacy/${row.id}`;
      merged.set(`${snapshot.ownerId}/${currentKind}/${id}`, { id, kind: currentKind, ownerId: snapshot.ownerId, revision: 0, body: { key: row.key || row.title || row.fieldName, content: row, expiresAt: row.expiresAt, allowContextUse: row.allowContextUse } });
    }
    rows.value = Array.from(merged.values());
    const cursor = snapshot.nextCursors?.[currentKind] || "";
    const legacyKinds: Record<string, string> = { memory: "legacyMemory", profile: "legacyProfile", episodic: "legacyEpisode" };
    const legacyKind = legacyKinds[currentKind];
    const legacyCursor = legacyKind ? snapshot.nextCursors?.[legacyKind] || "" : "";
    if (older && ((cursor && cursor === next.value) || (legacyCursor && legacyCursor === nextLegacy.value))) throw new Error("分页游标重复，请刷新后重试");
    next.value = cursor;
    nextLegacy.value = legacyCursor;
    if (source.value === "current" && ["vector", "graph"].includes(currentKind)) {
      try {
        const value = await owned.projections(props.characterId);
        if (JSON.stringify(value.executionScope, (key, item) => ["requestId", "turnId", "executionId"].includes(key) ? undefined : item) !== currentScope) throw new Error("索引数据归属已变化，请刷新后重试");
        if (ticket === generation) projection.value = value;
      } catch (cause) {
        if (ticket === generation) projectionNotice.value = cause instanceof Error ? cause.message : "索引状态暂不可用";
      }
    }
  } catch (cause) {
    if (ticket === generation) error.value = cause instanceof Error ? cause.message : "记忆加载失败";
  } finally {
    if (ticket === generation) loading.value = false;
  }
}

async function setUse(row: any) {
  try {
    await owned.edit("memory", row.id, { allowContextUse: row.body.allowContextUse === false }, { characterId: row.executionScope.roleId, expectedExecutionScope: row.executionScope, expectedOwnerId: row.ownerId, expectedRevision: row.revision });
    await load();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "修改失败"; }
}

async function editContent(row: any) {
  const original = JSON.parse(JSON.stringify(row));
  try {
    const result = await ElMessageBox.prompt("修改当前数据所有者保存的内容；原设备历史记录需在原设备管理。", row.kind === "summary" ? "编辑摘要" : "编辑记忆", { inputType: "textarea", inputValue: content(original), inputValidator: (value) => Boolean(String(value || "").trim()) && new TextEncoder().encode(value).length <= 131072 || "内容不能为空或超出128 KiB", confirmButtonText: "保存", cancelButtonText: "取消" });
    const text = result.value.trim();
    const current = original.body.content;
    const updated = typeof current === "object" && current !== null ? { ...current, [original.kind === "summary" ? Object.hasOwn(current, "text") && !Object.hasOwn(current, "summary") ? "text" : "summary" : Object.hasOwn(current, "text") && !Object.hasOwn(current, "value") ? "text" : "value"]: text } : text;
    await owned.edit(original.kind, original.id, { content: updated }, { characterId: original.executionScope.roleId, expectedExecutionScope: original.executionScope, expectedOwnerId: original.ownerId, expectedRevision: original.revision });
    await load();
  } catch (cause) { if (cause !== "cancel" && cause !== "close") error.value = cause instanceof Error ? cause.message : "内容保存失败"; }
}

async function setExpiry(row: any) {
  const original = JSON.parse(JSON.stringify(row));
  try {
    const result = await ElMessageBox.prompt("输入带时区的 ISO 时间，例如 2026-10-08T18:00:00+08:00；留空取消到期限制。", "记忆到期时间", { inputValue: original.body.expiresAt || "", inputValidator: (value) => !String(value || "").trim() || /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value.trim()) && Number.isFinite(Date.parse(value)) || "请输入有效的带时区 ISO 时间", confirmButtonText: "保存", cancelButtonText: "取消" });
    await owned.edit("memory", original.id, { expiresAt: result.value.trim() ? new Date(result.value.trim()).toISOString() : "" }, { characterId: original.executionScope.roleId, expectedExecutionScope: original.executionScope, expectedOwnerId: original.ownerId, expectedRevision: original.revision });
    await load();
  } catch (cause) { if (cause !== "cancel" && cause !== "close") error.value = cause instanceof Error ? cause.message : "到期时间保存失败"; }
}

async function setArchive(row: any) {
  try {
    await owned.edit("memory", row.id, { archivedAt: row.body.archivedAt ? "" : new Date().toISOString() }, { characterId: row.executionScope.roleId, expectedExecutionScope: row.executionScope, expectedOwnerId: row.ownerId, expectedRevision: row.revision });
    await load();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "记忆归档失败"; }
}

async function rebuild() {
  if (!projection.value || rebuilding.value) return;
  const ticket = generation;
  rebuilding.value = true;
  try {
    const value = await owned.projections(projection.value.executionScope.roleId, projection.value.executionScope);
    if (ticket === generation) {
      projection.value = value;
      projectionNotice.value = "已加入所有者的重建队列；处理完成后刷新查看状态。";
    }
  } catch (cause) {
    if (ticket === generation) projectionNotice.value = cause instanceof Error ? cause.message : "重建请求失败";
  } finally { rebuilding.value = false; }
}

async function remove(row: any) {
  try {
    await ElMessageBox.confirm(row.kind === "summary" ? "删除当前所有者保存的摘要，原始对话仍会保留。" : "删除原始记忆会同时停用其事实、向量和图谱数据。", row.kind === "summary" ? "删除摘要" : "删除记忆", { confirmButtonText: "删除", cancelButtonText: "取消", type: "warning" });
    await owned.edit(row.kind, row.id, {}, { characterId: row.executionScope.roleId, deleted: true, expectedExecutionScope: row.executionScope, expectedOwnerId: row.ownerId, expectedRevision: row.revision });
    await load();
  } catch (cause) {
    if (cause !== "cancel" && cause !== "close") error.value = cause instanceof Error ? cause.message : "删除失败";
  }
}

function clearAuthority() {
  generation++;
  historicalRoles.value = [];
  source.value = "current";
  rows.value = [];
  owner.value = "";
  projection.value = null;
  loadedScope.value = undefined;
  next.value = "";
  nextLegacy.value = "";
  scope = "";
  loading.value = false;
}
watch(() => [props.characterId, owned.coreId.value, owned.policy.value?.providerEpoch, owned.policy.value?.modeRevision, owned.policy.value?.permissionRevision], clearAuthority, { flush: "sync" });
watch(() => [props.visible, props.characterId, props.conversationId, kind.value, source.value, owned.coreId.value, owned.policy.value?.providerEpoch, owned.policy.value?.modeRevision, owned.policy.value?.permissionRevision], () => {
  generation++;
  loading.value = false;
  if (props.visible && props.characterId) void load();
}, { immediate: true });
onMounted(() => {
  window.addEventListener("amitia:runtime-connection-changed", clearAuthority);
  window.addEventListener("amitia:execution-scope-changed", clearAuthority);
  providerTimer = setInterval(() => { if (props.visible) void owned.refresh().catch(clearAuthority); }, 3000);
});
onUnmounted(() => {
  if (providerTimer) clearInterval(providerTimer);
  window.removeEventListener("amitia:runtime-connection-changed", clearAuthority);
  window.removeEventListener("amitia:execution-scope-changed", clearAuthority);
  clearAuthority();
});
</script>

<style scoped>
.owner-line { overflow-wrap: anywhere; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.layer-select { width: 100%; margin-bottom: 16px; }
.owned-memory-card { padding: 14px; margin-bottom: 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); }
.card-title { font-weight: 500; overflow-wrap: anywhere; }
.record-text { white-space: pre-wrap; overflow-wrap: anywhere; margin: 10px 0; }
.record-state, .record-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
</style>
