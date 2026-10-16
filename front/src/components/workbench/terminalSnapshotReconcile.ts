import type { TerminalSnapshot } from './useWorkbenchTerminal';

export type ReconciledTerminalChunk =
  | { kind: 'ignore'; cursor: number; truncated: false; bytes: Uint8Array }
  | { kind: 'apply'; cursor: number; truncated: boolean; bytes: Uint8Array };

export function reconcileTerminalChunk(snapshot: TerminalSnapshot, currentCursor: number): ReconciledTerminalChunk {
  if (!Number.isSafeInteger(currentCursor) || currentCursor < 0 ||
      !Number.isSafeInteger(snapshot.cursor) || snapshot.cursor < 0 ||
      typeof snapshot.data !== 'string' || snapshot.data.length > 90_000 ||
      !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(snapshot.data)) {
    throw new Error('终端输出分片协议无效');
  }
  const binary = atob(snapshot.data);
  const start = snapshot.fromCursor ?? snapshot.cursor - binary.length;
  if (!Number.isSafeInteger(start) || start < 0 || start > snapshot.cursor ||
      snapshot.cursor - start !== binary.length) {
    throw new Error('终端输出分片游标不一致');
  }
  if (snapshot.cursor < currentCursor) {
    return { kind: 'ignore', cursor: currentCursor, truncated: false, bytes: new Uint8Array(0) };
  }
  if (start > currentCursor && !snapshot.truncated) {
    throw new Error('终端输出存在缺口，需要从当前游标重新读取');
  }
  const overlap = Math.max(0, currentCursor - start);
  const bytes = Uint8Array.from(binary.slice(overlap), char => char.charCodeAt(0));
  return { kind: 'apply', cursor: snapshot.cursor, truncated: Boolean(snapshot.truncated && start > currentCursor), bytes };
}

export function reconcileTerminalStatus(
  ended: boolean,
  previousExitCode: number,
  next: Pick<TerminalSnapshot, 'running' | 'exitCode'>,
): { ended: boolean; running: boolean; exitCode: number } {
  const nowEnded = ended || !next.running;
  return { ended: nowEnded, running: !nowEnded, exitCode: ended ? previousExitCode : next.exitCode };
}
