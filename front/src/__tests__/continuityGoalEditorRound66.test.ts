import { describe, expect, it } from 'vitest';
import { canEditGoalDocument, goalEditAuthorityKey, prepareGoalEdit, type GoalEditDocument } from '../views/continuity/continuity-goal-editor';

const original: GoalEditDocument = {
  ownerId: 'owner-1',
  executionScope: { coreId: 'core-1', roleId: 'role-1', resourceOwnerId: 'owner-1', providerEpoch: 7,
    modeRevision: 2, permissionRevision: 3, roleRevision: 11, targetPermissionRevision: 4, targetProviderEpoch: 9 },
  thread: { id: 'thread-1', revision: 3, title: '目标', goal: '之前的内容', nextAction: '', status: 'paused' },
};
const copy = (): GoalEditDocument => JSON.parse(JSON.stringify(original));
const roles = [{ id: 'role-1' }];

describe('Round 66 Continuity goal editor', () => {
  it('accepts only active authorized Owner documents', () => {
    expect(canEditGoalDocument(original, 'core-1', roles, 'current')).toBe(true);
    expect(canEditGoalDocument(original, 'core-1', [{ id: 'role-1', revision: 12 }], 'current')).toBe(false);
    expect(canEditGoalDocument(original, 'core-1', [{ id: 'role-1', revision: 11 }], 'current')).toBe(true);
    expect(canEditGoalDocument(original, 'core-2', roles, 'current')).toBe(false);
    expect(canEditGoalDocument(original, 'core-1', [], 'current')).toBe(false);
    expect(canEditGoalDocument(original, 'core-1', roles, 'history:old')).toBe(false);
    expect(canEditGoalDocument({ ...original, readOnly: true }, 'core-1', roles, 'current')).toBe(false);
    expect(canEditGoalDocument({ ...original, ownerId: 'other' }, 'core-1', roles, 'current')).toBe(false);
  });
  it('rejects terminal, running and uncertain execution', () => {
    for (const status of ['completed', 'cancelled']) {
      expect(canEditGoalDocument({ ...original, thread: { ...original.thread, status } }, 'core-1', roles, 'current')).toBe(false);
    }
    for (const state of ['running', 'unknown']) {
      expect(canEditGoalDocument({ ...original, lease: { state } }, 'core-1', roles, 'current')).toBe(false);
    }
  });
  it('the authority fingerprint changes on every relevant revision', () => {
    const before = goalEditAuthorityKey(original);
    for (const [field, value] of [['roleRevision', 12], ['providerEpoch', 8], ['permissionRevision', 4],
      ['modeRevision', 3], ['targetPermissionRevision', 5], ['targetProviderEpoch', 10]] as const) {
      const changed = copy();
      Object.assign(changed.executionScope!, { [field]: value });
      expect(goalEditAuthorityKey(changed)).not.toBe(before);
    }
    const changed = copy();
    changed.thread.revision++;
    expect(goalEditAuthorityKey(changed)).not.toBe(before);
    expect(goalEditAuthorityKey({ ...original, thread: { ...original.thread, revision: Number.NaN } })).toBe('');
  });
  it('uses UTF-8 byte limits from the existing OwnedBusiness mutation contract', () => {
    expect(prepareGoalEdit({ title: ' 改后 ', goal: '新目标', nextAction: '接下来' }, original)).toEqual({ title: '改后', goal: '新目标', nextAction: '接下来' });
    expect(() => prepareGoalEdit({ title: '😊'.repeat(1025), goal: '', nextAction: '' }, original)).toThrow();
    expect(() => prepareGoalEdit({ title: 'X', goal: '中'.repeat(44000), nextAction: '' }, original)).toThrow();
    expect(() => prepareGoalEdit({ title: 'X', goal: '', nextAction: '中'.repeat(2800) }, original)).toThrow();
  });
  it('does not send empty titles or no-op revisions', () => {
    expect(() => prepareGoalEdit({ title: '', goal: '', nextAction: '' }, original)).toThrow();
    expect(() => prepareGoalEdit({ title: '目标', goal: '之前的内容', nextAction: '' }, original)).toThrow();
    expect(() => prepareGoalEdit({ title: '目标', goal: '', nextAction: '' }, original)).not.toThrow();
  });
});
