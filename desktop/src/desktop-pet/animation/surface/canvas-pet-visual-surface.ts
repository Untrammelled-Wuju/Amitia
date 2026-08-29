import type {
  PetVisualSurface,
  DecodedFrame,
  PresentedFrameInfo,
  AlphaHitMaskSnapshot,
} from "../contracts";

const HIT_MASK_SAMPLE_WIDTH = 64;
const HIT_MASK_SAMPLE_HEIGHT = 64;
const HIT_MASK_DEFAULT_THRESHOLD = 128;

interface CanvasConfig {
  width: number;
  height: number;
  scale: number;
  interpolationMode: "nearest" | "smooth";
}

interface PresentInput {
  anchor: { type: string; x: number; y: number };
  frameIndex: number;
  actionKey: string;
}

export class CanvasPetVisualSurface implements PetVisualSurface {
  private readonly canvas: HTMLCanvasElement;
  private readonly ctx: CanvasRenderingContext2D;
  private config: CanvasConfig | null = null;
  private dpr: number = 1;
  private lastFrame: DecodedFrame | null = null;
  private lastPresentInfo: PresentedFrameInfo | null = null;
  private retainFrame: boolean = false;
  private disposed: boolean = false;

  constructor(input: { canvas: HTMLCanvasElement }) {
    this.canvas = input.canvas;
    const ctx = this.canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) {
      throw new Error("Failed to acquire 2D rendering context");
    }
    this.ctx = ctx;
  }

  configureCanvas(input: {
    width: number;
    height: number;
    scale: number;
    interpolationMode?: "nearest" | "smooth";
  }): void {
    this.ensureActive();
    const interpolationMode: "nearest" | "smooth" = input.interpolationMode ?? "smooth";
    this.config = {
      width: input.width,
      height: input.height,
      scale: input.scale,
      interpolationMode,
    };
    this.dpr = this.resolveDevicePixelRatio();
    const physicalWidth = Math.max(1, Math.round(input.width * this.dpr));
    const physicalHeight = Math.max(1, Math.round(input.height * this.dpr));
    this.canvas.width = physicalWidth;
    this.canvas.height = physicalHeight;
    // Keep the backing store in package logical pixels, but let CSS fill the
    // BrowserWindow. The main process owns runtime scale by resizing the window;
    // fixed inline pixel sizes here would cause clipping/transparent padding at
    // non-1.0 scale values.
    this.canvas.style.width = "100%";
    this.canvas.style.height = "100%";
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.ctx.imageSmoothingEnabled = interpolationMode !== "nearest";
  }

  present(
    frame: DecodedFrame,
    input: PresentInput,
  ): PresentedFrameInfo {
    this.ensureActive();
    if (!this.config) {
      return {
        presented: false,
        frameIndex: input.frameIndex,
        timestamp: this.now(),
        error: "surface_not_configured",
      };
    }
    const scale = this.config.scale;
    const drawWidth = frame.width * scale;
    const drawHeight = frame.height * scale;
    const canvasWidth = this.config.width;
    const canvasHeight = this.config.height;
    if (!this.retainFrame) {
      this.ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    }
    const anchorType = input.anchor.type;
    let destX: number;
    let destY: number;
    if (anchorType === "bottom_center") {
      destX = (canvasWidth - drawWidth) / 2 + input.anchor.x * scale;
      destY = canvasHeight - drawHeight + input.anchor.y * scale;
    } else if (anchorType === "center") {
      destX = (canvasWidth - drawWidth) / 2 + input.anchor.x * scale;
      destY = (canvasHeight - drawHeight) / 2 + input.anchor.y * scale;
    } else {
      destX = input.anchor.x * scale;
      destY = input.anchor.y * scale;
    }
    try {
      this.ctx.drawImage(frame.bitmap, destX, destY, drawWidth, drawHeight);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      return {
        presented: false,
        frameIndex: input.frameIndex,
        timestamp: this.now(),
        error: message,
      };
    }
    this.lastFrame = frame;
    const info: PresentedFrameInfo = {
      presented: true,
      frameIndex: input.frameIndex,
      timestamp: this.now(),
    };
    this.lastPresentInfo = info;
    this.retainFrame = false;
    return info;
  }

  retainLastFrame(): void {
    this.ensureActive();
    this.retainFrame = true;
  }

  clear(reason: string): void {
    this.ensureActive();
    if (this.config) {
      this.ctx.clearRect(0, 0, this.config.width, this.config.height);
    }
    this.lastFrame = null;
    this.lastPresentInfo = null;
    this.retainFrame = false;
    void reason;
  }

  captureHitMask(): AlphaHitMaskSnapshot {
    this.ensureActive();
    if (!this.config) {
      return {
        width: 0,
        height: 0,
        data: new Uint8Array(0),
        threshold: HIT_MASK_DEFAULT_THRESHOLD,
      };
    }
    const sampleWidth = HIT_MASK_SAMPLE_WIDTH;
    const sampleHeight = HIT_MASK_SAMPLE_HEIGHT;
    const sourceWidth = this.canvas.width;
    const sourceHeight = this.canvas.height;
    const data = new Uint8Array(sampleWidth * sampleHeight);
    if (sourceWidth > 0 && sourceHeight > 0) {
      try {
        const imageData = this.ctx.getImageData(0, 0, sourceWidth, sourceHeight);
        const pixels = imageData.data;
        for (let y = 0; y < sampleHeight; y++) {
          const sourceY = Math.floor((y / sampleHeight) * sourceHeight);
          for (let x = 0; x < sampleWidth; x++) {
            const sourceX = Math.floor((x / sampleWidth) * sourceWidth);
            const pixelIndex = (sourceY * sourceWidth + sourceX) * 4;
            data[y * sampleWidth + x] = pixels[pixelIndex + 3];
          }
        }
      } catch {
        data.fill(0);
      }
    }
    return {
      width: sampleWidth,
      height: sampleHeight,
      data,
      threshold: HIT_MASK_DEFAULT_THRESHOLD,
    };
  }

  dispose(): void {
    if (this.disposed) {
      return;
    }
    this.disposed = true;
    this.lastFrame = null;
    this.lastPresentInfo = null;
    this.config = null;
    this.retainFrame = false;
  }

  private ensureActive(): void {
    if (this.disposed) {
      throw new Error("PetVisualSurface has been disposed");
    }
  }

  private resolveDevicePixelRatio(): number {
    if (typeof window !== "undefined" && typeof window.devicePixelRatio === "number") {
      const ratio = window.devicePixelRatio;
      if (ratio > 0 && Number.isFinite(ratio)) {
        return ratio;
      }
    }
    return 1;
  }

  private now(): number {
    if (typeof performance !== "undefined" && typeof performance.now === "function") {
      return performance.now();
    }
    return Date.now();
  }
}
