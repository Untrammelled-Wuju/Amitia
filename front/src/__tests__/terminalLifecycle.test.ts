import { describe, expect, it } from 'vitest';
import { appendLifecycleEvents, parseTerminalLifecyclePage } from '@/components/workbench/terminalLifecycle';

const createPage = (events: unknown[] = []) => ({
  sessionId: 'a'.repeat(40), scope: 'live-session-memory-only', latestSequence: 4,
  truncated: false, events,
});
const row = (sequence: number) => ({ sequence, kind: 'session.started', rows: 30, cols: 90, at: '2026-10-14T11:00:00Z' });

describe('terminal lifecycle protocol', () => {
  it('accepts sorted scoped events and merges without duplicates', () => {
    const first = parseTerminalLifecyclePage(createPage([row(1), row(2)]), 'a'.repeat(40));
    const second = parseTerminalLifecyclePage(createPage([row(2), row(4)]), 'a'.repeat(40));
    expect(appendLifecycleEvents(first.events, second).map(event => event.sequence)).toEqual([1, 2, 4]);
  });
  it('rejects cross-session and malformed lifecycle data', () => {
    expect(() => parseTerminalLifecyclePage(createPage(), 'b'.repeat(40))).toThrow();
    expect(() => parseTerminalLifecyclePage(createPage([row(3), row(2)]), 'a'.repeat(40))).toThrow();
    expect(() => parseTerminalLifecyclePage(createPage([{ ...row(1), kind: 'input.data' }]), 'a'.repeat(40))).toThrow();
    expect(() => parseTerminalLifecyclePage(createPage([{ ...row(1), rows: 999 }]), 'a'.repeat(40))).toThrow();
    expect(() => parseTerminalLifecyclePage(createPage([{ ...row(1), kind: 'process.exited' }]), 'a'.repeat(40))).toThrow();
  });
});
