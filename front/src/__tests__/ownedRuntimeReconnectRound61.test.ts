import { afterEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

const m = vi.hoisted(() => ({
  snapshot: vi.fn(),
  prepare: vi.fn(),
  eventFetch: vi.fn(),
}));

vi.mock('@/composables/useDeviceOwnedConversation', () => ({
  useDeviceOwnedConversation: () => ({ coreId: ref('core'), policy: ref({}) }),
}));
vi.mock('@/runtime/owned-chat-management', () => ({
  ownedChatRuntimeRequest: m.snapshot,
  prepareOwnedChatRequest: m.prepare,
}));
vi.mock('@/runtime/runtime-adapter', () => ({ resolveApiUrl: vi.fn(async () => '/events') }));
vi.mock('@/runtime/request-auth', () => ({ createAuthenticatedFetchInit: vi.fn(async (_path: string, init: unknown) => init) }));
vi.mock('@/runtime/desktop-pet-chat-state', () => ({ notifyDesktopPetChatState: vi.fn() }));
vi.mock('@/runtime/workbench-agent-event', () => ({ publishWorkbenchEvent: vi.fn() }));
vi.mock('@/composables/useReplyNotifications', () => ({ notifyReplyCompleted: vi.fn() }));

import { useConversationRuntime } from '@/composables/useConversationRuntime';

function stream(payload: string): Response {
  const encoded = new TextEncoder().encode(payload);
  return {
    ok: true,
    status: 200,
    headers: { get: () => 'text/event-stream' },
    body: new ReadableStream<Uint8Array>({
      start(controller) { controller.enqueue(encoded); controller.close(); },
    }),
  } as unknown as Response;
}

describe('Round 61 M08: mandatory snapshot before reconnecting a broken SSE replay', () => {
  afterEach(() => { vi.unstubAllGlobals(); vi.useRealTimers(); vi.clearAllMocks(); });

  it('does not reopen SSE until a failed snapshot recovery has subsequently succeeded', async () => {
    vi.useFakeTimers();
    m.prepare.mockResolvedValue({ payload: {}, expected: { coreId: 'core' } });
    m.snapshot.mockRejectedValueOnce(new Error('snapshot unavailable'));
    m.snapshot.mockRejectedValueOnce(new Error('snapshot still unavailable'));
    m.snapshot.mockResolvedValue({ version: 1, lastEventSequence: 0, messages: [], turns: [], approvals: [] });
    const fetcher = vi.fn()
      .mockResolvedValueOnce(stream('event: snapshot_required\ndata: {}\n\n'))
      .mockImplementation(() => new Promise(() => {}));
    vi.stubGlobal('fetch', fetcher);
    const runtime = useConversationRuntime(ref('conv'), ref<any[]>([]), ref(false), () => undefined);
    try {
      await runtime.connect(false);
      expect(m.snapshot).toHaveBeenCalledTimes(1);
      expect(fetcher).toHaveBeenCalledTimes(1);
      await vi.advanceTimersByTimeAsync(1500);
      expect(m.snapshot).toHaveBeenCalledTimes(2);
      expect(fetcher).toHaveBeenCalledTimes(1);
      await vi.advanceTimersByTimeAsync(1500);
      expect(m.snapshot).toHaveBeenCalledTimes(3);
      expect(fetcher).toHaveBeenCalledTimes(2);
    } finally {
      runtime.cleanup();
    }
  });
  it.each([401, 403])('discards prior authorized conversation data on HTTP %s without automatic reconnect', async (status) => {
    vi.useFakeTimers();
    m.prepare.mockResolvedValue({ payload: {}, expected: { coreId: 'core' } });
    const cancel = vi.fn(async () => undefined);
    const fetcher = vi.fn().mockResolvedValue({ ok: false, status, headers: { get: () => '' }, body: { cancel } });
    vi.stubGlobal('fetch', fetcher);
    const persisted = ref<any[]>([{ id: 'private-message', role: 'assistant', content: 'private', createdAt: '2026-10-23T00:00:00Z' }]);
    const revoked = vi.fn();
    window.addEventListener('amitia:workbench-task-revoked', revoked);
    const runtime = useConversationRuntime(ref('conv'), persisted, ref(false), () => undefined);
    try {
      await runtime.connect(false);
      expect(cancel).toHaveBeenCalledTimes(1);
      expect(persisted.value).toEqual([]);
      expect(runtime.messages.value).toEqual([]);
      expect(revoked).toHaveBeenCalledTimes(1);
      expect((revoked.mock.calls[0][0] as CustomEvent).detail.conversationId).toBe('conv');
      await vi.advanceTimersByTimeAsync(4500);
      expect(fetcher).toHaveBeenCalledTimes(1);
      expect(m.snapshot).not.toHaveBeenCalled();
    } finally {
      runtime.cleanup();
      window.removeEventListener('amitia:workbench-task-revoked', revoked);
    }
  });

  it('requires fresh authorization snapshot before replay after HTTP 409', async () => {
    vi.useFakeTimers();
    m.prepare.mockResolvedValue({ payload: {}, expected: { coreId: 'core' } });
    m.snapshot.mockResolvedValue({ version: 1, lastEventSequence: 0, messages: [], turns: [], approvals: [] });
    const cancel = vi.fn(async () => undefined);
    const fetcher = vi.fn()
      .mockResolvedValueOnce({ ok: false, status: 409, headers: { get: () => '' }, body: { cancel } })
      .mockImplementation(() => new Promise(() => {}));
    vi.stubGlobal('fetch', fetcher);
    const runtime = useConversationRuntime(ref('conv'), ref<any[]>([]), ref(false), () => undefined);
    try {
      await runtime.connect(false);
      expect(cancel).toHaveBeenCalledTimes(1);
      expect(fetcher).toHaveBeenCalledTimes(1);
      expect(m.snapshot).not.toHaveBeenCalled();
      await vi.advanceTimersByTimeAsync(1500);
      expect(m.snapshot).toHaveBeenCalledTimes(1);
      expect(fetcher).toHaveBeenCalledTimes(2);
    } finally {
      runtime.cleanup();
    }
  });

});
