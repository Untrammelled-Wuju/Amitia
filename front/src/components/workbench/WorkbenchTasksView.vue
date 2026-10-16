<template>
  <section class="task-list" aria-label="多 Agent 任务">
    <header>
      <strong>多 Agent · 任务状态</strong>
      <button type="button" :disabled="busy" @click="load(true)">{{ busy ? '查询中' : '刷新' }}</button>
    </header>
    <p class="note">持久化任务检查点与经过身份校验的 Runtime 快照。子任务成功不代表构建、审批和合并已通过。</p>
    <div class="summary" role="group" aria-label="任务状态汇总">
      <span>总数 <b>{{ summary.total }}</b></span>
      <span>进行中 <b>{{ summary.active }}</b></span>
      <span>成功结束 <b>{{ summary.succeeded }}</b></span>
      <span>失败/取消 <b>{{ summary.failed }}</b></span>
      <span>需协调 <b>{{ summary.needsReconciliation }}</b></span>
    </div>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-else-if="!busy && !tasks.length" class="note">当前对话尚无可读取的多 Agent 任务检查点。</p>
    <article v-for="task in tasks" :key="task.coordinationId" class="task-card">
      <div class="title"><strong>{{ statusName(task.status) }}</strong><small>{{ task.completedCount }}/{{ task.assignmentCount }} 个子任务结束</small></div>
      <small class="meta">{{ task.coordinationId }} · {{ recoveryName(task.recoveryState) }} · {{ formatDate(task.updatedAt) }}</small>
      <p v-if="task.recoveryState === 'needs-reconciliation'" class="warning">{{ task.recoveryReason === 'parent-state-conflict' ? '父任务已结束，但检查点仍显示未结束；需要核查状态。' : '持久化检查点与 Runtime 快照冲突，需人工核查。' }}</p>
      <button type="button" :aria-expanded="expandedId === task.coordinationId" @click="toggleHistory(task.coordinationId)">{{ expandedId === task.coordinationId ? '收起持久化检查点' : '查看持久化检查点' }}</button>
      <div v-if="expandedId === task.coordinationId" class="checkpoint-panel">
        <p class="note">仅展示最近保存的检查点样本，不是完整执行事件日志，也不代表测试已通过。</p>
        <p v-if="historyBusy" class="note">正在读取检查点…</p>
        <p v-else-if="historyError" class="error" role="alert">{{ historyError }}</p>
        <template v-else-if="history">
          <div v-for="(point, index) in history.records" :key="index" class="checkpoint-row">
            <span>{{ formatDate(point.updatedAt) }}</span>
            <span>{{ statusName(point.coordinationStatus) }} · {{ point.completedCount }}/{{ point.assignmentCount }} 已结束</span>
            <strong v-if="!point.consistent" class="warning">父任务状态不一致</strong>
          </div>
          <p v-if="history.truncated" class="warning">检查点样本已截断，不能据此重建完整执行历史。</p>
        </template>
      </div>
      <div v-for="child in task.assignments" :key="child.id" class="child">
        <span class="state">{{ statusName(child.status) }}</span>
        <span class="objective" :title="child.objective">{{ child.objective }}</span>
        <small v-if="child.reviewRequired">须独立审查</small>
      </div>
    </article>
    <p v-if="truncated" class="note">结果达到查询上限，仅显示最近 30 项；较旧任务未纳入上述统计。</p>
    <p v-if="liveObservationTruncated" class="note">实时快照只检查最近 8 个任务，其余任务状态来自持久化检查点。</p>
    <p v-if="summary.runtimeObserved" class="note">{{ summary.runtimeObserved }} 项已匹配实时协调器状态；其余任务仅代表历史快照。</p>
  </section>
