import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

const base = {
  protocolVersion: 1,
  conversationId: 'conv',
  source: 'durable-checkpoint',
  truncated: false,
  liveObservationTruncated: false,
  summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 0, needsReconciliation: 1, codingWorkersRequiringReview: 1 },
  tasks: [{ coordinationId: 'coord', status: 'succeeded', recoveryState: 'needs-reconciliation', recoveryReason: 'terminal-child-evidence-missing', updatedAt: '2026-10-23T01:00:00Z', assignmentCount: 1, completedCount: 1, failedCount: 0, assignments: [{ id: 'child', status: 'succeeded', role: 'coding_worker', objective: 'verify', reviewRequired: true }] }],
};

describe('M08 reconciliation is not a confirmed terminal result', () => {
  it('accepts an unverified terminal as active and requiring reconciliation', () => {
    const result = parseWorkbenchTaskList(base, 'conv');
    expect(result.summary.active).toBe(1);
    expect(result.summary.succeeded).toBe(0);
    expect(result.summary.needsReconciliation).toBe(1);
  });
  it('rejects an unverified terminal counted as success', () => {
    expect(() => parseWorkbenchTaskList({ ...base, summary: { ...base.summary, active: 0, succeeded: 1 } }, 'conv')).toThrow();
  });
  it('accepts a confirmed terminal as success', () => {
    const data = { ...base, summary: { ...base.summary, active: 0, succeeded: 1, needsReconciliation: 0, checkpointOnly: 1 }, tasks: [{ ...base.tasks[0], recoveryState: 'checkpoint-only', recoveryReason: undefined }] };
    expect(parseWorkbenchTaskList(data, 'conv').summary.succeeded).toBe(1);
  });
});
