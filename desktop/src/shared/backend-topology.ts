import type { DeploymentMode, DeploymentModeConfig } from "./types";

export type BackendEndpointRole =
  | "business-core"
  | "local-runtime";

export interface BackendEndpoint {
  role: BackendEndpointRole;
  baseURL: string;
  websocketBaseURL: string;
  remote: boolean;
}

export interface DesktopBackendTopology {
  deploymentMode: DeploymentMode;

  businessCore: BackendEndpoint;
  localRuntime: BackendEndpoint;

  localRuntimeProfile: "local" | "device-agent";
}

export function normalizeHTTPBaseURL(raw: string): string {
  const url = new URL(raw);
  return url.toString().replace(/\/+$/, "");
}

export function toWebSocketBaseURL(httpBaseURL: string): string {
  const url = new URL(httpBaseURL);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString().replace(/\/+$/, "");
}

export function resolveBackendTopology(
  config: DeploymentModeConfig,
  localBaseURL = "http://127.0.0.1:18899",
): DesktopBackendTopology {
  const localBase = localBaseURL.replace(/\/+$/, "");
  const localWebSocket = toWebSocketBaseURL(localBase);
  if (config.mode === "cloud") {
    const normalizedServerURL = normalizeHTTPBaseURL(config.serverURL!);
    return {
      deploymentMode: "cloud",
      businessCore: {
        role: "business-core",
        baseURL: normalizedServerURL,
        websocketBaseURL: toWebSocketBaseURL(normalizedServerURL),
        remote: true,
      },
      localRuntime: {
        role: "local-runtime",
        baseURL: localBase,
        websocketBaseURL: localWebSocket,
        remote: false,
      },
      localRuntimeProfile: "device-agent",
    };
  }

  return {
    deploymentMode: "local",
    businessCore: {
      role: "business-core",
      baseURL: localBase,
      websocketBaseURL: localWebSocket,
      remote: false,
    },
    localRuntime: {
      role: "local-runtime",
      baseURL: localBase,
      websocketBaseURL: localWebSocket,
      remote: false,
    },
    localRuntimeProfile: "local",
  };
}
