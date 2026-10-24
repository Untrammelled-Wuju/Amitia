import { describe, expect, it } from 'vitest';
import { OwnedRuntimeSSEDecoder } from '../runtime/owned-runtime-sse';

describe('Owner Runtime SSE production contract', () => {
  it('accepts the actual Go wire event names, not previous aliases', () => {
    const decoder = new OwnedRuntimeSSEDecoder();
    expect(decoder.push('event: agent_event\ndata: {"eventSequence":1}\n\n')).toEqual([
      { type: 'agent_event', data: '{"eventSequence":1}' },
    ]);
    expect(decoder.push('event: snapshot_required\ndata: {}\n\n')).toEqual([
      { type: 'snapshot_required', data: '{}' },
    ]);
    expect(() => decoder.push('event: agent_ui_event\ndata: {}\n\n')).toThrow();
    expect(() => decoder.push('event: snapshot.required\ndata: {}\n\n')).toThrow();
  });
  it('reassembles CRLF, split CRLF boundaries and multiline data', () => {
    const decoder = new OwnedRuntimeSSEDecoder();
    expect(decoder.push('event: agent_event\r\ndata: {"message":"你')).toEqual([]);
    expect(decoder.push('好"}\r')).toEqual([]);
    expect(decoder.push('\n\r')).toEqual([]);
    expect(decoder.push('\n')).toEqual([{ type: 'agent_event', data: '{"message":"你好"}' }]);
    expect(decoder.push('event: agent_event\ndata: one\ndata: two\n\n')).toEqual([{ type: 'agent_event', data: 'one\ntwo' }]);
  });
  it('skips heartbeat comments and validates malformed frames', () => {
    const decoder = new OwnedRuntimeSSEDecoder();
    expect(decoder.push(': heartbeat\n\n')).toEqual([]);
    expect(() => decoder.push('event: agent_event\n\n')).toThrow();
    expect(() => new OwnedRuntimeSSEDecoder().push('x'.repeat(1024 * 1024 + 1))).toThrow();
  });
  it('accepts a complete frame terminated by CR-only at stream end', () => {
    const decoder = new OwnedRuntimeSSEDecoder();
    expect(decoder.push('event: snapshot_required\rdata: {}\r\r')).toEqual([]);
    expect(decoder.finish()).toEqual([{ type: 'snapshot_required', data: '{}' }]);
  });
  it('ignores unfinished event rather than manufacturing a terminal state', () => {
    const decoder = new OwnedRuntimeSSEDecoder();
    expect(decoder.push('event: agent_event\ndata: {"eventSequence":1}')).toEqual([]);
    expect(decoder.finish()).toEqual([]);
  });
});
