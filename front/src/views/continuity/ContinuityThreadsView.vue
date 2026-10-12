<template>
  <div class="continuity-page">
    <div class="page-header">
      <div>
        <h2>持续事项</h2>
        <p>跨会话跟进目标、进展、下一步和等待条件。</p>
      </div>
      <div class="header-actions">
        <el-button type="primary" :disabled="source !== 'current'" @click="openCreateDialog">新建事项</el-button>
      </div>
    </div>

    <div class="toolbar" :class="{ 'has-source': mesh.coordinated.value }">
      <div v-if="mesh.coordinated.value" class="toolbar-field">
        <label :for="`${controlId}-source`">数据来源</label>
        <el-select :id="`${controlId}-source`" v-model="source" placeholder="事项数据来源" aria-label="事项数据来源" @change="refresh">
          <el-option label="当前数据所有者" value="current" />
          <el-option v-for="role in historicalRoles" :key="role.id" :label="`原设备历史 · ${role.name}`" :value="`history:${role.id}`" />
        </el-select>
      </div>
      <div class="toolbar-field search-field">
        <label :for="`${controlId}-search`">搜索事项</label>
        <el-input :id="`${controlId}-search`" v-model="query" clearable placeholder="搜索标题、目标或当前状态" aria-label="搜索事项" @keyup.enter="refresh" />
      </div>
      <div v-if="mesh.enabled.value" class="toolbar-field">
        <label :for="`${controlId}-scope`">查看范围</label>
        <el-select :id="`${controlId}-scope`" v-model="memoryView" aria-label="事项查看范围" @change="refresh">
          <el-option label="全部事项" value="all" /><el-option label="全局事项" value="global" />
          <el-option v-for="role in mesh.roles.value" :key="role.id" :label="role.name" :value="`role:${role.id}`" />
        </el-select>
      </div>
      <div class="toolbar-field">
        <label :for="`${controlId}-status`">状态</label>
        <el-select :id="`${controlId}-status`" v-model="statusFilter" clearable placeholder="全部状态" aria-label="事项状态" @change="refresh">
          <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
      </div>
      <el-button class="refresh-button" :loading="loading" @click="refresh">刷新</el-button>
    </div>
    <el-alert v-if="source !== 'current'" class="page-notice" title="旧设备持续事项仅供读取，不会在当前 Core 自动恢复执行，请在原设备管理。" type="info" :closable="false" />
    <el-alert v-if="mesh.notice.value" class="page-notice" :title="mesh.notice.value" type="warning" :closable="false" />

    <el-empty v-if="!loading && threads.length === 0" description="暂无持续事项" />
    <div v-else class="thread-grid">
      <button
        v-for="thread in threads"
        :key="`${thread.ownedDocument?.ownerId || ''}/${thread.id}`"
        type="button"
        class="thread-card"
        :class="{ active: selectedId === thread.id }"
        @click="selectThread(thread)"
      >
        <div class="thread-card-top">
          <strong>{{ thread.title }}</strong>
          <el-tag size="small" :type="tagType(thread.status)">{{ statusLabel(thread.status) }}</el-tag>
        </div>
        <div class="thread-state">{{ thread.currentState || thread.summary || thread.goal || "暂无状态摘要" }}</div>
        <div v-if="thread.nextAction" class="next-action">下一步：{{ thread.nextAction }}</div>
        <div class="thread-time">最近活动 {{ formatTime(thread.lastActiveAt) }}</div>
      </button>
    </div>

    <el-drawer v-model="detailOpen" size="min(720px, 92vw)" :title="detail?.thread.title || '持续事项'">
      <template v-if="detail">
        <div v-if="source === 'current' && detail.executionScope" class="detail-actions">
          <el-button :disabled="detail.lease?.state === 'running' || detail.lease?.state === 'unknown'" @click="openScopeTransfer">{{ detail.memoryPartition === GLOBAL_MEMORY_ROLE ? '降级为角色事项' : '提升为全局事项' }}</el-button>
        </div>
	    <el-alert v-if="detail.pausedReason" :title="detail.pausedReason" type="warning" :closable="false" show-icon />
        <div v-if="!detail.readOnly" class="detail-actions">
          <el-button v-if="detail.thread.status !== 'paused' && !terminal(detail.thread.status)" @click="setStatus('paused')">暂停</el-button>
          <el-button v-if="detail.thread.status === 'paused'" type="primary" :disabled="detail.lease?.state === 'unknown'" @click="setStatus('active')">恢复</el-button>
          <el-button v-if="detail.lease?.state === 'unknown'" :loading="saving" @click="confirmExecution('completed')">确认执行已完成</el-button>
          <el-button v-if="detail.lease?.state === 'unknown'" :loading="saving" type="warning" @click="confirmExecution('abandoned')">放弃本次执行</el-button>
          <el-button v-if="!terminal(detail.thread.status)" type="success" @click="setStatus('completed')">完成</el-button>
          <el-button @click="waitDialog = true" :disabled="terminal(detail.thread.status)">添加等待条件</el-button>
        </div>

        <el-descriptions :column="1" border>
          <el-descriptions-item label="目标">{{ detail.thread.goal || "—" }}</el-descriptions-item>
          <el-descriptions-item label="当前状态">{{ detail.thread.currentState || "—" }}</el-descriptions-item>
          <el-descriptions-item label="下一步">{{ detail.thread.nextAction || "—" }}</el-descriptions-item>
          <el-descriptions-item label="摘要">{{ detail.thread.summary || "—" }}</el-descriptions-item>
        </el-descriptions>

        <section class="detail-section">
          <h3>等待条件</h3>
          <el-empty v-if="detail.waits.length === 0" :image-size="64" description="没有等待条件" />
          <el-table v-else :data="detail.waits" size="small">
            <el-table-column prop="waitType" label="类型" width="110" />
            <el-table-column prop="description" label="说明" min-width="220" />
            <el-table-column label="状态" width="110">
              <template #default="scope"><el-tag size="small">{{ scope.row.status }}</el-tag></template>
            </el-table-column>
            <el-table-column label="唤醒" width="110">
              <template #default="scope">{{ scope.row.wakeState || "—" }}</template>
            </el-table-column>
            <el-table-column label="操作" width="110">
              <template #default="scope">
                <el-button v-if="!detail.readOnly && scope.row.status === 'waiting'" link type="primary" @click="resolveWait(scope.row)">解除</el-button>
                <el-button v-if="!detail.readOnly && scope.row.status === 'waiting'" link type="danger" @click="cancelWait(scope.row)">取消</el-button>
              </template>
            </el-table-column>
          </el-table>
        </section>

        <section class="detail-section">
          <h3>最近事件</h3>
          <el-timeline v-if="detail.events.length">
            <el-timeline-item v-for="event in detail.events" :key="event.id" :timestamp="formatTime(event.occurredAt)">
              <strong>{{ event.eventType }}</strong>
              <div class="event-summary">{{ eventSummary(event.payloadJson) }}</div>
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else :image-size="64" description="暂无事件" />
        </section>
      </template>
    </el-drawer>

    <el-dialog v-model="createDialog" title="新建持续事项" width="520px">
      <el-form label-position="top">
        <el-form-item label="标题"><el-input v-model="createForm.title" maxlength="120" /></el-form-item>
        <el-form-item label="目标"><el-input v-model="createForm.goal" type="textarea" :rows="3" /></el-form-item>
        <el-form-item label="下一步"><el-input v-model="createForm.nextAction" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="createThread">创建</el-button>
      </template>
    </el-dialog>
    <el-dialog v-model="scopeDialog" title="调整事项所属分区" width="min(480px, 92vw)" :close-on-click-modal="!scopeSaving" :show-close="!scopeSaving">
      <p>事项、等待条件和事件一起迁移，执行角色与权限保持原样。</p>
      <el-select v-model="scopeTarget" aria-label="目标事项分区" :disabled="scopeSaving"><el-option label="全局事项" value="global" /><el-option label="角色事项" value="character" /></el-select>
      <el-select v-if="scopeTarget === 'character'" v-model="scopeRole" filterable aria-label="目标事项角色" :disabled="scopeSaving"><el-option v-for="role in mesh.roles.value" :key="role.id" :label="role.name" :value="role.id" /></el-select>
      <template #footer><el-button :disabled="scopeSaving" @click="scopeDialog = false">取消</el-button><el-button type="primary" :loading="scopeSaving" :disabled="scopeTarget === 'character' && !scopeRole" @click="confirmScopeTransfer">确认迁移</el-button></template>
    </el-dialog>

    <el-dialog v-model="waitDialog" title="添加等待条件" width="560px">
      <el-form label-position="top">
        <el-form-item label="类型">
          <el-select v-model="waitForm.type">
            <el-option label="等待用户" value="user" />
            <el-option label="等待时间" value="time" />
            <el-option label="等待设备" value="device" />
            <el-option label="等待外部事件" value="external" />
            <el-option label="等待审批" value="approval" />
            <el-option label="等待依赖" value="dependency" />
          </el-select>
        </el-form-item>
        <el-form-item label="说明"><el-input v-model="waitForm.description" /></el-form-item>
        <el-form-item v-if="waitForm.type === 'time'" label="到期时间">
          <el-date-picker v-model="waitForm.dueAt" type="datetime" value-format="YYYY-MM-DDTHH:mm:ssZ" />
        </el-form-item>
        <el-form-item v-if="!['user', 'time'].includes(waitForm.type)" label="匹配条件 JSON">
          <el-input v-model="waitForm.conditionJson" type="textarea" :rows="4" placeholder='例如 {"deviceId":"..."}' />
        </el-form-item>
        <el-form-item label="恢复提示"><el-input v-model="waitForm.resumeHint" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="waitDialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="addWait">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref, watch, useId } from "vue";
