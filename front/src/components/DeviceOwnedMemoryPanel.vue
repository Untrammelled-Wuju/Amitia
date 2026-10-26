<template>
  <component :is="embedded ? 'section' : ElDrawer" :model-value="visible" title="记忆与数据归属" size="min(620px, 100vw)" @update:model-value="$emit('update:visible', $event)">
    <div v-if="!fixedKind" class="memory-controls">
      <div class="memory-filters">
        <div class="filter-field">
          <label :for="`${controlId}-source`">数据来源</label>
          <el-select :id="`${controlId}-source`" v-model="source" aria-label="记忆数据来源" class="layer-select">
            <el-option label="当前 Core 的数据" value="current" />
            <el-option v-for="role in kind === 'profile' ? historicalRoles.slice(0, 1) : historicalRoles" :key="role.id" :label="kind === 'profile' ? '原设备历史' : `原设备历史 · ${role.name}`" :value="`history:${role.id}`" />
          </el-select>
        </div>
        <div class="filter-field">
          <label :for="`${controlId}-scope`">查看范围</label>
          <el-select :id="`${controlId}-scope`" v-model="memoryView" aria-label="记忆查看范围" class="layer-select">
            <el-option v-for="option in viewOptions" :key="option.value" :label="option.label" :value="option.value" />
          </el-select>
        </div>
        <div v-if="!fixedKind" class="filter-field">
          <label :for="`${controlId}-kind`">记忆分类</label>
          <el-select :id="`${controlId}-kind`" v-model="kind" aria-label="记忆层" class="layer-select">
            <el-option v-for="layer in layers" :key="layer.id" :label="layer.label" :value="layer.id" />
          </el-select>
        </div>
      </div>
    </div>
    <el-alert v-if="source !== 'current'" title="原设备历史数据可供 Core 调用；请在原设备管理这些历史记录。" type="info" :closable="false" />
    <el-alert v-if="error" :title="error" type="error" :closable="false" />
    <DeviceOwnedMemoryManagement v-if="source === 'current' && kind === 'memory' && loadedScope && !loading" :execution-scope="loadedScope" :search="search" @saved="load()" @refresh="load(false)" />
    <div v-if="source === 'current' && !loadedScope?.memoryViewAll && ['vector', 'graph'].includes(kind)" class="projection-status">
      <p v-for="layer in projection?.status?.layers || []" :key="layer.kind">{{ layer.kind === 'vector' ? '向量' : '图谱' }}：已处理 {{ layer.current }}/{{ layer.total }} · 待更新 {{ layer.pending }} · 重试 {{ layer.retrying }}</p>
      <el-alert v-if="projectionNotice" :title="projectionNotice" type="info" :closable="false" />
      <el-button :disabled="!projection || loading" :loading="rebuilding" @click="rebuild">重建当前范围索引</el-button>
    </div>
    <div v-if="search === undefined || ['profile', 'episodic', 'graph'].includes(view)" class="memory-filters page-filters">
      <div v-if="search === undefined" class="filter-field">
        <el-input :id="`${controlId}-page-search`" v-model="searchKeyword" clearable aria-label="搜索记忆" placeholder="搜索标题或内容" class="layer-select" />
      </div>
      <el-button v-if="view === 'profile' && source === 'current'" :disabled="loading" @click="createProfileOpen = true">新增画像</el-button>
      <template v-if="view === 'graph' && source === 'current' && rows.length">
        <el-select v-model="graphTransferId" class="graph-transfer-select" filterable placeholder="选择要迁移的图谱关系" aria-label="待迁移图谱关系">
          <el-option v-for="row in rows" :key="row.id" :label="ownedMemoryTitle(row)" :value="row.id" />
        </el-select>
        <el-button :disabled="loading || !graphTransferId" @click="openTransfer(rows.find((row: any) => row.id === graphTransferId))">调整所属分区</el-button>
      </template>
      <el-button v-if="['profile', 'episodic', 'graph'].includes(view)" :disabled="loading" @click="load(false)">刷新</el-button>
    </div>
    <div v-loading="loading">
      <OwnedMemoryGraph v-if="view === 'graph'" :resources="displayRows" :loading="loading" :error="error" :hide-search="search !== undefined" />
      <div v-else-if="view === 'timeline'" class="timeline-view">
        <div class="timeline-toolbar">
          <el-select v-model="timelineLayers" multiple collapse-tags clearable placeholder="记忆层" class="tl-layers">
            <el-option v-for="layer in timelineLayerOptions" :key="layer.id" :label="layer.label" :value="layer.id" />
          </el-select>
          <el-button :disabled="timelineRows.length === 0" @click="exportTimelineCSV">CSV 导出</el-button>
          <span class="tl-count">共 {{ timelineRows.length }} 条</span>
        </div>
        <div class="timeline">
          <div v-for="row in timelineRows" :key="`${row.ownerId}/${row.kind}/${row.id}`" class="tl-item">
            <span class="tl-dot" :class="`layer-${row.kind}`" />
            <div class="tl-card">
              <div class="tl-header">
                <el-tag size="small">{{ timelineLayerLabel(row.kind) }}</el-tag>
                <span class="tl-time">{{ timelineTime(row) }}</span>
                <span class="tl-role">{{ row.roleId === GLOBAL_MEMORY_ROLE ? '全局记忆' : '角色记忆' }}</span>
              </div>
              <div class="card-title">{{ ownedMemoryTitle(row) }}</div>
              <div class="record-text">
                <div v-for="(field, index) in ownedMemoryFields(row)" :key="index" class="record-field">
                  <span class="record-field-label">{{ field.label }}</span>
                  <span class="record-field-value">{{ field.value }}</span>
                </div>
              </div>
            </div>
          </div>
          <el-empty v-if="!loading && timelineRows.length === 0" description="当前筛选下没有记忆记录" />
        </div>
      </div>
      <template v-else>
      <article v-for="row in displayRows" :key="`${row.ownerId}/${row.kind}/${row.id}`" class="owned-memory-card">
        <div class="card-title">{{ ownedMemoryTitle(row) }}</div>
        <div class="record-text">
          <div v-for="(field, index) in ownedMemoryFields(row)" :key="index" class="record-field">
            <span class="record-field-label">{{ field.label }}</span>
            <span class="record-field-value">{{ field.value }}</span>
          </div>
        </div>
        <div class="record-state">
          <el-tag v-if="['memory', 'episodic', 'graph', 'fact', 'vector', 'profile'].includes(row.kind)" size="small">{{ row.roleId === GLOBAL_MEMORY_ROLE ? '全局记忆' : '角色记忆' }}</el-tag>
          <el-tag size="small">版本 {{ row.revision }}</el-tag>
          <el-tag v-if="row.kind === 'memory'" size="small">L{{ (row.body.effectiveRetentionLevel ?? row.body.retentionLevel) || 3 }} · 强度 {{ Math.round(((row.body.effectiveStrength ?? row.body.memoryStrength) ?? 0.68) * 100) }}% · {{ row.body.pinned ? '已固定' : (row.body.effectiveDecayState ?? row.body.decayState) === 'archived' ? '自动归档' : (row.body.effectiveDecayState ?? row.body.decayState) === 'fading' ? '淡化' : '活跃' }} · 强化 {{ row.body.reinforceCount || 0 }} 次</el-tag>
          <el-tag v-if="row.body.allowContextUse === false" size="small" type="warning">已停用</el-tag>
          <el-tag v-if="row.body.allowProactiveMention === false" size="small" type="info">不主动提及</el-tag>
          <el-tag v-if="row.body.expiresAt" size="small" type="info">到期 {{ row.body.expiresAt }}</el-tag>
        </div>
        <div v-if="source === 'current' && ['memory', 'episodic', 'fact', 'vector', 'profile'].includes(row.kind) && row.executionScope" class="record-actions">
          <el-button size="small" :disabled="loading" @click="openTransfer(row)">{{ row.roleId === GLOBAL_MEMORY_ROLE ? '降级为角色记忆' : '提升为全局记忆' }}</el-button>
        </div>
        <div v-if="['memory', 'summary'].includes(row.kind) && source === 'current' && row.revision > 0" class="record-actions">
          <el-button size="small" :disabled="loading" @click="editContent(row)">编辑</el-button>
          <template v-if="row.kind === 'memory'">
            <el-button size="small" :disabled="loading" @click="openLifecycle(row)">保留等级与固定</el-button>
            <el-button size="small" :disabled="loading" @click="setUse(row)">{{ row.body.allowContextUse === false ? '允许用于对话' : '停用' }}</el-button>
            <el-button size="small" :disabled="loading" @click="setProactiveMention(row)">{{ row.body.allowProactiveMention === false ? '允许主动提及' : '禁止主动提及' }}</el-button>
            <el-button size="small" :disabled="loading" @click="setExpiry(row)">到期时间</el-button>
            <el-button size="small" :disabled="loading" @click="setArchive(row)">{{ row.body.archivedAt ? '取消归档' : '归档' }}</el-button>
          </template>
          <el-button size="small" type="danger" :disabled="loading" @click="remove(row)">删除</el-button>
        </div>
      </article>
      <el-empty v-if="!loading && !error && displayRows.length === 0" description="当前角色在此数据来源中暂无记录" />
    <el-dialog v-model="createProfileOpen" title="新增画像" width="min(520px, 94vw)">
      <el-form label-position="top">
        <el-form-item label="属性名"><el-input v-model="createProfileForm.key" maxlength="120" placeholder="如：饮食习惯" /></el-form-item>
        <el-form-item label="属性值"><el-input v-model="createProfileForm.value" type="textarea" :rows="3" maxlength="2000" /></el-form-item>
        <el-form-item label="类别">
          <el-select v-model="createProfileForm.memoryType">
            <el-option label="个人信息" value="personal_info" />
            <el-option label="偏好" value="preference" />
            <el-option label="习惯" value="habit" />
            <el-option label="计划" value="plan" />
            <el-option label="关系" value="relationship" />
            <el-option label="自定义" value="custom" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button :disabled="createProfileSaving" @click="createProfileOpen = false">取消</el-button>
        <el-button type="primary" :loading="createProfileSaving" :disabled="!createProfileForm.key.trim() || !createProfileForm.value.trim()" @click="submitCreateProfile">保存</el-button>
      </template>
    </el-dialog>
      </template>
    </div>
    <el-button v-if="next || nextLegacy" :loading="loading" @click="load(true)">加载更多</el-button>
    <el-button v-if="!['profile', 'episodic', 'graph'].includes(view) && !(source === 'current' && kind === 'memory' && loadedScope)" :disabled="loading" @click="load(false)">刷新</el-button>
    <el-dialog v-model="transferOpen" title="调整记忆所属分区" width="min(480px, 92vw)" :close-on-click-modal="!transferring" :show-close="!transferring">
      <p>移动这条记录及关联事实、向量和图谱，保留来源和历史。原分区不再保留副本。</p>
      <el-select v-model="transferTarget" aria-label="目标记忆分区" :disabled="transferring">
        <el-option label="全局记忆" value="global" /><el-option label="角色记忆" value="character" />
      </el-select>
      <el-select v-if="transferTarget === 'character'" v-model="transferRole" filterable aria-label="目标角色" placeholder="选择目标角色" :disabled="transferring">
        <el-option v-for="role in owned.roles.value" :key="role.id" :label="role.name" :value="role.id" />
      </el-select>
      <template #footer><el-button :disabled="transferring" @click="transferOpen = false">取消</el-button><el-button type="primary" :loading="transferring" :disabled="transferTarget === 'character' && !transferRole" @click="confirmTransfer">确认迁移</el-button></template>
    </el-dialog>
    <el-dialog v-model="lifecycleOpen" title="记忆保留设置" width="min(480px, 96vw)">
      <el-form label-position="top">
        <el-form-item label="保留等级"><el-select v-model="lifecycleForm.retentionLevel" :disabled="lifecycleForm.pinned"><el-option v-for="level in 5" :key="level" :label="`L${level}`" :value="level" /></el-select></el-form-item>
        <el-form-item label="固定记忆"><el-switch v-model="lifecycleForm.pinned" /><span>固定为 L1，停止自动衰减</span></el-form-item>
        <el-form-item label="记忆强度"><el-slider v-model="lifecycleForm.strength" :disabled="lifecycleForm.pinned || lifecycleForm.retentionLevel === 1" :min="0" :max="100" /></el-form-item>
      </el-form>
      <template #footer><el-button :disabled="lifecycleSaving" @click="lifecycleOpen = false">取消</el-button><el-button :loading="lifecycleSaving" @click="saveLifecycle">保存</el-button></template>
    </el-dialog>
  </component>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch, onMounted, onUnmounted, useId } from "vue";
