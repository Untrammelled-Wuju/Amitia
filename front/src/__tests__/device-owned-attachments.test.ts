import { beforeEach, describe, expect, it, vi } from "vitest";
import { uploadOwnedAttachment, ownedAttachmentURL, ownedImageURL, ownedAudioURL, ownedMessageText } from "../runtime/device-owned-attachments";
import { Sha256Stream } from "../runtime/sha256-stream";

vi.mock("../runtime/runtime-adapter", () => ({ resolveApiUrl: async (path: string) => `http://core.test${path}` }));
vi.mock("../runtime/request-auth", () => ({ createAuthenticatedFetchInit: async (_path: string, init: RequestInit) => init }));

const digest = `sha256:${"01".repeat(32)}`;

function response(kind: string, name: string, mimeType: string, sizeBytes = 4) {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: async () => ({ artifact: { artifactId: `art-${kind}`, ownerDeviceId: "device", storageDeviceId: "device", kind, filename: name, mimeType, blobDigest: digest, sizeBytes } }),
  } as Response;
}

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

describe("Owned attachments", () => {
  it.each([["file", "text/plain", "notes.txt"], ["video", "video/mp4", "clip.mp4"], ["image", "image/png", "picture.png"], ["audio", "audio/webm", "voice.webm"]] as const)("%s 上传后只保留完整制品引用", async (kind, mimeType, name) => {
    vi.mocked(fetch).mockResolvedValueOnce(response(kind, name, mimeType));
    const attachment = await uploadOwnedAttachment({ file: new Blob([Uint8Array.from([1, 2, 3, 4])], { type: mimeType }), kind, name });
    expect(attachment).toEqual({ artifactId: `art-${kind}`, ownerDeviceId: "device", storageDeviceId: "device", kind, name, mimeType, sha256: "01".repeat(32), sizeBytes: 4 });
    const context = { conversationId: "chat", messageId: "message", characterId: "role", conversationOwnerId: "device" };
    expect(ownedAttachmentURL(attachment, context)).toContain(`/attachments/art-${kind}/content?`);
    if (kind === "image") expect(ownedImageURL([attachment], context)).toContain("art-image");
    if (kind === "audio") expect(ownedAudioURL([attachment], context)).toContain("art-audio");
  });

  it("拒绝伪造上传元数据和没有上下文的裸附件地址", async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: true,
      status: 200,
      statusText: "OK",
      json: async () => ({ artifact: { artifactId: "art-file", ownerDeviceId: "device", storageDeviceId: "device", kind: "file", filename: "notes.txt", mimeType: "text/plain", blobDigest: "sha256:bad", sizeBytes: 4 } }),
    } as Response);
    await expect(uploadOwnedAttachment({ file: new Blob(["data"], { type: "text/plain" }), kind: "file", name: "notes.txt" })).rejects.toThrow("缺少完整元数据");
    expect(ownedAttachmentURL({ kind: "file", artifactId: "art-file" })).toBeUndefined();
  });

  it("保留原始错误响应", async () => {
    vi.mocked(fetch).mockResolvedValueOnce({
      ok: false,
      status: 502,
      statusText: "Bad Gateway",
      json: async () => ({ message: "上传失败", cause: "upstream reset" }),
    } as Response);
    await expect(uploadOwnedAttachment({ file: new Blob(["data"], { type: "text/plain" }), kind: "file", name: "notes.txt" })).rejects.toThrow("上传失败\nupstream reset");
    expect(ownedMessageText({ content: "[语音]", transcriptionSourceContent: "[语音]", transcription: "喝茶" })).toBe("喝茶");
    expect(ownedMessageText({ content: "编辑后的文字", transcriptionSourceContent: "[语音]", transcription: "喝茶" })).toBe("编辑后的文字");
  });

  it("流式计算请求体摘要", async () => {
    const hash = new Sha256Stream();
    hash.update(new TextEncoder().encode("a"));
    hash.update(new TextEncoder().encode("bc"));
    expect(hash.digestHex()).toBe("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
  });
});