import type { OwnedExecutionScope } from "@/runtime/device-owned-chat";
import { ElMessage, ElMessageBox } from "element-plus";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { GLOBAL_MEMORY_ROLE } from "@/runtime/owned-memory-scope";
import { transferMemoryScope } from "@/composables/useMemoryScopeTransfer";
import {
  cancelContinuityWait,
  confirmContinuityExecution,
  createContinuityThread,
  createContinuityWait,
  getContinuityThread,
  listContinuityThreads,
  resolveContinuityWait,
  updateContinuityThread,
  type ContinuityDetail,
  type ContinuityEvent,
  type ContinuityThread,
  type ContinuityWait,
  type ThreadStatus,
  type OwnedContinuityDocument,
} from "./api";

const loading = ref(false);
const mesh = useDeviceOwnedConversation();
const controlId = useId();
const characterId = ref("");
const saving = ref(false);
const threads = ref<ContinuityThread[]>([]);
const selectedId = ref("");
const detail = ref<ContinuityDetail | null>(null);
const detailIntent = ref<OwnedContinuityDocument>();
const createIntent = ref<OwnedExecutionScope>();
const source = ref("current");
const memoryView = ref("all");
const scopeDialog = ref(false);
const scopeSaving = ref(false);
const scopeTarget = ref<"global" | "character">("global");
const scopeRole = ref("");
const scopeRecord = ref<any>();
function openScopeTransfer() {
  if (!detail.value?.executionScope || scopeSaving.value) return;
  scopeRecord.value = JSON.parse(JSON.stringify({ kind: "continuity", id: detail.value.thread.id, revision: detail.value.thread.revision, ownerId: detail.value.ownerId, executionScope: detail.value.executionScope }));
  scopeTarget.value = detail.value.memoryPartition === GLOBAL_MEMORY_ROLE ? "character" : "global";
  scopeRole.value = characterId.value;
  scopeDialog.value = true;
}
async function confirmScopeTransfer() {
  if (!scopeRecord.value || scopeSaving.value) return;
  scopeSaving.value = true;
  try { await transferMemoryScope(scopeRecord.value, scopeTarget.value, scopeRole.value); scopeDialog.value = false; detailOpen.value = false; await refresh(); }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : "事项分区迁移失败"); }
  finally { scopeSaving.value = false; }
}
const historicalRoles = ref<Array<{ id: string; name: string }>>([]);
let generation = 0;
let providerTimer: ReturnType<typeof setInterval> | undefined;
const detailOpen = ref(false);
const query = ref("");
const statusFilter = ref<ThreadStatus | "">("");
const createDialog = ref(false);
const waitDialog = ref(false);
const createForm = reactive({ title: "", goal: "", nextAction: "" });
const waitForm = reactive({ type: "user", description: "", dueAt: "", conditionJson: "{}", resumeHint: "" });

