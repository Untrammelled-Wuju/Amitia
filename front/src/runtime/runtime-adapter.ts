import type { RuntimeConnection, DeploymentModeConfig, LocalVoiceASRFinalEvent } from "./runtime-types";
import type { DeviceManagementIntent } from "../composables/useDeviceManagementIntent";
import {
  getWebDeviceAuthHeaders,
  getWebDeviceCredential,
  getWebDeviceMeshIdentity,
  getWebPairingStatus,
  provisionWebDevice,
  createWebPairingOffer,
  clearWebDeviceCredential,
  signWebAuthenticatedFetch,
  type PairingInput,
  type PairingStatus,
  type PairingOffer,
} from "./web-device-mesh";

let cachedConnection: RuntimeConnection | null = null;
let cachedConfig: DeploymentModeConfig | null = null;
let roleSourceGeneration = 0;
let deploymentChanging = false;
const WEB_DEPLOYMENT_CONFIG_KEY = "amitia.web.deployment.v1";

export const LOCAL_DEVICE_RUNTIME_BASE_URL = "http://127.0.0.1:18899";

export async function getNativeProviderTransition() {
  return window.amitiaDesktop?.getMeshStatus ? window.amitiaDesktop.getMeshStatus() : null;
}

// Canonical registry for Desktop Pet state-authority routes. Cloud deployment
// code must consult this registry rather than open-coding singular/plural path
// checks in individual feature modules. Both namespaces are local because the
// API historically uses /desktop-pets for package/installation resources and
// /desktop-pet for Behavior control.
export const DEVICE_LOCAL_ROUTE_PREFIXES = [
  "/api/desktop-pets",
  "/api/desktop-pet",
  "/api/local/workflows",
  "/api/local/workflow-runs",
  "/api/local/workspaces",
  "/api/workspaces",
  "/api/storage",
  "/media",
  "/api/artifacts/v1/previews",
  "/internal/device-mesh",
] as const;

// Kept as an alias for callers that still use the old Desktop-Pet-specific
// name. Workflow local routing now shares the same canonical registry.
export const DESKTOP_PET_DEVICE_LOCAL_ROUTE_PREFIXES = DEVICE_LOCAL_ROUTE_PREFIXES;

