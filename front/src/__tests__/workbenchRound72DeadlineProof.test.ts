import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

function response(reason: string) {
  return {
    protocolVersion: 1, conversationId: 'conv', source: 'durable-checkpoint', truncated: false,
    summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 0, needsReconciliation: 1, codingWorkersRequiringReview: 0 },
    tasks: [{ coordinationId: 'coord', status: 'waiting', recoveryState: 'needs-reconciliation', recoveryReason: reason, deadlineAt: '2026-10-23T00:00:00Z', updatedAt: '2026-10-23T00:01:00Z', assignmentCount: 1, completedCount: 0, failedCount: 0,
      assignments: [{ id: 'child', status: 'waiting', role: 'review_worker', dependencies: [], objective: 'verify', reviewRequired: false }] }],
  };
}

describe('round 72 deadline proof protocol', () => {
  it('accepts unverified deadline success without reporting a completed task', () => {
    const parsed = parseWorkbenchTaskList(response('terminal-child-deadline-unverified'), 'conv');
    expect(parsed.tasks[0].recoveryReason).toBe('terminal-child-deadline-unverified');
    expect(parsed.summary.succeeded).toBe(0);
    expect(parsed.summary.needsReconciliation).toBe(1);
  });
  it('rejects unknown deadline reasons', () => {
    expect(() => parseWorkbenchTaskList(response('terminal-unknown-proof'), 'conv')).toThrow();
  });
});
