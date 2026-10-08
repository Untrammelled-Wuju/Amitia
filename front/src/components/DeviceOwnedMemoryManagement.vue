<template>
  <section class="owned-management">
    <div class="management-toolbar">
      <el-input v-model="query" clearable placeholder="检索当前角色的记忆" @keyup.enter="search()" />
      <el-select v-model="mode" aria-label="记忆检索方式"><el-option label="关键词" value="keyword" /><el-option label="混合检索" value="hybrid" /><el-option label="向量检索" value="vector" /></el-select>
      <el-select v-model="memoryType" clearable placeholder="全部记忆类型"><el-option label="全部类型" value="" /><el-option v-for="type in types" :key="type.value" :label="type.label" :value="type.value" /></el-select>
      <el-select v-model="sourceFilter" clearable placeholder="全部来源"><el-option label="全部来源" value="" /><el-option label="手动新增" value="manual" /><el-option label="审核保存" value="reviewed" /></el-select>
      <el-select v-model="sort" aria-label="记忆排序"><el-option label="默认排序" value="" /><el-option label="重要程度优先" value="importance_desc" /></el-select>
      <el-button :loading="loading" @click="search()">检索</el-button>
      <el-button :disabled="loading" @click="openCreate">新增记忆</el-button>
      <el-button :disabled="loading" @click="openGenerate">从对话生成候选</el-button>
      <el-button :disabled="loading" @click="loadCandidates">待审核候选</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <article v-for="row in results" :key="`${row.ownerId}/${row.id}`" class="management-row"><strong>{{ row.body.key }}</strong><p>{{ text(row) }}</p><small>{{ row.ownerId }} · 版本 {{ row.revision }}</small></article>
    <el-button v-if="nextCursor" :loading="loading" @click="search(true)">加载更多检索结果</el-button>
    <article v-for="row in candidates" :key="`${row.ownerId}/${row.id}`" class="management-row">
      <strong>{{ row.body.key }}</strong><p>{{ row.body.value }}</p><small>待审核 · {{ row.ownerId }} · 版本 {{ row.revision }}</small>
      <div><el-button :disabled="loading" @click="accept(row)">确认保存</el-button><el-button :disabled="loading" @click="editCandidate(row)">编辑候选</el-button><el-button :disabled="loading" @click="reject(row)">拒绝</el-button></div>
    </article>
    <el-dialog v-model="editorOpen" :title="editingCandidate ? '编辑候选记忆' : '新增记忆'" width="min(560px, 96vw)">
      <el-form label-position="top"><el-form-item label="标题"><el-input v-model="form.key" maxlength="512" /></el-form-item><el-form-item label="内容"><el-input v-model="form.value" type="textarea" :rows="5" /></el-form-item><el-form-item label="记忆类型"><el-select v-model="form.memoryType"><el-option v-for="kind in types" :key="kind.value" :label="kind.label" :value="kind.value" /></el-select></el-form-item><el-form-item label="重要程度"><el-input-number v-model="form.importance" :min="0" :max="10" /></el-form-item></el-form>
      <template #footer><el-button @click="editorOpen = false">取消</el-button><el-button :loading="loading" @click="save">保存</el-button></template>
    </el-dialog>
    <el-dialog v-model="generateOpen" title="从已保存对话提取候选" width="min(560px, 96vw)">
      <el-alert title="提取结果先保存为待审核候选，确认后才写入当前数据所有者的记忆。旧设备对话保持原位置，审核时会再次核验来源。" type="info" :closable="false" />
      <el-select v-model="generationSource" @change="loadGenerationConversations"><el-option label="当前数据所有者" value="current" /><el-option v-for="role in historicalRoles" :key="role.id" :label="`旧设备 · ${role.name || role.id}`" :value="role.id" /></el-select>
      <el-select v-model="conversationId" placeholder="选择对话"><el-option v-for="row in conversations" :key="row.id" :label="row.title || row.id" :value="row.id" /></el-select>
      <template #footer><el-button @click="generateOpen = false">取消</el-button><el-button :loading="loading" @click="generate">生成候选</el-button></template>
    </el-dialog>
    <el-dialog v-model="conflictOpen" title="确认记忆冲突" width="min(560px, 96vw)">
      <article v-for="row in conflictRows" :key="row.id"><strong>{{ row.body.key }}</strong><p>{{ text(row) }}</p><small>原数据所有者 {{ row.ownerId }} · 版本 {{ row.revision }}</small></article>
      <el-select v-model="resolution"><el-option label="替换原内容" value="replace" /><el-option label="合并原内容与新内容" value="merge" /><el-option label="同时保留两条" value="keep_both" /></el-select>
      <template #footer><el-button @click="conflictOpen = false">取消</el-button><el-button :loading="loading" :disabled="!resolution" @click="resolveConflict">按原版本处理</el-button></template>
    </el-dialog>
  </section>
