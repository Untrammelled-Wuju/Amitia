import { getMeshCloudAuth, signMeshPairingClaim } from "./local-agent-client";
import { getLocalAdminHeaders } from "../backend-session-client";
import { LOCAL_MESH_BASE_URL } from "./protocol";
import {
  type CloudBootstrapTicketResponse,
  type CloudDeviceListResponse,
  type CloudPairingClaimRequest,
  type CloudPairingOfferResponse,
  type CloudPairingStatusResponse,
  type CloudProbeResponse,
} from "./protocol";

function sameOrigin(a: string, b: string): boolean {
  try { return new URL(a).origin === new URL(b).origin; } catch (error) {
  console.error("[original-error]", error);
return false; }
}

async function deviceAuthHeaders(cloudBaseURL: string): Promise<Record<string, string>> {
  if (sameOrigin(cloudBaseURL, LOCAL_MESH_BASE_URL)) return getLocalAdminHeaders();
  const auth = await getMeshCloudAuth();
  if (!auth?.authorization || !sameOrigin(auth.cloudBaseUrl, cloudBaseURL)) return {};
  return {
    Authorization: auth.authorization,
    "X-Amitia-Space-ID": auth.spaceId,
    "X-Amitia-Device-ID": auth.deviceId,
    "X-Amitia-Runtime-ID": auth.runtimeId,
  };
}

async function cloudFetch(cloudBaseURL: string, path: string, method: string, body?: unknown, authenticated = true): Promise<Response> {
  let url = new URL(path, cloudBaseURL).toString();
  const headers: Record<string, string> = { Accept: "application/json" };
  const auth = authenticated ? await getMeshCloudAuth() : null;
  if (auth?.cloudBaseUrl && sameOrigin(auth.cloudBaseUrl, cloudBaseURL)) {
    url = LOCAL_MESH_BASE_URL + "/internal/device-mesh/provider" + path;
    Object.assign(headers, getLocalAdminHeaders());
  } else {
  if (authenticated) Object.assign(headers, await deviceAuthHeaders(cloudBaseURL));
  }
  const init: RequestInit = { method, headers };
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  return fetch(url, init);
}

async function responseError(res: Response, fallback: string): Promise<Error> {
  try {
    const body = (await res.json()) as { message?: string; msg?: string };
    return new Error(body.message || body.msg || `${fallback} (status=${res.status})`);
  } catch (error) {
  console.error("[original-error]", error);
return new Error(`${fallback} (status=${res.status}): ${error instanceof Error ? error.message : String(error)}`);
  }
}

export async function getPairingStatus(cloudBaseURL: string): Promise<CloudPairingStatusResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/status", "GET");
  if (!res.ok) throw await responseError(res, "pairing status failed");
  return res.json() as Promise<CloudPairingStatusResponse>;
}

export async function claimPairing(cloudBaseURL: string, req: CloudPairingClaimRequest): Promise<CloudBootstrapTicketResponse> {
  const status = await getPairingStatus(cloudBaseURL);
  const claim = async () => {
    const proof = await signMeshPairingClaim(status.spaceId, req);
    return cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/claim", "POST", { ...req, proof }, false);
  };
  let res = await claim();
  if (!res.ok) throw await responseError(res, "pairing claim failed");
  if (res.status === 202) {
    const pending = await res.json() as { requestId?: string };
    if (!pending.requestId) throw new Error("统筹中心未返回配对申请编号");
    await waitForPairingApproval(cloudBaseURL, pending.requestId);
    res = await claim();
    if (!res.ok) throw await responseError(res, "pairing claim failed");
  }
  return res.json() as Promise<CloudBootstrapTicketResponse>;
}

