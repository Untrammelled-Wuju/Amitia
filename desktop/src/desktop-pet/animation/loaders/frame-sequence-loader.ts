import { DecodedFrame, DecoderRegistry, NormalizedFrame } from "../contracts";
import { PLAYBACK_ERROR_CODES, PlaybackError } from "../errors";

export class FrameSequenceLoader {
  private decoderRegistry: DecoderRegistry;
  private maxConcurrency: number;
  private currentController: AbortController | null = null;

  constructor(input: {
    decoderRegistry: DecoderRegistry;
    maxConcurrency?: number;
  }) {
    this.decoderRegistry = input.decoderRegistry;
    this.maxConcurrency = input.maxConcurrency ?? 3;
  }

  async loadFrames(input: {
    frames: readonly NormalizedFrame[];
    signal: AbortSignal;
    onProgress?: (loaded: number, total: number) => void;
  }): Promise<DecodedFrame[]> {
    const internalController = new AbortController();
    this.currentController = internalController;

    const onExternalAbort = () => internalController.abort();
    if (input.signal.aborted) {
      internalController.abort();
    } else {
      input.signal.addEventListener("abort", onExternalAbort);
    }

    try {
      const frames = input.frames;
      const total = frames.length;
      const results: (DecodedFrame | null)[] = new Array(total).fill(null);
      let loaded = 0;
      let nextIndex = 0;

      const decodeFrame = async (
        frame: NormalizedFrame,
        index: number,
      ): Promise<void> => {
        let decoded: DecodedFrame;
        try {
          decoded = await this.decoderRegistry.decode({
            url: frame.resourceUrl,
            signal: internalController.signal,
          });
        } catch (error) {
          if (PlaybackError.isAbort(error)) throw error;
          throw new PlaybackError(
            PLAYBACK_ERROR_CODES.FRAME_DECODE_FAILED,
            `frame decode failed: ${frame.resourceUrl}`,
            { frameIndex: frame.index, resourceUrl: frame.resourceUrl },
          );
        }
        results[index] = {
          ...decoded,
          frameIndex: frame.index,
        };
        loaded++;
        input.onProgress?.(loaded, total);
      };

      const worker = async (): Promise<void> => {
        while (true) {
          if (internalController.signal.aborted) {
            throw new DOMException("Aborted", "AbortError");
          }
          const index = nextIndex++;
          if (index >= frames.length) break;
          await decodeFrame(frames[index], index);
        }
      };

      const workers: Promise<void>[] = [];
      const workerCount = Math.min(this.maxConcurrency, total);
      for (let i = 0; i < workerCount; i++) {
        workers.push(worker());
      }

      await Promise.all(workers);

      return results
        .filter((r): r is DecodedFrame => r !== null)
        .sort((a, b) => a.frameIndex - b.frameIndex);
    } finally {
      input.signal.removeEventListener("abort", onExternalAbort);
      this.currentController = null;
    }
  }

  cancelAll(): void {
    this.currentController?.abort();
  }
}