</template>
<script setup lang="ts">
import { onUnmounted, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { useOwnedMemoryManagement, ownedMemoryAuthority, type OwnedMemoryManagementResource } from "@/composables/useOwnedMemoryManagement";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import { parseConversationReference } from "@/runtime/device-owned-conversation-reference";

const props = defineProps<{ executionScope: OwnedExecutionScope }>();
const emit = defineEmits<{ saved: [] }>();
const owned = useDeviceOwnedConversation();
const api = useOwnedMemoryManagement();
const query = ref("");
const mode = ref("keyword");
const memoryType = ref("");
const sourceFilter = ref("");
const sort = ref("");
const loading = ref(false);
const error = ref("");
const results = ref<OwnedMemoryManagementResource[]>([]);
const candidates = ref<OwnedMemoryManagementResource[]>([]);
const nextCursor = ref("");
const editorOpen = ref(false);
const generateOpen = ref(false);
const conflictOpen = ref(false);
const editingCandidate = ref<OwnedMemoryManagementResource>();
const form = reactive({ key: "", value: "", memoryType: "custom", importance: 5 });
const types = [{ value: "fact", label: "事实" }, { value: "preference", label: "偏好" }, { value: "episodic", label: "情节" }, { value: "relationship", label: "关系" }, { value: "custom", label: "其他" }];
const conversations = ref<any[]>([]);
const conversationId = ref("");
const generationSource = ref("current");
const historicalRoles = ref<Array<{ id: string; name: string }>>([]);
const conflictRows = ref<OwnedMemoryManagementResource[]>([]);
const resolution = ref("");
let generation = 0;
let editorScope: OwnedExecutionScope | undefined;
let generateScope: OwnedExecutionScope | undefined;
let attempt = "";
let attemptFingerprint = "";
let conflictIntent: { scope: OwnedExecutionScope; input: Record<string, unknown>; candidates: boolean } | undefined;
const snapshot = () => JSON.parse(JSON.stringify(props.executionScope)) as OwnedExecutionScope;
const text = (row: OwnedMemoryManagementResource) => typeof row.body.content === "string" ? row.body.content : String(row.body.content?.value || row.body.content?.text || "");
function invalidate() { generation++; editorScope = undefined; generateScope = undefined; conflictIntent = undefined; results.value = []; candidates.value = []; nextCursor.value = ""; editorOpen.value = false; generateOpen.value = false; conflictOpen.value = false; loading.value = false; attempt = ""; attemptFingerprint = ""; }
watch(() => ownedMemoryAuthority(props.executionScope), invalidate, { flush: "sync" });
watch([query, mode, memoryType, sourceFilter, sort], () => { results.value = []; nextCursor.value = ""; });
onUnmounted(invalidate);
async function search(older = false) {
  const scope = snapshot(); const ticket = generation;
  loading.value = true; error.value = "";
  const filter = { query: query.value, mode: mode.value, memoryType: memoryType.value, source: sourceFilter.value, sort: sort.value };
  const filterKey = JSON.stringify(filter);
  try { const page = await api.read(scope, { ...filter, cursor: older ? nextCursor.value : "", limit: 32 }); if (ticket !== generation || filterKey !== JSON.stringify({ query: query.value, mode: mode.value, memoryType: memoryType.value, source: sourceFilter.value, sort: sort.value })) return; results.value = older ? [...results.value, ...page.resources] : page.resources; nextCursor.value = page.nextCursor; }
  catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "记忆检索失败"; }
  finally { if (ticket === generation) loading.value = false; }
}
async function loadCandidates() {
  const scope = snapshot(); const ticket = generation; loading.value = true; error.value = "";
  try {
    const collected = new Map<string, OwnedMemoryManagementResource>(); let cursor = ""; const visited = new Set<string>();
    do { const page = await api.read(scope, { cursor, limit: 128 }, true); if (ticket !== generation) return; for (const row of page.resources) collected.set(row.id, row); cursor = page.nextCursor; if (collected.size > 4096 || cursor && visited.has(cursor)) throw new Error("候选分页超出上限或重复"); visited.add(cursor); } while (cursor);
    candidates.value = [...collected.values()];
  } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "候选记忆加载失败"; }
  finally { if (ticket === generation) loading.value = false; }
}
function openCreate() { editorScope = snapshot(); editingCandidate.value = undefined; Object.assign(form, { key: "", value: "", memoryType: "custom", importance: 5 }); attempt = ""; editorOpen.value = true; }
function editCandidate(row: OwnedMemoryManagementResource) { editorScope = JSON.parse(JSON.stringify(row.executionScope)); editingCandidate.value = JSON.parse(JSON.stringify(row)); Object.assign(form, { key: row.body.key, value: row.body.value, memoryType: row.body.memoryType || "custom", importance: row.body.importance ?? 5 }); attempt = ""; editorOpen.value = true; }
async function mutate(scope: OwnedExecutionScope, input: Record<string, unknown>, candidate = false) {
  if (ownedMemoryAuthority(scope) !== ownedMemoryAuthority(props.executionScope)) { error.value = "服务或权限已变化，请重新加载原记录再操作"; return; }
  const ticket = generation; const fingerprint = JSON.stringify([ownedMemoryAuthority(scope), input, candidate]); if (!attempt || fingerprint !== attemptFingerprint) { attempt = crypto.randomUUID(); attemptFingerprint = fingerprint; }
  loading.value = true; error.value = "";
  try { await api.write(scope, attempt, input, candidate); if (ticket !== generation) return; attempt = ""; editorOpen.value = false; generateOpen.value = false; conflictOpen.value = false; ElMessage.success(input.action === "generate" ? "候选已由数据所有者确认保存，审核后再用于记忆" : "数据所有者已确认保存"); emit("saved"); if (ticket === generation) await loadCandidates(); }
  catch (cause) { if (ticket !== generation) return; try { const conflicts = api.conflicts(cause, scope); if (conflicts.length) { conflictRows.value = conflicts; conflictIntent = { scope, input, candidates: candidate }; resolution.value = ""; conflictOpen.value = true; } else error.value = cause instanceof Error ? cause.message : "操作尚未确认保存"; } catch (invalid) { error.value = invalid instanceof Error ? invalid.message : "冲突记录的数据归属无效"; } }
  finally { if (ticket === generation) loading.value = false; }
}
async function save() { if (!editorScope || !form.key.trim() || !form.value.trim()) { error.value = "标题和内容不能为空"; return; } const row = editingCandidate.value; await mutate(editorScope, { ...form, key: form.key.trim(), value: form.value.trim(), action: row ? "update" : "create", ...(row ? { id: row.id, expectedRevision: row.revision } : {}) }, !!row); }
async function accept(row: OwnedMemoryManagementResource) { const original = JSON.parse(JSON.stringify(row)); await mutate(original.executionScope, { action: "accept", id: original.id, expectedRevision: original.revision }, true); }
async function reject(row: OwnedMemoryManagementResource) { const original = JSON.parse(JSON.stringify(row)); try { await ElMessageBox.confirm("拒绝此候选，不会将其保存为可用于对话的记忆。", "拒绝候选", { type: "warning" }); await mutate(original.executionScope, { action: "reject", id: original.id, expectedRevision: original.revision }, true); } catch {} }
async function resolveConflict() { const original = conflictIntent; const row = conflictRows.value[0]; if (!original || !row || !resolution.value) return; await mutate(original.scope, { ...original.input, action: original.candidates ? "accept" : "resolve", conflictId: row.id, expectedConflictRevision: row.revision, resolution: resolution.value }, original.candidates); }
async function openGenerate() { const scope = snapshot(); const ticket = generation; try { const roles = scope.coordinated ? await owned.historicalRoles(scope.roleId) : []; if (ticket !== generation) return; historicalRoles.value = roles; generationSource.value = "current"; generateScope = scope; attempt = ""; generateOpen.value = true; await loadGenerationConversations(); } catch (cause) { if (ticket === generation) error.value = cause instanceof Error ? cause.message : "对话加载失败"; } }
async function loadGenerationConversations() {
  const scope = generateScope; if (!scope) return;
  const ticket = generation; const source = generationSource.value;
  conversationId.value = ""; conversations.value = []; loading.value = true;
  try {
    if (source === "current") { const rows = await owned.conversations(scope.roleId); if (ticket !== generation || source !== generationSource.value) return; conversations.value = rows.filter((row) => row.ownerId === scope.resourceOwnerId); return; }
    if (!scope.coordinated || !historicalRoles.value.some((role) => role.id === source)) throw new Error("旧设备角色目录已变化");
    const rows = new Map<string, any>(); let cursor = ""; let legacyCursor = ""; const visited = new Set<string>();
    do {
      const result = await owned.data("conversation", scope.roleId, "", "", { historicalRoleId: source, historicalCursor: cursor, historicalLegacyCursor: legacyCursor });
      if (ticket !== generation || source !== generationSource.value) return;
      if (ownedMemoryAuthority(result.executionScope) !== ownedMemoryAuthority(scope)) throw new Error("服务归属已变化，请重新打开候选提取");
      const history = result.historicalSnapshot;
      if (!history || history.ownerId !== scope.targetDeviceId || (history as typeof history & { role?: { id: string } }).role?.id !== source) throw new Error("旧设备会话来源无效");
      for (const resource of history.resources || []) { if (resource.kind !== "conversation" || resource.deleted) continue; if (resource.ownerId !== scope.targetDeviceId || resource.roleId !== source) throw new Error("旧设备会话归属无效"); rows.set(resource.id, { ...resource.body, id: resource.id, ownerId: resource.ownerId }); }
      for (const row of history.legacyConversations || []) { if (!row.id) throw new Error("旧设备会话缺少编号"); rows.set(row.id, { ...row, ownerId: scope.targetDeviceId }); }
      cursor = history.nextCursors?.conversation || ""; legacyCursor = history.nextCursors?.legacyConversation || "";
      const key = JSON.stringify([cursor, legacyCursor]); if (rows.size > 4096 || (cursor || legacyCursor) && visited.has(key)) throw new Error("旧设备会话分页无效"); visited.add(key);
    } while (cursor || legacyCursor);
    conversations.value = [...rows.values()];
  } catch (cause) { if (ticket === generation && source === generationSource.value) error.value = cause instanceof Error ? cause.message : "对话加载失败"; }
  finally { if (ticket === generation && source === generationSource.value) loading.value = false; }
}
async function generate() {
  if (!generateScope || !conversationId.value || loading.value) { error.value = "请选择对话"; return; }
  const row = conversations.value.find((item) => item.id === conversationId.value); const origin = parseConversationReference(conversationId.value);
  if (!row) { error.value = "对话目录已变化，请重新选择"; return; }
  const historical = generationSource.value !== "current";
  const ownerId = historical ? generateScope.targetDeviceId : generateScope.resourceOwnerId;
  if (row.ownerId !== ownerId || historical && !historicalRoles.value.some((role) => role.id === generationSource.value)) { error.value = "对话所属设备已变化，请重新选择"; return; }
  await mutate(generateScope, { action: "generate", conversationId: origin?.id || conversationId.value, conversationOrigin: { ownerId, id: origin?.id || conversationId.value }, ...(historical ? { historicalRoleId: generationSource.value } : {}) }, true);
}
</script>
<style scoped>
.owned-management { margin: 16px 0; }
.management-toolbar { display: flex; flex-wrap: wrap; gap: 8px; }
.management-toolbar :deep(.el-input) { width: 220px; }
.management-toolbar :deep(.el-select) { width: 130px; }
.management-row { padding: 12px; margin-top: 10px; background: var(--ac-color-bg-secondary); border-radius: var(--ac-radius-sm); }
.management-row p { white-space: pre-wrap; overflow-wrap: anywhere; }
.management-row small { color: var(--ac-color-text-secondary); }
</style>
