import { describe, expect, it } from "vitest";
import {
  getClientMessageId,
  getMessageUIKey,
  mergeChatMessage,
  normalizeRealtimeMessage,
} from "../utils/message-order";
import messagesAreaSource from "../components/MessagesArea.vue?raw";
import chatBubbleSource from "../components/ChatBubble.vue?raw";

describe("聊天消息身份与动效", () => {
  it("服务端消息 ID 回填后保持同一个渲染身份", () => {
    const message = {
      id: "user-local",
      role: "user",
      requestId: "request-1",
      clientMessageId: "request-1",
      uiKey: "client:request-1",
    };
    const before = getMessageUIKey(message);
    message.id = "server-message-1";
    expect(getMessageUIKey(message)).toBe(before);
  });

  it("实时回执按客户端消息标识对账并保留 UI 状态", () => {
    const messages = [
      {
        id: "user-local",
        role: "user",
        requestId: "request-1",
        clientMessageId: "request-1",
        uiKey: "client:request-1",
        animateIn: false,
        status: "sending",
      },
    ];
    expect(
      mergeChatMessage(messages, {
        id: "server-message-1",
        role: "user",
        requestId: "request-1",
        status: "queued",
      }),
    ).toBe(true);
    expect(messages).toHaveLength(1);
    expect(messages[0]).toMatchObject({
      id: "server-message-1",
      clientMessageId: "request-1",
      uiKey: "client:request-1",
      animateIn: false,
      status: "queued",
    });
  });

  it("历史用户消息从 requestId 恢复客户端身份", () => {
    const message = normalizeRealtimeMessage({
      id: "server-message-1",
      role: "user",
      request_id: "request-1",
    });
    expect(getClientMessageId(message)).toBe("request-1");
    expect(getMessageUIKey(message)).toBe("client:request-1");
  });

  it("同一请求下的助手消息仍按服务端 ID 区分", () => {
    const first = { id: "assistant-1", role: "assistant", requestId: "request-1" };
    const second = { id: "assistant-2", role: "assistant", requestId: "request-1" };
    expect(getMessageUIKey(first)).not.toBe(getMessageUIKey(second));
  });

  it("动画只绑定首次进入标记并支持减少动态效果", () => {
    expect(messagesAreaSource).toContain("item.message.animateIn === true");
    expect(messagesAreaSource).toContain("conversationMessageIn");
    expect(chatBubbleSource).not.toContain("animation: bubbleIn");
  });
});
