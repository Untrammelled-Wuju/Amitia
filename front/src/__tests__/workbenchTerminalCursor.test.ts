import { describe, expect, it } from 'vitest';
import { reconcileTerminalChunk, reconcileTerminalStatus } from '@/components/workbench/terminalSnapshotReconcile';
import type { TerminalSnapshot } from '@/components/workbench/useWorkbenchTerminal';

function frame(value: string, fromCursor: number, truncated = false): TerminalSnapshot {
  const bytes = new TextEncoder().encode(value);
  const encoded = btoa(Array.from(bytes, code => String.fromCharCode(code)).join(''));
  return { id: 'terminal', mode: 'pty', running: true, exitCode: -1, data: encoded,
    fromCursor, cursor: fromCursor + bytes.length, truncated };
}

describe('Workbench 终端跨传输字节级游标恢复', () => {
  it('重复和交叠分片只添加新字节', () => {
    const first = reconcileTerminalChunk(frame('12345', 0), 0);
    const overlap = reconcileTerminalChunk(frame('34567', 2), first.cursor);
    expect(new TextDecoder().decode(overlap.bytes)).toBe('67');
    expect(reconcileTerminalChunk(frame('12345', 0), overlap.cursor).kind).toBe('ignore');
  });
  it('仅接受带截断标记的真实输出缺口', () => {
    expect(() => reconcileTerminalChunk(frame('abc', 10), 0)).toThrow('缺口');
    const replay = reconcileTerminalChunk(frame('abc', 10, true), 0);
    expect(replay.truncated).toBe(true);
    expect(new TextDecoder().decode(replay.bytes)).toBe('abc');
  });
  it('结束状态不能被晚到的运行中快照复活', () => {
    const ended = reconcileTerminalStatus(false, -1, { running: false, exitCode: 2 });
    expect(reconcileTerminalStatus(ended.ended, ended.exitCode, { running: true, exitCode: -1 })).toMatchObject({ running: false, exitCode: 2 });
  });
  it('支持旧端没有 fromCursor 的字节数据', () => {
    const old = frame('中文', 2);
    delete old.fromCursor;
    expect(reconcileTerminalChunk(old, 2).bytes).toHaveLength(6);
  });
});
