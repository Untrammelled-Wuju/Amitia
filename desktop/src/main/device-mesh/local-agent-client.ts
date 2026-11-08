import http from "node:http";
import {
  LOCAL_MESH_BASE_URL,
  type DeviceMeshLocalIdentity,
  type DeviceMeshStatusResponse,
  type DeviceMeshBootstrapRequest,
  type DeviceMeshBootstrapResponse,
  type CloudPairingClaimRequest,
  type DeviceIdentityProof,
  type CloudBootstrapTicketResponse,
  type MeshPairingInput,
} from "./protocol";
import type { LocalVoiceASRFinalEvent } from "../../shared/types";
import { getLocalAdminHeaders } from "../backend-session-client";

export * from "./protocol";

function httpRequest(
  baseUrl: string,
  path: string,
  method: string,
  body?: unknown,
  headers?: Record<string, string>,
  timeoutMs = 10000,
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
        path: url.pathname + url.search,
        method,
        headers: reqHeaders,
        timeout: timeoutMs,
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
  } catch (error) {
  console.error("[original-error]", error);
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
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
}

export async function signMeshPairingClaim(coreId: string, body: CloudPairingClaimRequest): Promise<DeviceIdentityProof> {
  const res = await httpRequest(LOCAL_MESH_BASE_URL, "/internal/device-mesh/identity/sign-claim", "POST", {
    coreId,
    body: { deviceId: body.deviceId, runtimeId: body.runtimeId, platform: body.platform.trim(), label: body.label?.trim() || "", offerToken: body.offerToken?.trim() || "", setupCode: body.setupCode?.trim() || "" },
  }, getAuthHeaders());
  if (res.status !== 200) {
    const error = JSON.parse(res.data) as { message?: string };
    throw new Error(error.message || "无法为本机配对身份签名");
  }
  return JSON.parse(res.data) as DeviceIdentityProof;
}

export async function claimPinnedMeshPairing(baseURL: string, input: MeshPairingInput): Promise<CloudBootstrapTicketResponse> {
  const claim = () => httpRequest(LOCAL_MESH_BASE_URL, "/internal/device-mesh/pairing/claim", "POST", {
    endpoint: { url: baseURL, fingerprint: input.fingerprint, coreId: input.coreId },
    offerToken: input.offerToken, setupCode: input.setupCode,
  }, getAuthHeaders());
  let response = await claim();
  const data = JSON.parse(response.data) as CloudBootstrapTicketResponse & { message?: string };
  if (response.status === 202) {
    const pending = JSON.parse(response.data) as { requestId?: string };
    if (!pending.requestId) throw new Error("统筹中心未返回配对申请编号");
    const query = new URLSearchParams({
      url: baseURL,
      fingerprint: input.fingerprint || "",
      coreId: input.coreId || "",
    });
    const waited = await httpRequest(
      LOCAL_MESH_BASE_URL,
      `/internal/device-mesh/pairing/requests/${encodeURIComponent(pending.requestId)}/events?${query.toString()}`,
      "GET",
      undefined,
      { ...getAuthHeaders(), Accept: "text/event-stream" },
      125000,
    );
    if (waited.status !== 200) throw new Error(waited.data || "等待统筹中心批准配对失败");
    response = await claim();
  }
  const accepted = JSON.parse(response.data) as CloudBootstrapTicketResponse & { message?: string; requestId?: string };
  if (response.status !== 200) throw new Error(accepted.message || "局域网配对失败");
  if (!accepted.ticket || accepted.spaceId !== input.coreId) throw new Error("配对服务身份与二维码不一致");
  return accepted;
}


export interface DeviceMeshCloudAuth {
  authorization: string;
  cloudBaseUrl: string;
  spaceId: string;
  deviceId: string;
  runtimeId: string;
  expiresAt: string;
  fingerprint?: string;
}

export async function getMeshCloudAuth(): Promise<DeviceMeshCloudAuth | null> {
  try {
    const res = await httpRequest(
      LOCAL_MESH_BASE_URL,
      "/internal/device-mesh/cloud-auth",
      "GET",
      undefined,
      getAuthHeaders(),
    );
    if (res.status !== 200) return null;
    const parsed = JSON.parse(res.data) as DeviceMeshCloudAuth;
    if (!parsed.authorization?.startsWith("AmitiaDevice ")) return null;
    return parsed;
  } catch (error) {
  console.error("[original-error]", error);
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
  } catch (error) {
  console.error("[original-error]", error);
return null;
  }
}

