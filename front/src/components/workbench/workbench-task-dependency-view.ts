import type { WorkbenchTaskAssignment } from './workbench-task-list';

export interface WorkbenchDependencyView {
  assignment: WorkbenchTaskAssignment;
  depth: number;
  notice: string;
  inconsistent: boolean;
}

export function projectWorkbenchDependencies(assignments: readonly WorkbenchTaskAssignment[]): WorkbenchDependencyView[] {
  const byId = new Map(assignments.map(item => [item.id, item]));
  const depths = new Map<string, number>();
  const depth = (id: string, visiting = new Set<string>()): number => {
    const known = depths.get(id);
    if (known !== undefined) return known;
    if (visiting.has(id)) throw new Error('子任务依赖存在循环');
    const item = byId.get(id);
    if (!item) throw new Error('子任务依赖引用无效');
    visiting.add(id);
    const next = item.dependencies.length ? 1 + Math.max(...item.dependencies.map(parent => depth(parent, visiting))) : 0;
    visiting.delete(id);
    depths.set(id, next);
    return next;
  };
  const results = assignments.map(assignment => {
    const nextDepth = depth(assignment.id);
    const predecessors = assignment.dependencies.map(id => byId.get(id));
    if (predecessors.some(item => !item)) throw new Error('子任务依赖引用无效');
    const unresolved = predecessors.filter(item => item!.status !== 'succeeded');
    const inconsistent = unresolved.length > 0 && ['running', 'succeeded'].includes(assignment.status);
    const blocked = ['pending', 'waiting', 'paused'].includes(assignment.status);
    const failed = unresolved.some(item => ['failed', 'cancelled'].includes(item!.status));
    const notice = inconsistent ? '依赖证据冲突，需核查' : blocked && failed ? '前置任务失败或取消，需等待协调' : blocked && unresolved.length ? `等待 ${unresolved.length} 个前置任务` : '';
    return { assignment, depth: nextDepth, notice, inconsistent };
  });
  return results.sort((a, b) => a.depth - b.depth || assignments.indexOf(a.assignment) - assignments.indexOf(b.assignment));
}
