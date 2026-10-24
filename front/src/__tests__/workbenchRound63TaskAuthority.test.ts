import { describe, expect, it } from 'vitest';
import { currentAuthorizedTask, currentControlTarget, detailStillAuthorized, resolveLinkedTask } from '../views/kernel/tasks/task-link';

function task(overrides: Record<string, unknown> = {}) {
  return {
    taskRunId: 'run_63',
    ownerId: 'owner-1',
    executionScope: { coreId: 'core-1', spaceId: 'space-1' },
    managementExecutionScope: { coreId: 'core-1', permissionRevision: 2 },
    readOnly: false,
    generation: 3,
    attempt: 1,
    status: 'running',
    ...overrides,
  };
}

describe('M08 task center list and control fencing', () => {
  it('requires a current authorized list entry', () => {
    const previous = task();
    expect(currentAuthorizedTask(previous, [previous])).toBe(previous);
    expect(currentAuthorizedTask(previous, [])).toBeUndefined();
    expect(resolveLinkedTask('run_63', [previous])).toBe(previous);
  });
  it('blocks details and controls after ownership, permissions or execution change', () => {
    const before = task();
    for (const changed of [
      task({ ownerId: 'owner-2' }),
      task({ readOnly: true }),
      task({ generation: 4 }),
      task({ attempt: 2 }),
      task({ status: 'succeeded' }),
      task({ managementExecutionScope: { coreId: 'core-1', permissionRevision: 3 } }),
      task({ executionScope: { coreId: 'core-2', spaceId: 'space-1' } }),
    ]) {
      expect(detailStillAuthorized(before, [changed])).toBe(false);
      expect(currentControlTarget(before, [changed])).toBeUndefined();
    }
  });
  it('uses the latest authorized record and forbids stale state actions', () => {
    const before = task();
    const current = task();
    expect(currentControlTarget(before, [current])).toBe(current);
    expect(currentControlTarget(before, [task({ status: 'cancelled' })])).toBeUndefined();
    expect(currentControlTarget(task({ readOnly: true }), [task({ readOnly: true })])).toBeUndefined();
  });
  it('does not resolve forged IDs', () => {
    expect(resolveLinkedTask('../run_63', [task()])).toBeUndefined();
  });
});
