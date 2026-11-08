import { getWebDeviceCredential, getWebDeviceMeshIdentity } from "@/runtime/web-device-mesh";
import { getApiBaseURL, getBackendAuthHeaders, getDeploymentConfig } from "@/runtime/runtime-adapter";

const UI_DEVICE_ID_KEY = "amitia.ui.device-id.v1";

export function getUIHostDeviceId(): string {
  if (typeof window === "undefined") return "";
  if (!window.amitiaDesktop) {
    return getWebDeviceMeshIdentity().deviceId;
  }
  try {
    return window.localStorage.getItem(UI_DEVICE_ID_KEY)?.trim() || "";
  } catch (error) {
  console.error("[original-error]", error);
return "";
  }
}

export async function resolveUIHostDeviceId(): Promise<string> {
  if (typeof window === "undefined") return "";
  try {
    const identity = await window.amitiaDesktop?.getMeshIdentity?.();
    const meshDeviceId = identity?.deviceId?.trim() ?? "";
    if (meshDeviceId) return meshDeviceId;
  } catch (error) {
  console.error("[original-error]", error);
// The browser identity below remains stable when Desktop Agent is unavailable.
  }
  const baseURL = await getApiBaseURL();
  const credential = getWebDeviceCredential(baseURL);
  if (credential?.deviceId) return credential.deviceId;
  const deployment = await getDeploymentConfig();
  if (deployment.mode === "local") {
    const headers = new Headers({ Accept: "application/json" });
    for (const [key, value] of Object.entries(await getBackendAuthHeaders("business"))) {
      if (value) headers.set(key, value);
    }
    const response = await fetch(`${baseURL.replace(/\/+$/, "")}/api/device-mesh/v1/coordination/me`, {
      headers,
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error(`读取本机设备身份失败: HTTP ${response.status} ${await response.text()}`);
    }
    const payload = await response.json();
    const state = payload?.data ?? payload;
    const deviceId = String(state?.policy?.deviceId ?? state?.coreConsoleDeviceId ?? "").trim();
    if (!deviceId) throw new Error("本机设备身份响应缺少 deviceId");
    return deviceId;
  }
  return getWebDeviceMeshIdentity().deviceId;
}
