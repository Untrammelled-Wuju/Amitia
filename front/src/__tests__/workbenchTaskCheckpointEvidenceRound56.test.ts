import { describe, it, expect } from 'vitest';
import { parseTaskCheckpoints } from '../components/workbench/workbench-task-checkpoints';

const checkpoint = { updatedAt:'2026-10-23T00:00:00Z', interactionStatus:'running', coordinationStatus:'running', assignmentCount:1, completedCount:0, failedCount:0, parentTerminal:false, consistent:true };
function reply(record = checkpoint) {
  return { protocolVersion:1, conversationId:'conv', coordinationId:'coord', source:'sampled-durable-checkpoints', truncated:false, records:[record] };
}
describe('M08 authoritative checkpoint evidence', () => {
  it('accepts verified v1 history', () => { expect(parseTaskCheckpoints(reply(),'conv','coord').records.length).toBe(1); });
  it('rejects cross-conversation and cross-coordination history', () => {
    expect(() => parseTaskCheckpoints(reply(),'other','coord')).toThrow();
    expect(() => parseTaskCheckpoints(reply(),'conv','other')).toThrow();
  });
  it('rejects malformed persisted terminal statistics', () => {
    expect(() => parseTaskCheckpoints(reply({...checkpoint,completedCount:2}),'conv','coord')).toThrow();
  });
});