</template>
<script setup lang="ts">
import { ref, watch, onActivated, onDeactivated, onBeforeUnmount } from 'vue';
import { readWorkbenchTasks } from './workbench-task-api';
import { useDeviceOwnedConversation } from '@/composables/useDeviceOwnedConversation';
import { matchesWorkbenchTaskChange, workbenchTaskEventName, type WorkbenchTaskChange } from '@/runtime/owned-workbench-task-event';
import { normalizeTaskSummary, type TaskSummary } from './workbench-task-summary';
import { parseWorkbenchTaskList, type WorkbenchTaskItem } from './workbench-task-list';
import { loadTaskCheckpoints, type TaskCheckpointHistory } from './workbench-task-checkpoints';
const props = defineProps<{ conversationId: string; characterId?: string }>();
const owned = useDeviceOwnedConversation();
const tasks = ref<WorkbenchTaskItem[]>([]);
const summary = ref<TaskSummary>(normalizeTaskSummary(null));
const error = ref('');
const busy = ref(false);
const truncated = ref(false);
const liveObservationTruncated = ref(false);
const expandedId = ref('');
const history = ref<TaskCheckpointHistory | null>(null);
const historyError = ref('');
const historyBusy = ref(false);
let historyController: AbortController | undefined;
let historyGeneration = 0;
let historyTaskUpdatedAt = '';
function invalidateHistoryRequest() {
  historyGeneration++;
  historyController?.abort();
  historyController = undefined;
  historyBusy.value = false;
}
function resetHistory() {
  invalidateHistoryRequest();
  expandedId.value = '';
  historyTaskUpdatedAt = '';
  history.value = null;
  historyError.value = '';
}
async function fetchHistory(coordinationId: string, updatedAt: string) {
  invalidateHistoryRequest();
  if (!active || expandedId.value !== coordinationId) return;
  historyTaskUpdatedAt = updatedAt;
  history.value = null;
  historyError.value = '';
  const conversationId = props.conversationId;
  const generationId = historyGeneration;
  const abort = new AbortController();
  historyController = abort;
  historyBusy.value = true;
  try {
    const next = await loadTaskCheckpoints(conversationId, coordinationId, abort.signal, props.characterId || '');
    if (active && generationId === historyGeneration && conversationId === props.conversationId && expandedId.value === coordinationId) history.value = next;
  } catch (cause) {
    if (active && generationId === historyGeneration && conversationId === props.conversationId) historyError.value = (cause as Error)?.message || '检查点读取失败';
  } finally {
    if (generationId === historyGeneration) { historyBusy.value = false; historyController = undefined; }
  }
}
function toggleHistory(coordinationId: string) {
  if (expandedId.value === coordinationId) { resetHistory(); return; }
  resetHistory();
  if (!active) return;
  const task = tasks.value.find(item => item.coordinationId === coordinationId);
  if (!task) return;
  expandedId.value = coordinationId;
  void fetchHistory(coordinationId, task.updatedAt);
}
let generation = 0;
let active = false;
let timer: ReturnType<typeof setInterval> | undefined;
let controller: AbortController | undefined;
function formatDate(raw: string) { const time = Date.parse(raw); return Number.isFinite(time) ? new Date(time).toLocaleString() : '时间未知'; }
function statusName(raw: string) { return ({ planning: '规划中', running: '运行中', waiting: '等待中', aggregating: '结果整理', succeeded: '已结束', failed: '失败', cancelled: '已取消', paused: '已暂停', pending: '待运行' } as Record<string, string>)[raw] ?? raw; }
function recoveryName(raw: string) { return ({ 'runtime-observed': '已确认 Runtime', 'checkpoint-only': '历史检查点', 'needs-reconciliation': '需人工协调' } as Record<string, string>)[raw] ?? '状态未知'; }
function invalidate() { generation++; controller?.abort(); controller = undefined; busy.value = false; }
function resumeOnVisible() { if (!document.hidden) void load(true); }
function pollVisible() { if (!document.hidden && !busy.value) void load(); }
async function load(force = false) {
  if (!active || (busy.value && !force)) return;
  if (force) invalidate();
  const id = props.conversationId;
  if (!id || id === '__draft__') { invalidate(); tasks.value = []; summary.value = normalizeTaskSummary(null); return; }
  const token = ++generation;
  const abort = new AbortController();
  controller = abort;
  busy.value = true;
  try {
    const response = await readWorkbenchTasks(id, props.characterId || '', abort.signal);
    if (!active || token !== generation || id !== props.conversationId) return;
    const parsed = parseWorkbenchTaskList(response, id);
    tasks.value = parsed.tasks;
    summary.value = parsed.summary;
    truncated.value = parsed.truncated;
    liveObservationTruncated.value = parsed.liveObservationTruncated;
    error.value = '';
    if (expandedId.value) {
      const expanded = parsed.tasks.find(item => item.coordinationId === expandedId.value);
      if (!expanded) resetHistory();
      else if (expanded.updatedAt !== historyTaskUpdatedAt) void fetchHistory(expanded.coordinationId, expanded.updatedAt);
    }
  } catch (cause) {
    if (!active || token !== generation) return;
    tasks.value = [];
    summary.value = normalizeTaskSummary(null);
    truncated.value = false;
    liveObservationTruncated.value = false;
    error.value = (cause as Error)?.message || '查询任务失败';
  } finally {
    if (token === generation) { busy.value = false; controller = undefined; }
  }
}
function resetContext() { resetHistory(); invalidate(); tasks.value = []; summary.value = normalizeTaskSummary(null); truncated.value = false; liveObservationTruncated.value = false; error.value = ''; void load(); }
function taskChanged(event: Event) {
  if (matchesWorkbenchTaskChange((event as CustomEvent<WorkbenchTaskChange>).detail, owned.coreId.value, props.characterId || '', props.conversationId)) void load(true);
}
window.addEventListener(workbenchTaskEventName, taskChanged);
onBeforeUnmount(() => window.removeEventListener(workbenchTaskEventName, taskChanged));
watch(() => [props.conversationId, props.characterId], resetContext);
window.addEventListener('amitia:execution-scope-changed', resetContext);
window.addEventListener('amitia:runtime-connection-changed', resetContext);
onBeforeUnmount(() => { window.removeEventListener('amitia:execution-scope-changed', resetContext); window.removeEventListener('amitia:runtime-connection-changed', resetContext); });
onActivated(() => { active = true; void load(); if (timer) clearInterval(timer); timer = setInterval(pollVisible, 12000); document.addEventListener('visibilitychange', resumeOnVisible); window.addEventListener('online', resumeOnVisible); });
onDeactivated(() => { active = false; resetHistory(); invalidate(); if (timer) clearInterval(timer); timer = undefined; document.removeEventListener('visibilitychange', resumeOnVisible); window.removeEventListener('online', resumeOnVisible); });
onBeforeUnmount(() => { active = false; resetHistory(); invalidate(); if (timer) clearInterval(timer); document.removeEventListener('visibilitychange', resumeOnVisible); window.removeEventListener('online', resumeOnVisible); });
</script>
<style scoped>
.task-list{height:100%;overflow:auto;padding:12px;box-sizing:border-box;display:flex;flex-direction:column;gap:10px;font-size:13px}.task-list header,.title{display:flex;align-items:center;justify-content:space-between;gap:12px}.task-list button{border:1px solid var(--surface-border);border-radius:7px;background:transparent;color:inherit;padding:5px 12px;cursor:pointer}.note,.meta{color:var(--el-text-color-secondary);font-size:12px}.error,.warning{color:var(--el-color-danger)}.summary{display:flex;flex-wrap:wrap;gap:7px}.summary span{border-radius:6px;padding:5px 8px;background:var(--el-fill-color-light)}.task-card{padding:12px;border:1px solid var(--surface-border);border-radius:9px;display:flex;flex-direction:column;gap:8px;overflow-wrap:anywhere}.child{display:flex;align-items:center;gap:9px;padding:7px;background:var(--el-fill-color-light);border-radius:7px}.state{font-weight:600;min-width:60px}.objective{flex:1;min-width:0}.child small{color:var(--el-color-warning)}.checkpoint-panel{border:1px solid var(--surface-border);border-radius:7px;padding:8px}.checkpoint-row{display:flex;flex-wrap:wrap;gap:9px;padding:6px 0;border-bottom:1px solid var(--surface-border)}
</style>
