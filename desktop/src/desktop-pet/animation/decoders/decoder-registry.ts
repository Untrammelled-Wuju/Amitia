import { DecodedFrame, DecoderRegistry } from "../contracts";
import { PLAYBACK_ERROR_CODES, PlaybackError } from "../errors";

export interface FrameDecoder {
  canHandle(mime: string): boolean;
  decode(input: {
    url: string;
    signal: AbortSignal;
    frameIndex?: number;
    contentHash?: string;
  }): Promise<DecodedFrame>;
}

function inferMimeFromUrl(url: string): string {
  const lower = url.toLowerCase().split("?")[0].split("#")[0];
  if (lower.endsWith(".png")) return "image/png";
  if (lower.endsWith(".webp")) return "image/webp";
  if (lower.endsWith(".gif")) return "image/gif";
  return "";
}

export class DecoderRegistryImpl implements DecoderRegistry {
  private decoders: FrameDecoder[];

  constructor(decoders: FrameDecoder[]) {
    this.decoders = decoders.slice();
  }

  registerDecoder(decoder: FrameDecoder): void {
    this.decoders.push(decoder);
  }

  canHandle(mime: string): boolean {
    return this.decoders.some((d) => d.canHandle(mime));
  }

  async decode(input: {
    url: string;
    signal: AbortSignal;
    contentHash?: string;
  }): Promise<DecodedFrame> {
    const mime = inferMimeFromUrl(input.url);
    const decoder = this.decoders.find((d) => d.canHandle(mime));
    if (!decoder) {
      throw new PlaybackError(
        PLAYBACK_ERROR_CODES.FRAME_DECODE_FAILED,
        `no decoder available for mime: ${mime || "unknown"} (url: ${input.url})`,
        { resourceUrl: input.url },
      );
    }
    return decoder.decode(input);
  }
}
