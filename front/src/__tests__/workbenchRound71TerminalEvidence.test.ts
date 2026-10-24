import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

function response(reason: string) {
  return {
    protocolVersion: 1, conversationId: 'conv', source: 'durable-checkpoint', truncated: false,
    summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 0, needsReconciliation: 1, codingWorkersRequiringReview: 0 },
    tasks: [{ coordinationId: 'c1', status: 'waiting', recoveryState: 'needs-reconciliation', recoveryReason: reason, updatedAt: '2026-10-23T02:00:00Z', assignmentCount: 1, completedCount: 0, failedCount: 0, assignments: [{ id: 'a1', status: 'waiting', role: 'worker', dependencies: [], objective: 'test', reviewRequired: false }] }],
  };
}

describe('M08 terminal parent evidence contract', () => {
  it('accepts a persisted parent-terminal/child-pending reconciliation without claiming success', () => {
    const parsed = parseWorkbenchTaskList(response('parent-terminal-child-pending'), 'conv');
    expect(parsed.summary.succeeded).toBe(0);
    expect(parsed.summary.needsReconciliation).toBe(1);
    expect(parsed.tasks[0].recoveryReason).toBe('parent-terminal-child-pending');
  });
  it('rejects unknown recovery reasons rather than guessing terminal state', () => {
    expect(() => parseWorkbenchTaskList(response('unsupported-reason'), 'conv')).toThrow();
  });
});
