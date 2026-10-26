import { describe, expect, it } from "vitest";
import { conversationChannel } from "@/stores/chat";

describe("conversation channel classification", () => {
  it("keeps plain web conversations in the recent list", () => {
    expect(conversationChannel({ id: "sent-conversation" })).toBe("web");
    expect(conversationChannel({ id: "9b31a637-uuid" })).toBe("web");
    expect(conversationChannel({ id: "x", channel: "web" })).toBe("web");
  });

  it("classifies qq personal conversations as channel conversations", () => {
    expect(conversationChannel({ id: "conv-qq-personal-private-3151508592" })).toBe("qq_personal");
    expect(conversationChannel({ id: "conv-qq-personal-group-193879020" })).toBe("qq_personal");
    expect(conversationChannel({ id: "conv-qq-personal-anything", channel: "web" })).toBe("qq_personal");
  });

  it("keeps other channel conversations out of the recent list", () => {
    expect(conversationChannel({ id: "conv-group-100" })).toBe("qq");
    expect(conversationChannel({ id: "mobileconv:owner:conv-user@im.wechat" })).toBe("wechat");
    expect(conversationChannel({ id: "wechat-ipad860-account-peer" })).toBe("wechat");
    expect(conversationChannel({ id: "x", channel: "wechat" })).toBe("wechat");
  });
});
