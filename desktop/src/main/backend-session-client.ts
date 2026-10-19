import http from "node:http";
import { randomBytes } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { getBackendDataDir, ensureLocalToken } from "./path-manager";
import { getDesktopInstanceID } from "./desktop-identity";
import {
  coreEndpointIdentity,
  invalidateCoreEndpoint,
  waitForCoreEndpoint,
  type CoreEndpoint,
} from "./core-endpoint";

interface DesktopSessionResponse {
  sessionToken: string;
  expiresAt: string;
}

interface ApiEnvelope<T> {
  code?: number;
  msg?: string;
  data?: T;
}

const SESSION_RENEW_SKEW_MS = 2 * 60 * 1000;

function readTextFile(filePath: string): string {
  return fs.readFileSync(filePath, "utf8").trim();
}

function readLocalRootToken(): string {
  ensureLocalToken();
  const tokenFile = path.join(
    getBackendDataDir(),
    "security",
    "local-token",
  );
  let token = "";
  try {
    token = readTextFile(tokenFile);
  } catch {
    token = "";
  }
  if (token.length < 32) {
    throw new Error("local root token is missing or too short");
  }
  return token;
}

function isRetryableCoreConnectionError(error: unknown): boolean {
  const code = (error as NodeJS.ErrnoException | null)?.code;
  return code === "ECONNREFUSED" || code === "ECONNRESET" || code === "EPIPE";
}

async function requestJSONOnce<T>(
  endpoint: CoreEndpoint,
  method: string,
  requestPath: string,
  headers: Record<string, string>,
  body?: unknown,
): Promise<T> {
  const resolvedBaseURL = endpoint.httpBaseURL;
  return new Promise<T>((resolve, reject) => {
    const rawBody = body === undefined ? undefined : JSON.stringify(body);
    const coreURL = new URL(resolvedBaseURL);

    const req = http.request(
      {
        hostname: coreURL.hostname,
        port: Number(coreURL.port || (coreURL.protocol === "https:" ? 443 : 80)),
        path: requestPath,
        method,
        timeout: 5000,
        headers: {
          Accept: "application/json",
          ...(rawBody
            ? {
                "Content-Type": "application/json",
                "Content-Length": Buffer.byteLength(rawBody),
              }
            : {}),
          ...headers,
        },
      },
      (res) => {
        let raw = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => {
          raw += chunk;
        });
        res.on("end", () => {
          if ((res.statusCode ?? 500) < 200 || (res.statusCode ?? 500) >= 300) {
            reject(
              new Error(
                `backend request failed: ${res.statusCode ?? 0} ${raw}`,
              ),
            );
            return;
          }

          try {
            const parsed = JSON.parse(raw) as T | ApiEnvelope<T>;
            if (
              parsed &&
              typeof parsed === "object" &&
              "code" in parsed &&
              typeof (parsed as ApiEnvelope<T>).code === "number"
            ) {
              const envelope = parsed as ApiEnvelope<T>;
              if (envelope.code !== 200 || envelope.data === undefined) {
                reject(
                  new Error(
                    `backend api failed: ${envelope.code} ${envelope.msg ?? ""}`,
                  ),
                );
                return;
              }
              resolve(envelope.data);
              return;
            }
            resolve(parsed as T);
          } catch (error) {
            reject(
              new Error(
                `backend response parse failed: ${
                  error instanceof Error ? error.message : String(error)
                }`,
              ),
            );
          }
        });
      },
    );

    req.on("error", reject);
    req.on("timeout", () => {
      req.destroy(new Error("backend request timeout"));
    });

    if (rawBody !== undefined) req.write(rawBody);
    req.end();
  });
}

export async function requestJSON<T>(
  method: string,
  requestPath: string,
  headers: Record<string, string>,
  body?: unknown,
): Promise<T> {
  let lastError: unknown = null;
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const endpoint = await waitForCoreEndpoint();
    try {
      return await requestJSONOnce<T>(endpoint, method, requestPath, headers, body);
    } catch (error) {
      lastError = error;
      if (attempt > 0 || !isRetryableCoreConnectionError(error)) {
        throw error;
      }
      invalidateCoreEndpoint();
      try {
        await waitForCoreEndpoint(8000);
      } catch {
        throw error;
      }
    }
  }
  throw lastError instanceof Error ? lastError : new Error(String(lastError));
}

