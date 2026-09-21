import { describe, expect, it } from "vitest";
import {
  getClientMessageId,
  getMessageUIKey,
  normalizeRealtimeMessage,
} from "@/utils/message-order";

describe("v1 消息身份", () => {
  it("用户乐观消息仅使用 clientRequestId/requestId 保持 UI key 稳定", () => {
    const message: any = {
      role: "user",
      requestId: "request-1",
      content: "你好",
    };
    expect(getClientMessageId(message)).toBe("request-1");
    expect(getMessageUIKey(message)).toBe("client:request-1");
    message.id = "msg-server-1";
    expect(getMessageUIKey(message)).toBe("client:request-1");
  });

  it("持久消息没有 clientRequestId 时直接使用服务器 messageId", () => {
    expect(getMessageUIKey({ id: "msg-1", role: "assistant" })).toBe("server:msg-1");
  });

  it("Live Overlay 可显式提供 uiKey，且不依赖 requestId 匹配 assistant placeholder", () => {
    const live = {
      id: "turn:turn-1",
      role: "assistant",
      requestId: "request-1",
      uiKey: "turn:turn-1",
    };
    expect(getMessageUIKey(live)).toBe("turn:turn-1");
  });

  it("实时持久消息字段只做字段名归一化，不承担 Turn 关联", () => {
    const normalized = normalizeRealtimeMessage({
      messageId: "msg-1",
      conversation_id: "conv-1",
      request_id: "request-1",
      role: "assistant",
      created_at: "2026-09-21 10:00:00",
    });
    expect(normalized).toMatchObject({
      id: "msg-1",
      conversationId: "conv-1",
      requestId: "request-1",
      role: "assistant",
      createdAt: "2026-09-21 10:00:00",
    });
    expect(normalized.clientMessageId).toBeUndefined();
  });
});
