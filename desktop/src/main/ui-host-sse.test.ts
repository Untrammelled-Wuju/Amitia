import { beforeEach, expect, it, vi } from 'vitest';
import type { BrowserWindow } from 'electron';
import type { BusinessCoreClient } from './business-core-client';

const notifications = vi.hoisted(() => ({ entries: [] as Array<{ options: Record<string, unknown>; handlers: Record<string, () => void>; show: ReturnType<typeof vi.fn> }>, supported: true }));
vi.mock('electron', () => ({
  Notification: class {
    static isSupported() { return notifications.supported; }
    private entry;
    constructor(options: Record<string, unknown>) { this.entry = { options, handlers: {} as Record<string, () => void>, show: vi.fn() }; notifications.entries.push(this.entry); }
    on(name: string, handler: () => void) { this.entry.handlers[name] = handler; }
    show() { this.entry.show(); }
  },
  dialog: {},
  BrowserWindow: class {},
}));
vi.mock('./device-mesh/local-agent-client', () => ({ getMeshIdentity: vi.fn(), getMeshCloudAuth: vi.fn() }));
import { UIHostSSE } from './ui-host-sse';

beforeEach(() => { notifications.entries.length = 0; notifications.supported = true; });

function fixture() {
  const fetch = vi.fn().mockResolvedValue({ ok: true });
  const send = vi.fn();
  const win = { isDestroyed: () => false, show: vi.fn(), focus: vi.fn(), webContents: { send } };
  const host = new UIHostSSE({ businessCore: { fetch } as unknown as BusinessCoreClient, mainWindow: () => win as unknown as BrowserWindow });
  const dispatch = (id: string, expiresAt?: string) => (host as unknown as { parseAndDispatch(raw: string): void }).parseAndDispatch('event: scheduled_notification\ndata: ' + JSON.stringify({ requestId: id, expiresAt, payload: { title: '喝水', body: '休息一下', conversationId: 'conversation', characterId: 'role', ownerId: 'core', sound: false } }));
  return { fetch, send, win, dispatch };
}

it('acknowledges the OS display event and opens the bound owner and role on click', async () => {
  const f = fixture();
  f.dispatch('notification');
  expect(notifications.entries).toHaveLength(1);
  expect(f.fetch).not.toHaveBeenCalled();
  const shown = notifications.entries[0]!;
  expect(shown.options).toMatchObject({ title: '喝水', silent: true });
  shown.handlers.show!();
  expect(f.fetch).toHaveBeenLastCalledWith('/api/notifications/scheduled/notification/ack', expect.objectContaining({ body: JSON.stringify({ state: 'displayed' }) }));
  shown.handlers.click!();
  expect(f.send).toHaveBeenCalledWith('amitia:ui:navigate', '/chat?conversationId=conversation&characterId=role&ownerId=core');
  expect(f.win.focus).toHaveBeenCalledOnce();
  expect(f.fetch).toHaveBeenLastCalledWith('/api/notifications/scheduled/notification/ack', expect.objectContaining({ body: JSON.stringify({ state: 'clicked' }) }));
});

it('deduplicates retries and ignores expired notification envelopes', () => {
  const f = fixture();
  f.dispatch('notification');
  f.dispatch('notification');
  f.dispatch('expired', new Date(Date.now() - 1000).toISOString());
  expect(notifications.entries).toHaveLength(1);
});

it('reports actual OS failure instead of reporting a successful display', () => {
  const f = fixture();
  f.dispatch('notification');
  notifications.entries[0]!.handlers.failed!();
  expect(f.fetch).toHaveBeenLastCalledWith('/api/notifications/scheduled/notification/ack', expect.objectContaining({ body: JSON.stringify({ state: 'display_failed' }) }));
});
