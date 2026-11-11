import { notificationRegistrationPayload } from "./device-notifications";

export interface ScheduledBrowserEnvelope {
  requestId: string;
  payload: { title?: unknown; body?: unknown; conversationId?: unknown; characterId?: unknown; ownerId?: unknown; sound?: unknown; attentionId?: unknown; ackable?: unknown };
}

export interface ScheduledBrowserDelivery {
  acknowledge(state: 'displayed' | 'clicked' | 'display_failed'): Promise<void>;
  navigate(target: string): void;
}

export async function registerScheduledBrowserEndpoint(deviceId: string, permitted: boolean, register: (input: Record<string, unknown>) => Promise<unknown>): Promise<void> {
  return registerScheduledEndpoint('web', deviceId, permitted, register);
}

export async function registerScheduledDesktopEndpoint(deviceId: string, permitted: boolean, register: (input: Record<string, unknown>) => Promise<unknown>): Promise<void> {
  return registerScheduledEndpoint('desktop', deviceId, permitted, register);
}

async function registerScheduledEndpoint(platform: 'web' | 'desktop', deviceId: string, permitted: boolean, register: (input: Record<string, unknown>) => Promise<unknown>): Promise<void> {
  if (!deviceId) throw new Error('浏览器通知缺少设备身份');
  let response = await register(notificationRegistrationPayload(deviceId, platform, {
    ...(permitted ? {} : { pushEnabled: false, systemNotificationsEnabled: false }),
  }));
  for (let index = 0; index < 2 && response && typeof response === 'object' && 'data' in response; index++) response = (response as { data: unknown }).data;
  const endpoint = response as { id?: string; deviceId?: string; platform?: string } | null;
  if (!endpoint?.id || endpoint.deviceId !== deviceId || endpoint.platform !== platform) throw new Error('通知通道未确认保存');
}

export async function deliverScheduledBrowserNotification(envelope: ScheduledBrowserEnvelope, delivery: ScheduledBrowserDelivery): Promise<void> {
  const payload = envelope.payload;
  const acknowledge = (state: 'displayed' | 'clicked' | 'display_failed'): Promise<void> => payload.ackable === true ? delivery.acknowledge(state) : Promise.resolve();
  if (typeof Notification === 'undefined' || Notification.permission !== 'granted') {
    await acknowledge('display_failed');
    return;
  }
  try {
    const notification = new Notification(String(payload.title || 'Amitia'), { body: String(payload.body || ''), tag: envelope.requestId, silent: payload.sound === false });
    notification.onshow = () => { void acknowledge('displayed').catch((error) => {
  console.error("[original-error]", error);
}); };
    notification.onerror = () => { void acknowledge('display_failed').catch((error) => {
  console.error("[original-error]", error);
}); };
    notification.onclick = () => {
      void acknowledge('clicked').catch((error) => {
  console.error("[original-error]", error);
});
      notification.close();
      window.focus();
      const conversation = String(payload.conversationId || '');
      const attentionId = String(payload.attentionId || '');
      if (conversation) delivery.navigate('/chat?conversationId=' + encodeURIComponent(conversation) + '&characterId=' + encodeURIComponent(String(payload.characterId || '')) + '&ownerId=' + encodeURIComponent(String(payload.ownerId || '')) + (attentionId ? '&attentionId=' + encodeURIComponent(attentionId) : ''));
      else if (attentionId) delivery.navigate('/reminders?attentionId=' + encodeURIComponent(attentionId));
    };
  } catch (error) {
  console.error("[original-error]", error);
await acknowledge('display_failed');
  }
}
