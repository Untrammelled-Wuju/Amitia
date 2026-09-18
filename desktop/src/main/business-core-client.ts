import { getDesktopAuthHeaders } from "./backend-session-client";
import { getMeshCloudAuth } from "./device-mesh/local-agent-client";

export interface BusinessCoreProbeResult {
  reachable: boolean;
  ready: boolean;
  statusCode?: number;
  error?: string;
}

function sameOrigin(a: string, b: string): boolean {
  try {
    return new URL(a).origin === new URL(b).origin;
  } catch {
    return false;
  }
}

export class BusinessCoreClient {
  constructor(private readonly baseURL: string) {}

  url(path: string): URL {
    return new URL(path, this.baseURL);
  }

  async authHeaders(): Promise<Record<string, string>> {
    const cloudAuth = await getMeshCloudAuth();
    if (cloudAuth?.authorization && sameOrigin(cloudAuth.cloudBaseUrl, this.baseURL)) {
      return {
        Authorization: cloudAuth.authorization,
        "X-Amitia-Space-ID": cloudAuth.spaceId,
        "X-Amitia-Device-ID": cloudAuth.deviceId,
        "X-Amitia-Runtime-ID": cloudAuth.runtimeId,
      };
    }
    try {
      return getDesktopAuthHeaders();
    } catch {
      return {};
    }
  }

  async fetch(path: string, init: RequestInit = {}): Promise<Response> {
    const url = this.url(path).toString();
    const headers: Record<string, string> = {
      ...(await this.authHeaders()),
      ...((init.headers as Record<string, string>) || {}),
    };
    if (init.body && !headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }
    return fetch(url, { ...init, headers });
  }

  async probe(timeoutMs = 5000): Promise<BusinessCoreProbeResult> {
    const probeResult = await this.probePath("/readyz", timeoutMs);
    if (probeResult.reachable) return probeResult;
    if (probeResult.statusCode === 404 || probeResult.statusCode === 405) {
      return this.probePath("/livez", timeoutMs);
    }
    return probeResult;
  }

  private async probePath(path: "/readyz" | "/livez", timeoutMs: number): Promise<BusinessCoreProbeResult> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
      const res = await this.fetch(path, { signal: controller.signal });
      clearTimeout(timer);
      if (path === "/livez") return { reachable: true, ready: res.status === 200, statusCode: res.status };
      if (res.status === 200) return { reachable: true, ready: true, statusCode: res.status };
      if (res.status === 404 || res.status === 405) return { reachable: false, ready: false, statusCode: res.status };
      return { reachable: true, ready: false, statusCode: res.status };
    } catch (err) {
      clearTimeout(timer);
      return { reachable: false, ready: false, error: err instanceof Error ? err.message : String(err) };
    }
  }
}
