const DEVICE_ID_KEY = "amitia.ui.device-id.v1";

function randomDeviceId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return `ui-${crypto.randomUUID()}`;
  }
  return `ui-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 14)}`;
}

export function getUIHostDeviceId(): string {
  if (typeof window === "undefined") return "";
  try {
    const existing = window.localStorage.getItem(DEVICE_ID_KEY)?.trim();
    if (existing) return existing;
    const created = randomDeviceId();
    window.localStorage.setItem(DEVICE_ID_KEY, created);
    return created;
  } catch {
    return "";
  }
}

export async function resolveUIHostDeviceId(): Promise<string> {
  if (typeof window !== "undefined") {
    try {
      const identity = await window.amitiaDesktop?.getMeshIdentity?.();
      const meshDeviceId = identity?.deviceId?.trim() ?? "";
      if (meshDeviceId) return meshDeviceId;
    } catch {
      // Browser/local fallback below remains stable for UI profile scoping.
    }
  }
  return getUIHostDeviceId();
}
