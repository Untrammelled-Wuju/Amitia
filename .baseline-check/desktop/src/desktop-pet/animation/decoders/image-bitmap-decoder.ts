import { DecodedFrame } from "../contracts";
import { FrameDecoder } from "./decoder-registry";

export class ImageBitmapDecoder implements FrameDecoder {
  canHandle(mime: string): boolean {
    return mime === "image/png" || mime === "image/webp";
  }

  async decode(input: {
    url: string;
    signal: AbortSignal;
    frameIndex?: number;
    contentHash?: string;
  }): Promise<DecodedFrame> {
    if (typeof createImageBitmap !== "function") {
      throw new Error("createImageBitmap is not available");
    }

    const response = await fetch(input.url, { signal: input.signal });
    if (!response.ok) {
      throw new Error(
        `fetch failed: ${response.status} ${response.statusText}`,
      );
    }

    const blob = await response.blob();
    const bitmap = await createImageBitmap(blob);
    const width = bitmap.width;
    const height = bitmap.height;

    return {
      frameIndex: input.frameIndex ?? -1,
      bitmap,
      width,
      height,
      estimatedBytes: width * height * 4,
      sourceUrl: input.url,
      decoderName: "image-bitmap",
      contentHash: input.contentHash ?? "",
    };
  }
}
