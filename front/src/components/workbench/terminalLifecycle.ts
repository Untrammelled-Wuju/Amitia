import type { TerminalLifecycleEvent, TerminalLifecyclePage } from './useWorkbenchTerminal';

const eventKinds = new Set(['session.started', 'session.resized', 'process.exited']);

export function parseTerminalLifecyclePage(value: unknown, sessionId: string): TerminalLifecyclePage {
  if (!value || typeof value !== 'object') throw new Error('终端生命周期数据无效');
  const page = value as Record<string, unknown>;
  if (page.sessionId !== sessionId || page.scope !== 'live-session-memory-only' ||
      typeof page.latestSequence !== 'number' || !Number.isSafeInteger(page.latestSequence) || page.latestSequence < 0 ||
      typeof page.truncated !== 'boolean' || !Array.isArray(page.events) || page.events.length > 64) {
    throw new Error('终端生命周期数据不匹配');
  }
  let previous = 0;
  const events: TerminalLifecycleEvent[] = page.events.map((item: unknown) => {
    if (!item || typeof item !== 'object') throw new Error('终端事件无效');
    const row = item as Record<string, unknown>;
    if (typeof row.sequence !== 'number' || !Number.isSafeInteger(row.sequence) || row.sequence <= previous || row.sequence > (page.latestSequence as number) ||
        typeof row.kind !== 'string' || !eventKinds.has(row.kind) || typeof row.at !== 'string' || !Number.isFinite(Date.parse(row.at))) {
      throw new Error('终端事件顺序或类型无效');
    }
    previous = row.sequence;
    if (row.kind === 'process.exited' && (!Number.isInteger(row.exitCode))) throw new Error('终端退出事件缺少退出码');
    if (row.kind !== 'process.exited' &&
        (!Number.isInteger(row.rows) || !Number.isInteger(row.cols) || (row.rows as number) < 10 || (row.rows as number) > 120 || (row.cols as number) < 20 || (row.cols as number) > 300)) {
      throw new Error('终端窗口记录无效');
    }
    return item as TerminalLifecycleEvent;
  });
  return page as unknown as TerminalLifecyclePage;
}

export function appendLifecycleEvents(current: TerminalLifecycleEvent[], next: TerminalLifecyclePage): TerminalLifecycleEvent[] {
  const seen = new Set(current.map(item => item.sequence));
  const result = [...current];
  for (const event of next.events) {
    if (!seen.has(event.sequence)) { seen.add(event.sequence); result.push(event); }
  }
  result.sort((a, b) => a.sequence - b.sequence);
  return result.slice(-64);
}