import { ElMessageBox } from "element-plus";
import ElDrawer from "@/components/ui-runtime/WindowDrawer.vue";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { useOwnedMemoryManagement } from "@/composables/useOwnedMemoryManagement";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import DeviceOwnedMemoryManagement from "./DeviceOwnedMemoryManagement.vue";
import OwnedMemoryGraph from "./OwnedMemoryGraph.vue";
import { GLOBAL_MEMORY_ROLE, memoryStorageRole, memoryRecordScope } from "@/runtime/owned-memory-scope";
import { ownedMemoryTitle, ownedMemoryFields, ownedMemoryEditableText } from "@/runtime/owned-memory-format";
import { transferMemoryScope } from "@/composables/useMemoryScopeTransfer";
import { ownedMemoryViewOptions } from "@/runtime/owned-memory-view-options";

const props = defineProps<{ visible: boolean; characterId: string; conversationId: string; embedded?: boolean; initialKind?: string; fixedKind?: boolean; requestedMemoryView?: string; mode?: string; kinds?: string[]; source?: string; search?: string }>();
defineEmits<{ "update:visible": [value: boolean] }>();
const owned = useDeviceOwnedConversation();
const controlId = useId();
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
const internalSource = ref("current");
const source = computed({
  get: () => props.source ?? internalSource.value,
  set: (value: string) => { internalSource.value = value; },
});
const memoryView = ref(props.requestedMemoryView || "");
watch(() => props.requestedMemoryView, value => { memoryView.value = value || ""; });
const searchKeyword = ref("");
const activeSearch = computed(() => props.search ?? searchKeyword.value);
const createProfileOpen = ref(false);
const createProfileSaving = ref(false);
const createProfileForm = reactive({ key: "", value: "", memoryType: "personal_info" });
const lifecycleOpen = ref(false);
const lifecycleSaving = ref(false);
const lifecycleForm = reactive({ retentionLevel: 3, strength: 68, pinned: false });
let lifecycleRow: any;
function openLifecycle(row: any) {
  lifecycleRow = JSON.parse(JSON.stringify(row));
  Object.assign(lifecycleForm, { retentionLevel: (row.body.effectiveRetentionLevel ?? row.body.retentionLevel) || 3, strength: Math.round(((row.body.effectiveStrength ?? row.body.memoryStrength) ?? .68) * 100), pinned: row.body.pinned === true });
  lifecycleOpen.value = true;
}
async function saveLifecycle() {
  const original = lifecycleRow;
  if (!original || loading.value || lifecycleSaving.value) return;
  lifecycleSaving.value = true;
  try {
    await owned.edit("memory", original.id, { retentionLevel: lifecycleForm.pinned ? 1 : lifecycleForm.retentionLevel, memoryStrength: lifecycleForm.pinned || lifecycleForm.retentionLevel === 1 ? 1 : lifecycleForm.strength / 100, pinned: lifecycleForm.pinned }, { characterId: original.executionScope.roleId, expectedExecutionScope: original.executionScope, expectedOwnerId: original.ownerId, expectedRevision: original.revision });
    lifecycleOpen.value = false;
    await load();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "保留设置保存失败"; }
  finally { lifecycleSaving.value = false; }
}

