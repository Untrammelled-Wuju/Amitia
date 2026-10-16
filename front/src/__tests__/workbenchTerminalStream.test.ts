import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { connectTerminalStream } from '@/components/workbench/useWorkbenchTerminalStream';

const post = vi.fn();
vi.mock('@/composables/useApi', () => ({ apiClient: { post: (...args: unknown[]) => post(...args) } }));
vi.mock('@/runtime/runtime-adapter', () => ({ getApiBaseURLForPath: async () => 'http://127.0.0.1:18899' }));

class SocketFixture {
  static OPEN = 1;
  static current: SocketFixture;
  readyState = 0;
  readonly messages: unknown[] = [];
  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  constructor(readonly url: string) { SocketFixture.current = this; queueMicrotask(() => { this.readyState = 1; this.onopen?.(); }); }
  send(message: string) { this.messages.push(JSON.parse(message)); }
  close() { this.readyState = 3; this.onclose?.(); }
}

describe('workbench terminal stream transport', () => {
  beforeEach(() => {
    post.mockResolvedValue({ data: { ticket: 'a'.repeat(48) } });
    vi.stubGlobal('WebSocket', SocketFixture);
  });
  afterEach(() => { vi.unstubAllGlobals(); post.mockReset(); });

  it('acquires a scoped ticket and streams input and resize', async () => {
    const updates: string[] = [];
    const transport = await connectTerminalStream('session', 'conversation', 17, (snap) => updates.push(snap.id), () => {});
    expect(post).toHaveBeenCalledWith(
      '/api/workbench/terminal/session/stream-ticket',
      { conversationId: 'conversation', origin: window.location.origin },
      { signal: undefined, timeout: 12_000 },
    );
    expect(SocketFixture.current.url).toContain('ws://127.0.0.1:18899/api/workbench/terminal/stream?');
    expect(SocketFixture.current.url).toContain('cursor=17');
    expect(transport.input('你好\n')).toBe(true);
    expect(transport.resize(30, 100)).toBe(true);
    expect(SocketFixture.current.messages).toHaveLength(2);
    SocketFixture.current.onmessage?.({ data: JSON.stringify({ type: 'snapshot', snapshot: { id: 'session', cursor: 19 } }) });
    SocketFixture.current.onmessage?.({ data: JSON.stringify({ type: 'snapshot', snapshot: { id: 'other', cursor: 19 } }) });
    expect(updates).toEqual(['session']);
    transport.close();
    expect(transport.input('later')).toBe(false);
  });

  it('rejects invalid ticket and dimensions', async () => {
    post.mockResolvedValueOnce({ data: { ticket: '../invalid' } });
    await expect(connectTerminalStream('a', 'b', 0, () => {}, () => {})).rejects.toThrow('凭据');
    const transport = await connectTerminalStream('a', 'b', 0, () => {}, () => {});
    expect(transport.resize(1, 1)).toBe(false);
    expect(transport.input('x'.repeat(4097))).toBe(false);
    transport.close();
  });

  it('aborts a connecting socket and rejects without a disconnect notification', async () => {
    class SilentSocket {
      static OPEN = 1;
      readyState = 0;
      onopen: (() => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((event: { data: string }) => void) | null = null;
      constructor(readonly url: string) { SocketFixture.current = this as unknown as SocketFixture; }
      send(_message: string) {}
      close() { this.readyState = 3; this.onclose?.(); }
    }
    vi.stubGlobal('WebSocket', SilentSocket);
    const controller = new AbortController();
    const disconnected = vi.fn();
    const pending = connectTerminalStream('session', 'conversation', 0, () => {}, disconnected, controller.signal);
    await vi.waitFor(() => expect(SocketFixture.current).toBeInstanceOf(SilentSocket));
    controller.abort();
    await expect(pending).rejects.toThrow('取消');
    expect(SocketFixture.current.readyState).toBe(3);
    expect(disconnected).not.toHaveBeenCalled();
  });

  it('times out a stalled socket handshake and closes the unclaimed transport', async () => {
    class UnresponsiveSocket {
      static OPEN = 1;
      readyState = 0;
      onopen: (() => void) | null = null;
      onclose: (() => void) | null = null;
      onerror: (() => void) | null = null;
      onmessage: ((event: { data: string }) => void) | null = null;
      constructor(readonly url: string) { SocketFixture.current = this as unknown as SocketFixture; }
      send(_message: string) {}
      close() { this.readyState = 3; this.onclose?.(); }
    }
    vi.stubGlobal('WebSocket', UnresponsiveSocket);
    vi.useFakeTimers();
    try {
      const pending = connectTerminalStream('session', 'conversation', 0, () => {}, () => {});
      const rejection = expect(pending).rejects.toThrow('超时');
      await Promise.resolve();
      await Promise.resolve();
      await vi.advanceTimersByTimeAsync(12_100);
      await rejection;
      expect(SocketFixture.current.readyState).toBe(3);
    } finally { vi.useRealTimers(); }
  });

  it('refuses an already aborted signal or invalid cursor before requesting tickets', async () => {
    const controller = new AbortController();
    controller.abort();
    await expect(connectTerminalStream('session', 'conversation', 0, () => {}, () => {}, controller.signal)).rejects.toThrow('取消');
    await expect(connectTerminalStream('session', 'conversation', -1, () => {}, () => {})).rejects.toThrow('游标');
    expect(post).not.toHaveBeenCalled();
  });

  it('ignores late token results after cancellation', async () => {
    let resolveToken!: (value: unknown) => void;
    post.mockImplementationOnce(() => new Promise(resolve => { resolveToken = resolve; }));
    const controller = new AbortController();
    const pending = connectTerminalStream('session', 'conversation', 0, () => {}, () => {}, controller.signal);
    controller.abort();
    resolveToken({ data: { ticket: 'a'.repeat(48) } });
    await expect(pending).rejects.toThrow('取消');
  });

});
