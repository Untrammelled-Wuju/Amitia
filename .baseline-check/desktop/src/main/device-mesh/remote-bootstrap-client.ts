import { getAccessToken } from "../auth-token-store";
import {
  type CloudBootstrapTicketRequest,
  type CloudBootstrapTicketResponse,
  type CloudDeviceListResponse,
  type CloudProbeResponse,
} from "./protocol";

function authHeaders(): Record<string, string> {
  const token = getAccessToken();
  if (!token) return {};
  return { Authorization: `Bearer ${token}` };
}

async function cloudFetch(
  cloudBaseURL: string,
  path: string,
  method: string,
  body?: unknown,
): Promise<Response> {
  const url = new URL(path, cloudBaseURL).toString();
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...authHeaders(),
  };
  const init: RequestInit = { method, headers };
  if (body) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  return fetch(url, init);
}

export async function createBootstrapTicket(
  cloudBaseURL: string,
  req: CloudBootstrapTicketRequest,
): Promise<CloudBootstrapTicketResponse> {
  const res = await cloudFetch(
    cloudBaseURL,
    "/api/device-mesh/v1/bootstrap-tickets",
    "POST",
    req,
  );
  if (!res.ok) {
    let message = `create ticket failed (status=${res.status})`;
    try {
      const errBody = (await res.json()) as { message?: string };
      if (errBody.message) message = errBody.message;
    } catch {}
    throw new Error(message);
  }
  return res.json() as Promise<CloudBootstrapTicketResponse>;
}

export async function listDevices(cloudBaseURL: string): Promise<CloudDeviceListResponse> {
  const res = await cloudFetch(cloudBaseURL, "/api/device-mesh/v1/devices", "GET");
  if (!res.ok) {
    throw new Error(`list devices failed (status=${res.status})`);
  }
  return res.json() as Promise<CloudDeviceListResponse>;
}

export async function revokeDevice(cloudBaseURL: string, deviceId: string): Promise<void> {
  const res = await cloudFetch(
    cloudBaseURL,
    `/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}`,
    "DELETE",
  );
  if (!res.ok) {
    throw new Error(`revoke device failed (status=${res.status})`);
  }
}

export async function probeRuntime(
  cloudBaseURL: string,
  deviceId: string,
  runtimeId: string,
): Promise<CloudProbeResponse> {
  const res = await cloudFetch(
    cloudBaseURL,
    `/api/device-mesh/v1/devices/${encodeURIComponent(deviceId)}/runtimes/${encodeURIComponent(runtimeId)}/probe`,
    "POST",
  );
  if (!res.ok) {
    throw new Error(`probe runtime failed (status=${res.status})`);
  }
  return res.json() as Promise<CloudProbeResponse>;
}
