import { apiClient } from '@/composables/useApi';
import { getApiBaseURLForPath } from '@/runtime/runtime-adapter';
import { encodeInput, type TerminalSnapshot } from './useWorkbenchTerminal';

const STREAM_PATH = '/api/workbench/terminal/stream';
const HANDSHAKE_TIMEOUT_MS = 12_000;

type StreamFrame = { type: 'snapshot'; snapshot: TerminalSnapshot };

export type TerminalStream = {
  close: () => void;
  input: (value: string) => boolean;
  resize: (rows: number, cols: number) => boolean;
};

function socketURL(apiBase: string, ticket: string, cursor: number): string {
  const url = new URL(STREAM_PATH, apiBase || window.location.origin);
  if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new Error('终端运行时不支持 WebSocket');
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  url.searchParams.set('ticket', ticket);
  url.searchParams.set('cursor', String(cursor));
  return url.toString();
}

export async function connectTerminalStream(
  id: string,
  conversationId: string,
  cursor: number,
  onSnapshot: (snapshot: TerminalSnapshot) => void,
  onDisconnected: () => void,
  signal?: AbortSignal,
): Promise<TerminalStream> {
  if (!Number.isSafeInteger(cursor) || cursor < 0) throw new Error('终端输出游标无效');
  if (signal?.aborted) throw new Error('终端连接已取消');
  const origin = window.location.protocol === 'file:' ? 'null' : window.location.origin;
  const response = await apiClient.post<{ ticket: string }>(
    `/api/workbench/terminal/${encodeURIComponent(id)}/stream-ticket`,
    { conversationId, origin },
    { signal, timeout: HANDSHAKE_TIMEOUT_MS },
  );
  if (signal?.aborted) throw new Error('终端连接已取消');
  if (!/^[a-f0-9]{48}$/.test(response.data.ticket)) throw new Error('终端临时凭据无效');
  const base = await getApiBaseURLForPath(STREAM_PATH);
  if (signal?.aborted) throw new Error('终端连接已取消');
  const socket = new WebSocket(socketURL(base, response.data.ticket, cursor));
  let disposed = false;
  let opened = false;
  let handshakeTimer: ReturnType<typeof setTimeout> | undefined;
  let abortListener: (() => void) | undefined;
  const clearHandshake = () => {
    if (handshakeTimer !== undefined) clearTimeout(handshakeTimer);
    handshakeTimer = undefined;
  };
  const close = () => {
    if (disposed) return;
    disposed = true;
    clearHandshake();
    if (abortListener) signal?.removeEventListener('abort', abortListener);
    socket.close();
  };
  const send = (message: Record<string, unknown>) => {
    if (disposed || socket.readyState !== WebSocket.OPEN) return false;
    socket.send(JSON.stringify(message));
    return true;
  };
  const result: TerminalStream = {
    close,
    input: (value) => new TextEncoder().encode(value).length <= 4096 && send({ type: 'input', data: encodeInput(value) }),
    resize: (rows, cols) => Number.isInteger(rows) && Number.isInteger(cols) && rows >= 10 && rows <= 120 && cols >= 20 && cols <= 300 && send({ type: 'resize', rows, cols }),
  };
  return new Promise<TerminalStream>((resolve, reject) => {
    const fail = () => {
      clearHandshake();
      if (abortListener) signal?.removeEventListener('abort', abortListener);
      if (!opened && !disposed) reject(new Error('终端实时连接不可用'));
      else if (!disposed && opened) onDisconnected();
    };
    socket.onopen = () => {
      if (disposed) return;
      opened = true;
      clearHandshake();
      resolve(result);
    };
    socket.onclose = fail;
    socket.onerror = () => { if (!opened) socket.close(); };
    socket.onmessage = (event) => {
      if (disposed || typeof event.data !== 'string') return;
      try {
        const frame = JSON.parse(event.data) as StreamFrame;
        if (frame?.type === 'snapshot' && frame.snapshot?.id === id &&
          Number.isSafeInteger(frame.snapshot.cursor) && frame.snapshot.cursor >= 0) {
          onSnapshot(frame.snapshot);
        }
      } catch { socket.close(); }
    };
    abortListener = () => {
      close();
      if (!opened) reject(new Error('终端连接已取消'));
    };
    signal?.addEventListener('abort', abortListener, { once: true });
    if (signal?.aborted) {
      abortListener();
      return;
    }
    handshakeTimer = setTimeout(() => {
      if (opened || disposed) return;
      close();
      reject(new Error('终端实时连接握手超时'));
    }, HANDSHAKE_TIMEOUT_MS);
  });
}
