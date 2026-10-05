import { describe, expect, it } from "vitest";
import { createOwnedConversationExport } from "../runtime/device-owned-export";

describe("owned conversation export", () => {
  it("exports selected content and origin without authority tokens or runtime state", () => {
    const artifact = createOwnedConversationExport("meshconv1:device:chat", [
      { id: "same", ownerId: "device", role: "user", content: "private", createdAt: "now", executionScope: { secret: "hidden" }, attachments: [], arbitraryToken: "hidden" },
      { id: "same", ownerId: "core", role: "assistant", content: "reply", createdAt: "later", audioUrl: "data:audio/wav;base64,AAAA" },
      { id: "notice", ownerId: "core", role: "system", content: "delivery metadata" },
    ], "json");
    const data = JSON.parse(artifact.content);
    expect(data.messages).toHaveLength(2);
    expect(data.messages.map((row: any) => row.ownerId)).toEqual(["device", "core"]);
    expect(artifact.content).not.toContain("hidden");
    expect(artifact.content).not.toContain("delivery metadata");
    expect(artifact.mimeType).toContain("application/json");
  });

  it("retains text and inline media in Markdown", () => {
    const artifact = createOwnedConversationExport("chat", [{ id: "m", ownerId: "device", role: "user", content: "hello", imageUrl: "data:image/png;base64,AAAA" }], "markdown");
    expect(artifact.content).toContain("hello");
    expect(artifact.content).toContain("![图片](data:image/png;base64,AAAA)");
    expect(artifact.filename).toMatch(/\.md$/);
  });

  it("rejects missing origins, unsupported formats and excessive output", () => {
    expect(() => createOwnedConversationExport("chat", [{ id: "m", role: "user" }], "json")).toThrow("数据来源");
    expect(() => createOwnedConversationExport("chat", [], "html")).toThrow("格式");
    expect(() => createOwnedConversationExport("chat", Array.from({ length: 4097 }, (_, id) => ({ id: String(id), ownerId: "device", role: "user" })), "json")).toThrow("上限");
  });
});
