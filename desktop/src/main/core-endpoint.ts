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
let endpointEpoch = 0;
const endpointWaiters = new Set<{
  resolve: (endpoint: CoreEndpoint) => void;
  reject: (error: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}>();

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
    return parseCoreEndpoint(JSON.parse(raw));
  } catch {
    return null;
  }
}

export function removeCoreEndpoint(dataDir: string, pid?: number): boolean {
  const filePath = coreEndpointPath(dataDir);
  if (pid !== undefined) {
    const endpoint = readCoreEndpoint(dataDir);
    if (!endpoint || endpoint.pid !== pid) {
      return false;
    }
  }
  try {
    fs.rmSync(filePath, { force: true });
    return true;
  } catch {
    return false;
  }
}

export function isCoreEndpointOwnedByProcess(
  endpoint: CoreEndpoint | null,
  pid: number,
  startedAfterMs = 0,
): boolean {
  if (!endpoint || !Number.isInteger(pid) || pid <= 0 || endpoint.pid !== pid) {
    return false;
  }
  if (startedAfterMs <= 0) {
    return true;
  }
  const startedAt = Date.parse(endpoint.startedAt);
  return Number.isFinite(startedAt) && startedAt >= startedAfterMs - 2000;
}

export function setCoreEndpoint(endpoint: CoreEndpoint | null): void {
  if (!endpoint) {
    invalidateCoreEndpoint();
    return;
  }
  currentEndpoint = endpoint;
  endpointEpoch += 1;
  setLocalMeshBaseURL(endpoint.httpBaseURL);
  for (const waiter of endpointWaiters) {
    clearTimeout(waiter.timer);
    waiter.resolve(endpoint);
  }
  endpointWaiters.clear();
}

export function invalidateCoreEndpoint(): void {
  currentEndpoint = null;
  endpointEpoch += 1;
  setLocalMeshBaseURL(LEGACY_CORE_BASE_URL);
}

export function waitForCoreEndpoint(timeoutMs = 30000): Promise<CoreEndpoint> {
  if (currentEndpoint) {
    return Promise.resolve(currentEndpoint);
  }
  return new Promise((resolve, reject) => {
    let waiter: {
      resolve: (endpoint: CoreEndpoint) => void;
      reject: (error: Error) => void;
      timer: ReturnType<typeof setTimeout>;
    };
    const timer = setTimeout(() => {
      endpointWaiters.delete(waiter);
      reject(new Error("Amitia Core 尚未就绪"));
    }, timeoutMs);
    waiter = { resolve, reject, timer };
    endpointWaiters.add(waiter);
  });
}

export function getCoreEndpoint(): CoreEndpoint | null {
  return currentEndpoint;
}

export function getCoreEndpointEpoch(): number {
  return endpointEpoch;
}

export function coreEndpointIdentity(endpoint: CoreEndpoint): string {
  return endpoint.instanceId
    ? `${endpoint.instanceId}:${endpoint.pid}:${endpoint.startedAt}`
    : `${endpoint.pid}:${endpoint.startedAt}:${endpoint.httpBaseURL}`;
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