async function confirmExecution(outcome: "completed" | "abandoned") {
  const current = detail.value;
  const original = detailIntent.value;
  if (!current?.lease) return;
  let result = "";
  try {
    if (outcome === "completed") {
      const input = await ElMessageBox.prompt("请核实原设备的执行结果，填写结果摘要。确认后事项保持暂停，不会重新执行本次等待。", "确认执行已完成", { inputType: "textarea", inputValidator: (value) => !!value.trim() || "请填写结果摘要" });
      result = input.value;
    } else await ElMessageBox.confirm("请先核实原设备状态。放弃后不会重复执行本次等待，已发生的操作仍然保留，事项保持暂停。", "放弃本次执行", { type: "warning" });
  } catch { return; }
  saving.value = true;
  try { await confirmContinuityExecution(current.thread.id, current.lease.id, outcome, result, original); await reloadDetail(); }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : "执行确认失败"); }
  finally { saving.value = false; }
}

const statusOptions: Array<{ label: string; value: ThreadStatus }> = [
  { label: "进行中", value: "active" }, { label: "等待中", value: "waiting" }, { label: "已阻塞", value: "blocked" },
  { label: "已暂停", value: "paused" }, { label: "已完成", value: "completed" }, { label: "已取消", value: "cancelled" },
];

