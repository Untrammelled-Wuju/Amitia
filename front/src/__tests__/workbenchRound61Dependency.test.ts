import { describe, it, expect } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

function response() {
  return {
    protocolVersion: 1, conversationId: 'conversation', source: 'durable-checkpoint', truncated: false,
    summary: { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 1, needsReconciliation: 0, codingWorkersRequiringReview: 0 },
    tasks: [{ coordinationId: 'coord', status: 'running', updatedAt: '2026-10-23T00:00:00Z', recoveryState: 'checkpoint-only',
      completedCount: 1, failedCount: 0, assignmentCount: 2, assignments: [
        { id: 'first', status: 'succeeded', role: 'worker', objective: 'prepare', reviewRequired: false },
        { id: 'second', status: 'pending', role: 'worker', objective: 'verify', reviewRequired: false, dependencies: ['first'] },
      ],
    }],
  };
}

describe('Round 61 Workbench durable dependency projection', () => {
  it('preserves a valid DAG and accepts legacy absent dependencies', () => {
    const responseValue = response();
    const result = parseWorkbenchTaskList(responseValue, 'conversation');
    expect(result.tasks[0].assignments[0].dependencies).toEqual([]);
    expect(result.tasks[0].assignments[1].dependencies).toEqual(['first']);
  });
  it.each(['missing', 'self', 'cycle', 'duplicate'] as const)('rejects an invalid %s dependency', kind => {
    const data = response();
    const children = data.tasks[0].assignments;
    if (kind === 'missing') children[1].dependencies = ['foreign'];
    if (kind === 'self') children[1].dependencies = ['second'];
    if (kind === 'cycle') { children[0].dependencies = ['second']; children[1].dependencies = ['first']; }
    if (kind === 'duplicate') children[1].dependencies = ['first', 'first'];
    expect(() => parseWorkbenchTaskList(data, 'conversation')).toThrow('依赖');
  });
});
