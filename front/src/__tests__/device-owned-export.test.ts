import { describe, expect, it } from "vitest";
import { createOwnedConversationExport } from "../runtime/device-owned-export";

it("导出当前Owner的实际文件及视频字节，拒绝只有Source地址的附件", () => {
  const rows = [{ id: "m", ownerId: "source-a", role: "user", attachments: [{ kind: "file", name: "notes.txt", mimeType: "text/plain", data: "aGk=", sha256: "hash" }, { kind: "video", name: "clip.mp4", mimeType: "video/mp4", data: "AAAA", sha256: "hash" }] }];
  const result = JSON.parse(createOwnedConversationExport("chat", rows, "json").content);
  expect(result.messages[0].attachments).toHaveLength(2);
  expect(result.messages[0].attachments[0].url).toBe("data:text/plain;base64,aGk=");
  expect(createOwnedConversationExport("chat", rows, "markdown").content).toContain("[clip.mp4](data:video/mp4;base64,AAAA)");
  expect(() => createOwnedConversationExport("chat", [{ ...rows[0], attachments: [{ kind: "file", url: "http://source/file" }] }], "json")).toThrow("原所有者内容");
});

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
