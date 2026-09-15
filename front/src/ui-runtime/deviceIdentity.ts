import { getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";

const LEGACY_DEVICE_ID_KEY = "amitia.ui.device-id.v1";

export function getUIHostDeviceId(): string {
  if (typeof window === "undefined") return "";
  if (!window.amitiaDesktop) {
    return getWebDeviceMeshIdentity().deviceId;
  }
  try {
    return window.localStorage.getItem(LEGACY_DEVICE_ID_KEY)?.trim() || "";
  } catch {
    return "";
  }
}

export async function resolveUIHostDeviceId(): Promise<string> {
  if (typeof window === "undefined") return "";
  try {
    const identity = await window.amitiaDesktop?.getMeshIdentity?.();
    const meshDeviceId = identity?.deviceId?.trim() ?? "";
    if (meshDeviceId) return meshDeviceId;
  } catch {
    // The browser identity below remains stable when Desktop Agent is unavailable.
  }
  return getWebDeviceMeshIdentity().deviceId;
}
