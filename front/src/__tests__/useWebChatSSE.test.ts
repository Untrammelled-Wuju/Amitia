import { describe, expect, it } from "vitest";
import { isWebChatReplyEvent } from "../composables/useWebChatSSE";

describe("isWebChatReplyEvent", () => {
  it("只接受回复消息", () => {
    expect(isWebChatReplyEvent({ role: "assistant" })).toBe(true);
    expect(isWebChatReplyEvent({ role: "user" })).toBe(false);
    expect(isWebChatReplyEvent({ role: "tool" })).toBe(false);
    expect(isWebChatReplyEvent({})).toBe(false);
  });
});
