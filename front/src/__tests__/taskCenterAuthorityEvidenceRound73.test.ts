import { describe, expect, it } from 'vitest';
import { currentAuthorizedTask, currentControlTarget, detailStillAuthorized, resolveLinkedTask, taskEvidenceMatches } from '../views/kernel/tasks/task-link';

function task(fields: Record<string, unknown> = {}) {
  return {
    taskRunId: 'task-73', ownerId: 'owner-73', readOnly: false,
    executionScope: { coreId: 'core-73', roleId: 'role-73' },
    managementExecutionScope: { coreId: 'core-73', permissionRevision: 9 },
    generation: 5, attempt: 2, status: 'running',
    taskDefinitionId: 'definition-73', inputHash: 'input-73',
    scopeSnapshotId: 'scope-73', permissionSnapshotId: 'permission-73', dependencySnapshotId: 'dependencies-73',
    runtimeInstanceId: 'instance-73', checkpointId: 'checkpoint-73', resultArtifactId: '',
    deadlineAt: '2026-10-23T16:00:00Z', cancelRequestedAt: '',
    startedAt: '2026-10-23T12:00:00Z', finishedAt: '',
    ...fields,
  };
}

describe('M08 task authority and evidence refresh', () => {
  it('rejects duplicated TaskRun identifiers from an ostensibly authorized list', () => {
    const before = task();
    expect(resolveLinkedTask(before.taskRunId, [before, task({ ownerId: 'different' })])).toBeUndefined();
    expect(currentControlTarget(before, [before, task()])).toBeUndefined();
  });
  it('invalidates unchanged status after any evidence or cancellation revision', () => {
    const before = task();
    for (const [field, value] of Object.entries({
      taskDefinitionId: 'other', inputHash: 'changed', scopeSnapshotId: 'changed',
      permissionSnapshotId: 'changed', dependencySnapshotId: 'changed',
      runtimeInstanceId: 'new', checkpointId: 'new', resultArtifactId: 'new',
      deadlineAt: '2026-10-23T17:00:00Z', cancelRequestedAt: '2026-10-23T13:00:00Z',
      startedAt: '2026-10-23T11:00:00Z', finishedAt: '2026-10-23T13:00:00Z',
    })) {
      const refreshed = task({ [field]: value });
      expect(detailStillAuthorized(before, [refreshed]), field).toBe(false);
      expect(currentControlTarget(before, [refreshed]), field).toBeUndefined();
    }
  });
  it('rejects mismatched task details, result artifacts and checkpoint evidence', () => {
    const current = task();
    expect(taskEvidenceMatches(current, { taskRunId: 'task-73' }, { taskRunId: 'task-73' }, { taskRunId: 'task-73', checkpointId: 'checkpoint-73' })).toBe(true);
    expect(taskEvidenceMatches(current, { taskRunId: 'different' })).toBe(false);
    expect(taskEvidenceMatches(current, undefined, { taskRunId: 'different' })).toBe(false);
    expect(taskEvidenceMatches(current, undefined, undefined, { taskRunId: 'different' })).toBe(false);
    expect(taskEvidenceMatches(current, undefined, undefined, { taskRunId: 'task-73', checkpointId: 'other' })).toBe(false);
    expect(taskEvidenceMatches(task({ resultArtifactId: 'artifact-73' }), undefined, { taskRunId: 'task-73', artifactId: 'other' })).toBe(false);
    expect(taskEvidenceMatches(task({ resultArtifactId: 'artifact-73' }), undefined, { taskRunId: 'task-73', artifactId: 'artifact-73' })).toBe(true);
  });
  it('keeps the latest authorized record usable when all scope and evidence fields match', () => {
    const before = task();
    const current = task();
    expect(currentAuthorizedTask(before, [current])).toBe(current);
    expect(currentControlTarget(before, [current])).toBe(current);
  });
});
