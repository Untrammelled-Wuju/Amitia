import { describe, expect, it } from 'vitest';
import { parseWorkbenchTaskList } from '../components/workbench/workbench-task-list';

const summary = { total: 1, active: 1, paused: 0, succeeded: 0, failed: 0, runtimeObserved: 0, checkpointOnly: 1, needsReconciliation: 0, codingWorkersRequiringReview: 0 };
function reply(objective: string, role = 'coding_worker') {
  return {
    protocolVersion: 1, conversationId: 'meshconv1:owner:conv', source: 'durable-checkpoint', truncated: false,
    tasks: [{ coordinationId: 'coord', status: 'running', updatedAt: '2026-10-23T00:00:00Z', recoveryState: 'checkpoint-only', assignmentCount: 1, completedCount: 0, failedCount: 0, assignments: [{ id: 'child', status: 'running', role, objective, reviewRequired: false }] }], summary,
  };
}
describe('M08 UTF-8 Go rune contract', () => {
  it('accepts exactly 220 Unicode code points, including non-BMP emoji', () => {
    const result = parseWorkbenchTaskList(reply('😀'.repeat(220), 'r'.repeat(100)), 'meshconv1:owner:conv');
    expect(result.tasks[0].assignments[0].objective).toBe('😀'.repeat(220));
    expect(result.tasks[0].assignments[0].role).toHaveLength(100);
  });
  it('rejects actual out-of-contract lengths and does not truncate silently', () => {
    expect(() => parseWorkbenchTaskList(reply('😀'.repeat(221)), 'meshconv1:owner:conv')).toThrow();
    expect(() => parseWorkbenchTaskList(reply('a', 'r'.repeat(101)), 'meshconv1:owner:conv')).toThrow();
    expect(() => parseWorkbenchTaskList(reply('a'), 'meshconv1:other:conv')).toThrow();
  });
});
