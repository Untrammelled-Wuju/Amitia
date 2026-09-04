import { describe, expect, it } from "vitest";
import {
  isTransientModelErrorReplyEvent,
  isVisionErrorReplyEvent,
  isWebChatReplyEvent,
  shouldFinishSendingForModelError,
} from "../composables/useWebChatSSE";
import {
  compareChatMessages,
  insertTransientModelError,
} from "../utils/message-order";

describe("isWebChatReplyEvent", () => {
  it("只接受回复消息", () => {
    expect(isWebChatReplyEvent({ role: "assistant" })).toBe(true);
    expect(isWebChatReplyEvent({ role: "user" })).toBe(false);
    expect(isWebChatReplyEvent({ role: "tool" })).toBe(false);
    expect(isWebChatReplyEvent({})).toBe(false);
  });

  it("视觉模型错误仍属于回复消息", () => {
    expect(
      isVisionErrorReplyEvent({
        role: "assistant",
        msgType: "vision_error",
      }),
    ).toBe(true);
    expect(
      isVisionErrorReplyEvent({ role: "user", msgType: "vision_error" }),
    ).toBe(false);
  });

  it("四类模型错误都属于临时回复消息", () => {
    for (const msgType of [
      "vision_error",
      "text_error",
      "voice_error",
      "vector_error",
    ]) {
      expect(
        isTransientModelErrorReplyEvent({ role: "assistant", msgType }),
      ).toBe(true);
    }
    expect(
      isTransientModelErrorReplyEvent({
        role: "user",
        msgType: "text_error",
      }),
    ).toBe(false);
    expect(
      isTransientModelErrorReplyEvent({
        role: "assistant",
        msgType: "unknown_error",
      }),
    ).toBe(false);
  });

  it("只有文本模型错误会终止本次回复等待", () => {
    expect(shouldFinishSendingForModelError({ msgType: "text_error" })).toBe(
      true,
    );
    expect(
      shouldFinishSendingForModelError({ msgType: "vision_error" }),
    ).toBe(false);
    expect(
      shouldFinishSendingForModelError({ msgType: "vector_error" }),
    ).toBe(false);
    expect(
      shouldFinishSendingForModelError({ msgType: "voice_error" }),
    ).toBe(false);
  });
});

describe("模型错误消息定位", () => {
  it("同一秒内的错误也显示在对应用户消息下面", () => {
    const messages = [
      {
        id: "local-user",
        role: "user",
        requestId: "request-1",
        createdAt: "2026-07-26T10:00:00.900Z",
      },
    ];
    insertTransientModelError(messages, {
      id: "text-error",
      role: "assistant",
      msgType: "text_error",
      requestId: "request-1",
      anchorMessageId: "server-user",
      createdAt: "2026-07-26 18:00:00",
    });
    messages.sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual([
      "local-user",
      "text-error",
    ]);
  });

  it("四类错误按到达顺序排列在用户消息下面", () => {
    const messages: any[] = [
      {
        id: "user-1",
        role: "user",
        requestId: "request-1",
        createdAt: "2026-07-26T10:00:00.900Z",
      },
    ];
    for (const msgType of [
      "vision_error",
      "text_error",
      "voice_error",
      "vector_error",
    ]) {
      insertTransientModelError(messages, {
        id: msgType,
        role: "assistant",
        msgType,
        requestId: "request-1",
        anchorMessageId: "user-1",
        createdAt: "2026-07-26 18:00:00",
      });
    }
    messages.sort(compareChatMessages);
    expect(messages.map((message) => message.id)).toEqual([
      "user-1",
      "vision_error",
      "text_error",
      "voice_error",
      "vector_error",
    ]);
  });
});
