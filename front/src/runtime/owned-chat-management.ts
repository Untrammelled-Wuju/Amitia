import { apiClient } from "@/composables/useApi";
import { useDeviceOwnedConversation } from "@/composables/useDeviceOwnedConversation";
import { parseConversationReference } from "./device-owned-conversation-reference";
import { sameOwnedAuthority } from "./owned-speech-result";
import type { OwnedExecutionScope } from "./device-owned-chat";

export async function ownedManagementRequest<T = any>(operation: string, body: Record<string, any> = {}, method: "get" | "post" = "get", roleId = ""): Promise<T> {
  return ownedChatRequest<T>("chat-management", operation, body, roleId);
}

export async function prepareOwnedChatRequest(body: Record<string, any> = {}, roleId = "") {
  const owned = useDeviceOwnedConversation();
  if (!await owned.refresh()) throw new Error(owned.notice.value || "设备对话服务尚未就绪");
  const role = roleId || body.characterId || owned.selectInitialRole(localStorage.getItem(`webchat-char-id:${owned.coreId.value}:${owned.roleOwnerId.value}`) || "");
  if (!role) throw new Error("请先在聊天页面选择需要管理的角色");
  const prepared = await owned.data("working", role);
  const expected = (body.expectedExecutionScope || prepared.executionScope) as OwnedExecutionScope;
  if (!sameOwnedAuthority(expected, prepared.executionScope)) throw new Error("服务或数据归属已变化，请重新预览后操作");
  const origin = parseConversationReference(String(body.conversationId || ""));
  const payload = { ...body, ...(origin ? { conversationId: origin.id, conversationOwnerId: origin.ownerId } : {}), targetDeviceId: expected.targetDeviceId, characterId: role, expectedExecutionScope: expected };
  return { owned, expected, payload: { requestId: crypto.randomUUID(), ...payload } };
}

async function ownedChatRequest<T>(group: string, operation: string, body: Record<string, any>, roleId: string): Promise<T> {
  const { owned, expected, payload } = await prepareOwnedChatRequest(body, roleId);
  const path = `/api/device-mesh/v1/business/${group}/${operation}`;
  const response = await apiClient.post(path, payload);
  const result = response.data?.code === 200 && response.data.data !== undefined ? response.data.data : response.data;
  const resultScope = result?.executionScope;
  const comparableScope = resultScope && expected.conversationRoleId === undefined
    ? { ...resultScope, conversationRoleId: undefined }
    : resultScope;
  if (!resultScope || owned.coreId.value !== expected.coreId || !sameOwnedAuthority(expected, comparableScope)) throw new Error("服务或数据归属已变化，管理结果已丢弃");
  return result as T;
}

export function ownedChatRuntimeRequest<T = any>(operation: string, body: Record<string, any> = {}, roleId = ""): Promise<T> {
  return ownedChatRequest<T>("chat-runtime", operation, body, roleId);
}

export function saveOwnedChatExport(value: { content: string; filename?: string; fileName?: string; mimeType: string }) {
  if (typeof value.content !== "string" || !value.mimeType) throw new Error("导出结果无效");
  const url = URL.createObjectURL(new Blob([value.content], { type: value.mimeType }));
  const link = document.createElement("a");
  link.href = url;
  link.download = value.fileName || value.filename || "amitia-conversation.json";
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 0);
}