export function isDeviceLocalApiPath(path: string): boolean {
  const normalized = String(path || "").split("?", 1)[0];
  return DEVICE_LOCAL_ROUTE_PREFIXES.some(
    (prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`),
  );
}

export async function getApiBaseURLForPath(path: string): Promise<string> {
  if (window.amitiaDesktop && isDeviceLocalApiPath(path)) {
    return LOCAL_DEVICE_RUNTIME_BASE_URL;
  }
  if (window.amitiaDesktop && isDeviceRoleManagementPath(path) && (await getDeploymentConfig()).mode === "cloud") {
    const runtime = await getRuntimeConnection();
    const base = runtime.apiBaseURL;
    const generation = roleSourceGeneration;
    const response = await fetch(base + "/api/device-mesh/v1/coordination/me", { headers: await getBackendAuthHeaders("business"), redirect: "error" });
    if (!response.ok) throw new Error("无法确认角色所属设备，请恢复 Core 连接后重试");
    const payload = await response.json();
    const policy = payload.data?.policy || payload.policy;
    if (typeof policy?.coordinated !== "boolean") throw new Error("角色来源状态无效");
    if (generation !== roleSourceGeneration) throw new Error("Core 已切换，请重新加载角色");
    return !policy.coordinated ? LOCAL_DEVICE_RUNTIME_BASE_URL : base;
  }
  return getApiBaseURL();
}

export function isDeviceRoleManagementPath(path: string): boolean {
  const normalized = path.split("?", 1)[0];
  if (normalized === "/api/characters/generate-card" || normalized.endsWith("/test")) return false;
  return ["/api/characters", "/api/character-templates", "/api/companion/role-profile"].some((prefix) => normalized === prefix || normalized.startsWith(prefix+"/"));
}

function normalizeHTTPBaseURL(raw: string): string {
  const url = new URL(raw);
  return url.toString().replace(/\/+$/, "");
}

function toWebSocketBaseURL(httpBaseURL: string): string {
  const url = new URL(httpBaseURL);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString().replace(/\/+$/, "");
}

export async function getRuntimeConnection(): Promise<RuntimeConnection> {
  if (cachedConnection) return cachedConnection;

  const api = window.amitiaDesktop;
  if (!api) {
    const config = await getDeploymentConfig();
    const configuredBase = config.mode === "cloud" && config.serverURL
      ? config.serverURL
      : (import.meta as any).env?.VITE_API_URL || window.location.origin;
    const base = normalizeHTTPBaseURL(configuredBase);
    cachedConnection = {
      apiBaseURL: base,
      websocketBaseURL: toWebSocketBaseURL(base),
    };
    return cachedConnection;
  }

  const config = await api.getDeploymentConfig();
  cachedConfig = config;

  if (config.mode === "cloud" && config.serverURL) {
    const status = await api.getMeshStatus?.();
    if (status?.cloudBaseUrl) {
      const proxy = LOCAL_DEVICE_RUNTIME_BASE_URL + "/internal/device-mesh/provider";
      cachedConnection = { apiBaseURL: proxy, websocketBaseURL: toWebSocketBaseURL(proxy) };
      return cachedConnection;
    }
    throw new Error("本机尚未建立经过验证的云端绑定，请重新配对设备");
  }

  const isDev = (import.meta as any).env?.DEV === true;
  const devOrigin = window.location.origin;
  if (isDev && devOrigin) {
    cachedConnection = {
      apiBaseURL: devOrigin,
      websocketBaseURL: toWebSocketBaseURL(devOrigin),
    };
  } else {
    cachedConnection = {
      apiBaseURL: "http://127.0.0.1:18899",
      websocketBaseURL: "ws://127.0.0.1:18899",
    };
  }
  return cachedConnection;
}

export async function getApiBaseURL(): Promise<string> {
  const conn = await getRuntimeConnection();
  return conn.apiBaseURL;
}

export async function resolveApiUrl(path: string): Promise<string> {
  const base = await getApiBaseURLForPath(path);
  return base + path;
}

export async function resolveWebSocketUrl(path: string): Promise<string> {
  const conn = await getRuntimeConnection();
  return conn.websocketBaseURL + path;
}

export async function getBackendAuthHeaders(
  target: "local" | "business" = "business",
): Promise<Record<string, string>> {
  const api = window.amitiaDesktop;
  if (api) return api.getBackendAuthHeaders(target);
  if (target === "local") return {};
  const baseURL = await getApiBaseURL();
  return getWebDeviceAuthHeaders(baseURL);
}

export async function getDeploymentConfig(): Promise<DeploymentModeConfig> {
  if (deploymentChanging) throw new Error("运行方式正在切换，请稍后重试");
  if (cachedConfig) return cachedConfig;

  const api = window.amitiaDesktop;
  if (!api) {
    try {
      const raw = window.localStorage.getItem(WEB_DEPLOYMENT_CONFIG_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as DeploymentModeConfig;
        if (parsed?.mode === "cloud" && parsed.serverURL) {
          cachedConfig = { mode: "cloud", serverURL: normalizeHTTPBaseURL(parsed.serverURL) };
          return cachedConfig;
        }
        if (parsed?.mode === "local") {
          cachedConfig = { mode: "local" };
          return cachedConfig;
        }
      }
    } catch {
      // Invalid browser-local deployment config falls back to same-origin Cloud UI.
    }
    cachedConfig = { mode: "cloud", serverURL: normalizeHTTPBaseURL(window.location.origin) };
    return cachedConfig;
  }

  cachedConfig = await api.getDeploymentConfig();
  return cachedConfig;
}

export async function saveDeploymentConfig(
  config: DeploymentModeConfig,
): Promise<DeploymentModeConfig> {
  deploymentChanging = true;
  roleSourceGeneration++;
  cachedConnection = null;
  cachedConfig = null;

  window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));

  try {
    const api = window.amitiaDesktop;
    if (!api) {
    const normalized: DeploymentModeConfig = config.mode === "cloud"
      ? { mode: "cloud", serverURL: normalizeHTTPBaseURL(config.serverURL || window.location.origin) }
      : { mode: "local" };
    window.localStorage.setItem(WEB_DEPLOYMENT_CONFIG_KEY, JSON.stringify(normalized));
    cachedConfig = normalized;
      return normalized;
    }

    cachedConfig = await api.saveDeploymentConfig(config);
    return cachedConfig;
  } finally {
    deploymentChanging = false;
    window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));
  }
}

export function clearRuntimeCache(): void {
  roleSourceGeneration++;
  cachedConnection = null;
  cachedConfig = null;
  window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));
}

export function resetRuntimeConnectionCache(): void {
  roleSourceGeneration++;
  cachedConnection = null;
  cachedConfig = null;
  window.dispatchEvent(new CustomEvent("amitia:runtime-connection-changed"));
}


function sameOrigin(a: string, b: string): boolean {
  try {
    return new URL(a).origin === new URL(b).origin;
  } catch {
    return false;
  }
}

export async function getCurrentDevicePairingStatus(baseURL: string): Promise<PairingStatus> {
  const api = window.amitiaDesktop;
  if (api && baseURL.endsWith("/internal/device-mesh/provider")) baseURL = (await getDeploymentConfig()).serverURL || baseURL;
  if (api?.getMeshPairingStatus) return api.getMeshPairingStatus(baseURL);
  return getWebPairingStatus(baseURL);
}

export async function isCurrentDevicePaired(baseURL: string): Promise<boolean> {
  const api = window.amitiaDesktop;
  if (api?.getMeshStatus) {
    const status = await api.getMeshStatus();
    const state = String(status?.state || "").toLowerCase();
    if (!status || ["", "unprovisioned", "revoked", "stopped"].includes(state)) return false;
    const boundURL = String(status.cloudBaseUrl || "").trim();
    if (status.fingerprint && baseURL.endsWith("/internal/device-mesh/provider")) return sameOrigin(boundURL, (await getDeploymentConfig()).serverURL || "");
    return Boolean(boundURL) && sameOrigin(boundURL, baseURL);
  }
  return Boolean(getWebDeviceCredential(baseURL));
}

export async function provisionCurrentDeviceMesh(baseURL: string, pairing: PairingInput): Promise<void> {
  const api = window.amitiaDesktop;
  if (api?.provisionMesh) {
    await api.provisionMesh(baseURL, pairing);
    return;
  }
  await provisionWebDevice(baseURL, pairing);
}

export async function createCurrentDevicePairingOffer(baseURL: string, ttlSeconds = 600): Promise<PairingOffer> {
  const api = window.amitiaDesktop;
  if (api && (await getDeploymentConfig()).mode === "local") baseURL = LOCAL_DEVICE_RUNTIME_BASE_URL;
  if (api && baseURL.endsWith("/internal/device-mesh/provider")) baseURL = (await getDeploymentConfig()).serverURL || baseURL;
  if (api?.createMeshPairingOffer) return api.createMeshPairingOffer(baseURL, ttlSeconds);
  return createWebPairingOffer(baseURL, ttlSeconds);
}

export async function deprovisionCurrentDeviceMesh(baseURL: string, originalIntent?: DeviceManagementIntent): Promise<void> {
  const originalConnection = (await getRuntimeConnection()).apiBaseURL;
  if (originalIntent && (originalIntent.controller.signal.aborted || originalIntent.apiBaseURL !== originalConnection)) throw new Error("原解绑设备管理页面已变化，请重新确认");
  const api = window.amitiaDesktop;
  const identity = api?.getMeshIdentity
    ? await api.getMeshIdentity()
    : getWebDeviceMeshIdentity();
  const authHeaders = api?.getBackendAuthHeaders
    ? await api.getBackendAuthHeaders("business")
    : getWebDeviceAuthHeaders(baseURL);

  const nativeStatus = api?.getMeshStatus ? await api.getMeshStatus() : null;
  if (api && !nativeStatus?.cloudBaseUrl && (authHeaders.Authorization || authHeaders["X-Amitia-Desktop-Session"])) {
    throw new Error("当前设备没有可验证的云端绑定，无法撤销远端设备凭证");
  }
  const revokeBaseURL = api && nativeStatus?.cloudBaseUrl
    ? `${LOCAL_DEVICE_RUNTIME_BASE_URL}/internal/device-mesh/provider`
    : baseURL;
  if (identity?.deviceId && (authHeaders.Authorization || authHeaders["X-Amitia-Desktop-Session"])) {
    let response: Response;
    const controller = new AbortController();
    const invalidate = () => controller.abort("Core 或设备权限已变化，请重新确认解绑");
    window.addEventListener("amitia:runtime-connection-changed", invalidate);
    window.addEventListener("amitia:execution-scope-changed", invalidate);
    originalIntent?.controller.signal.addEventListener("abort", invalidate);
    try {
      const policyURL = `${revokeBaseURL.replace(/\/+$/, "")}/api/device-mesh/v1/coordination/me`;
      const policyInit: RequestInit = { method: "GET", headers: { ...authHeaders, Accept: "application/json" }, credentials: api ? undefined : "include", redirect: "error", signal: controller.signal };
      const policyResponse = await fetch(policyURL, api ? policyInit : await signWebAuthenticatedFetch(baseURL, policyURL, policyInit));
      if (!policyResponse.ok) throw new Error("无法确认原 Core 的设备管理权限，云端凭据尚未撤销");
      const policyPayload = await policyResponse.json();
      const state = policyPayload.data ?? policyPayload;
      const policy = state?.policy;
      const revisions = [policy?.providerEpoch, policy?.modeRevision, policy?.permissionRevision];
      if (typeof state?.coreId !== "string" || !state.coreId || state.coreId !== state.coreId.trim() || policy?.deviceId !== identity.deviceId || revisions.some(value => !Number.isSafeInteger(value) || value < 1)) throw new Error("原 Core 的设备管理权限无法确认");
      if (originalIntent) {
        const original = originalIntent.state;
        if (state.coreId !== original.coreId || state.canAdminister !== original.canAdminister || policy.deviceId !== original.policy.deviceId || policy.coordinated !== original.policy.coordinated || policy.administrator !== original.policy.administrator || revisions.join(":") !== [original.policy.providerEpoch, original.policy.modeRevision, original.policy.permissionRevision].join(":")) throw new Error("原解绑权限已变化，请重新确认");
      }
      if (controller.signal.aborted || (await getRuntimeConnection()).apiBaseURL !== originalConnection) throw new Error("Core 连接已变化，请重新确认解绑");
      if (api?.getMeshStatus && (await api.getMeshStatus())?.cloudBaseUrl !== nativeStatus?.cloudBaseUrl) throw new Error("云端绑定已变化，请重新确认解绑");
      const url = `${revokeBaseURL.replace(/\/+$/, "")}/api/device-mesh/v1/devices/${encodeURIComponent(identity.deviceId)}`;
      const init: RequestInit = {
        method: "DELETE",
        headers: { ...authHeaders, Accept: "application/json", "X-Amitia-Expected-Core-ID": state.coreId, "X-Amitia-Expected-Configuration-Policy": revisions.join(":") },
        credentials: api ? undefined : "include",
        redirect: "error",
        signal: controller.signal,
      };
      response = await fetch(url, api ? init : await signWebAuthenticatedFetch(baseURL, url, init));
      if (controller.signal.aborted || (await getRuntimeConnection()).apiBaseURL !== originalConnection) throw new Error("解绑期间 Core 已变化，原解绑结果不能清除新绑定");
      if (!response.ok) {
        let message = `Cloud Core 撤销设备失败 (${response.status})`;
        try {
          const payload = await response.json();
          message = String(payload?.message || payload?.msg || message);
        } catch {}
        throw new Error(message);
      }
      const acknowledgement = await response.json();
      if (acknowledgement?.ok !== true || acknowledgement.deviceId !== identity.deviceId) throw new Error("Core 未确认原设备凭据撤销，不能清除本地绑定");
      if (controller.signal.aborted || (await getRuntimeConnection()).apiBaseURL !== originalConnection) throw new Error("原解绑连接已变化，不能清除当前绑定");
      if (api?.getMeshStatus && (await api.getMeshStatus())?.cloudBaseUrl !== nativeStatus?.cloudBaseUrl) throw new Error("云端绑定已变化，不能清除当前绑定");
    } catch (error: any) {
      throw new Error(error?.message || "无法连接 Cloud Core，云端设备凭证尚未撤销");
    } finally {
      window.removeEventListener("amitia:runtime-connection-changed", invalidate);
      window.removeEventListener("amitia:execution-scope-changed", invalidate);
      originalIntent?.controller.signal.removeEventListener("abort", invalidate);
    }
  }

  if (api?.deprovisionMesh) {
    await api.deprovisionMesh();
  } else {
    clearWebDeviceCredential(baseURL);
  }
}

export async function publishLocalVoiceASRFinal(
  event: LocalVoiceASRFinalEvent,
): Promise<boolean> {
  const api = window.amitiaDesktop;
  if (!api?.publishLocalVoiceASRFinal) return false;
  await api.publishLocalVoiceASRFinal(event);
  return true;
}
