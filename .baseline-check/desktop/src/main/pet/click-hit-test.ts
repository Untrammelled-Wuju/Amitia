export interface BoundingBox {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

export interface NormalizedBoundingBox {
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

const EMPTY_NORMALIZED_BOUNDING_BOX: NormalizedBoundingBox = {
  minX: 0,
  minY: 0,
  maxX: -1,
  maxY: -1,
};

export class ClickHitTester {
  private currentFrame: FrameHitTestData | null = null;
  private boundingBoxPixels: BoundingBox | null = null;
  private alphaThreshold: number;

  constructor(alphaThreshold?: number) {
    const threshold =
      typeof alphaThreshold === "number" && Number.isFinite(alphaThreshold)
        ? alphaThreshold
        : DEFAULT_ALPHA_THRESHOLD;
    this.alphaThreshold = Math.min(255, Math.max(0, Math.round(threshold)));
  }

  setFrame(
    width: number,
    height: number,
    alphaData: Uint8Array,
    threshold?: number,
  ): void {
    if (!Number.isFinite(width) || !Number.isFinite(height)) {
      this.currentFrame = null;
      this.boundingBoxPixels = null;
      return;
    }
    const safeWidth = Math.max(0, Math.floor(width));
    const safeHeight = Math.max(0, Math.floor(height));
    if (safeWidth === 0 || safeHeight === 0) {
      this.currentFrame = null;
      this.boundingBoxPixels = null;
      return;
    }
    const expected = safeWidth * safeHeight;
    if (alphaData.length < expected) {
      this.currentFrame = null;
      this.boundingBoxPixels = null;
      return;
    }
    this.currentFrame = {
      width: safeWidth,
      height: safeHeight,
      alphaData,
    };
    this.boundingBoxPixels = null;
    if (typeof threshold === "number" && Number.isFinite(threshold)) {
      this.setThreshold(threshold);
    }
  }

  setThreshold(threshold: number): void {
    if (!Number.isFinite(threshold)) return;
    const clamped = Math.min(255, Math.max(0, Math.round(threshold)));
    if (clamped !== this.alphaThreshold) {
      this.alphaThreshold = clamped;
      this.boundingBoxPixels = null;
    }
  }

  /** @deprecated 请改用 isHitNormalized 传入 0-1 归一化坐标。 */
  isHit(x: number, y: number): boolean {
    const frame = this.currentFrame;
    if (!frame) return false;
    const ix = Math.floor(x);
    const iy = Math.floor(y);
    if (ix < 0 || iy < 0 || ix >= frame.width || iy >= frame.height) {
      return false;
    }
    const box = this.getBoundingBoxPixels();
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
    return frame.alphaData[idx] >= this.alphaThreshold;
  }

  isHitNormalized(x01: number, y01: number): boolean {
    const frame = this.currentFrame;
    if (!frame) return false;
    if (!Number.isFinite(x01) || !Number.isFinite(y01)) return false;
    if (x01 < 0 || x01 > 1 || y01 < 0 || y01 > 1) return false;
    const box = this.getNormalizedBoundingBox();
    if (
      box.maxX < box.minX ||
      box.maxY < box.minY ||
      x01 < box.minX ||
      x01 > box.maxX ||
      y01 < box.minY ||
      y01 > box.maxY
    ) {
      return false;
    }
    const ix = Math.min(
      frame.width - 1,
      Math.max(0, Math.floor(x01 * frame.width)),
    );
    const iy = Math.min(
      frame.height - 1,
      Math.max(0, Math.floor(y01 * frame.height)),
    );
    const idx = iy * frame.width + ix;
    return frame.alphaData[idx] >= this.alphaThreshold;
  }

  private getBoundingBoxPixels(): BoundingBox {
    if (this.boundingBoxPixels) return this.boundingBoxPixels;
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
        if (alphaData[rowStart + x] >= this.alphaThreshold) {
          if (x < minX) minX = x;
          if (x > maxX) maxX = x;
          if (y < minY) minY = y;
          if (y > maxY) maxY = y;
        }
      }
    }
    if (maxX < 0 || maxY < 0) {
      this.boundingBoxPixels = EMPTY_BOUNDING_BOX;
    } else {
      this.boundingBoxPixels = { minX, minY, maxX, maxY };
    }
    return this.boundingBoxPixels;
  }

  getBoundingBox(): BoundingBox {
    const frame = this.currentFrame;
    if (!frame) return EMPTY_NORMALIZED_BOUNDING_BOX;
    const pixels = this.getBoundingBoxPixels();
    if (pixels.maxX < 0 || pixels.maxY < 0) {
      return EMPTY_NORMALIZED_BOUNDING_BOX;
    }
    return {
      minX: pixels.minX / frame.width,
      minY: pixels.minY / frame.height,
      maxX: (pixels.maxX + 1) / frame.width,
      maxY: (pixels.maxY + 1) / frame.height,
    };
  }

  getNormalizedBoundingBox(): NormalizedBoundingBox {
    const frame = this.currentFrame;
    if (!frame) return EMPTY_NORMALIZED_BOUNDING_BOX;
    const pixels = this.getBoundingBoxPixels();
    if (pixels.maxX < 0 || pixels.maxY < 0) {
      return EMPTY_NORMALIZED_BOUNDING_BOX;
    }
    return {
      minX: pixels.minX / frame.width,
      minY: pixels.minY / frame.height,
      maxX: (pixels.maxX + 1) / frame.width,
      maxY: (pixels.maxY + 1) / frame.height,
    };
  }

  clear(): void {
    this.currentFrame = null;
    this.boundingBoxPixels = null;
  }

  hasFrame(): boolean {
    return this.currentFrame !== null;
  }

  getAlphaThreshold(): number {
    return this.alphaThreshold;
  }
}
