import { describe, expect, it } from "vitest";
import {
  getClientMessageId,
  getMessageUIKey,
  mergeChatMessage,
  mergeServerMessages,
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

  it("持久消息对账同时补齐用户消息和助手回复", () => {
    const messages = [
      {
        id: "user-local",
        role: "user",
        requestId: "request-1",
        clientMessageId: "request-1",
        uiKey: "client:request-1",
        status: "sending",
        createdAt: "2026-09-19T10:00:00.900Z",
      },
    ];
    const merged = mergeServerMessages(messages, [
      {
        id: "user-1",
        role: "user",
        requestId: "request-1",
        content: "你好",
        sequence: 1,
        status: "sent",
        createdAt: "2026-09-19 18:00:00",
      },
      {
        id: "assistant-1",
        role: "assistant",
        requestId: "request-1",
        content: "你好呀",
        sequence: 2,
        status: "sent",
        createdAt: "2026-09-19 18:00:00",
      },
    ]);

    expect(merged).toHaveLength(2);
    expect(merged.map((message) => message.id)).toEqual([
      "user-1",
      "assistant-1",
    ]);
    expect(merged[0]).toMatchObject({
      uiKey: "client:request-1",
      clientMessageId: "request-1",
      status: "sent",
    });
    expect(merged[1]).toMatchObject({
      id: "assistant-1",
      content: "你好呀",
      sequence: 2,
    });
  });

  it("历史对账未返回流式回复时保留本地助手消息", () => {
    const messages = [
      {
        id: "user-1",
        conversationId: "conversation-1",
        role: "user",
        requestId: "request-1",
        clientMessageId: "request-1",
        content: "你好",
        status: "sent",
        createdAt: new Date().toISOString(),
      },
      {
        id: "assistant-1",
        conversationId: "conversation-1",
        role: "assistant",
        content: "正在回复",
        status: "streaming",
        createdAt: new Date().toISOString(),
      },
    ];
    const merged = mergeServerMessages(messages, [
      {
        id: "user-1",
        conversationId: "conversation-1",
        role: "user",
        requestId: "request-1",
        content: "你好",
        sequence: 1,
        status: "sent",
        createdAt: new Date().toISOString(),
      },
    ]);

    expect(merged.map((message) => message.id)).toEqual([
      "user-1",
      "assistant-1",
    ]);
    expect(merged[1]).toMatchObject({
      content: "正在回复",
      status: "streaming",
    });
  });

  it("历史对账不会混入其他会话的本地消息", () => {
    const merged = mergeServerMessages(
      [
        {
          id: "failed-old",
          conversationId: "conversation-old",
          role: "user",
          content: "旧消息",
          status: "failed",
          createdAt: new Date().toISOString(),
        },
      ],
      [
        {
          id: "assistant-new",
          conversationId: "conversation-new",
          role: "assistant",
          content: "新回复",
          sequence: 1,
          status: "sent",
          createdAt: new Date().toISOString(),
        },
      ],
    );

    expect(merged.map((message) => message.id)).toEqual(["assistant-new"]);
  });

  it("动画只绑定首次进入标记并支持减少动态效果", () => {
    expect(messagesAreaSource).toContain("item.message.animateIn === true");
    expect(messagesAreaSource).toContain("conversationMessageIn");
    expect(chatBubbleSource).not.toContain("animation: bubbleIn");
  });

  it("思考折叠标题保持单行宽度", () => {
    expect(chatBubbleSource).toContain(".reasoning-block {\n  width: 100%;\n  max-width: 100%;");
    expect(chatBubbleSource).not.toMatch(/\.reasoning-block\s*\{[^}]*max-width:\s*min\(82%,\s*720px\)/);
    expect(chatBubbleSource).toContain("width: max-content");
    expect(chatBubbleSource).toContain("white-space: nowrap");
  });
});
