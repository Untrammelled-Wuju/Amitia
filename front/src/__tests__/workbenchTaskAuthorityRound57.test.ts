import { describe, expect, it } from 'vitest';
import { readWorkbenchPolicyAuthority, sameWorkbenchPolicyAuthority } from '../components/workbench/workbench-task-authority';

const base = { providerEpoch: 2, modeRevision: 3, permissionRevision: 4, coordinated: true };
describe('M08 workbench authority is stable across read and writeback', () => {
  it('accepts matching complete policy snapshots', () => {
    expect(sameWorkbenchPolicyAuthority(base, {...base}, {...base})).toBe(true);
  });
  it('rejects changed privilege version, mode, provider epoch and coordination mode', () => {
    for (const key of Object.keys(base)) {
      const changed = { ...base, [key]: key === 'coordinated' ? false : 100 };
      expect(sameWorkbenchPolicyAuthority(base, changed, changed)).toBe(false);
      expect(sameWorkbenchPolicyAuthority(base, base, changed)).toBe(false);
    }
  });
  it('rejects omitted, malformed and noninteger authority fields', () => {
    for (const key of Object.keys(base)) {
      const removed = {...base} as Record<string, unknown>;
      delete removed[key];
      expect(readWorkbenchPolicyAuthority(removed)).toBeNull();
      expect(sameWorkbenchPolicyAuthority(base, base, removed)).toBe(false);
    }
    expect(readWorkbenchPolicyAuthority({...base, modeRevision: 1.2})).toBeNull();
    expect(readWorkbenchPolicyAuthority({...base, providerEpoch: -1})).toBeNull();
    expect(readWorkbenchPolicyAuthority({...base, coordinated: 'true'})).toBeNull();
  });
});
