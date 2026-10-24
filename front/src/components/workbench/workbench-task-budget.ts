import type { WorkbenchTaskItem } from './workbench-task-list';

export type WorkbenchTimeBudgetState = 'unconfigured' | 'within-deadline' | 'deadline-passed' | 'terminal' | 'unverified';

export function workbenchTimeBudgetState(task: Pick<WorkbenchTaskItem, 'status' | 'deadlineAt' | 'recoveryState'>, now: number, stale = false): WorkbenchTimeBudgetState {
  if (stale || task.recoveryState === 'needs-reconciliation' || !Number.isFinite(now)) return 'unverified';
  if (!task.deadlineAt) return 'unconfigured';
  const deadline = Date.parse(task.deadlineAt);
  if (!Number.isFinite(deadline)) return 'unverified';
  if (task.status === 'succeeded' || task.status === 'failed' || task.status === 'cancelled') return 'terminal';
  return deadline <= now ? 'deadline-passed' : 'within-deadline';
}

export function workbenchTimeBudgetNotice(state: WorkbenchTimeBudgetState): string {
  switch (state) {
    case 'within-deadline': return '协调时间预算尚未到期；不代表存在 Token、调用次数或费用总额硬限制。';
    case 'deadline-passed': return '协调截止时间已过；执行是否停止及外部副作用是否撤销仍须通过正式回执核查。';
    case 'terminal': return '任务已报告终态；截止时间不能证明任务在时限内完成，请以持久回执为准。';
    case 'unverified': return '预算状态不可确认；请取得新的授权持久快照，不能据此判定执行停止。';
    default: return '未记录协调时间截止。新的多 Agent 委派可选 deadline_minutes（1–240 分钟）建立持久时间限制；Token、工具次数和费用限制须结合父任务配置及预算账本核对。';
  }
}
