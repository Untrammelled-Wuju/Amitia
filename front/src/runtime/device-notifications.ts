import { apiClient } from "@/composables/useApi";

export type DeviceNotificationPlatform = "desktop" | "web";

export interface DeviceNotificationEndpoint {
  id: string;
  deviceId: string;
  platform: DeviceNotificationPlatform;
  deviceName: string;
  osVersion: string;
  preferredProvider: string;
  pushEnabled: boolean;
  systemNotificationsEnabled: boolean;
  messagePushEnabled: boolean;
  executionActivityEnabled: boolean;
  callPushEnabled: boolean;
  reminderPushEnabled: boolean;
  previewMode: "full" | "sender_only" | "hidden";
  soundEnabled: boolean;
}

export interface DeviceNotificationPreferences {
  pushEnabled: boolean;
  systemNotificationsEnabled: boolean;
  messagePushEnabled: boolean;
  executionActivityEnabled: boolean;
  callPushEnabled: boolean;
  reminderPushEnabled: boolean;
  previewMode: DeviceNotificationEndpoint["previewMode"];
  soundEnabled: boolean;
}

function unwrap<T>(value: any): T {
  const responseData = value?.data ?? value;
  return (responseData && typeof responseData === "object" && "data" in responseData ? responseData.data : responseData) as T;
}

export function notificationPlatform(): DeviceNotificationPlatform {
  return window.amitiaDesktop ? "desktop" : "web";
}

export function notificationContext() {
  return {
    locale: navigator.language || "zh-CN",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
    appearance: window.matchMedia?.("(prefers-color-scheme: dark)").matches ? "dark" : "light",
  };
}

export function notificationRegistrationPayload(
  deviceId: string,
  platform: DeviceNotificationPlatform,
  overrides: Partial<DeviceNotificationPreferences> = {},
) {
  return {
    deviceId,
    platform,
    preferredProvider: "desktop-sse",
    ...notificationContext(),
    ...overrides,
  };
}

export async function listNotificationDevices(): Promise<DeviceNotificationEndpoint[]> {
  return unwrap<DeviceNotificationEndpoint[]>(await apiClient.get("/api/notifications/devices"));
}

export async function ensureNotificationDevice(deviceId: string): Promise<DeviceNotificationEndpoint> {
  const platform = notificationPlatform();
  const existing = (await listNotificationDevices()).find(
    (endpoint) => endpoint.deviceId === deviceId && endpoint.platform === platform,
  );
  if (existing) return existing;
  unwrap<DeviceNotificationEndpoint>(
    await apiClient.post("/api/notifications/devices/register", notificationRegistrationPayload(deviceId, platform)),
  );
  const registered = (await listNotificationDevices()).find(
    (endpoint) => endpoint.deviceId === deviceId && endpoint.platform === platform,
  );
  if (!registered) throw new Error("正式通知设备注册未确认");
  return registered;
}

export async function updateNotificationPreferences(
  deviceId: string,
  preferences: Partial<DeviceNotificationPreferences>,
): Promise<DeviceNotificationEndpoint> {
  return unwrap<DeviceNotificationEndpoint>(
    await apiClient.patch(
      `/api/notifications/devices/${encodeURIComponent(deviceId)}/preferences`,
      preferences,
    ),
  );
}

export async function testNotificationDevice(deviceId: string) {
  return unwrap<Record<string, any>>(
    await apiClient.post(`/api/notifications/devices/${encodeURIComponent(deviceId)}/test`),
  );
}

export async function reportNotificationPresence(
  deviceId: string,
  foreground: boolean,
  activeConversationId = "",
): Promise<void> {
  await apiClient.post("/api/notifications/presence", {
    deviceId,
    foreground,
    activeConversationId,
  });
}