async function waitForPairingApproval(cloudBaseURL: string, requestID: string): Promise<void> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(new Error("等待统筹中心批准配对超时")), 31 * 60_000);
  let reconnectAttempt = 0;
  let lastError: unknown;
  try {
    while (!controller.signal.aborted) {
      try {
        const response = await fetch(new URL(`/api/public/device-mesh/v1/pairing/requests/${encodeURIComponent(requestID)}/events`, cloudBaseURL), {
          headers: { Accept: "text/event-stream" },
          signal: controller.signal,
        });
        if (!response.ok || !response.body || !response.headers.get("content-type")?.includes("text/event-stream")) {
          const message = await response.text() || `等待配对审批失败: HTTP ${response.status}`;
          const error = new Error(message);
          if (response.status >= 500) throw new PairingApprovalReconnect(error);
          throw new PairingApprovalTerminal(error);
        }
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = "";
        let decision = "";
        try {
          while (!controller.signal.aborted && !decision) {
            const { done, value } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true }).replace(/\r/g, "");
            const boundary = buffer.lastIndexOf("\n\n");
            if (boundary < 0) continue;
            for (const block of buffer.slice(0, boundary).split("\n\n")) {
              let event = "message";
              const data: string[] = [];
              for (const line of block.split("\n")) {
                if (line.startsWith("event:")) event = line.slice(6).trim();
                if (line.startsWith("data:")) data.push(line.slice(5).trimStart());
              }
              if (event === "pairing_approved") decision = "approved";
              if (event === "pairing_rejected") throw new PairingApprovalTerminal(new Error(data.join("\n") || "统筹中心拒绝了本次配对"));
              if (event === "pairing_expired") throw new PairingApprovalTerminal(new Error("配对申请已过期，请重新生成二维码"));
            }
            buffer = buffer.slice(boundary + 2);
          }
        } finally {
          reader.releaseLock();
        }
        if (decision === "approved") return;
        throw new PairingApprovalReconnect(new Error("配对审批事件流在批准前结束"));
      } catch (error) {
        lastError = error;
        if (controller.signal.aborted) throw lastError;
        if (error instanceof PairingApprovalTerminal) throw error.cause;
        if (!(error instanceof PairingApprovalReconnect)) {
          lastError = new PairingApprovalReconnect(error instanceof Error ? error : new Error(String(error)));
        }
        reconnectAttempt++;
        const delay = Math.min(10_000, 1000 * 2 ** Math.min(reconnectAttempt - 1, 4));
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }
    throw lastError ?? new Error("等待配对审批超时，请重新生成二维码");
  } finally {
    clearTimeout(timeout);
  }
}

class PairingApprovalReconnect extends Error {
  constructor(cause: Error) {
    super(cause.message, { cause });
    this.name = "PairingApprovalReconnect";
  }
}

class PairingApprovalTerminal extends Error {
  constructor(readonly cause: Error) {
    super(cause.message, { cause });
    this.name = "PairingApprovalTerminal";
  }
}

export async function createPairingOffer(cloudBaseURL: string, ttlSeconds = 300): Promise<CloudPairingOfferResponse> {
  let endpoint = new URL(cloudBaseURL).origin;
  if (sameOrigin(cloudBaseURL, LOCAL_MESH_BASE_URL)) {
    const response = await fetch(LOCAL_MESH_BASE_URL + "/internal/device-mesh/lan", { headers: getLocalAdminHeaders() });
    if (!response.ok) throw await responseError(response, "无法读取局域网地址");
    const data = await response.json() as { endpoints: { url: string }[] };
    if (!data.endpoints?.length) throw new Error("当前设备没有可用的局域网加密地址");
    endpoint = data.endpoints[0].url;
  }
  const res = await cloudFetch(cloudBaseURL, "/api/device-mesh/v1/pairing/offers", "POST", { ttlSeconds, endpoint }, true);
  if (!res.ok) throw await responseError(res, "create pairing offer failed");
  const offer = await res.json() as CloudPairingOfferResponse;
  return offer;
}

export async function listDevices(cloudBaseURL: string): Promise<CloudDeviceListResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/device-mesh/v1/devices", "GET", undefined, true);
  if (!res.ok) throw await responseError(res, "list devices failed");
  return res.json() as Promise<CloudDeviceListResponse>;
}

export async function revokeDevice(cloudBaseURL: string, deviceId: string): Promise<void> {
  const res = await cloudFetch(cloudBaseURL, `/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}`, "DELETE", undefined, true);
  if (!res.ok) throw await responseError(res, "revoke device failed");
}

export async function probeRuntime(cloudBaseURL: string, deviceId: string, runtimeId: string): Promise<CloudProbeResponse> {
  const res = await cloudFetch(cloudBaseURL, `/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}/runtimes/${encodeURIComponent(runtimeId)}/probe`, "POST", undefined, true);
  if (!res.ok) throw await responseError(res, "probe runtime failed");
  return res.json() as Promise<CloudProbeResponse>;
}
