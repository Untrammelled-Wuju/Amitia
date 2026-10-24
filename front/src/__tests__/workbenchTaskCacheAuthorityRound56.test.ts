import { describe, expect, it } from 'vitest';
import { shouldInvalidateWorkbenchSnapshot } from '../components/workbench/workbench-task-cache-policy';

describe('M08 snapshot authority and checkpoint proof', () => {
  it('preserves only transient network failures with unchanged authority', () => {
    expect(shouldInvalidateWorkbenchSnapshot(new Error('offline'), 'owner:role:core:rev', 'owner:role:core:rev')).toBe(false);
  });
  it('invalidates parent list after child checkpoint authorization revocation', () => {
    expect(shouldInvalidateWorkbenchSnapshot({ invalidatesSnapshot: true }, 'owner:role:core:rev', 'owner:role:core:rev')).toBe(true);
  });
  it('invalidates after owner/role/core/permission revision changes', () => {
    for (const current of ['other:role:core:rev', 'owner:other:core:rev', 'owner:role:other:rev', 'owner:role:core:other']) {
      expect(shouldInvalidateWorkbenchSnapshot(new Error('network failure'), 'owner:role:core:rev', current)).toBe(true);
    }
  });
  it('does not trust truthy but non-boolean invalidation indicators', () => {
    expect(shouldInvalidateWorkbenchSnapshot({ invalidatesSnapshot: 'true' }, 'same', 'same')).toBe(false);
  });
});