function statusLabel(status: ThreadStatus) { return statusOptions.find((item) => item.value === status)?.label || status; }
function tagType(status: ThreadStatus) { return status === "completed" ? "success" : status === "blocked" ? "danger" : status === "waiting" ? "warning" : status === "paused" ? "info" : "primary"; }
function terminal(status: ThreadStatus) { return status === "completed" || status === "cancelled"; }
function formatTime(value?: string) { if (!value) return "—"; return new Date(value).toLocaleString(); }
function eventSummary(raw: string) { try { const data = JSON.parse(raw || "{}"); return String(data.summary || data.error || ""); } catch { return ""; } }

async function refresh() {
  let ticket = ++generation;
  loading.value = true;
  try {
    if (await mesh.refresh()) {
      const selected = memoryView.value.startsWith("role:") ? memoryView.value.slice(5) : "";
      if (selected && mesh.roles.value.some(role => role.id === selected)) characterId.value = selected;
      else if (!mesh.roles.value.some((role) => role.id === characterId.value)) characterId.value = mesh.selectInitialRole() || mesh.roles.value[0]?.id || "";
      if (!characterId.value) { threads.value = []; return; }
    }
    ticket = ++generation;
    loading.value = true;
    const roles = mesh.coordinated.value ? await mesh.historicalRoles(characterId.value) : [];
    const items = await listContinuityThreads({ characterId: characterId.value || undefined, memoryView: memoryView.value || undefined, historicalRoleId: source.value.startsWith("history:") ? source.value.slice(8) : undefined, q: query.value || undefined, status: statusFilter.value || undefined, limit: 100 });
    if (ticket === generation) { historicalRoles.value = roles; threads.value = items; }
  } catch (error) {
    if (ticket === generation) { threads.value = []; ElMessage.error(error instanceof Error ? error.message : "持续事项加载失败"); }
  } finally { if (ticket === generation) loading.value = false; }
}

