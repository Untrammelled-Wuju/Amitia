import { beforeEach, expect, it, vi } from 'vitest';
import type { BrowserWindow } from 'electron';
import type { BusinessCoreClient } from './business-core-client';

const notifications = vi.hoisted(() => ({ entries: [] as Array<{ options: Record<string, unknown>; handlers: Record<string, () => void>; show: ReturnType<typeof vi.fn> }>, supported: true }));
const identityMock = vi.hoisted(() => vi.fn());
const invalidateSessionMock = vi.hoisted(() => vi.fn());
vi.mock('electron', () => ({
  Notification: class {
    static isSupported() { return notifications.supported; }
    private entry;
    constructor(options: Record<string, unknown>) { this.entry = { options, handlers: {} as Record<string, () => void>, show: vi.fn() }; notifications.entries.push(this.entry); }
    on(name: string, handler: () => void) { this.entry.handlers[name] = handler; }
    show() { this.entry.show(); }
  },
  app: { getVersion: () => '26.2.0-beta.15', getLocale: () => 'zh-CN' },
  nativeTheme: { shouldUseDarkColors: false },
  dialog: {},
  BrowserWindow: class {},
}));
vi.mock('./device-mesh/local-agent-client', () => ({ getMeshIdentity: identityMock, getMeshCloudAuth: vi.fn() }));
vi.mock('./backend-session-client', () => ({ getBackendSessionClient: () => ({ invalidateSession: invalidateSessionMock }) }));
import { UIHostSSE } from './ui-host-sse';

beforeEach(() => { notifications.entries.length = 0; notifications.supported = true; });
beforeEach(() => identityMock.mockReset());
beforeEach(() => invalidateSessionMock.mockReset());

function fixture() {
  const fetch = vi.fn().mockResolvedValue({ ok: true });
  const send = vi.fn();
  const win = { isDestroyed: () => false, show: vi.fn(), focus: vi.fn(), webContents: { send } };
  const host = new UIHostSSE({ businessCore: { fetch, fetchLocal: vi.fn() } as unknown as BusinessCoreClient, mainWindow: () => win as unknown as BrowserWindow });
  const dispatch = (id: string, expiresAt?: string, payload: Record<string, unknown> = {}) => (host as unknown as { parseAndDispatch(raw: string): void }).parseAndDispatch('event: scheduled_notification\ndata: ' + JSON.stringify({ requestId: id, expiresAt, payload: { title: '喝水', body: '休息一下', conversationId: 'conversation', characterId: 'role', ownerId: 'core', sound: false, ...payload } }));
  return { fetch, send, win, dispatch };
}

it('registers the desktop notification endpoint from its confirmed write before opening SSE', async () => {
  identityMock.mockResolvedValue({ deviceId: 'desktop-device', runtimeId: 'runtime', platform: 'darwin' });
  const fetch = vi.fn()
    .mockResolvedValueOnce({ ok: true, json: async () => ({ code: 200, data: { id: 'endpoint', deviceId: 'desktop-device', platform: 'desktop' } }) })
    .mockRejectedValueOnce(new Error('test stop'));
  const fetchLocal = vi.fn().mockRejectedValueOnce(new Error('test stop'));
  const host = new UIHostSSE({ businessCore: { fetch, fetchLocal } as unknown as BusinessCoreClient, mainWindow: () => null });
  host.start();
  await vi.waitFor(() => expect(fetchLocal).toHaveBeenCalledOnce());
  host.stop();
  expect(fetch.mock.calls.map(([path]) => path)).toEqual([
    '/api/notifications/devices/register',
  ]);
  expect(fetchLocal.mock.calls.map(([path]) => path)).toEqual([
    '/api/proactive-sse?clientId=electron-ui-host',
  ]);
  expect(JSON.parse(fetch.mock.calls[0]![1]!.body as string)).toMatchObject({
    deviceId: 'desktop-device',
    platform: 'desktop',
    preferredProvider: 'desktop-sse',
  });
  expect(JSON.parse(fetch.mock.calls[0]![1]!.body as string)).toHaveProperty('deviceName');
  expect(JSON.parse(fetch.mock.calls[0]![1]!.body as string)).toHaveProperty('osVersion');
});

