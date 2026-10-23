import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { deliverScheduledBrowserNotification, registerScheduledBrowserEndpoint } from '../runtime/scheduled-notifications';

const entries: Array<{ title: string; options: NotificationOptions; onshow?: () => void; onclick?: () => void; onerror?: () => void; close: ReturnType<typeof vi.fn> }> = [];
beforeEach(() => {
  entries.length = 0;
  localStorage.clear();
  class BrowserNotification {
    static permission = 'granted';
    title: string;
    options: NotificationOptions;
    close = vi.fn();
    constructor(title: string, options: NotificationOptions) { this.title = title; this.options = options; entries.push(this); }
  }
  vi.stubGlobal('Notification', BrowserNotification);
  vi.stubGlobal('window', { focus: vi.fn() });
});
afterEach(() => vi.unstubAllGlobals());

it('only confirms display after the browser reports it and preserves notification owner on click', async () => {
  const acknowledge = vi.fn().mockResolvedValue(undefined);
  const navigate = vi.fn();
  await deliverScheduledBrowserNotification({ requestId: 'notification', payload: { title: '提醒', body: '喝水', conversationId: 'conversation', characterId: 'role', ownerId: 'core' } }, { acknowledge, navigate });
  expect(acknowledge).not.toHaveBeenCalled();
  expect(entries).toHaveLength(1);
  entries[0]!.onshow!();
  expect(acknowledge).toHaveBeenCalledWith('displayed');
  entries[0]!.onclick!();
  expect(acknowledge).toHaveBeenCalledWith('clicked');
  expect(navigate).toHaveBeenCalledWith('/chat?conversationId=conversation&characterId=role&ownerId=core');
});

it('does not show a browser notification when system permission is unavailable', async () => {
  const acknowledge = vi.fn().mockResolvedValue(undefined);
  (Notification as any).permission = 'denied';
  await deliverScheduledBrowserNotification({ requestId: 'notification', payload: {} }, { acknowledge, navigate: vi.fn() });
  expect(entries).toHaveLength(0);
  expect(acknowledge).toHaveBeenCalledWith('display_failed');
});

it('rejects an empty registration receipt and does not register a desktop endpoint for a browser', async () => {
  const register = vi.fn().mockResolvedValue({});
  await expect(registerScheduledBrowserEndpoint('browser-device', true, register)).rejects.toThrow('未确认保存');
  expect(register).toHaveBeenCalledWith(expect.objectContaining({ platform: 'web', deviceId: 'browser-device' }));
  expect(register.mock.calls[0]?.[0]).not.toHaveProperty('systemNotificationsEnabled');
  register.mockResolvedValue({ code: 200, data: { id: 'registered', deviceId: 'browser-device', platform: 'web' } });
  await expect(registerScheduledBrowserEndpoint('browser-device', true, register)).resolves.toBeUndefined();
});
