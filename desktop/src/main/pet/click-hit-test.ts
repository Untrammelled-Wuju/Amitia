export interface BoundingBox {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

export interface FrameHitTestData {
  width: number;
  height: number;
  alphaData: Uint8Array;
  boundingBox?: BoundingBox;
}

const DEFAULT_ALPHA_THRESHOLD = 10;

const EMPTY_BOUNDING_BOX: BoundingBox = {
  minX: 0,
  minY: 0,
  maxX: -1,
  maxY: -1,
};

export class ClickHitTester {
  private currentFrame: FrameHitTestData | null = null;
  private boundingBox: BoundingBox | null = null;
  private readonly alphaThreshold: number;

  constructor(alphaThreshold?: number) {
    const threshold =
      typeof alphaThreshold === "number" && Number.isFinite(alphaThreshold)
        ? alphaThreshold
        : DEFAULT_ALPHA_THRESHOLD;
    this.alphaThreshold = Math.min(255, Math.max(0, Math.round(threshold)));
  }

  setFrame(width: number, height: number, alphaData: Uint8Array): void {
    if (!Number.isFinite(width) || !Number.isFinite(height)) {
      this.currentFrame = null;
      this.boundingBox = null;
      return;
    }
    const safeWidth = Math.max(0, Math.floor(width));
    const safeHeight = Math.max(0, Math.floor(height));
    if (safeWidth === 0 || safeHeight === 0) {
      this.currentFrame = null;
      this.boundingBox = null;
      return;
    }
    const expected = safeWidth * safeHeight;
    if (alphaData.length < expected) {
      this.currentFrame = null;
      this.boundingBox = null;
      return;
    }
    this.currentFrame = {
      width: safeWidth,
      height: safeHeight,
      alphaData,
    };
    this.boundingBox = null;
  }

  isHit(x: number, y: number): boolean {
    const frame = this.currentFrame;
    if (!frame) return false;
    const ix = Math.floor(x);
    const iy = Math.floor(y);
    if (ix < 0 || iy < 0 || ix >= frame.width || iy >= frame.height) {
      return false;
    }
    const box = this.getBoundingBox();
    if (
      box.maxX < box.minX ||
      box.maxY < box.minY ||
      ix < box.minX ||
      ix > box.maxX ||
      iy < box.minY ||
      iy > box.maxY
    ) {
      return false;
    }
    const idx = iy * frame.width + ix;
    return frame.alphaData[idx] > this.alphaThreshold;
  }

  getBoundingBox(): BoundingBox {
    if (this.boundingBox) return this.boundingBox;
    const frame = this.currentFrame;
    if (!frame) return EMPTY_BOUNDING_BOX;
    const { width, height, alphaData } = frame;
    let minX = width;
    let minY = height;
    let maxX = -1;
    let maxY = -1;
    for (let y = 0; y < height; y++) {
      const rowStart = y * width;
      for (let x = 0; x < width; x++) {
        if (alphaData[rowStart + x] > this.alphaThreshold) {
          if (x < minX) minX = x;
          if (x > maxX) maxX = x;
          if (y < minY) minY = y;
          if (y > maxY) maxY = y;
        }
      }
    }
    if (maxX < 0 || maxY < 0) {
      this.boundingBox = EMPTY_BOUNDING_BOX;
    } else {
      this.boundingBox = { minX, minY, maxX, maxY };
    }
    return this.boundingBox;
  }

  clear(): void {
    this.currentFrame = null;
    this.boundingBox = null;
  }

  hasFrame(): boolean {
    return this.currentFrame !== null;
  }

  getAlphaThreshold(): number {
    return this.alphaThreshold;
  }
}