export class BackendSessionClient {
  private readonly desktopInstanceID = getDesktopInstanceID();
  private sessionToken: string | null = null;
  private expiresAt = 0;
  private endpointIdentity = "";
  private createInFlight: Promise<void> | null = null;

  getDesktopInstanceID(): string {
    return this.desktopInstanceID;
  }

  private async createSession(): Promise<void> {
    const rootToken = readLocalRootToken();

    const response = await requestJSON<DesktopSessionResponse>(
      "POST",
      "/api/local/sessions",
      {
        "X-Amitia-Local-Token": rootToken,
        "X-Amitia-Desktop-Instance": this.desktopInstanceID,
        "X-Request-ID": `desktop_${randomBytes(16).toString("hex")}`,
      },
      {
        desktopInstanceId: this.desktopInstanceID,
      },
    );

    if (!response.sessionToken) {
      throw new Error("backend returned empty desktop session token");
    }

    const parsedExpiry = Date.parse(response.expiresAt);
    if (!Number.isFinite(parsedExpiry)) {
      throw new Error("backend returned invalid desktop session expiry");
    }

    this.sessionToken = response.sessionToken;
    this.expiresAt = parsedExpiry;
    this.endpointIdentity = coreEndpointIdentity(await waitForCoreEndpoint());
  }

  async ensureSession(): Promise<void> {
    const endpoint = await waitForCoreEndpoint();
    const identity = coreEndpointIdentity(endpoint);
    if (this.endpointIdentity && this.endpointIdentity !== identity) {
      this.sessionToken = null;
      this.expiresAt = 0;
    }
    this.endpointIdentity = identity;

    if (
      this.sessionToken &&
      Date.now() + SESSION_RENEW_SKEW_MS < this.expiresAt
    ) {
      return;
    }

    if (!this.createInFlight) {
      this.createInFlight = this.createSession().finally(() => {
        this.createInFlight = null;
      });
    }

    await this.createInFlight;
  }

  async getRendererAuthHeaders(): Promise<Record<string, string>> {
    await this.ensureSession();

    if (!this.sessionToken) {
      throw new Error("desktop session is unavailable");
    }

    return {
      "X-Amitia-Desktop-Session": this.sessionToken,
      "X-Amitia-Desktop-Instance": this.desktopInstanceID,
    };
  }

  async getMainProcessAuthHeaders(): Promise<Record<string, string>> {
    return this.getRendererAuthHeaders();
  }

  invalidateSession(): void {
    this.sessionToken = null;
    this.expiresAt = 0;
    this.endpointIdentity = "";
  }
}

let singleton: BackendSessionClient | null = null;

export function getBackendSessionClient(): BackendSessionClient {
  if (!singleton) singleton = new BackendSessionClient();
  return singleton;
}

export async function getDesktopAuthHeaders(): Promise<
  Record<string, string>
> {
  return getBackendSessionClient().getRendererAuthHeaders();
}

export async function getLocalBackendAuthHeaders(): Promise<
  Record<string, string>
> {
  return getDesktopAuthHeaders();
}

export function getLocalRuntimeSessionClient(): BackendSessionClient {
  return getBackendSessionClient();
}

export function getLocalRootTokenForMainProcess(): string {
  return readLocalRootToken();
}

export interface RuntimeBootstrapTicketResponse {
  ticketId: string;
  ticket: string;
  spaceId: string;
  deviceId: string;
  runtimeId: string;
  expiresAt: string;
  ttlSeconds: number;
}

export async function createRuntimeBootstrapTicket(
  deviceId: string,
  runtimeId: string,
): Promise<RuntimeBootstrapTicketResponse> {
  const client = getBackendSessionClient();
  const headers = await client.getMainProcessAuthHeaders();

  return requestJSON<RuntimeBootstrapTicketResponse>(
    "POST",
    `/api/local/devices/${encodeURIComponent(
      deviceId,
    )}/runtime-bootstrap-tickets`,
    headers,
    {
      runtimeId,
    },
  );
}

export function getLocalAdminHeaders(): Record<string, string> {
  return {
    "X-Amitia-Local-Token": getLocalRootTokenForMainProcess(),
    "X-Amitia-Desktop-Instance":
      getBackendSessionClient().getDesktopInstanceID(),
  };
}