const viewOptions = computed(() => ownedMemoryViewOptions(source.value, owned.roles.value, historicalRoles.value, kind.value));

watch(source, (value) => {
  if (value !== "current") memoryView.value = "";
  else if (!memoryView.value) memoryView.value = "all";
});

const displayRows = computed(() => {
  const search = activeSearch.value.trim().toLowerCase();
  if (!search) return rows.value;
  return rows.value.filter((row) => {
    const text = [ownedMemoryTitle(row), ...ownedMemoryFields(row).map((field) => `${field.label}${field.value}`)].join(" ").toLowerCase();
    return text.includes(search);
  });
});
const transferOpen = ref(false);
const transferring = ref(false);
const transferTarget = ref<"global" | "character">("global");
const transferRole = ref("");
const transferRecord = ref<any>();
const graphTransferId = ref("");

function openTransfer(row: any) {
  if (!row || transferring.value) return;
  transferRecord.value = JSON.parse(JSON.stringify(row));
  transferTarget.value = row.roleId === GLOBAL_MEMORY_ROLE ? "character" : "global";
  transferRole.value = props.characterId;
  transferOpen.value = true;
}
async function confirmTransfer() {
  if (!transferRecord.value || transferring.value) return;
  const record = transferRecord.value;
  transferring.value = true;
  try { await transferMemoryScope(record, transferTarget.value, transferRole.value); transferOpen.value = false; await load(); }
  catch (cause) { error.value = cause instanceof Error ? cause.message : "分区迁移失败"; }
  finally { transferring.value = false; }
}
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
let authorityInvalidated = false;
let providerTimer: ReturnType<typeof setInterval> | undefined;

