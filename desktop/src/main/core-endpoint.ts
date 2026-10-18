import fs from "node:fs";
import path from "node:path";
import { setLocalMeshBaseURL } from "./device-mesh/protocol";

export interface CoreEndpoint {
  schemaVersion: number;
  version?: string;
  instanceId?: string;
  pid: number;
  startedAt: string;
  host: string;
  port: number;
  httpBaseURL: string;
  websocketBaseURL: string;
  compatibilityBaseURL?: string;
}

const LEGACY_CORE_BASE_URL = "http://127.0.0.1:18899";
const LEGACY_CORE_WS_URL = "ws://127.0.0.1:18899";

let currentEndpoint: CoreEndpoint | null = null;

export function coreEndpointPath(dataDir: string): string {
  return path.join(dataDir, "run", "endpoint.json");
}

export function parseCoreEndpoint(payload: unknown): CoreEndpoint | null {
  if (!payload || typeof payload !== "object") return null;
  const record = payload as Record<string, unknown>;
  const httpBaseURL = typeof record.httpBaseURL === "string" ? record.httpBaseURL.trim() : "";
  const websocketBaseURL = typeof record.websocketBaseURL === "string" ? record.websocketBaseURL.trim() : "";
  if (!httpBaseURL || !websocketBaseURL) return null;
  try {
    const httpURL = new URL(httpBaseURL);
    const wsURL = new URL(websocketBaseURL);
    if (!["http:", "https:"].includes(httpURL.protocol)) return null;
    if (!["ws:", "wss:"].includes(wsURL.protocol)) return null;
    const host = httpURL.hostname.replace(/^\[|\]$/g, "").toLowerCase();
    if (!isLoopbackHost(host)) return null;
    const port = Number(httpURL.port || (httpURL.protocol === "https:" ? 443 : 80));
    if (!Number.isInteger(port) || port <= 0 || port > 65535) return null;
    return {
      schemaVersion: Number(record.schemaVersion) || 1,
      version: typeof record.version === "string" ? record.version : undefined,
      instanceId: typeof record.instanceId === "string" ? record.instanceId : undefined,
      pid: Number(record.pid) || 0,
      startedAt: typeof record.startedAt === "string" ? record.startedAt : "",
      host,
      port,
      httpBaseURL: httpBaseURL.replace(/\/+$/, ""),
      websocketBaseURL: websocketBaseURL.replace(/\/+$/, ""),
      compatibilityBaseURL:
        typeof record.compatibilityBaseURL === "string" && record.compatibilityBaseURL.trim()
          ? record.compatibilityBaseURL.trim().replace(/\/+$/, "")
          : undefined,
    };
  } catch {
    return null;
  }
}

export function readCoreEndpoint(dataDir: string): CoreEndpoint | null {
  try {
    const raw = fs.readFileSync(coreEndpointPath(dataDir), "utf8");
    const endpoint = parseCoreEndpoint(JSON.parse(raw));
    if (endpoint) setCoreEndpoint(endpoint);
    return endpoint;
  } catch {
    return null;
  }
}

export function setCoreEndpoint(endpoint: CoreEndpoint | null): void {
  currentEndpoint = endpoint;
  setLocalMeshBaseURL(endpoint?.httpBaseURL || LEGACY_CORE_BASE_URL);
}

export function getCoreEndpoint(): CoreEndpoint | null {
  return currentEndpoint;
}

export function getCoreBaseURL(): string {
  return currentEndpoint?.httpBaseURL || LEGACY_CORE_BASE_URL;
}

export function getCoreWebSocketBaseURL(): string {
  return currentEndpoint?.websocketBaseURL || LEGACY_CORE_WS_URL;
}

export function legacyCoreEndpoint(): CoreEndpoint {
  return {
    schemaVersion: 1,
    pid: 0,
    startedAt: "",
    host: "127.0.0.1",
    port: 18899,
    httpBaseURL: LEGACY_CORE_BASE_URL,
    websocketBaseURL: LEGACY_CORE_WS_URL,
  };
}

function isLoopbackHost(host: string): boolean {
  if (host === "localhost" || host === "::1" || host === "0:0:0:0:0:0:0:1") return true;
  const octets = host.split(".");
  if (octets.length !== 4) return false;
  const parsed = octets.map((value) => Number.parseInt(value, 10));
  return parsed.every((value, index) => /^\d+$/.test(octets[index]) && value >= 0 && value <= 255) && parsed[0] === 127;
}