async function selectThread(thread: ContinuityThread) {
  const ticket = generation;
  try {
    const result = await getContinuityThread(thread.id, thread.ownedDocument);
    if (ticket !== generation) return;
    selectedId.value = thread.id;
    detail.value = result;
    detailIntent.value = result.executionScope ? result as OwnedContinuityDocument : undefined;
    detailOpen.value = true;
  } catch (error) { if (ticket === generation) ElMessage.error(error instanceof Error ? error.message : "持续事项详情加载失败"); }
}

async function reloadDetail() {
  if (!selectedId.value) return;
  const ticket = generation;
  const result = await getContinuityThread(selectedId.value, detailIntent.value);
  if (ticket !== generation) return;
  detail.value = result;
  detailIntent.value = result.executionScope ? result as OwnedContinuityDocument : undefined;
  await refresh();
}

async function setStatus(status: ThreadStatus) {
  if (!detail.value) return;
  saving.value = true;
  try { await updateContinuityThread(detail.value.thread.id, { status }, detailIntent.value); await reloadDetail(); }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : "持续事项状态修改失败"); }
  finally { saving.value = false; }
}

async function resolveWait(wait: ContinuityWait) {
  if (!detail.value) return;
  try { await resolveContinuityWait(detail.value.thread.id, wait.id, wait.autoResume, detailIntent.value); await reloadDetail(); }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : "等待条件解除失败"); }
}

async function cancelWait(wait: ContinuityWait) {
  if (!detail.value) return;
  const current = detailIntent.value;
  const threadId = detail.value.thread.id;
  try {
    await ElMessageBox.confirm("取消后不会触发自动恢复。", "取消等待条件", { type: "warning" });
  } catch {
    return;
  }
  try { await cancelContinuityWait(threadId, wait.id, current); await reloadDetail(); }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : "等待条件取消失败"); }
}

async function createThread() {
  if (!createForm.title.trim()) { ElMessage.warning("请输入标题"); return; }
  saving.value = true;
  try {
    const item = await createContinuityThread({ ...createForm, characterId: characterId.value || undefined }, createIntent.value);
    createDialog.value = false;
    createForm.title = ""; createForm.goal = ""; createForm.nextAction = "";
    await refresh();
    await selectThread(item);
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : "持续事项创建失败"); }
  finally { saving.value = false; }
}

async function addWait() {
  if (!detail.value) return;
  let condition: Record<string, unknown> = {};
  try { condition = JSON.parse(waitForm.conditionJson || "{}"); }
  catch { ElMessage.warning("匹配条件不是有效 JSON"); return; }
  saving.value = true;
  try {
    await createContinuityWait(detail.value.thread.id, {
      type: waitForm.type, description: waitForm.description, condition,
      dueAt: waitForm.type === "time" ? waitForm.dueAt || undefined : undefined,
      resumeHint: waitForm.resumeHint,
    }, detailIntent.value);
    waitDialog.value = false;
    waitForm.description = ""; waitForm.dueAt = ""; waitForm.conditionJson = "{}"; waitForm.resumeHint = "";
    await reloadDetail();
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : "等待条件保存失败"); }
  finally { saving.value = false; }
}

