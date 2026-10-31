<template>
  <section class="task-list" aria-label="多 Agent 任务">
    <header>
      <strong>多 Agent · 任务状态</strong>
      <button type="button" :disabled="busy" @click="load(true)">{{ busy ? '查询中' : '刷新' }}</button>
    </header>
    <p class="note">持久化任务检查点与经过身份校验的 Runtime 快照。子任务成功不代表构建、审批和合并已通过。</p>
    <p class="note">协调状态是只读投影。预算核对来自已有的持久模型/工具账本，记录的是保守预留和可信核销，不等于供应商最终账单。</p>
    <p class="note">这是协调状态的只读投影；正式 TaskRun 的暂停、恢复、取消与重试在 <router-link to="/kernel/tasks">任务中心</router-link> 完成，需以服务端确认结果为准。</p>
    <p class="note">角色配置请到 <router-link v-if="roleIsAuthorized" :to="{ path: `/character/${encodeURIComponent(props.characterId || '')}/life-rules` }">当前角色管理</router-link><router-link v-else to="/character">角色管理</router-link>；长期目标到 <router-link v-if="roleIsAuthorized" :to="{ path: '/continuity', query: { characterId: props.characterId } }">当前角色的 Continuity</router-link><router-link v-else to="/continuity">Continuity</router-link>。以上操作仍由原 Owner 服务鉴权，不新建 Agent 配置或目标。</p>
    <div class="summary" role="group" aria-label="任务状态汇总">
      <span>总数 <b>{{ summary.total }}</b></span>
      <span>进行中/待核实 <b>{{ summary.active }}</b></span>
      <span>成功结束 <b>{{ summary.succeeded }}</b></span>
      <span>失败/取消 <b>{{ summary.failed }}</b></span>
      <span>需协调 <b>{{ summary.needsReconciliation }}</b></span>
    </div>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="snapshotStale && tasks.length" class="warning" role="status">连接中断，仅显示本次授权会话最后成功验证的只读快照；任务状态可能已经变化，不能据此确认执行完成。</p>
    <p v-else-if="!busy && !error && !tasks.length" class="note">当前对话尚无可读取的多 Agent 任务检查点。</p>
    <article v-for="task in tasks" :key="task.coordinationId" class="task-card">
      <div class="title"><strong>{{ task.recoveryState === 'needs-reconciliation' ? '待对账（不可确认终态）' : statusName(task.status) }}</strong><small>{{ task.completedCount }}/{{ task.assignmentCount }} 个子任务结束</small></div>
      <small class="meta">{{ task.coordinationId }} · {{ recoveryName(task.recoveryState) }} · {{ formatDate(task.updatedAt) }}</small>
      <router-link v-if="task.taskRunId && task.recoveryState !== 'needs-reconciliation' && !snapshotStale" :to="{ path: '/kernel/tasks', query: { taskRunId: task.taskRunId } }">在正式任务中心查看关联 TaskRun（控制前需重新授权）</router-link>
      <p v-else-if="task.taskRunId" class="note">已关联 TaskRun {{ task.taskRunId }}；当前证据或连接状态未确认，请刷新后再前往任务中心。</p>
      <WorkbenchBudgetEvidencePanel v-if="task.parentInteractionId && !snapshotStale && verifiedAuthority" :key="`${task.coordinationId}:${task.updatedAt}:${verifiedAuthority}`" :conversation-id="props.conversationId" :character-id="props.characterId || ''" :coordination-id="task.coordinationId" :parent-interaction-id="task.parentInteractionId" :updated-at="task.updatedAt" :authority="verifiedAuthority" :stale="snapshotStale" @invalidated="resetContext" />
      <small v-if="task.goalId" class="meta">长期目标 {{ task.goalId }} · 修订 {{ task.goalRevision ?? 0 }}</small>
      <small v-if="task.deviceId || task.executionCoreId" class="meta">执行位置 {{ task.deviceId ? '设备 ' + task.deviceId : 'Core ' + task.executionCoreId }}</small>
      <p v-if="task.cancelRequested" class="warning">已请求取消，但不代表运行中的工具动作已经停止。</p>
      <small v-if="task.deadlineAt" class="meta">协调截止 {{ formatDate(task.deadlineAt) }}</small>
      <p class="note" role="status">{{ workbenchTimeBudgetNotice(workbenchTimeBudgetState(task, currentBudgetTime, snapshotStale)) }}</p>
      <p v-if="task.recoveryState === 'needs-reconciliation'" class="warning">{{ task.recoveryReason === 'parent-state-conflict' ? '父任务已结束，但检查点仍显示未结束；需要核查状态。' : task.recoveryReason === 'parent-terminal-child-pending' ? '父任务被标记为终态，但仍有未确认结束的子任务，必须对账。' : task.recoveryReason === 'terminal-child-deadline-unverified' ? '子任务未提供截止时间内完成的可信时间证据，成功状态暂不计入已完成，须对账。' : task.recoveryReason?.startsWith('terminal-') ? '子任务完成或取消的持久回执尚未核实，结果暂不计为已完成。' : '持久化检查点与 Runtime 快照冲突，需人工核查。' }}</p>
      <button type="button" :disabled="snapshotStale" :aria-expanded="expandedId === task.coordinationId" @click="toggleHistory(task.coordinationId)">{{ expandedId === task.coordinationId ? '收起持久化检查点' : '查看持久化检查点' }}</button>
      <div v-if="expandedId === task.coordinationId" class="checkpoint-panel">
        <p class="note">仅展示最近保存的检查点样本，不是完整执行事件日志，也不代表测试已通过。</p>
        <p v-if="historyBusy" class="note">正在读取检查点…</p>
        <p v-else-if="historyError" class="error" role="alert">{{ historyError }}</p>
        <template v-else-if="history">
          <div v-for="(point, index) in history.records" :key="index" class="checkpoint-row">
            <span>{{ formatDate(point.updatedAt) }}</span>
            <span>{{ statusName(point.coordinationStatus) }} · {{ point.completedCount }}/{{ point.assignmentCount }} 已结束</span>
            <strong v-if="!point.consistent" class="warning">检查点状态或子任务回执需核查</strong>
          </div>
          <p v-if="history.truncated" class="warning">检查点样本已截断，不能据此重建完整执行历史。</p>
        </template>
      </div>
      <div v-for="node in projectWorkbenchDependencies(task.assignments)" :key="node.assignment.id" class="child" :class="{ conflict: node.inconsistent }">
        <span class="depth">{{ node.depth ? `第 ${node.depth + 1} 层` : '起始' }}</span>
        <span class="state">{{ statusName(node.assignment.status) }}</span>
        <span class="objective" :title="node.assignment.objective">{{ node.assignment.objective }}</span>
        <small v-if="node.assignment.reviewRequired">须独立审查</small>
        <small v-if="node.notice" :class="{ warning: node.inconsistent }" role="status">{{ node.notice }}</small>
        <small v-if="node.assignment.dependencies.length" class="dependency" :title="node.assignment.dependencies.join('、')">前置 {{ dependencyStatuses(node.assignment.dependencies, task) }}</small>
      </div>
    </article>
    <p v-if="truncated" class="note">结果达到查询上限，仅显示最近 30 项；较旧任务未纳入上述统计。</p>
    <p v-if="liveObservationTruncated" class="note">实时快照只检查最近 8 个任务，其余任务状态来自持久化检查点。</p>
    <p v-if="summary.runtimeObserved" class="note">{{ summary.runtimeObserved }} 项已匹配实时协调器状态；其余任务仅代表历史快照。</p>
  </section>
