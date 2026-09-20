import { describe, expect, it } from "vitest";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";

function provider(channelIds: string[]): UIProviderDefinition {
  return {
    providerId: "channel.renderer",
    extensionId: "com.amitia/channel-test",
    capability: "conversation.message_renderer",
    mode: "replace",
    priority: 10,
    entries: { web: { type: "declarative" } },
    enabled: true,
    builtin: false,
    placement: "any",
    metadata: { channelIds },
  };
}

describe("message renderer channel selector", () => {
  it("only matches the declared channel", () => {
    const renderer = provider(["wechat_personal"]);
    expect(resolveMessageRenderer([renderer], null, { channelId: "wechat_personal", role: "user", msgType: "text" })).toBe(renderer);
    expect(resolveMessageRenderer([renderer], null, { channelId: "qq", role: "user", type: "text" })).toBeNull();
  });
});
