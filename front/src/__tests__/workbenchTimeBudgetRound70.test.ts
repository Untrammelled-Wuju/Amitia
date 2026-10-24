import { describe, it, expect } from 'vitest';
import { workbenchTimeBudgetState, workbenchTimeBudgetNotice } from '../components/workbench/workbench-task-budget';

const base = { status: 'running', recoveryState: 'checkpoint-only', deadlineAt: '2026-10-23T12:00:00Z' };
const before = Date.parse('2026-10-23T11:59:59Z');
const after = Date.parse('2026-10-23T12:00:00Z');

describe('M08 read-only coordination deadline and hard budget disclosure', () => {
  it('distinguishes active and overdue without falsely claiming cancelled effects', () => {
    expect(workbenchTimeBudgetState(base, before)).toBe('within-deadline');
    expect(workbenchTimeBudgetState(base, after)).toBe('deadline-passed');
    expect(workbenchTimeBudgetNotice('deadline-passed')).toContain('仍须通过正式回执');
  });
  it('never interprets a terminal record as proof of timely completion', () => {
    expect(workbenchTimeBudgetState({ ...base, status: 'succeeded' }, after)).toBe('terminal');
    expect(workbenchTimeBudgetNotice('terminal')).toContain('不能证明');
  });
  it('does not invent parent token/fee budget or trust offline/unknown evidence', () => {
    expect(workbenchTimeBudgetState({ ...base, deadlineAt: undefined }, before)).toBe('unconfigured');
    expect(workbenchTimeBudgetState(base, before, true)).toBe('unverified');
    expect(workbenchTimeBudgetState({ ...base, recoveryState: 'needs-reconciliation' }, before)).toBe('unverified');
    expect(workbenchTimeBudgetState(base, NaN)).toBe('unverified');
    expect(workbenchTimeBudgetNotice('unconfigured')).toContain('Token');
  });
});
