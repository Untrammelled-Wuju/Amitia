import { getAccessToken } from "./auth-token-store";

export interface BusinessCoreProbeResult {
  reachable: boolean;
  ready: boolean;
  statusCode?: number;
  error?: string;
}

export class BusinessCoreClient {
	constructor(
		private readonly baseURL: string,
		private readonly tokenProvider: () => string | null = getAccessToken,
	) {}

  url(path: string): URL {
    return new URL(path, this.baseURL);
  }

  authHeaders(): Record<string, string> {
    const token = this.tokenProvider();
    if (!token) {
      return {};
    }
    return {
      Authorization: `Bearer ${token}`,
    };
  }

  async fetch(path: string, init: RequestInit = {}): Promise<Response> {
    const url = this.url(path).toString();
    const token = this.tokenProvider();
    const headers: Record<string, string> = {
      ...(init.headers as Record<string, string> || {}),
    };
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
    if (init.body && !headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }
    return fetch(url, { ...init, headers });
  }

  async probe(timeoutMs = 5000): Promise<BusinessCoreProbeResult> {
    const probeResult = await this.probePath("/readyz", timeoutMs);
    if (probeResult.reachable) {
      return probeResult;
    }
    if (probeResult.statusCode === 404 || probeResult.statusCode === 405) {
      return this.probePath("/livez", timeoutMs);
    }
    return probeResult;
  }

  private async probePath(
    path: "/readyz" | "/livez",
    timeoutMs: number,
  ): Promise<BusinessCoreProbeResult> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    try {
      const res = await this.fetch(path, { signal: controller.signal });
      clearTimeout(timer);
      if (path === "/livez") {
        return { reachable: true, ready: res.status === 200, statusCode: res.status };
      }
      if (res.status === 200) {
        return { reachable: true, ready: true, statusCode: res.status };
      }
      if (res.status === 404 || res.status === 405) {
        return { reachable: false, ready: false, statusCode: res.status };
      }
      return { reachable: true, ready: false, statusCode: res.status };
    } catch (err) {
      clearTimeout(timer);
      return {
        reachable: false,
        ready: false,
        error: err instanceof Error ? err.message : String(err),
      };
    }
  }
}
