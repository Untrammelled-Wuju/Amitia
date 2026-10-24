import { normalizeTaskSummary, type TaskSummary } from './workbench-task-summary';

export interface WorkbenchTaskAssignment {
  id: string;
  status: string;
  role: string;
  dependencies: string[];
  objective: string;
  reviewRequired: boolean;
}

export interface WorkbenchTaskItem {
  coordinationId: string;
  parentInteractionId?: string;
  taskRunId?: string;
  goalId?: string;
  goalRevision?: number;
  executionCoreId?: string;
  deviceId?: string;
  deadlineAt?: string;
  cancelRequested?: boolean;
  status: string;
  completedCount: number;
  failedCount: number;
  assignmentCount: number;
  recoveryState: string;
  recoveryReason?: string;
  updatedAt: string;
  assignments: WorkbenchTaskAssignment[];
}

export interface WorkbenchTaskList {
  tasks: WorkbenchTaskItem[];
  summary: TaskSummary;
  truncated: boolean;
  liveObservationTruncated: boolean;
}

const taskStates = new Set(['planning', 'running', 'waiting', 'aggregating', 'succeeded', 'failed', 'cancelled', 'paused']);
const assignmentStates = new Set(['pending', 'running', 'waiting', 'succeeded', 'failed', 'cancelled', 'paused']);
const recoveryStates = new Set(['runtime-observed', 'checkpoint-only', 'needs-reconciliation']);
const recoveryReasons = new Set(['parent-state-conflict', 'parent-terminal-child-pending', 'runtime-checkpoint-conflict', 'terminal-assignment-missing', 'terminal-child-evidence-missing', 'terminal-child-reader-unavailable', 'terminal-child-read-budget-exceeded', 'terminal-child-evidence-conflict', 'terminal-child-deadline-unverified']);

function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('任务响应格式无效');
  return value as Record<string, unknown>;
}

function text(value: unknown, max: number, required = true): string {
  if (typeof value !== 'string' || [...value].length > max || (required && value.length === 0)) throw new Error('任务响应包含无效文本');
  return value;
}

function count(value: unknown, min: number, max: number): number {
  if (typeof value !== 'number' || !Number.isSafeInteger(value) || value < min || value > max) throw new Error('任务响应包含无效统计');
  return value;
}

function assignment(value: unknown): WorkbenchTaskAssignment {
  const data = object(value);
  const status = text(data.status, 24);
  if (!assignmentStates.has(status) || typeof data.reviewRequired !== 'boolean') throw new Error('子任务状态无效');
  return {
    id: text(data.id, 160),
    status,
    role: text(data.role, 100),
    dependencies: data.dependencies === undefined ? [] : (() => {
      if (!Array.isArray(data.dependencies) || data.dependencies.length > 8) throw new Error('子任务依赖无效');
      return data.dependencies.map(value => text(value, 160));
    })(),
    objective: text(data.objective, 220, false),
    reviewRequired: data.reviewRequired,
  };
}

export function linkedRunId(value: unknown): string {
  const id = text(value, 160);
  if (!/^[A-Za-z0-9_.:-]{1,160}$/.test(id)) throw new Error('TaskRun 关联标识无效');
  return id;
}

