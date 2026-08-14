import { getAuthToken } from "./auth-token-store";

export interface BusinessCoreProbeResult {
  reachable: boolean;
  ready: boolean;
  statusCode?: number;
  error?: string;
}

export class BusinessCoreClient {
  constructor(
    private readonly baseURL: string,
    private readonly tokenProvider: () => string | null = getAuthToken,
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
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);

    try {
      let res: Response;
      try {
        res = await this.fetch("/readyz", { signal: controller.signal });
      } catch {
        res = await this.fetch("/livez", { signal: controller.signal });
      }
      clearTimeout(timer);
      return { reachable: true, ready: res.status === 200, statusCode: res.status };
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
