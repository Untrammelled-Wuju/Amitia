import { DecodedFrame } from "../contracts";
import { FrameDecoder } from "./decoder-registry";

export class HtmlImageDecoder implements FrameDecoder {
  canHandle(mime: string): boolean {
    return (
      mime === "image/png" || mime === "image/webp" || mime === "image/gif"
    );
  }

  async decode(input: {
    url: string;
    signal: AbortSignal;
    frameIndex?: number;
    contentHash?: string;
  }): Promise<DecodedFrame> {
    if (input.signal.aborted) {
      throw new DOMException("Aborted", "AbortError");
    }

    const img = new Image();

    const loadPromise = new Promise<void>((resolve, reject) => {
      img.onload = () => resolve();
      img.onerror = () =>
        reject(new Error(`image load failed: ${input.url}`));

      input.signal.addEventListener(
        "abort",
        () => {
          img.src = "";
          reject(new DOMException("Aborted", "AbortError"));
        },
        { once: true },
      );
    });

    img.src = input.url;

    await loadPromise;

    if (input.signal.aborted) {
      throw new DOMException("Aborted", "AbortError");
    }

    const width = img.naturalWidth;
    const height = img.naturalHeight;

    return {
      frameIndex: input.frameIndex ?? -1,
      bitmap: img,
      width,
      height,
      estimatedBytes: width * height * 4,
      sourceUrl: input.url,
      decoderName: "html-image",
      contentHash: input.contentHash ?? "",
    };
  }
}