async function openCreateDialog() {
  const ticket = generation;
  createIntent.value = undefined;
  try {
    if (await mesh.refresh()) {
      const result = await mesh.data("continuity", characterId.value, "", "", memoryView.value ? { memoryView: memoryView.value } : {});
      if (ticket !== generation) return;
      const prepared = { ...result.executionScope };
      delete prepared.memoryViewAll;
      createIntent.value = prepared;
    }
    if (ticket === generation) createDialog.value = true;
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : "持续事项数据归属加载失败"); }
}
function invalidate() {
  generation++;
  threads.value = [];
  detail.value = null;
  detailIntent.value = undefined;
  createIntent.value = undefined;
  detailOpen.value = false;
  createDialog.value = false;
  waitDialog.value = false;
  selectedId.value = "";
  loading.value = false;
}
watch([characterId, source, memoryView, mesh.coreId, () => mesh.policy.value?.providerEpoch, () => mesh.policy.value?.modeRevision, () => mesh.policy.value?.permissionRevision], invalidate, { flush: "sync" });
onMounted(() => {
  void refresh();
  window.addEventListener("amitia:runtime-connection-changed", invalidate);
  window.addEventListener("amitia:execution-scope-changed", invalidate);
  window.addEventListener("amitia:memory-scope-changed", refresh);
  providerTimer = setInterval(() => { void mesh.refresh().catch(invalidate); }, 3000);
});
onUnmounted(() => {
  if (providerTimer) clearInterval(providerTimer);
  window.removeEventListener("amitia:runtime-connection-changed", invalidate);
  window.removeEventListener("amitia:execution-scope-changed", invalidate);
  window.removeEventListener("amitia:memory-scope-changed", refresh);
  invalidate();
});
</script>

<style scoped>
.continuity-page { max-width: 1280px; min-width: 0; margin: 0 auto; }
.page-header { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 16px; align-items: center; margin-bottom: 20px; }
.page-header h2 { margin: 0 0 6px; font-size: var(--ac-font-size-xl); font-weight: 600; line-height: 1.5; }
.page-header p { margin: 0; color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.6; }
.header-actions { display: flex; align-items: flex-end; gap: 12px; min-width: 0; max-width: 100%; }
.role-field { width: 240px; }
.toolbar, .toolbar.has-source { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 180px), 1fr)); align-items: end; gap: 16px; padding: 16px; margin-bottom: 20px; border: 1px solid var(--ac-color-border); border-radius: var(--ac-radius-md); background: var(--ac-color-surface); }
.toolbar-field { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.toolbar-field label { color: var(--ac-color-text-secondary); font-size: var(--ac-font-size-sm); line-height: 1.5; }
.toolbar-field :deep(.el-select), .toolbar-field :deep(.el-input) { width: 100%; min-width: 0; }
.header-actions > .el-button, .refresh-button { margin-left: 0; }
.page-notice { margin-bottom: 16px; }
.thread-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 12px; }
.thread-card { appearance: none; text-align: left; border: 1px solid var(--el-border-color); background: var(--el-bg-color); border-radius: 10px; padding: 16px; cursor: pointer; color: inherit; }
.thread-card:hover, .thread-card.active { border-color: var(--el-color-primary); }
.thread-card-top { display: flex; justify-content: space-between; gap: 12px; align-items: center; }
.thread-state { margin-top: 12px; line-height: 1.55; min-height: 44px; color: var(--el-text-color-regular); }
.next-action { margin-top: 10px; font-size: 13px; color: var(--el-text-color-secondary); }
.thread-time { margin-top: 12px; font-size: 12px; color: var(--el-text-color-placeholder); }
.detail-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 16px; }
.detail-section { margin-top: 24px; }
.detail-section h3 { margin: 0 0 12px; }
.event-summary { margin-top: 4px; color: var(--el-text-color-secondary); line-height: 1.5; }
@media (max-width: 1024px) {
  .toolbar.has-source { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
}
@media (max-width: 720px) {
  .toolbar, .toolbar.has-source { grid-template-columns: minmax(0, 1fr); gap: 12px; }
  .page-header { align-items: stretch; flex-direction: column; }
  .header-actions { align-items: stretch; flex-direction: column; }
  .role-field { width: 100%; }
  .toolbar-field :deep(.el-select__wrapper), .toolbar-field :deep(.el-input__wrapper), .header-actions > .el-button, .refresh-button { min-height: 44px; }
}
</style>
