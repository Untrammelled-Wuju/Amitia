import type { DecodedFrame } from "../contracts";

export interface DrawFrameOptions {
  canvasWidth: number;
  canvasHeight: number;
  anchor: { type: string; x: number; y: number };
  scale: number;
}

export interface DrawInfo {
  x: number;
  y: number;
  w: number;
  h: number;
}

export class FrameRenderer {
  private readonly ctx: CanvasRenderingContext2D;
  private dpr: number;

  constructor(input: { ctx: CanvasRenderingContext2D; dpr?: number }) {
    this.ctx = input.ctx;
    this.dpr = input.dpr ?? this.resolveDevicePixelRatio();
  }

  setDpr(dpr: number): void {
    this.dpr = dpr > 0 && Number.isFinite(dpr) ? dpr : 1;
  }

  getDpr(): number {
    return this.dpr;
  }

  drawFrame(frame: DecodedFrame, options: DrawFrameOptions): DrawInfo {
    const scale = options.scale;
    const drawWidth = frame.width * scale;
    const drawHeight = frame.height * scale;
    const anchorType = options.anchor.type;
    let destX: number;
    let destY: number;
    if (anchorType === "bottom_center") {
      destX = (options.canvasWidth - drawWidth) / 2 + options.anchor.x * scale;
      destY = options.canvasHeight - drawHeight + options.anchor.y * scale;
    } else if (anchorType === "center") {
      destX = (options.canvasWidth - drawWidth) / 2 + options.anchor.x * scale;
      destY = (options.canvasHeight - drawHeight) / 2 + options.anchor.y * scale;
    } else {
      destX = options.anchor.x * scale;
      destY = options.anchor.y * scale;
    }
    this.ctx.save();
    this.ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.ctx.drawImage(frame.bitmap, destX, destY, drawWidth, drawHeight);
    this.ctx.restore();
    return { x: destX, y: destY, w: drawWidth, h: drawHeight };
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
}