it('renews the desktop session once when the notification API returns a business 401', async () => {
  identityMock.mockResolvedValue({ deviceId: 'desktop-device', runtimeId: 'runtime', platform: 'darwin' });
  const fetch = vi.fn()
    .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ code: 401, msg: '本地凭据无效' }) })
    .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ code: 200, data: { id: 'endpoint', deviceId: 'desktop-device', platform: 'desktop' } }) })
    .mockRejectedValueOnce(new Error('test stop'));
  const fetchLocal = vi.fn().mockRejectedValueOnce(new Error('test stop'));
  const host = new UIHostSSE({ businessCore: { fetch, fetchLocal } as unknown as BusinessCoreClient, mainWindow: () => null });
  host.start();
  await vi.waitFor(() => expect(fetchLocal).toHaveBeenCalledOnce());
  host.stop();
  expect(invalidateSessionMock).toHaveBeenCalledOnce();
  expect(fetch.mock.calls.map(([path]) => path)).toEqual([
    '/api/notifications/devices/register',
    '/api/notifications/devices/register',
  ]);
  expect(fetchLocal.mock.calls.map(([path]) => path)).toEqual([
    '/api/proactive-sse?clientId=electron-ui-host',
  ]);
});

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

it('opens the exact attention item in its conversation when the notification is clicked', () => {
  const f = fixture();
  f.dispatch('attention', undefined, { type: 'attention.required', attentionId: 'attention:linked' });
  notifications.entries[0]!.handlers.click!();
  expect(f.send).toHaveBeenCalledWith('amitia:ui:navigate', '/chat?conversationId=conversation&characterId=role&ownerId=core&attentionId=attention%3Alinked');
});

it('opens the attention center when the item has no conversation', () => {
  const f = fixture();
  f.dispatch('attention', undefined, { type: 'attention.required', conversationId: '', characterId: '', attentionId: 'attention:linked' });
  notifications.entries[0]!.handlers.click!();
  expect(f.send).toHaveBeenCalledWith('amitia:ui:navigate', '/reminders?attentionId=attention%3Alinked');
});

it('deduplicates retries and ignores expired notification envelopes', () => {
  const f = fixture();
  f.dispatch('notification');
  f.dispatch('notification');
  f.dispatch('expired', new Date(Date.now() - 1000).toISOString());
  expect(notifications.entries).toHaveLength(1);
});

it('notifies the desktop snapshot host for extension and ui changes without polling', () => {
  const onExtensionChange = vi.fn();
  const host = new UIHostSSE({
    businessCore: { fetch: vi.fn() } as unknown as BusinessCoreClient,
    mainWindow: () => null,
    onExtensionChange,
  });
  const dispatch = (event: string, id: string) => (host as unknown as { parseAndDispatch(raw: string): void }).parseAndDispatch(
    `event: ${event}\ndata: ${JSON.stringify({ requestId: id, extensionId: 'com.amitia/plugin-market', payload: {} })}`,
  );

  dispatch('extension_contributions_changed', 'change-1');
  dispatch('ui_provider_changed', 'change-2');
  dispatch('ui_notify', 'notify-1');

  expect(onExtensionChange.mock.calls).toEqual([
    ['extension_contributions_changed', 'com.amitia/plugin-market'],
    ['ui_provider_changed', 'com.amitia/plugin-market'],
    ['ui_notify', 'com.amitia/plugin-market'],
  ]);
});

it('reports actual OS failure instead of reporting a successful display', () => {
  const f = fixture();
  f.dispatch('notification');
  notifications.entries[0]!.handlers.failed!();
  expect(f.fetch).toHaveBeenLastCalledWith('/api/notifications/scheduled/notification/ack', expect.objectContaining({ body: JSON.stringify({ state: 'display_failed' }) }));
});
