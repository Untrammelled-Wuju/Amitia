import { describe, expect, it, vi } from "vitest";
import { ownedImageAttachment, ownedImageURL, ownedAudioAttachment, ownedAudioURL, ownedMessageText } from "../runtime/device-owned-attachments";

describe("owned image attachments", () => {
  it("hashes the selected bytes and restores the owner message image", async () => {
    const data = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLttAAAAABJRU5ErkJggg==";
    const hash = "d8aeb7b31114535bbf9035568464d5eb3dadfabaa70525a487d2c04571203426";
    const digest = vi.fn(async (algorithm: string, bytes: Uint8Array) => {
      expect(algorithm).toBe("SHA-256");
      expect(Array.from(bytes)).toEqual(Array.from(atob(data), (value) => value.charCodeAt(0)));
      return Uint8Array.from(hash.match(/../g)!, (value) => parseInt(value, 16)).buffer;
    });
    vi.stubGlobal("crypto", { subtle: { digest } });
    const uri = `data:image/png;base64,${data}`;
    const result = await ownedImageAttachment(uri, "picture.png");
    expect(result).toEqual({ kind: "image", name: "picture.png", mimeType: "image/png", data, sha256: hash });
    expect(digest).toHaveBeenCalledOnce();
    expect(ownedImageURL([result])).toBe(uri);
    vi.unstubAllGlobals();
  });
  it("rejects remote URLs, unsafe image types and oversized payloads", async () => {
    for (const uri of ["https://internal.example/private", "amitia://artifacts/image", "data:image/svg+xml;base64,AAAA", `data:image/png;base64,${"A".repeat(1398108)}`]) {
      await expect(ownedImageAttachment(uri)).rejects.toThrow();
    }
    expect(ownedImageURL([{ kind: "image", mimeType: "image/svg+xml", data: "AAAA" }])).toBeUndefined();
  });
});

describe("owned audio attachments", () => {
	it("hashes audio bytes, restores audio and uses only the matching transcription", async () => {
		vi.stubGlobal("crypto", { subtle: { digest: vi.fn(async () => new Uint8Array(32).fill(1).buffer) } });
		const blob = { type: "audio/webm;codecs=opus", size: 4, arrayBuffer: async () => Uint8Array.from([1, 2, 3, 4]).buffer } as Blob;
		try {
			const attachment = await ownedAudioAttachment(blob);
			expect(attachment.kind).toBe("audio");
			expect(attachment.mimeType).toBe("audio/webm");
			expect(attachment.sha256).toBe("01".repeat(32));
			expect(ownedAudioURL([attachment])).toBe("data:audio/webm;base64,AQIDBA==");
			expect(ownedMessageText({ content: "[语音]", transcriptionSourceContent: "[语音]", transcription: "喝茶" })).toBe("喝茶");
			expect(ownedMessageText({ content: "编辑后的文字", transcriptionSourceContent: "[语音]", transcription: "喝茶" })).toBe("编辑后的文字");
		} finally { vi.unstubAllGlobals(); }
	});
	it("rejects unsupported and oversized audio without reading its bytes", async () => {
		const read = vi.fn();
		for (const blob of [{ type: "audio/mp3", size: 10 }, { type: "audio/wav", size: 1048577 }]) await expect(ownedAudioAttachment({ ...blob, arrayBuffer: read } as unknown as Blob)).rejects.toThrow();
		expect(read).not.toHaveBeenCalled();
		expect(ownedAudioURL([{ kind: "audio", mimeType: "text/html", data: "AAAA" }])).toBeUndefined();
	});
});
