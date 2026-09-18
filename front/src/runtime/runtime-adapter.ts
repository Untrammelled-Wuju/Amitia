import type { RuntimeConnection, DeploymentModeConfig, LocalVoiceASRFinalEvent } from "./runtime-types";
import {
  getWebDeviceAuthHeaders,
  getWebDeviceCredential,
  getWebDeviceMeshIdentity,
  getWebPairingStatus,
  provisionWebDevice,
  createWebPairingOffer,
  clearWebDeviceCredential,
  type PairingInput,
  type PairingStatus,
  type PairingOffer,
} from "./web-device-mesh";

let cachedConnection: RuntimeConnection | null = null;
let cachedConfig: DeploymentModeConfig | null = null;
const WEB_DEPLOYMENT_CONFIG_KEY = "amitia.web.deployment.v1";

export const LOCAL_DEVICE_RUNTIME_BASE_URL = "http://127.0.0.1:18899";

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
  return getApiBaseURL();
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
    const normalizedURL = normalizeHTTPBaseURL(config.serverURL);
    cachedConnection = {
      apiBaseURL: normalizedURL,
      websocketBaseURL: toWebSocketBaseURL(normalizedURL),
    };
    return cachedConnection;
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
  cachedConnection = null;
  cachedConfig = null;

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
}

export function clearRuntimeCache(): void {
  cachedConnection = null;
  cachedConfig = null;
}

export function resetRuntimeConnectionCache(): void {
  cachedConnection = null;
  cachedConfig = null;
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
  if (api?.createMeshPairingOffer) return api.createMeshPairingOffer(baseURL, ttlSeconds);
  return createWebPairingOffer(baseURL, ttlSeconds);
}

export async function deprovisionCurrentDeviceMesh(baseURL: string): Promise<void> {
  const api = window.amitiaDesktop;
  const identity = api?.getMeshIdentity
    ? await api.getMeshIdentity()
    : getWebDeviceMeshIdentity();
  const authHeaders = api?.getBackendAuthHeaders
    ? await api.getBackendAuthHeaders("business")
    : getWebDeviceAuthHeaders(baseURL);

  if (identity?.deviceId && authHeaders.Authorization) {
    let response: Response;
    try {
      response = await fetch(`${baseURL.replace(/\/+$/, "")}/api/device-mesh/v1/devices/${encodeURIComponent(identity.deviceId)}`, {
        method: "DELETE",
        headers: { ...authHeaders, Accept: "application/json" },
        credentials: api ? undefined : "include",
      });
    } catch (error: any) {
      throw new Error(error?.message || "无法连接 Cloud Core，云端设备凭证尚未撤销");
    }
    if (!response.ok && response.status !== 401 && response.status !== 404) {
      let message = `Cloud Core 撤销设备失败 (${response.status})`;
      try {
        const payload = await response.json();
        message = String(payload?.message || payload?.msg || message);
      } catch {}
      throw new Error(message);
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
