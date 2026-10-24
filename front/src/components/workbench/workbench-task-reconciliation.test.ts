import { describe, expect, it } from 'vitest';
import { parseBudgetEvidencePage } from './workbench-task-reconciliation';

const expected = { coordinationId: 'coord-a', parentInteractionId: 'parent-a', kind: 'model' as const };
const base = { protocolVersion: 1, source: 'durable-budget-ledger', coordinationId: 'coord-a', parentInteractionId: 'parent-a', kind: 'model', nextPagePossible: false };

describe('M08 Workbench Owner budget evidence', () => {
  it('accepts an explicitly scoped persisted model reservation', () => {
    const page = parseBudgetEvidencePage({ ...base, records: [{ id: 'model-a', status: 'interrupted', startedAt: '2026-10-23T16:00:00Z', reservedTokens: 512, reservedCostMicroUSD: 140, usageReported: false }] }, expected);
    expect(page.records).toEqual([{ id: 'model-a', status: 'interrupted', startedAt: '2026-10-23T16:00:00Z', reservedTokens: 512, reservedCostMicroUSD: 140, usageReported: false, budgetOverrun: false, toolName: '', reviewEvidenceRef: '', reviewActorId: '', reviewRecordedAt: '' }]);
    expect(page.next).toBeUndefined();
  });

  it('does not accept a foreign parent, forged source, missing cost or malformed cursor', () => {
    for (const changed of [
      { parentInteractionId: 'foreign-parent' },
      { source: 'frontend-calculated' },
      { records: [{ id: 'model-a', status: 'running', startedAt: '2026-10-23T16:00:00Z', reservedTokens: 128, usageReported: false }] },
      { nextPagePossible: true, records: [], nextAfterSequence: 1 },
    ]) {
      const valid = { ...base, records: [], ...changed };
      expect(() => parseBudgetEvidencePage(valid, expected)).toThrow();
    }
  });

  it('shows manual evidence references without converting them into confirmed spending', () => {
    const page = parseBudgetEvidencePage({ ...base, records: [{ id: 'model-a', status: 'interrupted', startedAt: '2026-10-23T16:00:00Z', reservedTokens: 512, reservedCostMicroUSD: 140, usageReported: false, reviewEvidenceRef: 'case:provider-report-1234', reviewActorId: 'device-a', reviewRecordedAt: '2026-10-23T17:00:00Z' }] }, expected);
    expect(page.records[0].reservedTokens).toBe(512);
    expect(page.records[0].usageReported).toBe(false);
    expect(page.records[0].reviewEvidenceRef).toBe('case:provider-report-1234');
    expect(() => parseBudgetEvidencePage({ ...base, records: [{ id: 'model-a', status: 'interrupted', startedAt: '2026-10-23T16:00:00Z', reservedTokens: 512, reservedCostMicroUSD: 140, usageReported: false, reviewEvidenceRef: 'case:fake', reviewActorId: '', reviewRecordedAt: '' }] }, expected)).toThrow();
  });


  it('flags provider-reported budget overruns without treating them as settled usage', () => {
    const page = parseBudgetEvidencePage({ ...base, records: [{ id: 'model-overrun', status: 'succeeded', startedAt: '2026-10-23T18:00:00Z', reservedTokens: 160, reservedCostMicroUSD: 500, usageReported: false, budgetOverrun: true }] }, expected);
    expect(page.records[0].budgetOverrun).toBe(true);
    expect(page.records[0].usageReported).toBe(false);
    expect(() => parseBudgetEvidencePage({ ...base, records: [{ id: 'model-overrun', status: 'succeeded', startedAt: '2026-10-23T18:00:00Z', reservedTokens: 160, reservedCostMicroUSD: 500, usageReported: true, budgetOverrun: true }] }, expected)).toThrow();
    expect(() => parseBudgetEvidencePage({ ...base, records: [{ id: 'model-overrun', status: 'succeeded', startedAt: '2026-10-23T18:00:00Z', reservedTokens: 160, reservedCostMicroUSD: 500, usageReported: false, budgetOverrun: 'true' }] }, expected)).toThrow();
  });

  it('treats tool effects as unconfirmed records without inventing a completion receipt', () => {
    const page = parseBudgetEvidencePage({ ...base, kind: 'tool', records: [{ EffectID: 'effect-1', ToolName: 'shell_run', CreatedAt: '2026-10-23T16:00:00Z' }] }, { ...expected, kind: 'tool' });
    expect(page.records[0].status).toBe('unconfirmed');
    expect(page.records[0].id).toBe('effect-1');
    expect(page.records[0].reservedTokens).toBe(0);
  });
});
