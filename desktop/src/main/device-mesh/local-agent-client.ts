import http from "node:http";
import {
  LOCAL_MESH_BASE_URL,
  type DeviceMeshLocalIdentity,
  type DeviceMeshStatusResponse,
  type DeviceMeshBootstrapRequest,
  type DeviceMeshBootstrapResponse,
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


const MAX_VOICE_ASR_TRANSCRIPT_CHARS = 16_384;
const MAX_WORKFLOW_EVENT_ID_LENGTH = 200;
const MAX_CONTEXT_ID_LENGTH = 256;
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
    } catch {}
    throw new Error(message);
  }
  try {
    return JSON.parse(res.data) as { accepted: boolean; eventId: string; eventType: string };
  } catch {
    return { accepted: true, eventId, eventType: "voice.asr.final" };
  }
}
