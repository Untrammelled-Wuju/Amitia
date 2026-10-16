import { describe, it, expect } from 'vitest';
import { parseTaskCheckpoints } from '../components/workbench/workbench-task-checkpoints';

const point = {updatedAt: '2026-10-14T11:00:00Z', interactionStatus: 'processing', coordinationStatus: 'running', assignmentCount: 2, completedCount: 1, failedCount: 0, parentTerminal: false, consistent: true};
const payload = {protocolVersion:1, conversationId:'conversation', coordinationId:'coord', source:'sampled-durable-checkpoints', records:[point], truncated:false};

describe('workbench task durable checkpoint protocol', () => {
  it('accepts a correctly scoped bounded response', () => {
    expect(parseTaskCheckpoints(payload,'conversation','coord').records[0].completedCount).toBe(1);
  });
  it('rejects mismatched identity and forged state', () => {
    expect(() => parseTaskCheckpoints(payload,'foreign','coord')).toThrow();
    expect(() => parseTaskCheckpoints({...payload,records:[{...point, completedCount:-1}]},'conversation','coord')).toThrow();
    expect(() => parseTaskCheckpoints({...payload,records:new Array(21).fill(point)},'conversation','coord')).toThrow();
    expect(() => parseTaskCheckpoints({...payload,source:'live-runtime'},'conversation','coord')).toThrow();
  });
});
