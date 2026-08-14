import type { RuntimeConnection, DeploymentModeConfig } from "./runtime-types";

let cachedConnection: RuntimeConnection | null = null;
let cachedConfig: DeploymentModeConfig | null = null;

const TOKEN_KEY = "ai-companion-token";

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
  const base = await getApiBaseURL();
  if (base === "http://127.0.0.1:18899") {
    return "http://127.0.0.1:19877/api";
  }
  return base;
}

export async function resolveApiUrl(path: string): Promise<string> {
  const base = await getApiBaseURL();
  return base + path;
}

export async function resolveWebSocketUrl(path: string): Promise<string> {
  const conn = await getRuntimeConnection();
  return conn.websocketBaseURL + path;
}

export async function createAuthorizedRequestInit(
  init?: RequestInit,
): Promise<RequestInit> {
  const deployment = await getDeploymentConfig();
  const headers: Record<string, string> = {};

  if (deployment.mode === "local" && window.amitiaDesktop) {
    Object.assign(headers, await getBackendAuthHeaders());
  } else {
    const token = localStorage.getItem(TOKEN_KEY);
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  return {
    ...init,
    headers: {
      ...headers,
      ...((init?.headers as Record<string, string>) || {}),
    },
  };
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