function content(row: any): string {
  return ownedMemoryEditableText(row);
}

const view = computed(() => {
  if (props.mode) return props.mode;
  const current = props.fixedKind ? props.initialKind || "memory" : kind.value;
  return ["graph", "profile", "episodic", "timeline"].includes(current) ? current : "list";
});

const timelineLayerOptions = [
  { id: "memory", label: "原始记忆" },
  { id: "working", label: "工作记忆" },
  { id: "episodic", label: "情景记忆" },
  { id: "fact", label: "事实记忆" },
  { id: "profile", label: "用户画像" },
  { id: "graph", label: "关系图谱" },
  { id: "summary", label: "对话摘要" },
];
const timelineLayers = ref<string[]>([]);

function timelineLayerLabel(kindValue: string) {
  return timelineLayerOptions.find((layer) => layer.id === kindValue)?.label || kindValue || "记忆";
}

function timelineStamp(row: any): number {
  const parsed = Date.parse(String(row?.updatedAt || ""));
  return Number.isFinite(parsed) ? parsed : 0;
}

function timelineTime(row: any) {
  const value = String(row?.updatedAt || "");
  if (!value) return "时间未知";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value : parsed.toLocaleString("zh-CN");
}

const timelineRows = computed(() => {
  const search = activeSearch.value.trim().toLowerCase();
  return rows.value
    .filter((row) => !row.deleted)
    .filter((row) => timelineLayers.value.length === 0 || timelineLayers.value.includes(row.kind))
    .filter((row) => {
      if (!search) return true;
      const text = [ownedMemoryTitle(row), ...ownedMemoryFields(row).map((field) => `${field.label}${field.value}`)].join(" ").toLowerCase();
      return text.includes(search);
    })
    .slice()
    .sort((left, right) => timelineStamp(right) - timelineStamp(left));
});