function task(value: unknown): WorkbenchTaskItem {
  const data = object(value);
  const status = text(data.status, 24);
  const recoveryState = text(data.recoveryState, 48);
  if (!taskStates.has(status) || !recoveryStates.has(recoveryState) || !Array.isArray(data.assignments) || data.assignments.length > 8) throw new Error('任务状态无效');
  const assignments = data.assignments.map(assignment);
  const ids = new Set(assignments.map(row => row.id));
  const assignmentCount = count(data.assignmentCount, 1, 8);
  const completedCount = count(data.completedCount, 0, assignmentCount);
  const failedCount = count(data.failedCount, 0, completedCount);
  if (ids.size !== assignments.length || assignments.length !== assignmentCount) throw new Error('子任务标识或数量不一致');
  const byId = new Map(assignments.map(child => [child.id, child]));
  const visited = new Map<string, number>();
  function visit(id: string): void {
    if (visited.get(id) === 1) throw new Error('子任务依赖存在循环');
    if (visited.get(id) === 2) return;
    visited.set(id, 1);
    const edges = new Set<string>();
    for (const dependency of byId.get(id)!.dependencies) {
      if (!byId.has(dependency) || edges.has(dependency)) throw new Error('子任务依赖引用无效');
      edges.add(dependency);
      visit(dependency);
    }
    visited.set(id, 2);
  }
  for (const child of assignments) visit(child.id);
  const actualCompleted = assignments.filter(row => ['succeeded', 'failed', 'cancelled'].includes(row.status)).length;
  const actualFailed = assignments.filter(row => ['failed', 'cancelled'].includes(row.status)).length;
  if (completedCount !== actualCompleted || failedCount !== actualFailed) throw new Error('子任务终态统计不一致');
  const updatedAt = text(data.updatedAt, 80);
  if (!Number.isFinite(Date.parse(updatedAt))) throw new Error('任务更新时间无效');
  if (data.cancelRequested !== undefined && typeof data.cancelRequested !== 'boolean') throw new Error('取消状态无效');
  const result: WorkbenchTaskItem = {
    coordinationId: text(data.coordinationId, 160),
    ...(data.parentInteractionId === undefined ? {} : { parentInteractionId: text(data.parentInteractionId, 512) }),
    ...(data.taskRunId === undefined ? {} : { taskRunId: linkedRunId(data.taskRunId) }),
    ...(data.goalId === undefined ? {} : { goalId: text(data.goalId, 160) }),
    ...(data.goalRevision === undefined ? {} : { goalRevision: count(data.goalRevision, 0, Number.MAX_SAFE_INTEGER) }),
    ...(data.executionCoreId === undefined ? {} : { executionCoreId: text(data.executionCoreId, 160) }),
    ...(data.deviceId === undefined ? {} : { deviceId: text(data.deviceId, 160) }),
    ...(data.deadlineAt === undefined ? {} : { deadlineAt: text(data.deadlineAt, 80) }),
    ...(data.cancelRequested === undefined ? {} : { cancelRequested: data.cancelRequested }),
    status,
    completedCount,
    failedCount,
    assignmentCount,
    recoveryState,
    updatedAt,
    assignments,
  };
  if (result.deadlineAt && !Number.isFinite(Date.parse(result.deadlineAt))) throw new Error('任务截止时间无效');
  if (data.recoveryReason !== undefined) {
    const reason = text(data.recoveryReason, 64);
    if (!recoveryReasons.has(reason)) throw new Error('任务对账原因无效');
    result.recoveryReason = reason;
  }
  return result;
}

export function parseWorkbenchTaskList(value: unknown, conversationId: string): WorkbenchTaskList {
  const data = object(value);
  if (data.protocolVersion !== 1 || data.conversationId !== conversationId || data.source !== 'durable-checkpoint' ||
      typeof data.truncated !== 'boolean' || !Array.isArray(data.tasks) || data.tasks.length > 30 ||
      (data.liveObservationTruncated !== undefined && typeof data.liveObservationTruncated !== 'boolean')) {
    throw new Error('任务响应与当前对话或协议版本不匹配');
  }
  const sourceSummary = object(data.summary);
  const requiredCounts = ['total', 'active', 'paused', 'succeeded', 'failed', 'runtimeObserved', 'checkpointOnly', 'needsReconciliation', 'codingWorkersRequiringReview'];
  for (const field of requiredCounts) count(sourceSummary[field], 0, Number.MAX_SAFE_INTEGER);
  const tasks = data.tasks.map(task);
  const taskIds = new Set(tasks.map(item => item.coordinationId));
  if (taskIds.size !== tasks.length) throw new Error('任务列表存在重复标识');
  const summary = normalizeTaskSummary(sourceSummary);
  const derived = {
    total: tasks.length, active: 0, paused: 0, succeeded: 0, failed: 0,
    runtimeObserved: 0, checkpointOnly: 0, needsReconciliation: 0, codingWorkersRequiringReview: 0,
  };
  for (const item of tasks) {
    if (item.recoveryState === 'needs-reconciliation') derived.active++;
    else if (item.status === 'succeeded') derived.succeeded++;
    else if (item.status === 'failed' || item.status === 'cancelled') derived.failed++;
    else if (item.status === 'paused') derived.paused++;
    else derived.active++;
    if (item.recoveryState === 'runtime-observed') derived.runtimeObserved++;
    else if (item.recoveryState === 'needs-reconciliation') derived.needsReconciliation++;
    else derived.checkpointOnly++;
    derived.codingWorkersRequiringReview += item.assignments.filter(child => child.reviewRequired).length;
  }
  for (const key of requiredCounts) {
    if (summary[key as keyof TaskSummary] !== derived[key as keyof typeof derived]) throw new Error('任务分类统计与任务明细不一致');
  }
  return {
    tasks,
    summary,
    truncated: data.truncated,
    liveObservationTruncated: data.liveObservationTruncated === true,
  };
}
