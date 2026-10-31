import { resolveApiUrl } from "./runtime-adapter";
import { createAuthenticatedFetchInit } from "./request-auth";
import type { OwnedAttachment } from "./device-owned-attachments";
import type { OwnedConversationOrigin } from "./device-owned-conversation-reference";
import { sameOwnedAuthority } from "./owned-speech-result";
import { normalizeCitationSnapshot, type CitationSnapshotEntry } from "@/conversation/runtime/citationSnapshot";
import { recordClientError } from "./client-error-tracker";

export interface OwnedExecutionScope {
  memoryRoleId?: string;
  memoryViewAll?: boolean;
  continuityRoleId?: string;
  authorizationRealm?: string;
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
  conversationRoleId?: string;
}

export interface OwnedChatResponse {
  resourceConversationId?: string;
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
  citationSnapshot?: CitationSnapshotEntry[];
  saved: boolean;
  interrupted?: boolean;
  memoryStatus: string;
  memoryError?: string;
  groupRoundId?: string;
  deliveryId?: string;
  characterName?: string;
  characterAvatar?: string;
}

export interface OwnedChatToolProgress {
  toolCallId: string;
  toolName: string;
  arguments?: string;
  result?: string;
  status?: string;
  errorCode?: string;
  error?: string;
  durationMs?: number;
}

export interface OwnedChatEvent {
  type: "started" | "transcribed" | "delta" | "task" | "tool-started" | "tool-completed" | "reasoning-completed" | "output-completed" | "group.delivery.started" | "group.delivery.completed" | "group.delivery.failed" | "group.round.completed" | "group.completed" | "completed" | "interrupted" | "failed";
  task?: { coordinationId: string; status: string };
  progress?: OwnedChatToolProgress;
  executionScope?: OwnedExecutionScope;
  conversationId?: string;
  text?: string;
  reasoning?: boolean;
  message?: string;
  errorCode?: string;
  internalMessage?: string;
  retryable?: boolean;
  data?: OwnedChatResponse;
  groupRoundId?: string;
  deliveryId?: string;
  childRequestId?: string;
  deliveryIndex?: number;
  characterId?: string;
  characterName?: string;
  characterAvatar?: string;
  roleOwnerId?: string;
}

export interface OwnedGroupMember {
  memberId: string;
  characterId: string;
  roleOwnerId: string;
  characterName: string;
  characterAvatar?: string;
  status: "active" | "left";
  orderIndex: number;
  joinedSequence: number;
  historyStartSequence: number;
  leftSequence?: number;
  characterRevision: number;
}

export interface OwnedGroupSettings {
  allowCharacterMentions: boolean;
  maxAutoReplies: number;
}

export interface OwnedGroupState {
  conversationId: string;
  conversationRoleId: string;
  roleOwnerId: string;
  title: string;
  conversationType: "single" | "group";
  membershipRevision: number;
  settingsRevision: number;
  lastMessageSequence: number;
  groupSettings: OwnedGroupSettings;
  members: OwnedGroupMember[];
}

export interface OwnedGroupMention {
  memberId: string;
  characterId: string;
  roleOwnerId?: string;
}

