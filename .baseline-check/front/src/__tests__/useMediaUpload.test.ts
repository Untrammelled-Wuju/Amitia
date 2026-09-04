import { afterEach, describe, expect, it, vi } from "vitest";
import {
  convertImageToPng,
  isVisionImageTypeSupported,
} from "../composables/useMediaUpload";

describe("useMediaUpload", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("识别视觉模型可直接接收的图片类型", () => {
    expect(isVisionImageTypeSupported("image/png")).toBe(true);
    expect(isVisionImageTypeSupported("IMAGE/JPEG")).toBe(true);
    expect(isVisionImageTypeSupported("image/gif")).toBe(true);
    expect(isVisionImageTypeSupported("image/webp")).toBe(true);
    expect(isVisionImageTypeSupported("image/bmp")).toBe(true);
    expect(isVisionImageTypeSupported("image/x-icon")).toBe(false);
    expect(isVisionImageTypeSupported("image/svg+xml")).toBe(false);
  });

  it("将非支持图片转换为 PNG 文件", async () => {
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", {
      configurable: true,
      value: vi.fn(() => "blob:test"),
    });
    Object.defineProperty(URL, "revokeObjectURL", {
      configurable: true,
      value: revokeObjectURL,
    });

    class TestImage {
      naturalWidth = 32;
      naturalHeight = 24;
      width = 32;
      height = 24;
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;

      set src(_value: string) {
        queueMicrotask(() => this.onload?.());
      }
    }

    vi.stubGlobal("Image", TestImage);
    const originalCreateElement = document.createElement.bind(document);
    vi.spyOn(document, "createElement").mockImplementation((tagName: string) => {
      if (tagName !== "canvas") return originalCreateElement(tagName);
      return {
        width: 0,
        height: 0,
        getContext: () => ({ drawImage: vi.fn() }),
        toBlob: (callback: BlobCallback) =>
          callback(new Blob(["png"], { type: "image/png" })),
      } as unknown as HTMLCanvasElement;
    });

    const source = new File(["ico"], "avatar.ico", {
      type: "image/x-icon",
      lastModified: 123,
    });
    const converted = await convertImageToPng(source);

    expect(converted.name).toBe("avatar.png");
    expect(converted.type).toBe("image/png");
    expect(converted.lastModified).toBe(123);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:test");
  });
});
