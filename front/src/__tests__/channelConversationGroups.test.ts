import { describe, expect, it } from "vitest";
import { buildChannelConversationGroups } from "@/views/channels/channelConversationGroups";

const presentations = [
  { providerId: "wechat", extensionId: "wechat", channelId: "wechat", displayName: "微信", icon: "channel", order: 30, priority: 200, capabilities: {} },
  { providerId: "ipad", extensionId: "ipad", channelId: "wechat_ipad860", displayName: "个人微信 Ipad860", icon: "channel", order: 42, priority: 220, capabilities: {} },
  { providerId: "qq", extensionId: "qq", channelId: "qq", displayName: "QQ 渠道", icon: "channel", order: 40, priority: 210, capabilities: {} },
];

describe("channel conversation groups", () => {
  it("keeps installed channels visible when they have no conversations", () => {
    const groups = buildChannelConversationGroups(presentations, []);
    expect(groups.map((group) => group.label)).toEqual(["微信", "QQ 渠道", "个人微信 Ipad860"]);
    expect(groups.every((group) => group.items.length === 0)).toBe(true);
  });

  it("places legacy wechat conversations under their inferred channels", () => {
    const groups = buildChannelConversationGroups(presentations, [
      { id: "meshconv1:owner:conv-user@im.wechat", resourceId: "conv-user@im.wechat", title: "微信会话" },
      { id: "meshconv1:owner:wechat-ipad860-account-peer", resourceId: "wechat-ipad860-account-peer", title: "个人微信会话" },
    ]);
    expect(groups.find((group) => group.channel === "wechat")?.items).toHaveLength(1);
    expect(groups.find((group) => group.channel === "wechat_ipad860")?.items).toHaveLength(1);
    expect(groups.find((group) => group.channel === "qq")?.items).toHaveLength(0);
  });
});
