import { describe, it, expect } from 'vitest';
import { linkedRunId } from '../components/workbench/workbench-task-list';
import { resolveLinkedTask, detailStillAuthorized } from '../views/kernel/tasks/task-link';

describe('M08 explicit TaskRun navigation', () => {
  const visible = [{ taskRunId: 'run_123' }, { taskRunId: 'run_456' }];
  it('requires the task to be present in the currently authorized list', () => {
    expect(resolveLinkedTask('run_123', visible)).toBe(visible[0]);
    expect(resolveLinkedTask('coordination-only', visible)).toBeUndefined();
    expect(resolveLinkedTask('run_999', visible)).toBeUndefined();
  });
  it('revokes stale TaskCenter details on owner, permission or list changes', () => {
    const original = { taskRunId: 'run_123', executionScope: { coreId: 'old' }, readOnly: false };
    expect(detailStillAuthorized(original, [original])).toBe(true);
    expect(detailStillAuthorized(original, [{ ...original, executionScope: { coreId: 'new' } }])).toBe(false);
    expect(detailStillAuthorized(original, [{ ...original, readOnly: true }])).toBe(false);
    expect(detailStillAuthorized(original, [])).toBe(false);
  });
  it('rejects forged or malformed IDs', () => {
    for (const id of ['../run', '%2e%2e', ' run_123', '', 'a'.repeat(161)]) {
      expect(resolveLinkedTask(id, visible)).toBeUndefined();
      expect(() => linkedRunId(id)).toThrow();
    }
    expect(linkedRunId('run_123')).toBe('run_123');
  });
});
