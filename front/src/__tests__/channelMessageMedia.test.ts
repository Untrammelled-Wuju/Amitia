import { describe, expect, it } from "vitest";
import { normalizeChannelMessage, resolveConversationChannelId } from "@/views/channels/channelMessageMedia";

describe("channel message media normalization", () => {
  it("normalizes QQ image messages and hides the placeholder text", () => {
    const message = normalizeChannelMessage({
      channelId: "qq",
      role: "user",
      content: "[图片]",
      imageUrl: "data:image/png;base64,AA==",
    });
    expect(message.media).toHaveLength(1);
    expect(message.media[0]?.kind).toBe("image");
    expect(message.text).toBe("");
  });

  it("normalizes voice and file attachments", () => {
    const message = normalizeChannelMessage({
      role: "user",
      content: "语音转写",
      attachments: [
        { type: "voice", url: "https://example.test/voice.ogg", mimeType: "audio/ogg" },
        { kind: "file", url: "https://example.test/report.pdf", name: "report.pdf", mimeType: "application/pdf" },
      ],
    });
    expect(message.media.map((item) => item.kind)).toEqual(["audio", "file"]);
    expect(message.media[1]?.name).toBe("report.pdf");
    expect(message.text).toBe("语音转写");
  });

  it("rejects unsafe media protocols", () => {
    const message = normalizeChannelMessage({ imageUrl: "javascript:alert(1)", videoUrl: "ftp://example.test/a.mp4" });
    expect(message.media).toEqual([]);
  });

  it("infers legacy channel ids for installed wechat conversations", () => {
    const available = new Set(["wechat", "wechat_ipad860", "qq"]);
    expect(resolveConversationChannelId({ id: "conv-user@im.wechat" }, available)).toBe("wechat");
    expect(resolveConversationChannelId({ id: "wechat-ipad860-account-peer" }, available)).toBe("wechat_ipad860");
    expect(resolveConversationChannelId({ id: "conv-group-10001" }, available)).toBe("qq");
  });
});
