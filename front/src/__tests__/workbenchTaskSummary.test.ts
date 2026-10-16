import { describe, it, expect } from 'vitest';
import { normalizeTaskSummary } from '../components/workbench/workbench-task-summary';

describe('task summary protocol', () => {
  it('initializes unavailable snapshots safely', () => {
    expect(normalizeTaskSummary(null).total).toBe(0);
    expect(normalizeTaskSummary({total: -2, active: NaN, failed: '6'}).failed).toBe(0);
  });
  it('preserves valid nonnegative integer counts', () => {
    expect(normalizeTaskSummary({total: 8, active: 3, needsReconciliation: 2})).toMatchObject({total: 8, active: 3, needsReconciliation: 2, failed: 0});
  });
});
