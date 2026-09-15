import { getMeshCloudAuth } from "./local-agent-client";
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
  const url = new URL(path, cloudBaseURL).toString();
  const headers: Record<string, string> = { Accept: "application/json" };
  if (authenticated) Object.assign(headers, await deviceAuthHeaders(cloudBaseURL));
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
  const res = await cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/status", "GET", undefined, false);
  if (!res.ok) throw await responseError(res, "pairing status failed");
  return res.json() as Promise<CloudPairingStatusResponse>;
}

export async function claimPairing(cloudBaseURL: string, req: CloudPairingClaimRequest): Promise<CloudBootstrapTicketResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/public/device-mesh/v1/pairing/claim", "POST", req, false);
  if (!res.ok) throw await responseError(res, "pairing claim failed");
  return res.json() as Promise<CloudBootstrapTicketResponse>;
}

export async function createPairingOffer(cloudBaseURL: string, ttlSeconds = 300): Promise<CloudPairingOfferResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/device-mesh/v1/pairing/offers", "POST", { ttlSeconds }, true);
  if (!res.ok) throw await responseError(res, "create pairing offer failed");
  const offer = await res.json() as CloudPairingOfferResponse;
  const endpoint = new URL(cloudBaseURL).origin;
  const pairingURL = new URL("amitia://pair");
  pairingURL.searchParams.set("endpoint", endpoint);
  pairingURL.searchParams.set("offer", offer.offerToken);
  return { ...offer, qrPayload: pairingURL.toString() };
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
