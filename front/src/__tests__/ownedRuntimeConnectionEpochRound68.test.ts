import { afterEach, describe, expect, it, vi } from 'vitest';
import { ref } from 'vue';

const mocks = vi.hoisted(() => ({
  prepare: vi.fn(),
  snapshot: vi.fn(),
}));

vi.mock('@/composables/useDeviceOwnedConversation', () => ({
  useDeviceOwnedConversation: () => ({ coreId: ref('core'), policy: ref({}) }),
}));
vi.mock('@/runtime/owned-chat-management', () => ({
  ownedChatRuntimeRequest: mocks.snapshot,
  prepareOwnedChatRequest: mocks.prepare,
}));
vi.mock('@/runtime/runtime-adapter', () => ({ resolveApiUrl: vi.fn(async () => '/events') }));
vi.mock('@/runtime/request-auth', () => ({ createAuthenticatedFetchInit: vi.fn(async (_path: string, init: unknown) => init) }));
vi.mock('@/runtime/desktop-pet-chat-state', () => ({ notifyDesktopPetChatState: vi.fn() }));
vi.mock('@/runtime/workbench-agent-event', () => ({ publishWorkbenchEvent: vi.fn() }));
vi.mock('@/composables/useReplyNotifications', () => ({ notifyReplyCompleted: vi.fn() }));

import { useConversationRuntime } from '@/composables/useConversationRuntime';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

const prepared = { payload: {}, expected: { coreId: 'core' } };

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

describe('Round 68 M08 SSE connection epochs', () => {
  afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks(); });

  it('reuses the same in-flight authorization and SSE connection', async () => {
    const authorization = deferred<typeof prepared>();
    mocks.prepare.mockImplementationOnce(() => authorization.promise);
    const fetcher = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal('fetch', fetcher);
    const runtime = useConversationRuntime(ref('conv'), ref<any[]>([]), ref(false), () => undefined);
    try {
      const first = runtime.connect(false);
      await vi.waitFor(() => expect(mocks.prepare).toHaveBeenCalledTimes(1));
      const second = runtime.connect(false);
      expect(second).toBe(first);
      authorization.resolve(prepared);
      await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
      expect(mocks.prepare).toHaveBeenCalledTimes(1);
    } finally { runtime.cleanup(); }
  });

  it('does not reopen an active stream when a refresh event arrives', async () => {
    mocks.prepare.mockResolvedValue(prepared);
    const fetcher = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal('fetch', fetcher);
    const runtime = useConversationRuntime(ref('conv'), ref<any[]>([]), ref(false), () => undefined);
    try {
      void runtime.connect(false);
      await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
      window.dispatchEvent(new CustomEvent('amitia:owned-runtime-refresh', { detail: { conversationId: 'conv' } }));
      await new Promise((resolve) => setTimeout(resolve, 20));
      expect(fetcher).toHaveBeenCalledTimes(1);
    } finally { runtime.cleanup(); }
  });

  it('accepts the canonical server conversation id for a referenced conversation', async () => {
    const conversation = ref('meshconv1:space:conversation');
    mocks.prepare.mockResolvedValue({ payload: { conversationId: 'conversation' }, expected: { coreId: 'core' } });
    const fetcher = vi.fn().mockResolvedValue(stream('event: agent_event\ndata: {"version":1,"eventId":"event-1","eventSequence":1,"conversationId":"conversation","turnId":"turn-1","type":"turn.started","status":"running","executionScope":{"coreId":"core"}}\n\n'));
    vi.stubGlobal('fetch', fetcher);
    const runtime = useConversationRuntime(conversation, ref<any[]>([]), ref(false), () => undefined);
    try {
      await runtime.connect(false);
      await vi.waitFor(() => expect(runtime.messages.value.some((message) => message.turnId === 'turn-1' && message.conversationId === conversation.value)).toBe(true));
      expect(mocks.snapshot).not.toHaveBeenCalled();
    } finally { runtime.cleanup(); }
  });

  it('revokes an active forbidden preparation without reconnecting', async () => {
    mocks.prepare.mockRejectedValue({ response: { status: 401 } });
    const fetcher = vi.fn();
    vi.stubGlobal('fetch', fetcher);
    const stored = ref<any[]>([{ id: 'private', content: 'secret', role: 'assistant' }]);
    const revoked = vi.fn();
    window.addEventListener('amitia:workbench-task-revoked', revoked);
    const runtime = useConversationRuntime(ref('conv'), stored, ref(false), () => undefined);
    try {
      await runtime.connect(false);
      expect(stored.value).toEqual([]);
      expect(revoked).toHaveBeenCalledTimes(1);
      expect(fetcher).not.toHaveBeenCalled();
    } finally { runtime.cleanup(); window.removeEventListener('amitia:workbench-task-revoked', revoked); }
  });

  it.each(['amitia:runtime-connection-changed', 'amitia:execution-scope-changed'])('clears cached conversation on %s', async (eventName) => {
    mocks.prepare.mockResolvedValue(prepared);
    const fetcher = vi.fn(() => new Promise<Response>(() => {}));
    vi.stubGlobal('fetch', fetcher);
    const stored = ref<any[]>([{ id: 'private', content: 'secret', role: 'assistant' }]);
    const runtime = useConversationRuntime(ref('conv'), stored, ref(false), () => undefined);
    try {
      void runtime.connect(false);
      await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));
      window.dispatchEvent(new Event(eventName));
      expect(stored.value).toEqual([]);
      expect(runtime.messages.value).toEqual([]);
    } finally { runtime.cleanup(); }
  });
});
