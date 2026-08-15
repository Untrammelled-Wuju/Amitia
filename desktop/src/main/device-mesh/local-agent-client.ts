import http from "node:http";
import {
  LOCAL_MESH_BASE_URL,
  type DeviceMeshLocalIdentity,
  type DeviceMeshStatusResponse,
  type DeviceMeshBootstrapRequest,
  type DeviceMeshBootstrapResponse,
} from "./protocol";
import { getLocalAdminHeaders } from "../backend-session-client";

export * from "./protocol";

function httpRequest(
  baseUrl: string,
  path: string,
  method: string,
  body?: unknown,
  headers?: Record<string, string>,
): Promise<{ status: number; data: string }> {
  return new Promise((resolve, reject) => {
    const url = new URL(path, baseUrl);
    const bodyData = body ? JSON.stringify(body) : undefined;
    const reqHeaders: Record<string, string> = {
      Accept: "application/json",
      ...headers,
    };
    if (bodyData) {
      reqHeaders["Content-Type"] = "application/json";
    }

    const req = http.request(
      {
        hostname: url.hostname,
        port: url.port,
        path: url.pathname,
        method,
        headers: reqHeaders,
        timeout: 10000,
      },
      (res) => {
        let data = "";
        res.setEncoding("utf8");
        res.on("data", (chunk) => { data += chunk; });
        res.on("end", () => {
          resolve({ status: res.statusCode ?? 0, data });
        });
      },
    );
    req.on("error", reject);
    req.on("timeout", () => {
      req.destroy();
      reject(new Error("request timeout"));
    });
    if (bodyData) {
      req.write(bodyData);
    }
    req.end();
  });
}

function getAuthHeaders(): Record<string, string> {
  try {
    return getLocalAdminHeaders();
  } catch {
    return {};
  }
}

export async function getMeshIdentity(): Promise<DeviceMeshLocalIdentity | null> {
  try {
    const res = await httpRequest(
      LOCAL_MESH_BASE_URL,
      "/internal/device-mesh/identity",
      "GET",
      undefined,
      getAuthHeaders(),
    );
    if (res.status !== 200) return null;
    return JSON.parse(res.data) as DeviceMeshLocalIdentity;
  } catch {
    return null;
  }
}

export async function getMeshStatus(): Promise<DeviceMeshStatusResponse | null> {
  try {
    const res = await httpRequest(
      LOCAL_MESH_BASE_URL,
      "/internal/device-mesh/status",
      "GET",
      undefined,
      getAuthHeaders(),
    );
    if (res.status !== 200) return null;
    return JSON.parse(res.data) as DeviceMeshStatusResponse;
  } catch {
    return null;
  }
}

export async function postMeshBootstrap(req: DeviceMeshBootstrapRequest): Promise<DeviceMeshBootstrapResponse> {
  const res = await httpRequest(
    LOCAL_MESH_BASE_URL,
    "/internal/device-mesh/bootstrap",
    "POST",
    req,
    getAuthHeaders(),
  );
  if (res.status !== 200) {
    let message = `bootstrap failed (status=${res.status})`;
    try {
      const errBody = JSON.parse(res.data) as { message?: string };
      if (errBody.message) message = errBody.message;
    } catch {}
    throw new Error(message);
  }
  return JSON.parse(res.data) as DeviceMeshBootstrapResponse;
}

export async function deleteMeshCredential(): Promise<void> {
  try {
    await httpRequest(
      LOCAL_MESH_BASE_URL,
      "/internal/device-mesh/credential",
      "DELETE",
      undefined,
      getAuthHeaders(),
    );
  } catch {
  }
}