function exportTimelineCSV() {
  const cell = (value: unknown) => `"${String(value ?? "").replace(/"/g, '""')}"`;
  const lines = [["时间", "记忆层", "分区", "标题", "正文", "版本"].map(cell).join(",")];
  for (const row of timelineRows.value) {
    const body = ownedMemoryFields(row).map((field) => `${field.label}：${field.value}`).join(" | ");
    lines.push([timelineTime(row), timelineLayerLabel(row.kind), row.roleId === GLOBAL_MEMORY_ROLE ? "全局记忆" : "角色记忆", ownedMemoryTitle(row), body, row.revision ?? ""].map(cell).join(","));
  }
  const blob = new Blob([`\ufeff${lines.join("\r\n")}`], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `记忆时间线-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-")}.csv`;
  anchor.click();
  URL.revokeObjectURL(url);
}

const loadKinds = computed(() => (props.kinds && props.kinds.length > 0 ? props.kinds : [kind.value]));

async function loadAggregate() {
  const ticket = ++generation;
  rows.value = [];
  next.value = "";
  nextLegacy.value = "";
  error.value = "";
  loading.value = true;
  try {
    if (historicalRoles.value.length === 0) {
      historicalRoles.value = await owned.historicalRoles(props.characterId);
      if (ticket !== generation || !props.visible) return;
    }
    const historical = source.value.startsWith("history:");
    const collected: any[] = [];
    for (const currentKind of loadKinds.value) {
      const result = await owned.data(currentKind, props.characterId, historical ? "" : props.conversationId, "", historical ? { historicalRoleId: source.value.slice(8) } : { ...(memoryView.value ? { memoryView: memoryView.value } : {}) });
      if (ticket !== generation || !props.visible) return;
      const snapshot = historical ? result.historicalSnapshot : result.snapshot;
      if (!snapshot) continue;
      for (const row of snapshot.resources.filter((item: any) => item.kind === currentKind)) {
        if (row.ownerId !== snapshot.ownerId || !row.roleId) continue;
        if (!result.executionScope.memoryViewAll && row.roleId !== (historical ? source.value.slice(8) : memoryStorageRole(result.executionScope, currentKind))) continue;
        const recordScope = result.executionScope.memoryViewAll ? memoryRecordScope(result.executionScope, row.roleId) : result.executionScope;
        collected.push({ ...row, executionScope: recordScope });
      }
      owner.value = snapshot.ownerId;
      loadedScope.value = result.executionScope;
    }
    rows.value = collected;
  } catch (cause) {
    if (ticket === generation) error.value = cause instanceof Error ? cause.message : "记忆加载失败";
  } finally {
    if (ticket === generation) loading.value = false;
  }
}

async function createProfile(payload: { key: string; value: string; memoryType: string }) {
  const scope = loadedScope.value;
  if (!scope || scope.memoryViewAll) { error.value = "请先选择具体记忆分区后再新增画像"; return; }
  try {
    const api = useOwnedMemoryManagement();
    await api.write(scope, crypto.randomUUID(), { action: "create", key: payload.key, value: payload.value, memoryType: payload.memoryType, importance: 5, partition: "profile" });
    await load();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "画像保存失败"; }
}

async function load(older = false) {
  if (props.kinds && props.kinds.length > 0) return loadAggregate();
  authorityInvalidated = false;
  const ticket = ++generation;
  const currentKind = kind.value;
  if (!older) { projection.value = null; projectionNotice.value = ""; }
  if (!older) loadedScope.value = undefined;
  if (!older) { rows.value = []; next.value = ""; nextLegacy.value = ""; scope = ""; }
  error.value = "";
  loading.value = true;
  try {
    if (historicalRoles.value.length === 0) {
      const catalog = await owned.historicalRoles(props.characterId);
      if (ticket !== generation || !props.visible) return;
      historicalRoles.value = catalog;
    }
    const historical = source.value.startsWith("history:");
	    const result = await owned.data(currentKind, props.characterId, historical ? "" : props.conversationId, !historical && older ? next.value : "", historical ? { historicalRoleId: source.value.slice(8), historicalCursor: older ? next.value : "", historicalLegacyCursor: older ? nextLegacy.value : "" } : { legacyCursor: older ? nextLegacy.value : "", ...(memoryView.value ? { memoryView: memoryView.value } : {}) });
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
      if (row.ownerId !== snapshot.ownerId || !row.roleId || !result.executionScope.memoryViewAll && row.roleId !== (historical ? source.value.slice(8) : memoryStorageRole(result.executionScope, currentKind))) throw new Error("记忆所属设备或角色无效，请重新加载");
      const recordScope = result.executionScope.memoryViewAll ? memoryRecordScope(result.executionScope, row.roleId) : result.executionScope;
      merged.set(`${row.ownerId}/${row.kind}/${row.id}`, { ...row, executionScope: recordScope });
    }
    rows.value = Array.from(merged.values());
    const cursor = snapshot.nextCursors?.[currentKind] || "";
    if (older && cursor && cursor === next.value) throw new Error("分页游标重复，请刷新后重试");
    next.value = cursor;
    nextLegacy.value = "";
    if (source.value === "current" && !result.executionScope.memoryViewAll && ["vector", "graph"].includes(currentKind)) {
      try {
        const value = await owned.projections(props.characterId, undefined, memoryView.value || undefined);
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

async function submitCreateProfile() {
  const scope = loadedScope.value;
  const key = createProfileForm.key.trim();
  const value = createProfileForm.value.trim();
  if (!key || !value) return;
  if (!scope) { error.value = "记忆服务尚未就绪"; return; }
  createProfileSaving.value = true;
  try {
    const api = useOwnedMemoryManagement();
    await api.write(scope, crypto.randomUUID(), {
      action: "create",
      key,
      value,
      memoryType: createProfileForm.memoryType,
      importance: 5,
      partition: "profile",
    });
    createProfileOpen.value = false;
    createProfileForm.key = "";
    createProfileForm.value = "";
    await load();
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "画像保存失败";
  } finally {
    createProfileSaving.value = false;
  }
}

async function setUse(row: any) {
  try {
    await owned.edit("memory", row.id, { allowContextUse: row.body.allowContextUse === false }, { characterId: row.executionScope.roleId, expectedExecutionScope: row.executionScope, expectedOwnerId: row.ownerId, expectedRevision: row.revision });
    await load();
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "修改失败"; }
}

async function setProactiveMention(row: any) {
  try {
    await owned.edit("memory", row.id, { allowProactiveMention: row.body.allowProactiveMention === false }, { characterId: row.executionScope.roleId, expectedExecutionScope: row.executionScope, expectedOwnerId: row.ownerId, expectedRevision: row.revision });
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
  authorityInvalidated = true;
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
function reloadScope() { if (props.visible && props.characterId) void load(); }
watch([() => props.characterId, owned.coreId, () => owned.policy.value?.providerEpoch, () => owned.policy.value?.modeRevision, () => owned.policy.value?.permissionRevision], clearAuthority, { flush: "sync" });
watch([() => props.visible, () => props.characterId, () => props.conversationId, kind, source, memoryView, owned.coreId, () => owned.policy.value?.providerEpoch, () => owned.policy.value?.modeRevision, () => owned.policy.value?.permissionRevision], () => {
  generation++;
  loading.value = false;
  if (props.visible && props.characterId) void load();
}, { immediate: true });
onMounted(() => {
  window.addEventListener("amitia:runtime-connection-changed", clearAuthority);
  window.addEventListener("amitia:execution-scope-changed", clearAuthority);
  window.addEventListener("amitia:memory-scope-changed", reloadScope);
  providerTimer = setInterval(() => {
    if (!props.visible) return;
    void owned.refresh().then((ready) => {
      if (ready && props.visible && props.characterId && authorityInvalidated && !loading.value) void load();
    }).catch(clearAuthority);
  }, 3000);
});
onUnmounted(() => {
  if (providerTimer) clearInterval(providerTimer);
  window.removeEventListener("amitia:runtime-connection-changed", clearAuthority);
  window.removeEventListener("amitia:execution-scope-changed", clearAuthority);
  window.removeEventListener("amitia:memory-scope-changed", reloadScope);
  clearAuthority();
});
</script>

<style scoped>
.memory-controls { margin-bottom: 20px; border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); background: var(--ac-color-surface); }
.memory-filters { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 240px), 1fr)); gap: 16px; padding: 16px; }
.filter-field { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.filter-field label { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.5; }
.layer-select { width: 100%; }
.ownership-info { padding: 12px 16px; }
.memory-filters + .ownership-info { border-top: 1px solid var(--ac-color-border); }
.ownership-description { margin: 0 0 8px; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.6; }
.ownership-details { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: 8px 24px; margin: 0; }
.ownership-item { display: flex; align-items: baseline; gap: 8px; min-width: 0; font-size: var(--ac-font-size-xs); line-height: 1.6; }
.ownership-item dt { flex-shrink: 0; color: var(--ac-color-text-secondary); }
.ownership-item dd { min-width: 0; margin: 0; color: var(--ac-color-text-primary); }
.identity-value { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-variant-numeric: tabular-nums; }
.identity-value:focus-visible { outline: 2px solid var(--ac-color-primary); outline-offset: 2px; border-radius: 2px; }
@media (max-width: 640px) {
  .layer-select :deep(.el-select__wrapper) { min-height: 44px; }
}
.owned-memory-card { padding: 14px; margin-bottom: 10px; border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); }
.card-title { font-weight: 500; overflow-wrap: anywhere; }
.record-text { white-space: pre-wrap; overflow-wrap: anywhere; margin: 10px 0; }
.record-field { display: flex; gap: 8px; align-items: baseline; min-width: 0; }
.record-field-label { flex: 0 0 auto; color: var(--ac-color-text-secondary); font-size: 12px; }
.record-field-value { flex: 1 1 auto; min-width: 0; overflow-wrap: anywhere; }

.timeline-view { min-width: 0; }
.page-filters { display: flex; align-items: center; flex-wrap: wrap; padding: 0; margin-bottom: 16px; }
.page-filters .filter-field { flex: 1 1 280px; max-width: 480px; }
.graph-transfer-select { width: min(100%, 320px); }
.timeline-toolbar { display: flex; align-items: center; gap: 10px; flex-wrap: wrap; margin-bottom: 16px; }
.tl-search { width: 220px; max-width: 100%; }
.tl-layers { width: 260px; max-width: 100%; }
.tl-count { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); }
.timeline { position: relative; padding-left: 24px; }
.timeline::before { content: ""; position: absolute; left: 7px; top: 0; bottom: 0; width: 2px; background: var(--ac-color-border); }
.tl-item { position: relative; margin-bottom: 16px; }
.tl-dot {
  position: absolute; left: -20px; top: 14px; width: 12px; height: 12px;
  border-radius: 50%; border: 2px solid var(--ac-color-bg); z-index: 1;
  background: var(--ac-color-primary);
}
.tl-dot.layer-working { background: var(--ac-color-info); }
.tl-dot.layer-episodic { background: var(--ac-color-success); }
.tl-dot.layer-fact { background: var(--ac-color-warning); }
.tl-dot.layer-graph { background: var(--ac-color-danger); }
.tl-dot.layer-summary { background: var(--ac-color-text-muted); }
.tl-card {
  background: var(--ac-color-surface); border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md); padding: 12px 14px;
}
.tl-header { display: flex; align-items: center; gap: 8px; margin-bottom: 6px; flex-wrap: wrap; }
.tl-time { font-size: 12px; color: var(--ac-color-text-muted); }
.tl-role { font-size: 12px; color: var(--ac-color-text-secondary); margin-left: auto; }
.record-state, .record-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
</style>
