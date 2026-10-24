import { describe, expect, it } from 'vitest';
import { authorizedWorkbenchRole, unchangedWorkbenchCheckpoint } from '../components/workbench/workbench-role-link';

const roles = [{ id: 'role:A' }, { id: 'role.B' }];
const task = { coordinationId: 'coord-1', updatedAt: '2026-10-23T10:00:00Z' };
describe('M08 Workbench role context and checkpoint verification', () => {
  it('permits only an authorized current role and Core', () => {
    expect(authorizedWorkbenchRole('role:A', roles, 'core-1')).toBe(true);
    expect(authorizedWorkbenchRole('role.B', roles, 'core-1')).toBe(true);
    expect(authorizedWorkbenchRole('role:C', roles, 'core-1')).toBe(false);
    expect(authorizedWorkbenchRole('role:A', roles, '')).toBe(false);
    expect(authorizedWorkbenchRole('../role:A', roles, 'core-1')).toBe(false);
    expect(authorizedWorkbenchRole(['role:A'], roles, 'core-1')).toBe(false);
  });
  it('accepts only an unmodified authorized checkpoint', () => {
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role:A', task.coordinationId, task.updatedAt, false, [task])).toBe(true);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v2', 'role:A', 'role:A', task.coordinationId, task.updatedAt, false, [task])).toBe(false);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role.B', task.coordinationId, task.updatedAt, false, [task])).toBe(false);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role:A', task.coordinationId, task.updatedAt, true, [task])).toBe(false);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role:A', task.coordinationId, 'old-revision', false, [task])).toBe(false);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role:A', 'other-coordination', task.updatedAt, false, [task])).toBe(false);
    expect(unchangedWorkbenchCheckpoint('scope-v1', 'scope-v1', 'role:A', 'role:A', task.coordinationId, task.updatedAt, false, [])).toBe(false);
  });
});