export function watchMeshState(
  onState: (state: string) => void,
  onEnd: (error?: Error) => void,
): () => void {
  const url = new URL("/internal/device-mesh/status/events", LOCAL_MESH_BASE_URL);
  let closed = false;
  const req = http.request({
    hostname: url.hostname,
    port: url.port,
    path: url.pathname,
    method: "GET",
    headers: { ...getAuthHeaders(), Accept: "text/event-stream" },
  }, (res) => {
    if (res.statusCode !== 200 || !String(res.headers["content-type"] || "").includes("text/event-stream")) {
      res.resume();
      onEnd(new Error(`设备状态事件流连接失败: HTTP ${res.statusCode ?? 0}`));
      return;
    }
    res.setEncoding("utf8");
    res.on("data", (chunk: string) => {
      for (const line of chunk.split("\n")) {
        if (!line.startsWith("data:")) continue;
        try {
          const payload = JSON.parse(line.slice(5).trim()) as { state?: string };
          if (payload.state) onState(payload.state);
        } catch (error) {
          onEnd(error instanceof Error ? error : new Error(String(error)));
          req.destroy();
          return;
        }
      }
    });
    res.on("end", () => {
      if (!closed) onEnd();
    });
    res.on("error", (error) => {
      if (!closed) onEnd(error);
    });
  });
  req.on("error", (error) => {
    if (!closed) onEnd(error);
  });
  req.end();
  return () => {
    closed = true;
    req.destroy();
  };
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
    } catch (error) {
  console.error("[original-error]", error);
}
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
  } catch (error) {
  console.error("[original-error]", error);
}
}


const MAX_VOICE_ASR_TRANSCRIPT_CHARS = 16_384;
const MAX_WORKFLOW_EVENT_ID_LENGTH = 200;
const MAX_CONTEXT_ID_LENGTH = 256;
const MAX_VISUAL_CONTEXT_CHARS = 6_000;
const WORKFLOW_EVENT_ID_RE = /^[A-Za-z0-9._:-]+$/;

function assertOptionalContextID(value: unknown, field: string): string {
  if (value === undefined || value === null || value === "") return "";
  if (typeof value !== "string") throw new Error(`${field} must be a string`);
  const normalized = value.trim();
  if (normalized.length > MAX_CONTEXT_ID_LENGTH) {
    throw new Error(`${field} is too long`);
  }
  return normalized;
}

export async function publishLocalVoiceASRFinal(
  event: LocalVoiceASRFinalEvent,
): Promise<{ accepted: boolean; eventId: string; eventType: string }> {
  if (!event || typeof event !== "object") {
    throw new Error("voice ASR final event is required");
  }
  const eventId = typeof event.eventId === "string" ? event.eventId.trim() : "";
  if (
    !eventId ||
    eventId.length > MAX_WORKFLOW_EVENT_ID_LENGTH ||
    !WORKFLOW_EVENT_ID_RE.test(eventId)
  ) {
    throw new Error("voice ASR final eventId is invalid");
  }
  const transcript = typeof event.transcript === "string" ? event.transcript.trim() : "";
  if (!transcript) throw new Error("voice ASR final transcript is required");
  if ([...transcript].length > MAX_VOICE_ASR_TRANSCRIPT_CHARS) {
    throw new Error(`voice ASR final transcript exceeds ${MAX_VOICE_ASR_TRANSCRIPT_CHARS} characters`);
  }

  const sessionId = assertOptionalContextID(event.sessionId, "sessionId");
  const conversationId = assertOptionalContextID(event.conversationId, "conversationId");
  const characterId = assertOptionalContextID(event.characterId, "characterId");
  const visualContext = typeof event.visualContext === "string" ? event.visualContext.trim() : "";
  if ([...visualContext].length > MAX_VISUAL_CONTEXT_CHARS) {
    throw new Error(`visualContext exceeds ${MAX_VISUAL_CONTEXT_CHARS} characters`);
  }
  const visualSource = event.visualSource === "camera" || event.visualSource === "screen" ? event.visualSource : "";
  let occurredAt = new Date().toISOString();
  if (event.occurredAt) {
    if (typeof event.occurredAt !== "string") throw new Error("occurredAt must be a string");
    const parsed = Date.parse(event.occurredAt);
    if (Number.isFinite(parsed)) occurredAt = new Date(parsed).toISOString();
  }

  const identity = await getMeshIdentity();
  const headers = getAuthHeaders();
  if (identity?.deviceId) headers["X-Amitia-Device-ID"] = identity.deviceId;
  const res = await httpRequest(
    LOCAL_MESH_BASE_URL,
    "/api/local/workflows/events/voice.asr.final",
    "POST",
    {
      eventId,
      source: "voice.asr",
      occurredAt,
      payload: {
        transcript,
        sessionId,
        conversationId,
        characterId,
        visualContext,
        visualSource,
        final: true,
      },
    },
    headers,
  );
  if (res.status < 200 || res.status >= 300) {
    let message = `local voice workflow event failed (status=${res.status})`;
    try {
      const parsed = JSON.parse(res.data) as { error?: string; message?: string };
      message = parsed.error || parsed.message || message;
    } catch (error) {
  console.error("[original-error]", error);
}
    throw new Error(message);
  }
  try {
    return JSON.parse(res.data) as { accepted: boolean; eventId: string; eventType: string };
  } catch (error) {
  console.error("[original-error]", error);
return { accepted: true, eventId, eventType: "voice.asr.final" };
  }
}
