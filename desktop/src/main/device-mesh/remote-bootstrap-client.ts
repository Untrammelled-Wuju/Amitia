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
  try { return new URL(a).origin === new URL(b).origin; } catch { return false; }
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
  } catch {
    return new Error(`${fallback} (status=${res.status})`);
  }
}

export async function getPairingStatus(cloudBaseURL: string): Promise<CloudPairingStatusResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/status", "GET");
  if (!res.ok) throw await responseError(res, "pairing status failed");
  return res.json() as Promise<CloudPairingStatusResponse>;
}

export async function claimPairing(cloudBaseURL: string, req: CloudPairingClaimRequest): Promise<CloudBootstrapTicketResponse> {
  const status = await getPairingStatus(cloudBaseURL);
  const deadline = Date.now() + 120_000;
  while (Date.now() < deadline) {
  const proof = await signMeshPairingClaim(status.spaceId, req);
  const res = await cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/claim", "POST", { ...req, proof }, false);
  if (!res.ok) throw await responseError(res, "pairing claim failed");
  if (res.status === 202) { await new Promise<void>((resolve) => setTimeout(resolve, 1000)); continue; }
  return res.json() as Promise<CloudBootstrapTicketResponse>;
  }
  throw new Error("等待服务提供设备批准配对超时，请再次扫码");
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