export interface OwnedGroupDelivery {
  deliveryId: string;
  groupRoundId: string;
  childRequestId: string;
  memberId: string;
  characterId: string;
  roleOwnerId: string;
  characterName: string;
  orderIndex: number;
  turnIndex?: number;
  source: string;
  status: "queued" | "starting" | "running" | "completed" | "failed" | "cancelled";
  error?: string;
  messageId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface OwnedGroupChatResponse {
  conversationId: string;
  requestId: string;
  groupRoundId: string;
  status: string;
  userMessageId: string;
  deliveries: OwnedChatResponse[];
  saved: boolean;
}

export interface OwnedGroupChatRequest extends Omit<OwnedChatRequest, "characterId"> {
  characterId: string;
  group: {
    expectedMembershipRevision: number;
    expectedSettingsRevision: number;
    replyToMessageId?: string;
    mode?: "standard" | "discussion";
  };
  mentions: OwnedGroupMention[];
}

export interface OwnedChatRequest {
  quote?: { ownerId: string; characterId: string; conversationId: string; messageId: string; expectedRevision: number; contentHash: string; expectedExecutionScope: OwnedExecutionScope };
  conversationOrigin?: OwnedConversationOrigin;
  attachments?: OwnedAttachment[];
  expectedExecutionScope?: OwnedExecutionScope;
  context?: { previousCoreId: string; conversationId: string; summary?: string; messages: Array<{ id: string; ownerId?: string; role: string; content: string; status?: string }> };
  requestId: string;
  conversationId?: string;
  projectId?: string;
  characterId?: string;
  historicalRoleId?: string;
  targetDeviceId?: string;
  permissionMode?: string;
  modelConfigId?: number;
  reasoningEffort?: string;
  reasoningEnabled?: boolean;
  messageStyle?: string;
  workingDirectory?: string;
  message: string;
}

function scopeFingerprint(scope: OwnedExecutionScope): string {
  return JSON.stringify([
    scope.authorizationRealm,
    scope.spaceId, scope.initiatorDeviceId, scope.targetDeviceId, scope.coreId,
    scope.providerEpoch, scope.coordinated, scope.modeRevision, scope.permissionRevision,
    scope.targetPermissionRevision, scope.targetProviderEpoch, scope.roleId, scope.roleRevision,
    scope.roleOwnerId, scope.resourceOwnerId, scope.requestId, scope.turnId, scope.executionId,
    scope.conversationRoleId,
  ]);
}

function validateScope(scope: OwnedExecutionScope | undefined, requestId: string): OwnedExecutionScope {
  if (!scope || scope.requestId !== requestId || !scope.coreId || !scope.roleId || !scope.resourceOwnerId || !scope.roleOwnerId || !scope.turnId || !scope.executionId || !Number.isSafeInteger(scope.providerEpoch) || scope.providerEpoch < 1 || !Number.isSafeInteger(scope.modeRevision) || scope.modeRevision < 1) {
    throw new Error("回复缺少有效的服务提供者和数据归属信息");
  }
  return scope;
}

function validateToolProgress(event: OwnedChatEvent, conversationId: string): OwnedChatToolProgress {
  const progress = event.progress;
  if (!conversationId || event.conversationId !== conversationId || !progress || typeof progress !== "object") {
    throw new Error("工具事件归属或载荷无效");
  }
  const toolCallId = typeof progress.toolCallId === "string" ? progress.toolCallId.trim() : "";
  const toolName = typeof progress.toolName === "string" ? progress.toolName.trim() : "";
  if (!toolCallId || toolCallId.length > 256 || !toolName || toolName.length > 128 || !/^[A-Za-z0-9._:-]+$/.test(toolName)) {
    throw new Error("工具事件身份无效");
  }
  const argumentsText = progress.arguments == null ? "" : String(progress.arguments);
  if (new TextEncoder().encode(argumentsText).byteLength > 256 * 1024) throw new Error("工具事件参数超出上限");
  if (argumentsText) {
    try {
      JSON.parse(argumentsText);
    } catch (error) {
  console.error("[original-error]", error);
throw new Error("工具事件参数不是有效 JSON");
    }
  }
  const result = progress.result == null ? "" : String(progress.result);
  if (new TextEncoder().encode(result).byteLength > 1024 * 1024) throw new Error("工具事件结果超出上限");
  if (progress.durationMs != null && (!Number.isFinite(progress.durationMs) || progress.durationMs < 0)) {
    throw new Error("工具事件耗时无效");
  }
  if (event.type === "tool-completed") {
    const status = typeof progress.status === "string" ? progress.status.trim() : "";
    if (!status) throw new Error("工具完成事件缺少状态");
    const errorCode = progress.errorCode == null ? "" : String(progress.errorCode);
    const errorMessage = progress.error == null ? "" : String(progress.error);
    if (["failed", "cancelled", "canceled", "timed_out"].includes(status.toLowerCase()) && !errorCode && !errorMessage) {
      throw new Error("工具失败事件缺少错误信息");
    }
  }
  return { ...progress, toolCallId, toolName };
}

export async function consumeOwnedChatStream(
  body: ReadableStream<Uint8Array>,
  requestId: string,
  onEvent: (event: OwnedChatEvent) => void,
  expectedExecutionScope?: OwnedExecutionScope,
): Promise<OwnedChatResponse> {
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffered = "";
  let fingerprint = "";
  let completed: OwnedChatResponse | undefined;
  let terminal = false;
  let totalBytes = 0;
  let transcription: string | undefined;
	let conversationId = "";
  const processFrame = (frame: string) => {
    const lines = frame.split(/\r?\n/).filter((line) => line.startsWith("data:"));
    if (!lines.length) return;
    if (terminal) throw new Error("回复结束后仍收到数据，已拦截");
    const event = JSON.parse(lines.map((line) => line.slice(5).trimStart()).join("\n")) as OwnedChatEvent;
    const suppliedScope = event.executionScope || event.data?.executionScope;
    if (expectedExecutionScope && suppliedScope && !sameOwnedAuthority(expectedExecutionScope, suppliedScope)) throw new Error("服务提供者、角色或数据归属已变化，迟到回复已拦截");
    if (!["started", "transcribed", "delta", "task", "tool-started", "tool-completed", "reasoning-completed", "output-completed", "completed", "interrupted", "failed"].includes(event.type)) throw new Error("未知的回复事件");
    if (event.type === "started" || event.type === "delta" || event.type === "transcribed" || event.type === "task" || event.type === "tool-started" || event.type === "tool-completed" || event.type === "reasoning-completed" || event.type === "output-completed") {
      const current = scopeFingerprint(validateScope(event.executionScope, requestId));
      if (event.type === "started") {
        if (fingerprint) throw new Error("重复的回复开始事件");
        fingerprint = current;
		conversationId = event.conversationId || "";
      } else if (!fingerprint || current !== fingerprint) {
        throw new Error("服务提供者、角色或数据归属已变化，迟到回复已拦截");
      }
      if (event.type === "transcribed") {
        if (transcription !== undefined || typeof event.text !== "string" || !event.text.trim() || new TextEncoder().encode(event.text).byteLength > 65536) throw new Error("语音转写事件无效");
        transcription = event.text;
      }
      if (event.type === "task" && (!conversationId || event.conversationId !== conversationId || !event.task || typeof event.task.coordinationId !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(event.task.coordinationId) || !["planning", "running", "waiting", "aggregating", "succeeded", "failed", "cancelled", "paused"].includes(event.task.status))) throw new Error("任务事件归属或状态无效");
      if (event.type === "tool-started" || event.type === "tool-completed") {
        event.progress = validateToolProgress(event, conversationId);
      }
    } else {
      if (event.data?.executionScope) {
        const current = scopeFingerprint(validateScope(event.data.executionScope, requestId));
        if (fingerprint && current !== fingerprint) throw new Error("回复结果与当前服务提供者不一致");
      }
      if (event.type === "completed") {
        if (!event.data?.saved || event.data.requestId !== requestId) throw new Error("回复尚未确认保存");
        validateScope(event.data.executionScope, requestId);
        if (event.data.citationSnapshot != null && !Array.isArray(event.data.citationSnapshot)) throw new Error("引用快照格式无效");
        event.data.citationSnapshot = normalizeCitationSnapshot(event.data.citationSnapshot);
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
    if (event.type === "interrupted" || event.type === "failed") throw new Error(event.message || (event.type === "interrupted" ? "当前回复已停止" : "任务执行失败，未返回具体原因"));
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
    await reader.cancel().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
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
    const error = await response.json().catch((error) => {
  console.error("[original-error]", error);
  return null;
});
    throw new Error(error?.message || error?.msg || `设备对话服务不可用 (${response.status})`);
  }
  return consumeOwnedChatStream(response.body, request.requestId, onEvent, request.expectedExecutionScope);
}

export async function streamOwnedGroupChat(request: OwnedGroupChatRequest, signal: AbortSignal, onEvent: (event: OwnedChatEvent) => void): Promise<OwnedGroupChatResponse> {
  const path = "/api/device-mesh/v1/business/messages";
  const url = await resolveApiUrl(path);
  const init = await createAuthenticatedFetchInit(path, {
    method: "POST",
    signal,
    headers: { "Content-Type": "application/json", Accept: "text/event-stream" },
    body: JSON.stringify(request),
  });
  const response = await fetch(url, init);
  if (!response.ok || !response.body || !response.headers.get("Content-Type")?.includes("text/event-stream")) {
    const error = await response.json().catch((error) => {
  console.error("[original-error]", error);
  return null;
});
    throw new Error(error?.message || error?.msg || `群聊服务不可用 (${response.status})`);
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffered = "";
  let totalBytes = 0;
  let roundId = "";
  let conversationId = "";
  let terminal = false;
  let completed: OwnedGroupChatResponse | undefined;
  const scopes = new Map<string, string>();
  const processFrame = (frame: string) => {
    const lines = frame.split(/\r?\n/).filter((line) => line.startsWith("data:"));
    if (!lines.length) return;
    if (terminal) throw new Error("群聊结束后仍收到数据，已拦截");
    const event = JSON.parse(lines.map((line) => line.slice(5).trimStart()).join("\n")) as OwnedChatEvent;
    if (event.type === "completed") return;
    if (event.type === "group.completed") {
      const data = event.data as unknown as OwnedGroupChatResponse;
      if (!data || data.saved !== true || data.requestId !== request.requestId || !data.groupRoundId || !Array.isArray(data.deliveries)) throw new Error("群聊结果尚未确认保存");
      if (conversationId && data.conversationId !== conversationId) throw new Error("群聊结果与会话归属不一致");
      if (roundId && data.groupRoundId !== roundId) throw new Error("群聊结果与轮次归属不一致");
      const deliveryIDs = new Set<string>();
      for (const delivery of data.deliveries) {
        const deliveryId = String(delivery?.deliveryId || "").trim();
        if (!deliveryId || deliveryIDs.has(deliveryId) || delivery.saved !== true || delivery.groupRoundId !== data.groupRoundId || delivery.conversationId !== data.conversationId || !delivery.requestId) throw new Error("群成员回复归属无效");
        const scope = validateScope(delivery.executionScope, delivery.requestId);
        const knownScope = scopes.get(deliveryId);
        if (knownScope && knownScope !== scopeFingerprint(scope)) throw new Error("群成员最终回复与流式投递身份不一致");
        deliveryIDs.add(deliveryId);
      }
      completed = data;
      terminal = true;
      onEvent(event);
      return;
    }
    if (event.type === "failed" || event.type === "interrupted") {
      terminal = true;
      onEvent(event);
      throw new Error(event.message || "当前群聊已中断");
    }
    if (event.groupRoundId) {
      if (roundId && roundId !== event.groupRoundId) throw new Error("群聊轮次归属已变化");
      roundId = event.groupRoundId;
    }
    if (event.conversationId) {
      if (conversationId && conversationId !== event.conversationId) throw new Error("群聊会话归属已变化");
      conversationId = event.conversationId;
    }
    if (event.type === "started") {
      if (!event.deliveryId || !event.executionScope) throw new Error("群成员回复缺少投递身份");
      const scope = validateScope(event.executionScope, event.executionScope.requestId);
      const fingerprint = scopeFingerprint(scope);
      const known = scopes.get(event.deliveryId);
      if (known && known !== fingerprint) throw new Error("群成员投递身份在流中发生变化");
      scopes.set(event.deliveryId, fingerprint);
    } else if (event.type === "delta" || event.type === "transcribed" || event.type === "task" || event.type === "tool-started" || event.type === "tool-completed" || event.type === "reasoning-completed" || event.type === "output-completed" || event.type === "group.delivery.completed") {
      if (!event.deliveryId || !event.executionScope || !scopes.get(event.deliveryId) || scopes.get(event.deliveryId) !== scopeFingerprint(validateScope(event.executionScope, event.executionScope.requestId))) throw new Error("群成员流事件与投递身份不一致");
    }
    onEvent(event);
  };
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (value) {
        totalBytes += value.byteLength;
        if (totalBytes > 8 * 1024 * 1024) throw new Error("群聊数据超出接收上限");
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
      if (buffered.length > 512 * 1024) throw new Error("群聊事件超出接收上限");
      if (done) break;
    }
    if (buffered.trim() || !terminal || !completed) throw new Error("群聊连接已断开，结果尚未确认；不会自动重新发送");
    return completed;
  } catch (error) {
    if (!signal.aborted) recordClientError({ source: "group-chat-stream", message: `${error instanceof Error ? error.message : String(error)}; receivedBytes=${totalBytes}; requestId=${request.requestId}` });
    throw error;
  } finally {
    await reader.cancel().catch((error) => {
  console.error("[original-error]", error);
  return undefined;
});
    reader.releaseLock();
  }
}
