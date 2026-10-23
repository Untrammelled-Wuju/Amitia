import type { OwnedExecutionScope } from "./device-owned-chat";
import { ownedManagementRequest } from "./owned-chat-management";

export interface MessageFeedback {
  id: number;
  messageId: string;
  conversationId: string;
  feedbackType: string;
  reason: string;
  revision: number;
  createdAt: string;
  updatedAt: string;
}

export async function loadMessageFeedback(messageId: string, roleId: string, expectedExecutionScope?: OwnedExecutionScope): Promise<MessageFeedback | null> {
  if (!messageId || !roleId) return null;
  const result = await ownedManagementRequest<{ items?: MessageFeedback[] }>("feedback/list", { messageId, expectedExecutionScope }, "post", roleId);
  return Array.isArray(result.items) && result.items.length ? result.items[0] : null;
}

export async function createMessageFeedback(messageId: string, roleId: string, feedbackType: string, reason = "", expectedExecutionScope?: OwnedExecutionScope): Promise<MessageFeedback> {
  const result = await ownedManagementRequest<{ feedback?: MessageFeedback }>("feedback/create", { messageId, feedbackType, reason, expectedExecutionScope }, "post", roleId);
  if (!result.feedback?.id) throw new Error("反馈保存结果无效");
  return result.feedback;
}

export async function deleteMessageFeedback(feedbackId: number, roleId: string, expectedExecutionScope?: OwnedExecutionScope): Promise<void> {
  const result = await ownedManagementRequest<{ deleted?: boolean; revision?: number }>("feedback/delete", { feedbackId, expectedExecutionScope }, "post", roleId);
  if (result.deleted !== true || !Number.isFinite(result.revision)) throw new Error("反馈删除尚未确认");
}
