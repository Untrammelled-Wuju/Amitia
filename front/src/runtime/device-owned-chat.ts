import { resolveApiUrl } from "./runtime-adapter";
import { createAuthenticatedFetchInit } from "./request-auth";
import type { OwnedAttachment } from "./device-owned-attachments";
import type { OwnedConversationOrigin } from "./device-owned-conversation-reference";

export interface OwnedExecutionScope {
  spaceId: string;
  initiatorDeviceId: string;
  targetDeviceId: string;
  coreId: string;
  providerEpoch: number;
  coordinated: boolean;
  modeRevision: number;
  permissionRevision: number;
  targetPermissionRevision: number;
  targetProviderEpoch: number;
  roleId: string;
  roleRevision: number;
  roleOwnerId: string;
  resourceOwnerId: string;
  requestId: string;
  turnId: string;
  executionId: string;
}

export interface OwnedChatResponse {
  conversationOrigin?: OwnedConversationOrigin;
	transcription?: string;
	userRevision?: number;
  conversationId: string;
  requestId: string;
  turnId: string;
  executionId: string;
  executionScope: OwnedExecutionScope;
  reply: string;
  reasoning?: string;
  saved: boolean;
  interrupted?: boolean;
  memoryStatus: string;
  memoryError?: string;
}

export interface OwnedChatEvent {
  type: "started" | "transcribed" | "delta" | "completed" | "interrupted" | "failed";
  executionScope?: OwnedExecutionScope;
  conversationId?: string;
  text?: string;
  reasoning?: boolean;
  message?: string;
  data?: OwnedChatResponse;
}

export interface OwnedChatRequest {
  conversationOrigin?: OwnedConversationOrigin;
  attachments?: OwnedAttachment[];
  expectedExecutionScope?: OwnedExecutionScope;
  context?: { previousCoreId: string; conversationId: string; summary?: string; messages: Array<{ id: string; ownerId?: string; role: string; content: string; status?: string }> };
  requestId: string;
  conversationId?: string;
  characterId?: string;
  historicalRoleId?: string;
  targetDeviceId?: string;
  message: string;
}

function scopeFingerprint(scope: OwnedExecutionScope): string {
  return JSON.stringify([
    scope.spaceId, scope.initiatorDeviceId, scope.targetDeviceId, scope.coreId,
    scope.providerEpoch, scope.coordinated, scope.modeRevision, scope.permissionRevision,
    scope.targetPermissionRevision, scope.targetProviderEpoch, scope.roleId, scope.roleRevision,
    scope.roleOwnerId, scope.resourceOwnerId, scope.requestId, scope.turnId, scope.executionId,
  ]);
}

function validateScope(scope: OwnedExecutionScope | undefined, requestId: string): OwnedExecutionScope {
  if (!scope || scope.requestId !== requestId || !scope.coreId || !scope.roleId || !scope.resourceOwnerId || !scope.roleOwnerId || !scope.turnId || !scope.executionId || !Number.isSafeInteger(scope.providerEpoch) || scope.providerEpoch < 1 || !Number.isSafeInteger(scope.modeRevision) || scope.modeRevision < 1) {
    throw new Error("回复缺少有效的服务提供者和数据归属信息");
  }
  return scope;
}

export async function consumeOwnedChatStream(
  body: ReadableStream<Uint8Array>,
  requestId: string,
  onEvent: (event: OwnedChatEvent) => void,
): Promise<OwnedChatResponse> {
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffered = "";
  let fingerprint = "";
  let completed: OwnedChatResponse | undefined;
  let terminal = false;
  let totalBytes = 0;
  let transcription: string | undefined;
  const processFrame = (frame: string) => {
    const lines = frame.split(/\r?\n/).filter((line) => line.startsWith("data:"));
    if (!lines.length) return;
    if (terminal) throw new Error("回复结束后仍收到数据，已拦截");
    const event = JSON.parse(lines.map((line) => line.slice(5).trimStart()).join("\n")) as OwnedChatEvent;
    if (!["started", "transcribed", "delta", "completed", "interrupted", "failed"].includes(event.type)) throw new Error("未知的回复事件");
    if (event.type === "started" || event.type === "delta" || event.type === "transcribed") {
      const current = scopeFingerprint(validateScope(event.executionScope, requestId));
      if (event.type === "started") {
        if (fingerprint) throw new Error("重复的回复开始事件");
        fingerprint = current;
      } else if (!fingerprint || current !== fingerprint) {
        throw new Error("服务提供者、角色或数据归属已变化，迟到回复已拦截");
      }
      if (event.type === "transcribed") {
        if (transcription !== undefined || typeof event.text !== "string" || !event.text.trim() || new TextEncoder().encode(event.text).byteLength > 65536) throw new Error("语音转写事件无效");
        transcription = event.text;
      }
    } else {
      if (event.data?.executionScope) {
        const current = scopeFingerprint(validateScope(event.data.executionScope, requestId));
        if (fingerprint && current !== fingerprint) throw new Error("回复结果与当前服务提供者不一致");
      }
      if (event.type === "completed") {
        if (!event.data?.saved || event.data.requestId !== requestId) throw new Error("回复尚未确认保存");
        validateScope(event.data.executionScope, requestId);
        if (transcription !== undefined || event.data.transcription !== undefined) {
          const text = event.data.transcription;
          if (typeof text !== "string" || !text.trim() || new TextEncoder().encode(text).byteLength > 65536 || event.data.userRevision !== 2 || (transcription !== undefined && text !== transcription)) throw new Error("语音转写结果与保存确认不一致");
        } else if (event.data.userRevision !== undefined && event.data.userRevision !== 1) {
          throw new Error("消息版本与保存确认不一致");
        }
        completed = event.data;
      }
      terminal = true;
    }
    onEvent(event);
    if (event.type === "interrupted" || event.type === "failed") throw new Error(event.message || "当前回复已中断");
  };
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (value) {
        totalBytes += value.byteLength;
        if (totalBytes > 4 * 1024 * 1024) throw new Error("回复数据超出接收上限");
        buffered += decoder.decode(value, { stream: true });
      }
      if (done) buffered += decoder.decode();
      for (;;) {
        const match = /\r?\n\r?\n/.exec(buffered);
        if (!match || match.index === undefined) break;
        const frame = buffered.slice(0, match.index);
        buffered = buffered.slice(match.index + match[0].length);
        processFrame(frame);
      }
      if (buffered.length > 512 * 1024) throw new Error("回复事件超出接收上限");
      if (done) break;
    }
    if (buffered.trim() || !terminal || !completed) throw new Error("回复连接已断开，结果尚未确认；不会自动重新发送");
    return completed;
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
}

export async function streamOwnedChat(request: OwnedChatRequest, signal: AbortSignal, onEvent: (event: OwnedChatEvent) => void): Promise<OwnedChatResponse> {
  const path = "/api/device-mesh/v1/business/messages";
  const url = await resolveApiUrl(path);
  const init = await createAuthenticatedFetchInit(path, {
    method: "POST", signal,
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify(request),
  });
  const response = await fetch(url, init);
  if (!response.ok || !response.body || !response.headers.get("Content-Type")?.includes("text/event-stream")) {
    const error = await response.json().catch(() => null);
    throw new Error(error?.message || error?.msg || `云端对话服务不可用 (${response.status})`);
  }
  return consumeOwnedChatStream(response.body, request.requestId, onEvent);
}