</template>
<script setup lang="ts">
import { computed, ref, watch, onActivated, onDeactivated, onBeforeUnmount } from 'vue';
import WorkbenchBudgetEvidencePanel from './WorkbenchBudgetEvidencePanel.vue';
import { readWorkbenchTasks, WorkbenchTaskReadError } from './workbench-task-api';
import { shouldInvalidateWorkbenchSnapshot } from './workbench-task-cache-policy';
import { useDeviceOwnedConversation } from '@/composables/useDeviceOwnedConversation';
import { matchesWorkbenchTaskChange, workbenchTaskEventName, type WorkbenchTaskChange } from '@/runtime/owned-workbench-task-event';
import { normalizeTaskSummary, type TaskSummary } from './workbench-task-summary';
import { parseWorkbenchTaskList, type WorkbenchTaskItem } from './workbench-task-list';
import { projectWorkbenchDependencies } from './workbench-task-dependency-view';
import { workbenchTimeBudgetState, workbenchTimeBudgetNotice } from './workbench-task-budget';
import { loadTaskCheckpoints, type TaskCheckpointHistory } from './workbench-task-checkpoints';
import { authorizedWorkbenchRole, unchangedWorkbenchCheckpoint } from './workbench-role-link';
const props = defineProps<{ conversationId: string; characterId?: string }>();
const owned = useDeviceOwnedConversation();
const roleIsAuthorized = computed(() => !snapshotStale.value && authorizedWorkbenchRole(props.characterId, owned.roles.value, owned.coreId.value));
const tasks = ref<WorkbenchTaskItem[]>([]);
const summary = ref<TaskSummary>(normalizeTaskSummary(null));
const error = ref('');
const busy = ref(false);
const snapshotStale = ref(false);
const currentBudgetTime = ref(Date.now());
const truncated = ref(false);
const liveObservationTruncated = ref(false);
const expandedId = ref('');
const history = ref<TaskCheckpointHistory | null>(null);
const historyError = ref('');
const historyBusy = ref(false);
let verifiedAuthority = '';
function authorityKey() {
  return JSON.stringify([props.conversationId, props.characterId || '', owned.coreId.value, owned.policy.value, owned.roles.value.map(role => role.id).sort()]);
}
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
  const startedAuthority = authorityKey();
  const startedCharacterId = props.characterId || '';
  const abort = new AbortController();
  historyController = abort;
  historyBusy.value = true;
  try {
    const next = await loadTaskCheckpoints(conversationId, coordinationId, abort.signal, props.characterId || '');
    if (active && generationId === historyGeneration && conversationId === props.conversationId && expandedId.value === coordinationId) {
      if (!unchangedWorkbenchCheckpoint(startedAuthority, authorityKey(), startedCharacterId, props.characterId || '', coordinationId, updatedAt, snapshotStale.value, tasks.value)) {
        resetContext();
        return;
      }
      history.value = next;
    }
  } catch (cause) {
    if (active && generationId === historyGeneration && conversationId === props.conversationId) {
      if (shouldInvalidateWorkbenchSnapshot(cause, verifiedAuthority, authorityKey())) {
        resetContext();
        return;
      }
      historyError.value = (cause as Error)?.message || '检查点读取失败';
    }
  } finally {
    if (generationId === historyGeneration) { historyBusy.value = false; historyController = undefined; }
  }
}
function toggleHistory(coordinationId: string) {
  if (expandedId.value === coordinationId) { resetHistory(); return; }
  resetHistory();
  if (!active || snapshotStale.value) return;
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
function dependencyStatuses(dependencies: string[], task: WorkbenchTaskItem) {
  return dependencies.map(id => {
    const predecessor = task.assignments.find(child => child.id === id);
    return `${id}（${predecessor ? statusName(predecessor.status) : '状态未知'}）`;
  }).join('、');
}
function invalidate() { generation++; controller?.abort(); controller = undefined; busy.value = false; }
function resumeOnVisible() { if (!document.hidden) void load(true); }
function pollVisible() { if (!document.hidden && !busy.value) void load(); }
async function load(force = false) {
  currentBudgetTime.value = Date.now();
  if (!active || (busy.value && !force)) return;
  if (force) invalidate();
  const id = props.conversationId;
  const startedAuthority = authorityKey();
  if (!id || id === '__draft__') { invalidate(); tasks.value = []; summary.value = normalizeTaskSummary(null); snapshotStale.value = false; return; }
  const token = ++generation;
  const abort = new AbortController();
  controller = abort;
  busy.value = true;
  try {
    const response = await readWorkbenchTasks(id, props.characterId || '', abort.signal);
    if (!active || token !== generation || id !== props.conversationId) return;
    if (startedAuthority !== authorityKey()) { resetContext(); return; }
    let parsed;
    try {
      parsed = parseWorkbenchTaskList(response, id);
    } catch (error) {
  console.error("[original-error]", error);
throw new WorkbenchTaskReadError('任务响应证据无效，已丢弃缓存', true);
    }
    tasks.value = parsed.tasks;
    summary.value = parsed.summary;
    truncated.value = parsed.truncated;
    liveObservationTruncated.value = parsed.liveObservationTruncated;
    error.value = '';
    snapshotStale.value = false;
    verifiedAuthority = authorityKey();
    if (expandedId.value) {
      const expanded = parsed.tasks.find(item => item.coordinationId === expandedId.value);
      if (!expanded) resetHistory();
      else if (expanded.updatedAt !== historyTaskUpdatedAt) void fetchHistory(expanded.coordinationId, expanded.updatedAt);
    }
  } catch (cause) {
    if (!active || token !== generation) return;
    if (shouldInvalidateWorkbenchSnapshot(cause, verifiedAuthority, authorityKey())) {
      resetHistory();
      tasks.value = [];
      summary.value = normalizeTaskSummary(null);
      truncated.value = false;
      liveObservationTruncated.value = false;
      snapshotStale.value = false;
      verifiedAuthority = '';
    } else {
      snapshotStale.value = tasks.value.length > 0;
      resetHistory();
    }
    error.value = (cause as Error)?.message || '查询任务失败';
  } finally {
    if (token === generation) { busy.value = false; controller = undefined; }
  }
}
function resetContext() { resetHistory(); invalidate(); tasks.value = []; summary.value = normalizeTaskSummary(null); truncated.value = false; liveObservationTruncated.value = false; snapshotStale.value = false; verifiedAuthority = ''; error.value = ''; void load(); }
function resyncWorkbenchTasks(event: Event) {
  if ((event as CustomEvent<{ conversationId?: string }>).detail?.conversationId === props.conversationId && active) void load(true);
}
window.addEventListener('amitia:workbench-task-resync', resyncWorkbenchTasks);
onBeforeUnmount(() => window.removeEventListener('amitia:workbench-task-resync', resyncWorkbenchTasks));
function revokeWorkbenchTasks(event: Event) {
  if ((event as CustomEvent<{ conversationId?: string }>).detail?.conversationId !== props.conversationId) return;
  resetHistory();
  invalidate();
  tasks.value = [];
  summary.value = normalizeTaskSummary(null);
  truncated.value = false;
  liveObservationTruncated.value = false;
  snapshotStale.value = false;
  verifiedAuthority = '';
  error.value = '会话授权已失效，任务快照已清除；请重新授权后刷新。';
}
window.addEventListener('amitia:workbench-task-revoked', revokeWorkbenchTasks);
onBeforeUnmount(() => window.removeEventListener('amitia:workbench-task-revoked', revokeWorkbenchTasks));
function taskChanged(event: Event) {
  if (matchesWorkbenchTaskChange((event as CustomEvent<WorkbenchTaskChange>).detail, owned.coreId.value, props.characterId || '', props.conversationId)) void load(true);
}
window.addEventListener(workbenchTaskEventName, taskChanged);
onBeforeUnmount(() => window.removeEventListener(workbenchTaskEventName, taskChanged));
watch(() => [props.conversationId, props.characterId], resetContext);
window.addEventListener('amitia:execution-scope-changed', resetContext);
window.addEventListener('amitia:runtime-connection-changed', resetContext);
onBeforeUnmount(() => { window.removeEventListener('amitia:execution-scope-changed', resetContext); window.removeEventListener('amitia:runtime-connection-changed', resetContext); });
onActivated(() => { active = true; resetContext(); if (timer) clearInterval(timer); timer = setInterval(pollVisible, 12000); document.addEventListener('visibilitychange', resumeOnVisible); window.addEventListener('online', resumeOnVisible); });
onDeactivated(() => { active = false; resetHistory(); invalidate(); tasks.value = []; summary.value = normalizeTaskSummary(null); verifiedAuthority = ''; snapshotStale.value = false; if (timer) clearInterval(timer); timer = undefined; document.removeEventListener('visibilitychange', resumeOnVisible); window.removeEventListener('online', resumeOnVisible); });
onBeforeUnmount(() => { active = false; resetHistory(); invalidate(); if (timer) clearInterval(timer); document.removeEventListener('visibilitychange', resumeOnVisible); window.removeEventListener('online', resumeOnVisible); });
</script>
<style scoped>
.task-list{height:100%;overflow:auto;padding:12px;box-sizing:border-box;display:flex;flex-direction:column;gap:10px;font-size:13px}.task-list header,.title{display:flex;align-items:center;justify-content:space-between;gap:12px}.task-list button{border:1px solid var(--surface-border);border-radius:7px;background:transparent;color:inherit;padding:5px 12px;cursor:pointer}.note,.meta{color:var(--el-text-color-secondary);font-size:12px}.error,.warning{color:var(--el-color-danger)}.summary{display:flex;flex-wrap:wrap;gap:7px}.summary span{border-radius:6px;padding:5px 8px;background:var(--el-fill-color-light)}.task-card{padding:12px;border:1px solid var(--surface-border);border-radius:9px;display:flex;flex-direction:column;gap:8px;overflow-wrap:anywhere}.child{display:flex;align-items:center;gap:9px;padding:7px;background:var(--el-fill-color-light);border-radius:7px}.state{font-weight:600;min-width:60px}.objective{flex:1;min-width:0}.child small{color:var(--el-color-warning)}.child .dependency{max-width:240px;overflow-wrap:anywhere;color:var(--el-text-color-secondary)}.child.conflict{border:1px solid var(--el-color-danger)}.child .depth{flex-shrink:0;color:var(--el-text-color-secondary);font-size:11px}.child .warning{color:var(--el-color-danger)}.checkpoint-panel{border:1px solid var(--surface-border);border-radius:7px;padding:8px}.checkpoint-row{display:flex;flex-wrap:wrap;gap:9px;padding:6px 0;border-bottom:1px solid var(--surface-border)}
</style>
