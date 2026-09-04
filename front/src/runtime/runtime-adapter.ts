import type { RuntimeConnection, DeploymentModeConfig, LocalVoiceASRFinalEvent } from "./runtime-types";

let cachedConnection: RuntimeConnection | null = null;
let cachedConfig: DeploymentModeConfig | null = null;

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
    const base =
      (import.meta as any).env?.VITE_API_URL || window.location.origin;
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

export async function getQQApiBaseURL(): Promise<string> {
  const isDev = (import.meta as any).env?.DEV === true;
  if (isDev) {
    return "/qq-api";
  }
  const base = (await getApiBaseURL()).replace(/\/+$/, "");
  if (base === "http://127.0.0.1:18899") {
    return "http://127.0.0.1:19877/api";
  }
  // In cloud mode QQ is a Business Core capability mounted under /api/qq.
  // The local 19877 sidecar remains a local-only implementation detail.
  return `${base}/api/qq`;
}

export async function resolveApiUrl(path: string): Promise<string> {
  const base = await getApiBaseURLForPath(path);
  return base + path;
}

export async function resolveWebSocketUrl(path: string): Promise<string> {
  const conn = await getRuntimeConnection();
  return conn.websocketBaseURL + path;
}

export async function getBackendAuthHeaders(): Promise<
  Record<string, string>
> {
  const api = window.amitiaDesktop;
  if (!api) return {};
  return api.getBackendAuthHeaders();
}

export async function getDeploymentConfig(): Promise<DeploymentModeConfig> {
  if (cachedConfig) return cachedConfig;

  const api = window.amitiaDesktop;
  if (!api) {
    cachedConfig = { mode: "local" };
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
    throw new Error("当前环境不支持保存部署配置");
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

export async function publishLocalVoiceASRFinal(
  event: LocalVoiceASRFinalEvent,
): Promise<boolean> {
  const api = window.amitiaDesktop;
  if (!api?.publishLocalVoiceASRFinal) return false;
  await api.publishLocalVoiceASRFinal(event);
  return true;
}
